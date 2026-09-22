package identity

import (
	"context"
	"strconv"
	"strings"
	"time"

	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/identity/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type GRPCClient struct {
	client  pb.IdentityServiceClient
	health  healthpb.HealthClient
	timeout time.Duration
}

func NewGRPC(conn grpc.ClientConnInterface, timeout time.Duration) *GRPCClient {
	return &GRPCClient{pb.NewIdentityServiceClient(conn), healthpb.NewHealthClient(conn), timeout}
}

func (c *GRPCClient) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	v, err := c.health.Check(ctx, &healthpb.HealthCheckRequest{Service: "smartquarter.identity.v1.IdentityService"})
	if err != nil {
		return err
	}
	if v.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return status.Error(codes.Unavailable, "identity not ready")
	}
	return nil
}
func userDTO(v *pb.User) (User, error) {
	if v == nil || !ValidID(v.Id) || v.MaxUserId <= 0 || v.CreatedAt == nil || v.UpdatedAt == nil || v.CreatedAt.CheckValid() != nil || v.UpdatedAt.CheckValid() != nil {
		return User{}, status.Error(codes.Internal, "invalid identity user")
	}
	return User{ID: v.Id, MaxUserID: strconv.FormatInt(v.MaxUserId, 10), DisplayName: v.DisplayName, Username: v.Username, CreatedAt: v.CreatedAt.AsTime(), UpdatedAt: v.UpdatedAt.AsTime()}, nil
}
func membershipDTO(v *pb.Membership) (Membership, error) {
	if v == nil || !ValidID(v.Id) || !ValidID(v.UserId) || !ValidID(v.HouseId) {
		return Membership{}, status.Error(codes.Internal, "invalid identity membership")
	}
	role := map[pb.Role]string{pb.Role_ROLE_RESIDENT: "RESIDENT", pb.Role_ROLE_CHAIRMAN: "CHAIRMAN", pb.Role_ROLE_ADMIN: "ADMIN"}[v.Role]
	state := map[pb.MembershipStatus]string{pb.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE: "ACTIVE", pb.MembershipStatus_MEMBERSHIP_STATUS_INACTIVE: "INACTIVE"}[v.Status]
	if role == "" || state == "" {
		return Membership{}, status.Error(codes.Internal, "unknown identity enum")
	}
	return Membership{ID: v.Id, UserID: v.UserId, HouseID: v.HouseId, Role: role, Status: state}, nil
}
func (c *GRPCClient) UpsertMaxUser(ctx context.Context, m MaxUser) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	v, err := c.client.UpsertMaxUser(ctx, &pb.UpsertMaxUserRequest{MaxUserId: m.ID, DisplayName: strings.TrimSpace(m.FirstName + " " + m.LastName), Username: m.Username})
	if err != nil {
		return User{}, err
	}
	return userDTO(v)
}
func (c *GRPCClient) GetUserContext(ctx context.Context, id string) (UserContext, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	v, err := c.client.GetUserContext(ctx, &pb.GetUserContextRequest{UserId: id})
	if err != nil {
		return UserContext{}, err
	}
	u, err := userDTO(v.GetUser())
	if err != nil {
		return UserContext{}, err
	}
	result := UserContext{User: u, Houses: []House{}, Memberships: []Membership{}, DefaultHouseID: v.DefaultHouseId}
	for _, h := range v.Houses {
		if h == nil || !ValidID(h.Id) {
			return UserContext{}, status.Error(codes.Internal, "invalid identity house")
		}
		result.Houses = append(result.Houses, House{ID: h.Id, Name: h.Name, Address: h.Address, City: h.City})
	}
	for _, m := range v.Memberships {
		d, e := membershipDTO(m)
		if e != nil {
			return UserContext{}, e
		}
		if d.UserID != id {
			return UserContext{}, status.Error(codes.Internal, "identity user mismatch")
		}
		result.Memberships = append(result.Memberships, d)
	}
	return result, nil
}
func (c *GRPCClient) GetMembership(ctx context.Context, user, house string) (Membership, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	v, err := c.client.GetMembership(ctx, &pb.GetMembershipRequest{UserId: user, HouseId: house})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return Membership{}, status.Error(codes.PermissionDenied, "active membership required")
		}
		return Membership{}, err
	}
	return membershipDTO(v)
}
func (c *GRPCClient) ListMemberships(ctx context.Context, user string) ([]Membership, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	v, err := c.client.ListMemberships(ctx, &pb.ListMembershipsRequest{UserId: user})
	if err != nil {
		return nil, err
	}
	result := []Membership{}
	for _, m := range v.GetItems() {
		d, e := membershipDTO(m)
		if e != nil {
			return nil, e
		}
		if d.UserID != user {
			return nil, status.Error(codes.Internal, "identity user mismatch")
		}
		result = append(result, d)
	}
	return result, nil
}
