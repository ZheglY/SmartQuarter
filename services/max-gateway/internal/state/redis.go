package state

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct{ R *redis.Client }
type Session struct {
	UserID        string    `json:"user_id"`
	ActiveHouseID string    `json:"active_house_id"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

func Digest(s string) string         { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func sessionKey(token string) string { return "gateway:session:" + Digest(token) }
func (s Store) Create(ctx context.Context, user, house string, ttl time.Duration) (string, Session, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", Session{}, e
	}
	token := hex.EncodeToString(b)
	now := time.Now().UTC()
	v := Session{user, house, now, now.Add(ttl)}
	data, _ := json.Marshal(v)
	e := s.R.Set(ctx, sessionKey(token), data, ttl).Err()
	return token, v, e
}
func (s Store) Get(ctx context.Context, token string) (Session, error) {
	var v Session
	if len(token) != 64 {
		return v, redis.Nil
	}
	if _, e := hex.DecodeString(token); e != nil {
		return v, redis.Nil
	}
	b, e := s.R.Get(ctx, sessionKey(token)).Bytes()
	if e != nil {
		return v, e
	}
	if json.Unmarshal(b, &v) != nil {
		return v, errors.New("invalid session")
	}
	if !time.Now().Before(v.ExpiresAt) {
		return v, redis.Nil
	}
	return v, nil
}
func (s Store) Delete(ctx context.Context, token string) error {
	return s.R.Del(ctx, sessionKey(token)).Err()
}

// SET XX KEEPTTL never resurrects a logged-out or expired session.
func (s Store) Switch(ctx context.Context, token string, v Session) error {
	b, _ := json.Marshal(v)
	return s.R.SetArgs(ctx, sessionKey(token), b, redis.SetArgs{Mode: "XX", KeepTTL: true}).Err()
}

var rate = redis.NewScript(`local n=redis.call('INCR',KEYS[1]);if n==1 then redis.call('PEXPIRE',KEYS[1],ARGV[1]) end;return n`)

func (s Store) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	n, e := rate.Run(ctx, s.R, []string{"gateway:rate:" + Digest(key)}, window.Milliseconds()).Int64()
	return n <= int64(limit), e
}

type Result struct {
	Hash   string          `json:"hash"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// Reserve blocks ambiguous commands for 24h; never blindly repeat a dispatched RPC.
func (s Store) Reserve(ctx context.Context, key, hash string) (bool, Result, error) {
	key = "gateway:idem:" + Digest(key)
	b, _ := json.Marshal(Result{Hash: hash})
	ok, e := s.R.SetNX(ctx, key, b, 24*time.Hour).Result()
	if e != nil || ok {
		return ok, Result{}, e
	}
	b, e = s.R.Get(ctx, key).Bytes()
	var v Result
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return false, v, e
}
func (s Store) Finish(ctx context.Context, key, hash string, code int, body []byte) error {
	b, _ := json.Marshal(Result{hash, code, body})
	return s.R.SetArgs(ctx, "gateway:idem:"+Digest(key), b, redis.SetArgs{Mode: "XX", KeepTTL: true}).Err()
}
