package house_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"

	pb "github.com/ZheglY/SmartQuarter/services/identity-service/internal/gen/smartquarter/identity/v1"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/house"
	transport "github.com/ZheglY/SmartQuarter/services/identity-service/internal/transport/grpc"
	"github.com/ZheglY/SmartQuarter/services/identity-service/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestNormalizeAddress(t *testing.T) {
	for _, address := range []string{" УЛ. Ёлочная, 5 ", "ул елочная 5", "ул.; елочная   5"} {
		if got := house.NormalizeAddress("  МОСКВА ", address); got != "москва|ул елочная 5" {
			t.Fatal(got)
		}
	}
}

func TestPostgresHouseWorkflow(t *testing.T) {
	dsn := os.Getenv("HOUSE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HOUSE_TEST_DATABASE_URL to isolated identity_test_house database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if db.Config().ConnConfig.Database != "identity_test_house" {
		t.Fatal("refusing non-test database")
	}
	if e = migrations.Up(ctx, dsn); e != nil {
		t.Fatal(e)
	}
	users := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	for _, id := range users {
		_, e = db.Exec(ctx, `INSERT INTO users(id,max_user_id,display_name) VALUES($1,(SELECT COALESCE(max(max_user_id),0)+1 FROM users),'Workflow test')`, id)
		if e != nil {
			t.Fatal(e)
		}
	}
	admin, chair, resident, third, foreign := users[0], users[1], users[2], users[3], users[4]
	s := &house.Service{DB: db, Admins: map[string]bool{admin: true}}
	actor := func(user, h string) context.Context {
		return metadata.NewIncomingContext(ctx, metadata.Pairs("x-actor-user-id", user, "x-house-id", h, "x-actor-role", "CHAIRMAN"))
	}
	call := func(user, h, op string, c house.Command) map[string]any {
		t.Helper()
		raw, e := s.Execute(actor(user, h), op, c)
		if e != nil {
			t.Fatalf("%s: %v", op, e)
		}
		var r map[string]any
		if e = json.Unmarshal(raw, &r); e != nil {
			t.Fatal(e)
		}
		return r
	}
	deny := func(user, h, op string, c house.Command, want codes.Code) {
		t.Helper()
		_, e := s.Execute(actor(user, h), op, c)
		if status.Code(e) != want {
			t.Fatalf("%s: got %v want %v", op, e, want)
		}
	}
	reg := call(chair, "", "CreateHouseRegistration", house.Command{Name: "Test house", City: "Москва", Address: "Ёлочная, " + uuid.NewString()})
	id := reg["id"].(string)
	deny(chair, "", "ApproveHouseRegistration", house.Command{ID: id}, codes.PermissionDenied)
	deny(foreign, "", "GetHouseRegistration", house.Command{ID: id}, codes.PermissionDenied)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Execute(actor(admin, ""), "ApproveHouseRegistration", house.Command{ID: id})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	approved := call(chair, "", "GetHouseRegistration", house.Command{ID: id})
	h := approved["resulting_house_id"].(string)
	var n int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM memberships WHERE house_id=$1 AND role='CHAIRMAN' AND status='ACTIVE'`, h).Scan(&n); e != nil || n != 1 {
		t.Fatalf("chairmen=%d err=%v", n, e)
	}
	// Verify typed gRPC serialization, including nullable timestamps and enums.
	rpc := &transport.HouseHandler{Workflow: s}
	v, e := rpc.GetHouseRegistration(actor(chair, ""), &pb.GetHouseRegistrationRequest{Id: id})
	if e != nil || v.Status != pb.HouseRegistrationStatus_HOUSE_REGISTRATION_STATUS_APPROVED {
		t.Fatalf("RPC response %+v %v", v, e)
	}
	join := call(resident, "", "CreateJoinRequest", house.Command{HouseID: h})
	jid := join["id"].(string)
	access := call(chair, h, "GetHouseAccessState", house.Command{})
	if access["incoming_join_requests"] != float64(1) || access["can_manage_active_house"] != true {
		t.Fatal("active-house management state", access)
	}
	access = call(chair, "", "GetHouseAccessState", house.Command{})
	if access["incoming_join_requests"] != float64(0) || access["can_manage_active_house"] != false {
		t.Fatal("cross-house management badge", access)
	}
	deny(resident, h, "ApproveJoinRequest", house.Command{ID: jid}, codes.PermissionDenied)
	call(chair, h, "ApproveJoinRequest", house.Command{ID: jid})
	invite := call(chair, h, "CreateHouseInvitation", house.Command{MaxUses: 1, ExpiresInHours: 1})
	token := invite["token"].(string)
	redeemed := call(third, "", "RedeemHouseInvitation", house.Command{Token: token})
	again := call(third, "", "RedeemHouseInvitation", house.Command{Token: token})
	if redeemed["id"] != again["id"] {
		t.Fatal("duplicate redemption")
	}
	deny(foreign, "", "RedeemHouseInvitation", house.Command{Token: token}, codes.FailedPrecondition)
	var rawToken string
	if e = db.QueryRow(ctx, `SELECT token_hash FROM house_invitations WHERE id=$1`, invite["invitation"].(map[string]any)["id"]).Scan(&rawToken); e != nil || rawToken == token || len(rawToken) != 64 {
		t.Fatal("token storage", e)
	}
	call(chair, h, "ApproveJoinRequest", house.Command{ID: redeemed["id"].(string)})
	transfer := call(chair, h, "CreateChairmanTransfer", house.Command{TargetUserID: resident})
	tid := transfer["id"].(string)
	deny(third, h, "AcceptChairmanTransfer", house.Command{ID: tid}, codes.PermissionDenied)
	call(resident, h, "AcceptChairmanTransfer", house.Command{ID: tid})
	call(resident, h, "AcceptChairmanTransfer", house.Command{ID: tid})
	deny(chair, h, "CreateHouseInvitation", house.Command{MaxUses: 1, ExpiresInHours: 1}, codes.PermissionDenied)
	members := call(resident, h, "ListHouseMembers", house.Command{})
	for _, item := range members["items"].([]any) {
		m := item.(map[string]any)
		if m["user_id"] == resident {
			deny(resident, h, "DeactivateMembership", house.Command{ID: m["id"].(string)}, codes.PermissionDenied)
		}
	}
	call(third, h, "UpdateNotificationPreferences", house.Command{})
	recipients := call("", "", "ListNotificationRecipients", house.Command{HouseID: h, Category: "announcement"})
	if len(recipients["items"].([]any)) != 2 {
		t.Fatal("preferences not enforced", recipients)
	}
	// Transactional events and audit must survive only successful commands.
	if e = db.QueryRow(ctx, `SELECT count(*) FROM identity_outbox WHERE payload->>'house_id'=$1`, h).Scan(&n); e != nil || n < 8 {
		t.Fatal("missing events", n, e)
	}
	t.Run("membership lifecycle and explicit admin override", func(t *testing.T) {
		var thirdID, chairID string
		for _, item := range members["items"].([]any) {
			m := item.(map[string]any)
			if m["user_id"] == third {
				thirdID = m["id"].(string)
			}
			if m["user_id"] == resident {
				chairID = m["id"].(string)
			}
		}
		call(resident, h, "DeactivateMembership", house.Command{ID: thirdID})
		call(resident, h, "ReactivateMembership", house.Command{ID: thirdID})
		call(resident, h, "RemoveMembership", house.Command{ID: thirdID})
		deny(admin, h, "DeactivateMembership", house.Command{ID: chairID}, codes.PermissionDenied)
		deny(resident, h, "DeactivateMembership", house.Command{ID: chairID, PlatformAdminOverride: true}, codes.PermissionDenied)
		call(admin, h, "DeactivateMembership", house.Command{ID: chairID, PlatformAdminOverride: true})
		call(admin, h, "ReactivateMembership", house.Command{ID: chairID, PlatformAdminOverride: true})
	})
	t.Run("concurrent invitation quota and expiry", func(t *testing.T) {
		inv := call(resident, h, "CreateHouseInvitation", house.Command{MaxUses: 1, ExpiresInHours: 1})
		token := inv["token"].(string)
		results := make(chan error, 2)
		for _, user := range []string{third, foreign} {
			wg.Add(1)
			go func(user string) {
				defer wg.Done()
				_, e := s.Execute(actor(user, ""), "RedeemHouseInvitation", house.Command{Token: token})
				results <- e
			}(user)
		}
		wg.Wait()
		close(results)
		success, exhausted := 0, 0
		for e := range results {
			if e == nil {
				success++
			} else if status.Code(e) == codes.FailedPrecondition {
				exhausted++
			} else {
				t.Fatal(e)
			}
		}
		if success != 1 || exhausted != 1 {
			t.Fatalf("quota race: %d succeeded, %d exhausted", success, exhausted)
		}
		expired := call(resident, h, "CreateHouseInvitation", house.Command{MaxUses: 2, ExpiresInHours: 1})
		eid := expired["invitation"].(map[string]any)["id"].(string)
		if _, e = db.Exec(ctx, `UPDATE house_invitations SET expires_at=now()-interval '1 second' WHERE id=$1`, eid); e != nil {
			t.Fatal(e)
		}
		deny(foreign, "", "PreviewHouseInvitation", house.Command{Token: expired["token"].(string)}, codes.FailedPrecondition)
		got := call(resident, h, "GetHouseInvitation", house.Command{ID: eid})
		if got["status"] != "INVITATION_STATUS_EXPIRED" {
			t.Fatal(got)
		}
	})
	t.Run("rejection and cancellation cannot grant access", func(t *testing.T) {
		r := call(foreign, "", "CreateHouseRegistration", house.Command{Name: "Rejected", City: "Test", Address: uuid.NewString()})
		id := r["id"].(string)
		call(admin, "", "RejectHouseRegistration", house.Command{ID: id, Reason: "Address not verified"})
		deny(admin, "", "ApproveHouseRegistration", house.Command{ID: id}, codes.FailedPrecondition)
		r = call(third, "", "CreateHouseRegistration", house.Command{Name: "Cancelled", City: "Test", Address: uuid.NewString()})
		id = r["id"].(string)
		call(third, "", "CancelHouseRegistration", house.Command{ID: id})
		deny(admin, "", "ApproveHouseRegistration", house.Command{ID: id}, codes.FailedPrecondition)
		joins := call(foreign, "", "ListMyJoinRequests", house.Command{})
		for _, item := range joins["items"].([]any) {
			j := item.(map[string]any)
			if j["status"] == "JOIN_REQUEST_STATUS_PENDING" {
				call(foreign, "", "CancelJoinRequest", house.Command{ID: j["id"].(string)})
				deny(resident, h, "ApproveJoinRequest", house.Command{ID: j["id"].(string)}, codes.FailedPrecondition)
			}
		}
	})
}
