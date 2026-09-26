package house

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Service owns house workflow transactions. Admins is an explicit operator-configured
// allowlist of internal user UUIDs; a membership role never grants platform access.
type Service struct {
	DB     *pgxpool.Pool
	Admins map[string]bool
}
type actor struct {
	User, House string
	Admin       bool
}
type record map[string]any

func (r record) str(k string) string { v, _ := r[k].(string); return v }
func invalid() error                 { return status.Error(codes.InvalidArgument, "invalid request") }
func denied() error                  { return status.Error(codes.PermissionDenied, "access denied") }
func conflict() error                { return status.Error(codes.FailedPrecondition, "invalid state transition") }
func notFound() error                { return status.Error(codes.NotFound, "resource not found") }
func mapped(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound()
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			return status.Error(codes.AlreadyExists, "resource already exists")
		case "23503":
			return notFound()
		case "40001", "40P01":
			return status.Error(codes.Aborted, "retry transaction")
		}
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "cancelled")
	}
	return status.Error(codes.Internal, "house workflow unavailable")
}
func one(ctx context.Context, tx pgx.Tx, q string, args ...any) (record, error) {
	var raw []byte
	err := tx.QueryRow(ctx, q, args...).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var r record
	err = json.Unmarshal(raw, &r)
	return r, err
}
func rows(ctx context.Context, tx pgx.Tx, q string, args ...any) ([]record, error) {
	rs, err := tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []record{}
	for rs.Next() {
		var raw []byte
		if err = rs.Scan(&raw); err != nil {
			return nil, err
		}
		var r record
		if err = json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}
func decorate(r record, kind string) record {
	if r == nil {
		return nil
	}
	if v := r.str("status"); v != "" {
		r["status"] = kind + "_" + v
	}
	if v := r.str("source"); v != "" {
		r["source"] = "JOIN_REQUEST_SOURCE_" + v
	}
	return r
}
func list(items []record, kind string) record {
	if items == nil {
		items = []record{}
	}
	for _, r := range items {
		if kind != "" {
			decorate(r, kind)
		}
	}
	return record{"items": items}
}
func (s *Service) Execute(ctx context.Context, op string, c Command) (result []byte, err error) {
	defer func() { workflowOperations.WithLabelValues(op, status.Code(err).String()).Inc() }()
	if len(c.Reason) > 4000 || (c.Reason != "" && !textOK(c.Reason, 1000)) {
		return nil, invalid()
	}
	for _, id := range []string{c.ID, c.HouseID, c.TargetUserID, c.UserID, c.AfterUserID} {
		if id != "" && !validID(id) {
			return nil, invalid()
		}
	}
	md, _ := metadata.FromIncomingContext(ctx)
	a := actor{}
	if values := md.Get("x-actor-user-id"); len(values) == 1 {
		a.User = values[0]
	}
	if values := md.Get("x-house-id"); len(values) == 1 {
		a.House = values[0]
	}
	if op != "ListNotificationRecipients" && !validID(a.User) {
		return nil, status.Error(codes.Unauthenticated, "authenticated actor required")
	}
	if a.House != "" && !validID(a.House) {
		return nil, invalid()
	}
	a.Admin = s.Admins[a.User]
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, mapped(err)
	}
	defer tx.Rollback(ctx)
	// House lifecycle writes are low-volume MVP operations. One transaction lock
	// gives consistent lock ordering across invitations, reviews and role changes.
	// SQL unique indexes remain the final protection, including operator commands.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1937135231)"); err != nil {
		return nil, mapped(err)
	}
	if op != "ListNotificationRecipients" {
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)", a.User).Scan(&exists); err != nil {
			return nil, mapped(err)
		}
		if !exists {
			return nil, status.Error(codes.Unauthenticated, "unknown actor")
		}
	}
	var data record
	switch op {
	case "ListPlatformUsers", "GrantChairmanPermission", "RevokeChairmanPermission", "ListAdminHouses", "AssignHouseChairman", "RemoveHouseChairman":
		data, err = s.administration(ctx, tx, a, op, c)
	case "CreateHouseRegistration", "GetHouseRegistration", "ListMyHouseRegistrations", "CancelHouseRegistration", "ListPendingHouseRegistrations", "ApproveHouseRegistration", "RejectHouseRegistration", "SearchHouses":
		data, err = s.registration(ctx, tx, a, op, c)
	case "CancelJoinRequest", "CreateJoinRequest", "ListMyJoinRequests", "ListHouseJoinRequests", "ApproveJoinRequest", "RejectJoinRequest":
		data, err = s.join(ctx, tx, a, op, c)
	case "CreateHouseInvitation", "ListHouseInvitations", "GetHouseInvitation", "RevokeHouseInvitation", "PreviewHouseInvitation", "RedeemHouseInvitation":
		data, err = s.invitation(ctx, tx, a, op, c)
	case "CreateChairmanTransfer", "ListChairmanTransfers", "AcceptChairmanTransfer", "RejectChairmanTransfer", "CancelChairmanTransfer":
		data, err = s.transfer(ctx, tx, a, op, c)
	case "ListHouseMembers", "DeactivateMembership", "ReactivateMembership", "RemoveMembership":
		data, err = s.members(ctx, tx, a, op, c)
	case "GetHouseAccessState", "GetNotificationPreferences", "UpdateNotificationPreferences", "ListNotificationRecipients":
		data, err = s.preferences(ctx, tx, a, op, c)
	default:
		err = status.Error(codes.Unimplemented, "unknown operation")
	}
	if err != nil {
		return nil, mapped(err)
	}
	if strings.Contains(op, "Registration") || strings.Contains(op, "JoinRequest") || op == "RedeemHouseInvitation" {
		if err = displayNames(ctx, tx, data); err != nil {
			return nil, mapped(err)
		}
	}
	result, err = json.Marshal(data)
	if err != nil {
		return nil, mapped(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, mapped(err)
	}
	return result, nil
}
func manager(ctx context.Context, tx pgx.Tx, a actor, house string) error {
	if !validID(house) || house != a.House {
		return denied()
	}
	if a.Admin {
		return nil
	}
	var role string
	err := tx.QueryRow(ctx, "SELECT role FROM memberships WHERE user_id=$1 AND house_id=$2 AND status='ACTIVE' FOR UPDATE", a.User, house).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return denied()
	}
	if err != nil {
		return err
	}
	if role != "CHAIRMAN" && role != "ADMIN" {
		return denied()
	}
	return nil
}
func active(ctx context.Context, tx pgx.Tx, user, house string) (bool, error) {
	var yes bool
	err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM memberships WHERE user_id=$1 AND house_id=$2 AND status='ACTIVE')", user, house).Scan(&yes)
	return yes, err
}
func audit(ctx context.Context, tx pgx.Tx, a actor, op, id, house string) error {
	_, err := tx.Exec(ctx, "INSERT INTO house_audit(actor_user_id,operation,resource_id,house_id) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid)", a.User, op, id, house)
	return err
}
func event(ctx context.Context, tx pgx.Tx, kind, house, user, id string) error {
	payload, err := json.Marshal(record{"house_id": house, "recipient_user_id": user, "resource_id": id})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO identity_outbox(event_type,payload) VALUES($1,$2)", kind, payload)
	return err
}
func emit(ctx context.Context, tx pgx.Tx, a actor, kind, house, user, id string) error {
	if err := audit(ctx, tx, a, kind, id, house); err != nil {
		return err
	}
	return event(ctx, tx, kind, house, user, id)
}
func transition(current, target string) error {
	if current != target && current != "PENDING" {
		return conflict()
	}
	return nil
}
func expired(r record) bool {
	t, e := time.Parse(time.RFC3339Nano, r.str("expires_at"))
	return e != nil || !t.After(time.Now())
}
func houseSummary(ctx context.Context, tx pgx.Tx, id string) (record, error) {
	return one(ctx, tx, `SELECT to_jsonb(h) || jsonb_build_object('has_chairman',EXISTS(SELECT 1 FROM memberships m WHERE m.house_id=h.id AND m.role='CHAIRMAN' AND m.status='ACTIVE'),'join_available',true) FROM houses h WHERE h.id=$1`, id)
}
func shortStatus(op string) string {
	if strings.HasPrefix(op, "Approve") {
		return "APPROVED"
	}
	if strings.HasPrefix(op, "Reject") {
		return "REJECTED"
	}
	return "CANCELLED"
}
