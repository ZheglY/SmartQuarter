// Package identity owns the Gateway port pending an agreed Identity protobuf.
// There is no identity.proto in this repository. Production RPCs are not invented.
package identity

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MaxUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}
type User struct {
	ID          string    `json:"id"`
	MaxUserID   string    `json:"max_user_id"`
	DisplayName string    `json:"display_name"`
	Username    string    `json:"username"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type House struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	City    string `json:"city"`
}
type Membership struct {
	ID      string `json:"id"`
	UserID  string `json:"user_id"`
	HouseID string `json:"house_id"`
	Role    string `json:"role"`
	Status  string `json:"status"`
}
type UserContext struct {
	User           User         `json:"user"`
	Houses         []House      `json:"houses"`
	Memberships    []Membership `json:"memberships"`
	DefaultHouseID string       `json:"default_house_id"`
	ActiveHouseID  string       `json:"active_house_id"`
}
type Client interface {
	UpsertMaxUser(context.Context, MaxUser) (User, error)
	GetUserContext(context.Context, string) (UserContext, error)
	GetMembership(context.Context, string, string) (Membership, error)
	ListMemberships(context.Context, string) ([]Membership, error)
	Ready(context.Context) error
}

func ValidID(s string) bool {
	v, e := uuid.Parse(s)
	return e == nil && v != uuid.Nil && v.String() == s
}
func (m Membership) Authorizes(user, house string) bool {
	return ValidID(user) && ValidID(house) && m.UserID == user && m.HouseID == house && m.Status == "ACTIVE" && (m.Role == "RESIDENT" || m.Role == "CHAIRMAN" || m.Role == "ADMIN")
}

type Unavailable struct{}

func (Unavailable) Ready(context.Context) error {
	return status.Error(codes.Unavailable, "identity contract unavailable")
}
func (u Unavailable) UpsertMaxUser(c context.Context, _ MaxUser) (User, error) {
	return User{}, u.Ready(c)
}
func (u Unavailable) GetUserContext(c context.Context, _ string) (UserContext, error) {
	return UserContext{}, u.Ready(c)
}
func (u Unavailable) GetMembership(c context.Context, _, _ string) (Membership, error) {
	return Membership{}, u.Ready(c)
}
func (u Unavailable) ListMemberships(c context.Context, _ string) ([]Membership, error) {
	return nil, u.Ready(c)
}
