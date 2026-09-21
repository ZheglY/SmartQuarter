package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/rpc"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/state"
)

type traceKey struct{}
type trace struct{ ID, User, House, Error string }
type actorKey struct{}
type actor struct {
	Session     state.Session
	Token, Role string
}

func requestID(r *http.Request) string {
	t, _ := r.Context().Value(traceKey{}).(*trace)
	if t == nil {
		return ""
	}
	return t.ID
}
func current(r *http.Request) actor { v, _ := r.Context().Value(actorKey{}).(actor); return v }

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(c int) {
	if w.status == 0 {
		w.status = c
		w.ResponseWriter.WriteHeader(c)
	}
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (a *API) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !identity.ValidID(id) {
			id = uuid.NewString()
		}
		t := &trace{ID: id}
		ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), traceKey{}, t), a.Config.RequestTimeout)
		defer cancel()
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
		w.Header().Set("X-Request-Id", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		sw := &statusWriter{ResponseWriter: w}
		start := time.Now()
		defer func() {
			if recover() != nil {
				a.fail(sw, r, 500, "INTERNAL_ERROR", "internal error")
			}
			if sw.status == 0 {
				sw.status = 200
			}
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			method := r.Method
			switch method {
			case "GET", "POST", "PATCH", "OPTIONS", "HEAD":
			default:
				method = "OTHER"
			}
			a.Metrics.HTTP.WithLabelValues(method, route, strconv.Itoa(sw.status)).Inc()
			a.Metrics.HTTPDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
			a.Logger.Info("http request", zap.String("request_id", id), zap.String("http_method", method), zap.String("http_route", route), zap.Int("http_status", sw.status), zap.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000), zap.String("user_id", t.User), zap.String("house_id", t.House), zap.String("error_code", t.Error))
		}()
		// OPTIONS must be processed before ServeMux method matching.
		if r.Method == "OPTIONS" {
			a.origin(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })(sw, r)
			return
		}
		next.ServeHTTP(sw, r)
	})
}
func (a *API) origin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, v := range a.Config.Origins {
			if origin == v {
				allowed = true
			}
		}
		mutation := r.Method != "GET" && r.Method != "HEAD"
		if (origin != "" && !allowed) || (mutation && !allowed) {
			a.fail(w, r, 403, "ORIGIN_DENIED", "trusted Origin required")
			return
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, X-Request-Id")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")
		}
		next(w, r)
	}
}
func (a *API) limit(w http.ResponseWriter, r *http.Request, key string, n int) bool {
	ok, e := a.Store.Allow(r.Context(), key, n, time.Minute)
	if e != nil {
		a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "rate limiter unavailable")
		return false
	}
	if !ok {
		a.Metrics.RateRejected.Inc()
		w.Header().Set("Retry-After", "60")
		a.fail(w, r, 429, "RATE_LIMITED", "rate limit exceeded")
		return false
	}
	return true
}
func remote(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return r.RemoteAddr
	}
	return host
}
func (a *API) authenticate(house, manager bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie(a.Config.CookieName)
		if e != nil {
			a.fail(w, r, 401, "UNAUTHENTICATED", "session required")
			return
		}
		s, e := a.Store.Get(r.Context(), c.Value)
		if e != nil {
			a.Metrics.SessionErrors.Inc()
			if e == redis.Nil {
				a.fail(w, r, 401, "UNAUTHENTICATED", "session expired or invalid")
			} else {
				a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "session store unavailable")
			}
			return
		}
		if !a.limit(w, r, "business:"+s.UserID, a.Config.BusinessRate) {
			return
		}
		v := actor{Session: s, Token: c.Value}
		if house {
			m, e := a.Identity.GetMembership(r.Context(), s.UserID, s.ActiveHouseID)
			if e != nil {
				a.rpcError(w, r, e)
				return
			}
			if !m.Authorizes(s.UserID, s.ActiveHouseID) {
				a.fail(w, r, 403, "PERMISSION_DENIED", "active membership required")
				return
			}
			v.Role = m.Role
			if manager && m.Role != "CHAIRMAN" && m.Role != "ADMIN" {
				a.fail(w, r, 403, "PERMISSION_DENIED", "chairman role required")
				return
			}
		}
		t := r.Context().Value(traceKey{}).(*trace)
		t.User = s.UserID
		t.House = s.ActiveHouseID
		ctx := rpc.Actor(r.Context(), t.ID, s.UserID, s.ActiveHouseID, v.Role)
		r = r.WithContext(context.WithValue(ctx, actorKey{}, v))
		next(w, r)
	}
}

// Commands with no key execute once. With a key, payload and route are bound to
// user+house. Ambiguous failures retain the reservation until operator reconciliation.
func (a *API) idempotent(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next(w, r)
			return
		}
		if !identity.ValidID(key) {
			a.fail(w, r, 400, "INVALID_ARGUMENT", "Idempotency-Key must be a UUID")
			return
		}
		b, e := io.ReadAll(r.Body)
		if e != nil {
			a.fail(w, r, 413, "BODY_TOO_LARGE", "request body too large")
			return
		}
		var compact bytes.Buffer
		if json.Compact(&compact, b) != nil {
			a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid JSON")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(b))
		v := current(r)
		scope := v.Session.UserID + ":" + v.Session.ActiveHouseID + ":" + key
		hash := state.Digest(r.Method + " " + r.URL.Path + "\n" + compact.String())
		fresh, result, e := a.Store.Reserve(r.Context(), scope, hash)
		if e != nil {
			a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "idempotency store unavailable")
			return
		}
		if !fresh {
			if result.Hash != hash {
				a.fail(w, r, 409, "IDEMPOTENCY_KEY_REUSED", "key already used with another command")
				return
			}
			if result.Status == 0 {
				a.fail(w, r, 409, "IDEMPOTENCY_IN_PROGRESS", "command pending or outcome unknown")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(result.Status)
			_, _ = w.Write(result.Body)
			return
		}
		recorder := &bufferWriter{header: make(http.Header)}
		next(recorder, r)
		if recorder.code > 0 && recorder.code < 400 {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), time.Second)
			defer cancel()
			if a.Store.Finish(ctx, scope, hash, recorder.code, recorder.body.Bytes()) != nil {
				a.fail(w, r, 503, "IDEMPOTENCY_OUTCOME_UNKNOWN", "command may have completed; reconcile before retry")
				return
			}
		}
		for k, v := range recorder.header {
			w.Header()[k] = v
		}
		w.WriteHeader(recorder.code)
		_, _ = w.Write(recorder.body.Bytes())
	}
}

type bufferWriter struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func (w *bufferWriter) Header() http.Header { return w.header }
func (w *bufferWriter) WriteHeader(c int) {
	if w.code == 0 {
		w.code = c
	}
}
func (w *bufferWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	return w.body.Write(b)
}
