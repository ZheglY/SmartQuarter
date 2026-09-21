package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/maxapi"
)

func (a *API) cookie(w http.ResponseWriter, token string, expires time.Time, maxAge int) {
	same := http.SameSiteLaxMode
	if a.Config.SecureCookie {
		same = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{Name: a.Config.CookieName, Value: token, Path: "/", HttpOnly: true, Secure: a.Config.SecureCookie, SameSite: same, Expires: expires, MaxAge: maxAge})
}
func (a *API) bootstrap(w http.ResponseWriter, r *http.Request) {
	if !a.limit(w, r, "auth:"+remote(r), a.Config.AuthRate) {
		return
	}
	var req struct {
		InitData string `json:"init_data"`
	}
	if !a.decode(w, r, &req) {
		return
	}
	maxUser, e := maxapi.Validate(req.InitData, a.Config.BotToken, time.Now(), a.Config.InitDataTTL)
	if e != nil {
		a.fail(w, r, 401, "INVALID_INIT_DATA", "invalid or expired MAX init data")
		return
	}
	user, e := a.Identity.UpsertMaxUser(r.Context(), maxUser)
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	if !identity.ValidID(user.ID) || user.MaxUserID != strconv.FormatInt(maxUser.ID, 10) {
		a.fail(w, r, 502, "INVALID_UPSTREAM_RESPONSE", "invalid identity response")
		return
	}
	uc, e := a.Identity.GetUserContext(r.Context(), user.ID)
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	if uc.User.ID != user.ID || uc.User.MaxUserID != user.MaxUserID {
		a.fail(w, r, 502, "INVALID_UPSTREAM_RESPONSE", "invalid identity response")
		return
	}
	house := uc.DefaultHouseID
	if house != "" {
		m, e := a.Identity.GetMembership(r.Context(), user.ID, house)
		if e != nil {
			a.rpcError(w, r, e)
			return
		}
		if !m.Authorizes(user.ID, house) {
			a.fail(w, r, 403, "PERMISSION_DENIED", "active membership required")
			return
		}
	}
	token, s, e := a.Store.Create(r.Context(), user.ID, house, a.Config.SessionTTL)
	if e != nil {
		a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "session store unavailable")
		return
	}
	uc.ActiveHouseID = house
	a.cookie(w, token, s.ExpiresAt, int(a.Config.SessionTTL.Seconds()))
	write(w, 200, struct {
		UserContext identity.UserContext `json:"user_context"`
		ExpiresAt   time.Time            `json:"expires_at"`
	}{uc, s.ExpiresAt})
}
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	v := current(r)
	uc, e := a.Identity.GetUserContext(r.Context(), v.Session.UserID)
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	if uc.User.ID != v.Session.UserID {
		a.fail(w, r, 502, "INVALID_UPSTREAM_RESPONSE", "invalid identity response")
		return
	}
	uc.ActiveHouseID = v.Session.ActiveHouseID
	write(w, 200, uc)
}
func (a *API) switchHouse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HouseID string `json:"house_id"`
	}
	if !a.decode(w, r, &req) {
		return
	}
	if !identity.ValidID(req.HouseID) {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid house id")
		return
	}
	v := current(r)
	m, e := a.Identity.GetMembership(r.Context(), v.Session.UserID, req.HouseID)
	if e != nil {
		a.rpcError(w, r, e)
		return
	}
	if !m.Authorizes(v.Session.UserID, req.HouseID) {
		a.fail(w, r, 403, "PERMISSION_DENIED", "active membership required")
		return
	}
	v.Session.ActiveHouseID = req.HouseID
	if a.Store.Switch(r.Context(), v.Token, v.Session) != nil {
		a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "session update failed")
		return
	}
	write(w, 200, map[string]string{"active_house_id": req.HouseID, "role": m.Role})
}
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if !a.decode(w, r, &struct{}{}) {
		return
	}
	if a.Store.Delete(r.Context(), current(r).Token) != nil {
		a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "session store unavailable")
		return
	}
	a.cookie(w, "", time.Unix(1, 0), -1)
	w.WriteHeader(204)
}
