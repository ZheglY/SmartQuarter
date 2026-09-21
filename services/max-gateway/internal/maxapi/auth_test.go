package maxapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Controlled vector follows the official MAX algorithm; not a live platform token.
func signed(date int64) string {
	user := `{"id":67890,"first_name":"Max","last_name":"User","username":null}`
	check := fmt.Sprintf("auth_date=%d\nuser=%s", date, user)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte("test-only-token"))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(check))
	return fmt.Sprintf("auth_date=%d&user=%s&hash=%s", date, url.QueryEscape(user), hex.EncodeToString(mac.Sum(nil)))
}
func TestValidate(t *testing.T) {
	now := time.Unix(1771409719, 0)
	raw := signed(now.Unix())
	u, e := Validate(raw, "test-only-token", now, 5*time.Minute)
	if e != nil || u.ID != 67890 {
		t.Fatal(u, e)
	}
	for _, bad := range []string{raw + "&hash=abcd", raw + "&auth_date=0", strings.Replace(raw, "67890", "67891", 1), signed(now.Add(-6 * time.Minute).Unix()), signed(now.Add(time.Minute).Unix()), "user=67890"} {
		if _, e := Validate(bad, "test-only-token", now, 5*time.Minute); e == nil {
			t.Fatal("forgery or stale accepted")
		}
	}
}
func TestBot429AndContract(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "test-token" || r.URL.Query().Get("user_id") != "123" || r.URL.Query().Has("access_token") {
			t.Error("bad auth/recipient")
		}
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message":{"body":{"mid":"test"}}}`)
	}))
	defer s.Close()
	c := &Client{BaseURL: s.URL, Token: "test-token", BotUsername: "test_bot"}
	if e := c.Send(context.Background(), 123, "test", true); e != nil || calls != 2 {
		t.Fatal(e, calls)
	}
}
func TestBotNoBlindRetry(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer s.Close()
	c := &Client{BaseURL: s.URL, Token: "test-token"}
	if c.Send(context.Background(), 123, "test", false) == nil || calls != 1 {
		t.Fatal("ambiguous delivery retried")
	}
}
