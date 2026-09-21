package rpc

import (
	"context"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	cpb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/community/v1"
	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/observability"
)

type contractServer struct {
	pb.UnimplementedIssueServiceServer
	t *testing.T
}

func (s contractServer) ListIssues(ctx context.Context, r *pb.ListIssuesRequest) (*pb.ListIssuesResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	for k, want := range map[string]string{"x-request-id": "request", "x-actor-user-id": "user", "x-house-id": "house", "x-actor-role": "RESIDENT"} {
		if len(md.Get(k)) != 1 || md.Get(k)[0] != want {
			s.t.Errorf("metadata %s", k)
		}
	}
	if _, ok := ctx.Deadline(); !ok {
		s.t.Error("deadline missing")
	}
	return &pb.ListIssuesResponse{}, nil
}

type communityServer struct {
	cpb.UnimplementedCommunityServiceServer
}

func (communityServer) CreateAnnouncement(ctx context.Context, r *cpb.CreateAnnouncementRequest) (*cpb.Announcement, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if md.Get("x-actor-role")[0] != "CHAIRMAN" {
		return nil, status.Error(codes.PermissionDenied, "manager required")
	}
	return &cpb.Announcement{HouseId: r.HouseId, Title: r.Title, Body: r.Body, Status: 1}, nil
}
func TestClientContractAndMetadata(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	s := grpc.NewServer()
	pb.RegisterIssueServiceServer(s, contractServer{t: t})
	cpb.RegisterCommunityServiceServer(s, communityServer{})
	go s.Serve(l)
	defer s.Stop()
	conn, e := Dial(l.Addr().String(), time.Second, zap.NewNop(), observability.New())
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	ctx := Actor(context.Background(), "request", "user", "house", "RESIDENT")
	if _, e = pb.NewIssueServiceClient(conn).ListIssues(ctx, &pb.ListIssuesRequest{HouseId: "house"}); e != nil {
		t.Fatal(e)
	}
	v, e := cpb.NewCommunityServiceClient(conn).CreateAnnouncement(Actor(context.Background(), "request", "user", "house", "CHAIRMAN"), &cpb.CreateAnnouncementRequest{HouseId: "house", Title: "title", Body: "body"})
	if e != nil || v.Title != "title" || v.Status != 1 {
		t.Fatal(v, e)
	}
	expired, stop := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer stop()
	_, e = pb.NewIssueServiceClient(conn).ListIssues(expired, &pb.ListIssuesRequest{})
	if status.Code(e) != codes.DeadlineExceeded {
		t.Fatal(e)
	}
}
