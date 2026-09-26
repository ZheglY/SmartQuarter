package http

import (
	ipb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/identity/v1"
	"net/http"
)

// Recheck authority before replaying an idempotency response. A former chairman
// must not recover an invitation token from Redis after losing the role.
func (a *API) housePermission(access string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.House == nil {
			a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "house workflow unavailable")
			return
		}
		state, e := a.House.GetHouseAccessState(r.Context(), &ipb.GetHouseAccessStateRequest{})
		if e != nil {
			a.rpcError(w, r, e)
			return
		}
		allowed := state.CanManageActiveHouse
		if access == "admin" {
			allowed = state.PlatformAdmin
		}
		if access == "chairman" {
			allowed = state.CanRegisterHouse
		}
		if !allowed {
			a.fail(w, r, 403, "PERMISSION_DENIED", "access denied")
			return
		}
		next(w, r)
	}
}
