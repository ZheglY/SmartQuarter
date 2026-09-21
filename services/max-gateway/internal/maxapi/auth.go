package maxapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
)

// Validate implements https://dev.max.ru/docs/webapps/validation (2026-09-21).
// Input is raw initData, not the URL fragment or initDataUnsafe.
func Validate(raw, token string, now time.Time, maxAge time.Duration) (identity.MaxUser, error) {
	invalid := errors.New("invalid MAX init data")
	var user identity.MaxUser
	if len(raw) > 16384 || token == "" || maxAge <= 0 {
		return user, invalid
	}
	values, e := url.ParseQuery(raw)
	if e != nil {
		return user, invalid
	}
	keys := make([]string, 0, len(values))
	for k, v := range values {
		if len(v) != 1 || strings.ContainsAny(k, "\n\r=") {
			return user, invalid
		}
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	hash, e := hex.DecodeString(values.Get("hash"))
	if e != nil || len(hash) != 32 {
		return user, invalid
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+values.Get(k))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))
	if !hmac.Equal(hash, mac.Sum(nil)) {
		return user, invalid
	}
	date, e := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if e != nil {
		return user, invalid
	}
	age := now.Sub(time.Unix(date, 0))
	if age < -30*time.Second || age > maxAge {
		return user, invalid
	}
	if json.Unmarshal([]byte(values.Get("user")), &user) != nil || user.ID <= 0 {
		return identity.MaxUser{}, invalid
	}
	return user, nil
}
