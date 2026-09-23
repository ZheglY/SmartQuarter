package identity

import (
	"context"
	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/identity/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net"
	"testing"
	"time"
)

const userID = "11111111-1111-4111-8111-111111111111"
const houseID = "22222222-2222-4222-8222-222222222222"

type identityServer struct {
	pb.UnimplementedIdentityServiceServer
}

func (identityServer) UpsertMaxUser(ctx context.Context, r *pb.UpsertMaxUserRequest) (*pb.User, error) {
	if r.MaxUserId == 2 {
		<-ctx.Done()
		return nil, status.Error(codes.DeadlineExceeded, "timeout")
	}
	return &pb.User{Id: userID, MaxUserId: r.MaxUserId, DisplayName: r.DisplayName, Username: r.Username, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}, nil
}
func (identityServer) GetMembership(context.Context, *pb.GetMembershipRequest) (*pb.Membership, error) {
	return nil, status.Error(codes.NotFound, "absent")
}
func (identityServer) GetUserContext(context.Context, *pb.GetUserContextRequest) (*pb.UserContext, error) {
	return &pb.UserContext{User: &pb.User{Id: userID, MaxUserId: 1, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}}, nil
}
func (identityServer) ListMemberships(context.Context, *pb.ListMembershipsRequest) (*pb.ListMembershipsResponse, error) {
	return &pb.ListMembershipsResponse{Items: []*pb.Membership{{Id: houseID, UserId: userID, HouseId: houseID, Role: pb.Role_ROLE_CHAIRMAN, Status: pb.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE}}}, nil
}
func TestGRPCAdapter(t *testing.T) {
	l := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	pb.RegisterIdentityServiceServer(s, identityServer{})
	healthServer := health.NewServer()
	healthServer.SetServingStatus("smartquarter.identity.v1.IdentityService", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthServer)
	go s.Serve(l)
	defer s.Stop()
	conn, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return l.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := NewGRPC(conn, 5*time.Second)
	ctx := context.Background()
	u, err := c.UpsertMaxUser(ctx, MaxUser{ID: 1, FirstName: "Test", LastName: "Resident", Username: "user"})
	if err != nil || u.MaxUserID != "1" || u.DisplayName != "Test Resident" {
		t.Fatalf("%+v %v", u, err)
	}
	v, err := c.GetUserContext(ctx, userID)
	if err != nil || v.Houses == nil || v.Memberships == nil || v.DefaultHouseID != "" {
		t.Fatalf("%+v %v", v, err)
	}
	ms, err := c.ListMemberships(ctx, userID)
	if err != nil || len(ms) != 1 || !ms[0].Authorizes(userID, houseID) || ms[0].Role != "CHAIRMAN" {
		t.Fatalf("%+v %v", ms, err)
	}
	_, err = c.GetMembership(ctx, userID, houseID)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatal(err)
	}
	if err = c.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	healthServer.SetServingStatus("smartquarter.identity.v1.IdentityService", healthpb.HealthCheckResponse_NOT_SERVING)
	if status.Code(c.Ready(ctx)) != codes.Unavailable {
		t.Fatal("unhealthy identity accepted")
	}
	timeoutClient := NewGRPC(conn, 100*time.Millisecond)
	_, err = timeoutClient.UpsertMaxUser(ctx, MaxUser{ID: 2})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatal(err)
	}
}
func TestUnknownRoleFailsClosed(t *testing.T) {
	_, err := membershipDTO(&pb.Membership{Id: houseID, UserId: userID, HouseId: houseID, Role: 999, Status: pb.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE})
	if err == nil {
		t.Fatal("unknown role accepted")
	}
}
