//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/config"
	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/maxapi"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/notification"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/rpc"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/state"
	transport "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/transport/http"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/tests/testdata"
)

func initData(id int64) string {
	date := time.Now().Unix()
	user := fmt.Sprintf(`{"id":%d,"first_name":"Test"}`, id)
	check := fmt.Sprintf("auth_date=%d\nuser=%s", date, user)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte("gateway-test-token"))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(check))
	return fmt.Sprintf("auth_date=%d&user=%s&hash=%s", date, url.QueryEscape(user), hex.EncodeToString(mac.Sum(nil)))
}
func TestGatewayIssueE2EFiveRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if os.Getenv("GATEWAY_E2E") != "1" {
		t.Fatal("set GATEWAY_E2E=1 using scripts/test.ps1")
	}
	r := redis.NewClient(&redis.Options{Addr: "localhost:16379", MaxRetries: -1, ContextTimeoutEnabled: true})
	defer r.Close()
	if e := r.Ping(ctx).Err(); e != nil {
		t.Fatal(e)
	}
	db, e := pgxpool.New(ctx, os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.Ping(ctx); e != nil {
		t.Fatal(e)
	}
	ids := testdata.Start(t)
	metrics := observability.New()
	conn, e := rpc.Dial("localhost:18082", 10*time.Second, zap.NewNop(), metrics)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	issue := pb.NewIssueServiceClient(conn)
	var delivered atomic.Int64
	maxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "gateway-test-token" {
			t.Error("MAX auth header lost")
		}
		var b map[string]any
		if json.NewDecoder(r.Body).Decode(&b) != nil {
			t.Error("MAX body invalid")
		}
		if r.URL.Path == "/messages" {
			if r.URL.Query().Get("user_id") != "101" {
				t.Errorf("unexpected MAX recipient %s", r.URL.Query().Get("user_id"))
			}
			delivered.Add(1)
		}
		fmt.Fprint(w, `{"message":{"body":{"mid":"test-only"}},"success":true}`)
	}))
	defer maxServer.Close()
	cfg := config.Config{Environment: "test", CookieName: "sq_session", SessionTTL: time.Hour, InitDataTTL: 5 * time.Minute, RequestTimeout: 10 * time.Second, Origins: []string{"https://test.example"}, AuthRate: 1000, BusinessRate: 10000, WebhookRate: 1000, BotToken: "gateway-test-token", WebhookSecret: "test-webhook-secret"}
	bot := &maxapi.Client{BaseURL: maxServer.URL, Token: cfg.BotToken, BotUsername: "test_bot"}
	api := &transport.API{Config: cfg, Store: state.Store{R: r}, Identity: ids, Issue: issue, Bot: bot, Metrics: metrics, Logger: zap.NewNop(), IssueReady: func(ctx context.Context) error { return nil }}
	gateway := httptest.NewServer(api.Handler())
	defer gateway.Close()
	client := &http.Client{Timeout: 15 * time.Second}
	call := func(t *testing.T, cookie *http.Cookie, method, path string, body any, key string, want int) map[string]any {
		t.Helper()
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		req, _ := http.NewRequest(method, gateway.URL+path, bytes.NewReader(b))
		req.Header.Set("Origin", "https://test.example")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-Id", "55555555-5555-4555-8555-555555555555")
		req.Header.Set("x-actor-role", "ADMIN")
		req.Header.Set("x-house-id", testdata.ForeignHouse)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s = %d want %d: %s", method, path, res.StatusCode, want, data)
		}
		if res.Header.Get("X-Request-Id") != "55555555-5555-4555-8555-555555555555" {
			t.Fatal("request id lost")
		}
		v := map[string]any{}
		if len(data) > 0 {
			if json.Unmarshal(data, &v) != nil {
				t.Fatal(string(data))
			}
		}
		return v
	}
	login := func(id int64) *http.Cookie {
		t.Helper()
		b, _ := json.Marshal(map[string]string{"init_data": initData(id)})
		req, _ := http.NewRequest("POST", gateway.URL+"/api/v1/session/max", bytes.NewReader(b))
		req.Header.Set("Origin", "https://test.example")
		req.Header.Set("Content-Type", "application/json")
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			data, _ := io.ReadAll(res.Body)
			t.Fatalf("login %d: %s", res.StatusCode, data)
		}
		for _, c := range res.Cookies() {
			if c.Name == cfg.CookieName {
				if !c.HttpOnly {
					t.Fatal("cookie not HttpOnly")
				}
				return c
			}
		}
		t.Fatal("no session")
		return nil
	}
	author, neighbor, chairman, foreign := login(101), login(102), login(103), login(104)
	call(t, author, "GET", "/api/v1/me", nil, "", 200)
	call(t, author, "POST", "/api/v1/session/active-house", map[string]string{"house_id": testdata.ForeignHouse}, "", 403)
	call(t, nil, "GET", "/api/v1/issues", nil, "", 401)
	call(t, &http.Cookie{Name: cfg.CookieName, Value: "invalid"}, "GET", "/api/v1/issues", nil, "", 401)
	group := "gateway-e2e-" + uuid.NewString()
	if e = r.XGroupCreateMkStream(ctx, "stream:notifications", group, "$").Err(); e != nil {
		t.Fatal(e)
	}
	defer r.XGroupDestroy(context.Background(), "stream:notifications", group)
	worker := &notification.Consumer{Redis: r, Stream: "stream:notifications", Group: group, Identity: ids, Bot: bot, Metrics: metrics, Logger: zap.NewNop()}
	done := make(chan struct{})
	go func() { defer close(done); worker.Run(ctx) }()
	defer func() { cancel(); <-done }()
	for run := 1; run <= 5; run++ {
		t.Run(strconv.Itoa(run), func(t *testing.T) {
			var photo bytes.Buffer
			_ = png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 2, 2)))
			upload := call(t, author, "POST", "/api/v1/uploads", map[string]any{"filename": "photo.png", "mime_type": "image/png", "size_bytes": photo.Len()}, "", 201)
			id := upload["upload_id"].(string)
			put, _ := http.NewRequest("PUT", upload["presigned_url"].(string), bytes.NewReader(photo.Bytes()))
			for k, v := range upload["required_headers"].(map[string]any) {
				put.Header.Set(k, v.(string))
			}
			res, e := client.Do(put)
			if e != nil {
				t.Fatal(e)
			}
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatal("S3 PUT", res.StatusCode)
			}
			call(t, neighbor, "POST", "/api/v1/uploads/"+id+"/complete", map[string]any{}, "", 403)
			attachment := call(t, author, "POST", "/api/v1/uploads/"+id+"/complete", map[string]any{}, "", 200)
			if attachment["status"] != "READY" || attachment["issue_id"] != nil {
				t.Fatal(attachment)
			}
			body := map[string]any{"category": "INFRASTRUCTURE", "description": "Test lighting issue", "location_text": "Entrance 1", "attachment_ids": []string{id}}
			call(t, neighbor, "POST", "/api/v1/issues", body, "", 403)
			key := uuid.NewString()
			created := call(t, author, "POST", "/api/v1/issues", body, key, 201)
			issueID := created["id"].(string)
			if created["house_id"] != testdata.House || created["house_address_snapshot"] != "Test address 1" {
				t.Fatal("untrusted house metadata")
			}
			replay := call(t, author, "POST", "/api/v1/issues", body, key, 201)
			if replay["id"] != issueID {
				t.Fatal("idempotency failed")
			}
			call(t, author, "POST", "/api/v1/issues", map[string]any{"category": "OTHER"}, key, 409)
			spoof := map[string]any{"category": "OTHER", "role": "ADMIN", "house_id": testdata.ForeignHouse}
			call(t, author, "POST", "/api/v1/issues", spoof, "", 400)
			path := "/api/v1/issues/" + issueID
			call(t, author, "GET", "/api/v1/issues", nil, "", 200)
			details := call(t, author, "GET", path, nil, "", 200)
			if details["latest_statement"] != nil {
				t.Fatal("statement leaked")
			}
			call(t, foreign, "GET", path, nil, "", 403)
			call(t, foreign, "GET", "/api/v1/attachments/"+id+"/download-url", nil, "", 403)
			call(t, neighbor, "POST", path+"/confirm", map[string]any{}, "", 200)
			call(t, neighbor, "POST", path+"/confirm", map[string]any{}, "", 409)
			call(t, author, "POST", path+"/statement", map[string]any{}, "", 403)
			call(t, author, "PATCH", path+"/status", map[string]string{"new_status": "RESOLVED"}, "", 403)
			call(t, chairman, "GET", "/api/v1/chairman/issues", nil, "", 200)
			statementKey := uuid.NewString()
			statement := call(t, chairman, "POST", path+"/statement", map[string]string{"chairman_note": "Please repair"}, statementKey, 201)
			again := call(t, chairman, "POST", path+"/statement", map[string]string{"chairman_note": "Please repair"}, statementKey, 201)
			if statement["id"] != again["id"] {
				t.Fatal("statement repeated")
			}
			call(t, chairman, "GET", path+"/statement", nil, "", 200)
			call(t, chairman, "PATCH", path+"/status", map[string]string{"new_status": "INVALID"}, "", 400)
			call(t, chairman, "PATCH", path+"/status", map[string]string{"new_status": "MARKED_SENT"}, "", 409)
			call(t, chairman, "PATCH", path+"/status", map[string]string{"new_status": "CONFIRMING"}, "", 200)
			download := call(t, author, "GET", "/api/v1/attachments/"+id+"/download-url", nil, "", 200)
			res, e = client.Get(download["url"].(string))
			if e != nil {
				t.Fatal(e)
			}
			data, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if !bytes.Equal(data, photo.Bytes()) {
				t.Fatal("download differs")
			}
			var outbox int
			if e = db.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1", issueID).Scan(&outbox); e != nil || outbox != 4 {
				t.Fatalf("outbox %d: %v", outbox, e)
			}
			deadline := time.Now().Add(12 * time.Second)
			for delivered.Load() < int64(run*4) && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
			if delivered.Load() < int64(run*4) {
				t.Fatal("notifications not delivered", delivered.Load())
			}
			var pending int
			if e = db.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND published_at IS NULL", issueID).Scan(&pending); e != nil || pending != 0 {
				t.Fatal("outbox not published")
			}
			entries, e := r.XRange(ctx, "stream:notifications", "-", "+").Result()
			if e != nil {
				t.Fatal(e)
			}
			found := 0
			for _, entry := range entries {
				var event notification.Event
				raw, _ := entry.Values["data"].(string)
				_ = json.Unmarshal([]byte(raw), &event)
				if event.Payload.IssueID == issueID {
					found++
					if found == 1 {
						before := delivered.Load()
						r.XAdd(ctx, &redis.XAddArgs{Stream: "stream:notifications", Values: entry.Values})
						time.Sleep(200 * time.Millisecond)
						if delivered.Load() != before {
							t.Fatal("duplicate event delivered")
						}
					}
				}
			}
			if found != 4 {
				t.Fatalf("stream events %d", found)
			}
		})
	}
	// Real upstream validation, cancellation, and service unavailability.
	actor := rpc.Actor(ctx, uuid.NewString(), testdata.Users[101], testdata.House, "RESIDENT")
	_, e = issue.ListIssues(actor, &pb.ListIssuesRequest{HouseId: testdata.House, Status: []pb.IssueStatus{999}})
	if status.Code(e) != codes.InvalidArgument {
		t.Fatal(e)
	}
	canceled, stop := context.WithCancel(actor)
	stop()
	_, e = issue.ListIssues(canceled, &pb.ListIssuesRequest{HouseId: testdata.House})
	if status.Code(e) != codes.Canceled {
		t.Fatal(e)
	}
	badConn, e := rpc.Dial("127.0.0.1:1", time.Second, zap.NewNop(), metrics)
	if e != nil {
		t.Fatal(e)
	}
	api.Issue = pb.NewIssueServiceClient(badConn)
	call(t, author, "GET", "/api/v1/issues", nil, "", 503)
	badConn.Close()
	api.Issue = issue
	// Actual MAX Update envelope and deduplication, still using a fake MAX endpoint.
	body := map[string]any{"update_type": "bot_started", "timestamp": time.Now().UnixMilli(), "user": map[string]any{"user_id": 101, "is_bot": false}}
	for i := 0; i < 2; i++ {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", gateway.URL+"/webhooks/max", bytes.NewReader(b))
		req.Header.Set("X-Max-Bot-Api-Secret", cfg.WebhookSecret)
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatal("webhook", res.StatusCode)
		}
	}
	if delivered.Load() != 21 {
		t.Fatal("webhook dedup", delivered.Load())
	}

	// Dedicated stack failure injection; always resume the database even on failure.
	if err := exec.Command("docker", "pause", "smartquarter-issue-local-postgres-1").Run(); err != nil {
		t.Fatal("cannot inject PostgreSQL outage", err)
	}
	func() {
		defer func() {
			if err := exec.Command("docker", "unpause", "smartquarter-issue-local-postgres-1").Run(); err != nil {
				t.Error("cannot resume PostgreSQL", err)
			}
		}()
		req, _ := http.NewRequest("GET", gateway.URL+"/api/v1/issues", nil)
		req.AddCookie(neighbor)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 503 && res.StatusCode != 504 {
			t.Fatal("PostgreSQL outage status", res.StatusCode)
		}
	}()
	call(t, neighbor, "GET", "/api/v1/issues", nil, "", 200)
	// A real closed Redis connection must fail closed, not authenticate a cached actor.
	unavailableRedis := redis.NewClient(&redis.Options{Addr: "localhost:16379", MaxRetries: -1})
	unavailableRedis.Close()
	api.Store = state.Store{R: unavailableRedis}
	call(t, neighbor, "GET", "/api/v1/issues", nil, "", 503)
	api.Store = state.Store{R: r}
	call(t, author, "POST", "/api/v1/session/logout", map[string]any{}, "", 204)
	call(t, author, "GET", "/api/v1/issues", nil, "", 401)
	t.Log("PASS: five HTTP -> real Issue gRPC -> PostgreSQL/S3 -> outbox/Redis -> fake MAX runs; test-only Identity gRPC")
}
