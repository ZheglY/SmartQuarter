//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/config"
	cpb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/community/v1"
	ipb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/identity/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/maxapi"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/notification"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/rpc"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/state"
	transport "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/transport/http"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestHouseWorkflowHTTP(t *testing.T) {
	if os.Getenv("IDENTITY_TEST_ADDR") == "" {
		t.Skip("real acceptance stack required")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, os.Getenv("IDENTITY_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if db.Config().ConnConfig.Database != "identity_test" {
		t.Fatal("not an isolated test database")
	}
	_, e = db.Exec(ctx, `INSERT INTO users(id,max_user_id,display_name) VALUES('99999999-9999-4999-8999-999999999999',201,'Platform administrator') ON CONFLICT(id) DO NOTHING`)
	if e != nil {
		t.Fatal(e)
	}
	metrics := observability.New()
	conn, e := rpc.Dial(os.Getenv("IDENTITY_TEST_ADDR"), 10*time.Second, zap.NewNop(), metrics)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	community, e := rpc.Dial(os.Getenv("COMMUNITY_TEST_ADDR"), 10*time.Second, zap.NewNop(), metrics)
	if e != nil {
		t.Fatal(e)
	}
	defer community.Close()
	r := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_TEST_ADDR"), MaxRetries: -1, ContextTimeoutEnabled: true})
	defer r.Close()
	var mu sync.Mutex
	deliveries := map[string]map[string]int{}
	maxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid MAX payload")
		}
		user := r.URL.Query().Get("user_id")
		mu.Lock()
		if deliveries[user] == nil {
			deliveries[user] = map[string]int{}
		}
		deliveries[user][body.Text]++
		mu.Unlock()
		fmt.Fprint(w, `{"success":true}`)
	}))
	defer maxServer.Close()
	count := func(user, text string) int { mu.Lock(); defer mu.Unlock(); return deliveries[user][text] }
	bot := &maxapi.Client{BaseURL: maxServer.URL, Token: "gateway-test-token", BotUsername: "test_bot"}
	cfg := config.Config{Environment: "test", CookieName: "sq_house_test", SessionTTL: time.Hour, InitDataTTL: 5 * time.Minute, RequestTimeout: 10 * time.Second, Origins: []string{"https://test.example"}, AuthRate: 1000, BusinessRate: 10000, WebhookRate: 1000, BotToken: "gateway-test-token", WebhookSecret: "test-webhook-secret"}
	api := &transport.API{Config: cfg, Store: state.Store{R: r}, Identity: identity.NewGRPC(conn, 10*time.Second), House: ipb.NewHouseServiceClient(conn), Community: cpb.NewCommunityServiceClient(community), Bot: bot, Metrics: metrics, Logger: zap.NewNop()}
	gateway := httptest.NewServer(api.Handler())
	defer gateway.Close()
	client := &http.Client{Timeout: 15 * time.Second}
	call := func(cookie *http.Cookie, method, path string, body any, key string, want int) map[string]any {
		t.Helper()
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, gateway.URL+"/api/v1"+path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://test.example")
		req.Header.Set("x-actor-role", "ADMIN")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, res.StatusCode, want, raw)
		}
		out := map[string]any{}
		if len(raw) > 0 && json.Unmarshal(raw, &out) != nil {
			t.Fatal("invalid JSON", string(raw))
		}
		return out
	}
	login := func(id int64) *http.Cookie {
		t.Helper()
		raw, _ := json.Marshal(map[string]string{"init_data": initData(id)})
		req, _ := http.NewRequest("POST", gateway.URL+"/api/v1/session/max", bytes.NewReader(raw))
		req.Header.Set("Origin", "https://test.example")
		req.Header.Set("Content-Type", "application/json")
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			raw, _ := io.ReadAll(res.Body)
			t.Fatalf("login %d: %s", id, raw)
		}
		for _, c := range res.Cookies() {
			if c.Name == cfg.CookieName {
				return c
			}
		}
		t.Fatal("no cookie")
		return nil
	}
	// Fresh users keep reruns independent of memberships and preferences from prior runs.
	base := int64(uuid.New().ID())*10 + 1000
	chairID, residentID, thirdID, outsiderID := base+1, base+2, base+3, base+4
	admin, chair, resident, third, outsider := login(201), login(chairID), login(residentID), login(thirdID), login(outsiderID)
	empty := map[string]any{}
	group := "house-test-" + uuid.NewString()
	stream := "stream:notifications"
	if e = r.XGroupCreateMkStream(ctx, stream, group, "$").Err(); e != nil {
		t.Fatal(e)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	worker := &notification.Consumer{Redis: r, Stream: stream, Group: group, Identity: api.Identity, House: api.House, Bot: bot, Metrics: metrics, Logger: zap.NewNop()}
	go func() { defer close(done); worker.Run(workerCtx) }()
	defer func() { cancel(); <-done; r.XGroupDestroy(ctx, stream, group) }()
	t.Log("1-3: application -> platform approval -> single chairman")
	address := "Лесная " + uuid.NewString()
	body := map[string]any{"name": "Workflow house", "city": "Москва", "address": address}
	key := uuid.NewString()
	registration := call(chair, "POST", "/house-registrations", body, key, 202)
	rid := registration["id"].(string)
	if replay := call(chair, "POST", "/house-registrations", body, key, 202); replay["id"] != rid {
		t.Fatal("registration replay created duplicate")
	}
	call(chair, "POST", "/admin/house-registrations/"+rid+"/approve", empty, "", 403)
	call(outsider, "GET", "/house-registrations/"+rid, nil, "", 403)
	approved := call(admin, "POST", "/admin/house-registrations/"+rid+"/approve", empty, "", 200)
	house := approved["resulting_house_id"].(string)
	call(chair, "POST", "/session/active-house", map[string]string{"house_id": house}, "", 200)
	call(outsider, "POST", "/house-registrations", body, "", 409)
	t.Log("4-5: chairman contacts; public residents cannot mutate or read audit fields")
	contact := map[string]any{"category": "EMERGENCY_DISPATCH", "title": "Аварийная служба", "phone": "+79991234567", "emergency": true}
	created := call(chair, "POST", "/chairman/service-contacts", contact, uuid.NewString(), 201)
	cid := created["id"].(string)
	t.Log("6-8: search -> join request -> approval")
	found := call(resident, "GET", "/houses/search?query=Workflow", nil, "", 200)
	if len(found["items"].([]any)) == 0 {
		t.Fatal("house absent from search")
	}
	join := call(resident, "POST", "/houses/"+house+"/join-requests", empty, "", 202)
	jid := join["id"].(string)
	call(resident, "GET", "/house/service-contacts", nil, "", 403)
	call(chair, "POST", "/chairman/join-requests/"+jid+"/approve", empty, "", 200)
	call(resident, "POST", "/session/active-house", map[string]string{"house_id": house}, "", 200)
	public := call(resident, "GET", "/house/service-contacts", nil, "", 200)
	items := public["items"].([]any)
	if len(items) != 1 {
		t.Fatal("contact not visible")
	}
	if _, ok := items[0].(map[string]any)["created_by"]; ok {
		t.Fatal("contact audit leaked")
	}
	call(resident, "PATCH", "/chairman/service-contacts/"+cid, contact, "", 403)
	t.Log("9-11: bounded invitation; no membership before approval")
	inviteKey := uuid.NewString()
	invitation := call(chair, "POST", "/chairman/invitations", map[string]int{"max_uses": 1, "expires_in_hours": 1}, inviteKey, 201)
	token := invitation["token"].(string)
	redeemed := call(third, "POST", "/invitations/redeem", map[string]string{"token": token}, "", 202)
	replay := call(third, "POST", "/invitations/redeem", map[string]string{"token": token}, "", 202)
	if redeemed["id"] != replay["id"] {
		t.Fatal("duplicate redemption")
	}
	call(third, "POST", "/session/active-house", map[string]string{"house_id": house}, "", 403)
	call(outsider, "POST", "/invitations/redeem", map[string]string{"token": token}, "", 409)
	call(chair, "POST", "/chairman/join-requests/"+redeemed["id"].(string)+"/approve", empty, "", 200)
	call(third, "POST", "/session/active-house", map[string]string{"house_id": house}, "", 200)
	t.Log("12-13: transfer acceptance revokes former chairman, even idempotency replay")
	me := call(resident, "GET", "/me", nil, "", 200)
	target := me["user"].(map[string]any)["id"].(string)
	transfer := call(chair, "POST", "/chairman/transfers", map[string]string{"target_user_id": target}, "", 202)
	tid := transfer["id"].(string)
	call(third, "POST", "/chairman/transfers/"+tid+"/accept", empty, "", 403)
	call(resident, "POST", "/chairman/transfers/"+tid+"/accept", empty, "", 200)
	call(chair, "POST", "/chairman/invitations", map[string]int{"max_uses": 1, "expires_in_hours": 1}, inviteKey, 403)
	call(chair, "PATCH", "/chairman/service-contacts/"+cid, contact, "", 403)
	contact["title"] = "Новая аварийная служба"
	call(resident, "PATCH", "/chairman/service-contacts/"+cid, contact, uuid.NewString(), 200)
	call(resident, "DELETE", "/chairman/service-contacts/"+cid, empty, uuid.NewString(), 200)
	if len(call(chair, "GET", "/house/service-contacts", nil, "", 200)["items"].([]any)) != 0 {
		t.Fatal("archived contact visible")
	}
	t.Log("14-16: notification preferences, recipient isolation, duplicate event")
	prefs := map[string]bool{"notifications_enabled": true, "issue_notifications_enabled": true, "announcement_notifications_enabled": false, "membership_notifications_enabled": true, "bot_notifications_enabled": true}
	call(third, "PUT", "/notifications/settings", prefs, "", 200)
	eventID := uuid.NewString()
	event := map[string]any{"event_id": eventID, "event_type": "poll.created", "event_version": 1, "occurred_at": time.Now().UTC(), "producer": "community-service", "payload": map[string]string{"house_id": house}}
	raw, _ := json.Marshal(event)
	values := map[string]any{"event_id": eventID, "event_type": "poll.created", "data": string(raw)}
	first, e := r.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Result()
	if e != nil {
		t.Fatal(e)
	}
	wait := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("notification condition timed out")
	}
	text := "В вашем доме началось голосование."
	wait(func() bool {
		return count(fmt.Sprint(chairID), text) == 1 && count(fmt.Sprint(residentID), text) == 1 && r.Exists(ctx, "gateway:notification:"+eventID+":done").Val() == 1
	})
	second, e := r.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Result()
	if e != nil {
		t.Fatal(e)
	}
	wait(func() bool {
		info, _ := r.XInfoGroups(ctx, stream).Result()
		for _, g := range info {
			if g.Name == group {
				var deliveredMS, deliveredSeq, secondMS, secondSeq uint64
				if _, err := fmt.Sscanf(g.LastDeliveredID, "%d-%d", &deliveredMS, &deliveredSeq); err != nil {
					return false
				}
				if _, err := fmt.Sscanf(second, "%d-%d", &secondMS, &secondSeq); err != nil {
					return false
				}
				if deliveredMS < secondMS || (deliveredMS == secondMS && deliveredSeq < secondSeq) {
					return false
				}
				pending, err := r.XPendingExt(ctx, &redis.XPendingExtArgs{Stream: stream, Group: group, Start: second, End: second, Count: 1}).Result()
				return err == nil && len(pending) == 0
			}
		}
		return false
	})
	if first == second || count(fmt.Sprint(chairID), text) != 1 || count(fmt.Sprint(residentID), text) != 1 || count(fmt.Sprint(thirdID), text) != 0 || count(fmt.Sprint(outsiderID), text) != 0 {
		t.Fatal("deduplication/preferences/house isolation failed")
	}
	// Real transactional outbox must also publish, not only the synthetic replay.
	wait(func() bool {
		var n int
		e := db.QueryRow(ctx, `SELECT count(*) FROM identity_outbox WHERE payload->>'house_id'=$1 AND published_at IS NOT NULL`, house).Scan(&n)
		return e == nil && n >= 8
	})
	t.Log("house workflow passed with real Identity, Community, PostgreSQL and Redis")
}
