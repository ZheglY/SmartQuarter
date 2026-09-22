package usecase

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/domain"
	"testing"
)

const uid = "11111111-1111-4111-8111-111111111111"
const first = "22222222-2222-4222-8222-222222222222"
const second = "33333333-3333-4333-8333-333333333333"

type contextRepo struct {
	domain.Repository
	user    domain.User
	members []domain.Membership
}

func (r contextRepo) GetUserByID(context.Context, string) (*domain.User, error) { return &r.user, nil }
func (r contextRepo) ListMembershipsByUserID(context.Context, string) ([]domain.Membership, error) {
	return r.members, nil
}
func (r contextRepo) ListHousesByIDs(context.Context, []string) ([]domain.House, error) {
	return []domain.House{{ID: first}, {ID: second}}, nil
}
func TestDefaultHouseRequiresActiveMembership(t *testing.T) {
	for _, tc := range []struct {
		name      string
		preferred *string
		members   []domain.Membership
		want      string
	}{
		{"new user", nil, nil, ""},
		{"inactive only", ptr(first), []domain.Membership{{HouseID: first, Status: domain.MembershipStatusInactive}}, ""},
		{"revoked default falls back", ptr(first), []domain.Membership{{HouseID: first, Status: domain.MembershipStatusInactive}, {HouseID: second, Status: domain.MembershipStatusActive}}, second},
		{"valid preference", ptr(second), []domain.Membership{{HouseID: first, Status: domain.MembershipStatusActive}, {HouseID: second, Status: domain.MembershipStatusActive}}, second},
		{"foreign preference", ptr(second), []domain.Membership{{HouseID: first, Status: domain.MembershipStatusActive}}, first},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := New(contextRepo{user: domain.User{ID: uid, DefaultHouseID: tc.preferred}, members: tc.members}).GetUserContext(context.Background(), uid)
			if err != nil || v.DefaultHouseID != tc.want {
				t.Fatalf("context=%+v err=%v", v, err)
			}
		})
	}
}
func ptr(v string) *string { return &v }
func TestInvalidUserNeverReachesDatabase(t *testing.T) {
	for _, id := range []string{"", "00000000-0000-0000-0000-000000000000", "11111111111141118111111111111111", "garbage"} {
		_, err := New(contextRepo{}).GetUserContext(context.Background(), id)
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("id %q: %v", id, err)
		}
	}
}
