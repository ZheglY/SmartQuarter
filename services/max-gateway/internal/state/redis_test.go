//go:build integration

package state

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisSessionsAndAtomicCommands(t *testing.T) {
	if os.Getenv("GATEWAY_E2E") != "1" {
		t.Fatal("dedicated Redis required")
	}
	ctx := context.Background()
	r := redis.NewClient(&redis.Options{Addr: "localhost:16379", MaxRetries: -1})
	defer r.Close()
	s := Store{r}
	token, v, e := s.Create(ctx, uuid.NewString(), uuid.NewString(), 200*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(ctx, token); e != nil {
		t.Fatal(e)
	}
	if e = s.Delete(ctx, token); e != nil {
		t.Fatal(e)
	}
	if e = s.Switch(ctx, token, v); e != redis.Nil {
		t.Fatal("session resurrected", e)
	}
	if _, e = s.Get(ctx, token); e != redis.Nil {
		t.Fatal(e)
	}
	token, _, e = s.Create(ctx, uuid.NewString(), uuid.NewString(), 50*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(70 * time.Millisecond)
	if _, e = s.Get(ctx, token); e != redis.Nil {
		t.Fatal("expired session valid", e)
	}
	key := uuid.NewString()
	defer r.Del(ctx, "gateway:idem:"+Digest(key))
	var wg sync.WaitGroup
	results := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fresh, _, e := s.Reserve(ctx, key, "body-hash")
			if e != nil {
				t.Error(e)
			}
			results <- fresh
		}()
	}
	wg.Wait()
	close(results)
	n := 0
	for fresh := range results {
		if fresh {
			n++
		}
	}
	if n != 1 {
		t.Fatal("multiple concurrent commands", n)
	}
	if e = s.Finish(ctx, key, "body-hash", 201, []byte(`{"id":"test"}`)); e != nil {
		t.Fatal(e)
	}
	fresh, res, e := s.Reserve(ctx, key, "other-hash")
	if e != nil || fresh || res.Hash != "body-hash" || res.Status != 201 {
		t.Fatal("idempotency state lost")
	}
	for i := 0; i < 3; i++ {
		ok, e := s.Allow(ctx, key, 2, time.Minute)
		if e != nil || ok != (i < 2) {
			t.Fatal("rate counter", i, e)
		}
	}
	defer r.Del(ctx, "gateway:rate:"+Digest(key))
	r.Close()
	if _, e = s.Get(ctx, token); e == nil {
		t.Fatal("Redis outage ignored")
	}
}
