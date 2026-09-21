//go:build integration

// Package testdata provides a TEST-ONLY Identity protocol. It is NOT the agreed
// smartquarter.identity.v1 contract and cannot be linked into the production app.
package testdata

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
)

const House = "22222222-2222-4222-8222-222222222222"
const ForeignHouse = "99999999-9999-4999-8999-999999999999"

var Users = map[int64]string{101: "11111111-1111-4111-8111-111111111111", 102: "33333333-3333-4333-8333-333333333333", 103: "77777777-7777-4777-8777-777777777777", 104: "88888888-8888-4888-8888-888888888888"}

type service interface {
	Call(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)
}
type server struct{}
type command struct {
	Method      string
	Max         identity.MaxUser
	User, House string
}

func (server) Call(ctx context.Context, b *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	var c command
	if json.Unmarshal(b.Value, &c) != nil {
		return nil, status.Error(codes.InvalidArgument, "test command")
	}
	if c.Method == "Ready" {
		return wrapperspb.Bytes([]byte(`{}`)), nil
	}
	if c.Method == "UpsertMaxUser" {
		c.User = Users[c.Max.ID]
	}
	maxID := int64(0)
	for id, u := range Users {
		if u == c.User {
			maxID = id
		}
	}
	if maxID == 0 {
		return nil, status.Error(codes.NotFound, "unknown test user")
	}
	house := House
	role := "RESIDENT"
	if maxID == 103 {
		role = "CHAIRMAN"
	}
	if maxID == 104 {
		house = ForeignHouse
	}
	now := time.Now().UTC()
	user := identity.User{ID: c.User, MaxUserID: strconv.FormatInt(maxID, 10), DisplayName: "Test", CreatedAt: now, UpdatedAt: now}
	m := identity.Membership{ID: c.User, UserID: c.User, HouseID: house, Role: role, Status: "ACTIVE"}
	var result any
	switch c.Method {
	case "UpsertMaxUser":
		result = user
	case "GetUserContext":
		result = identity.UserContext{User: user, Houses: []identity.House{{ID: house, Name: "Test house", Address: "Test address 1", City: "Test"}}, Memberships: []identity.Membership{m}, DefaultHouseID: house}
	case "GetMembership":
		if c.House != house {
			return nil, status.Error(codes.PermissionDenied, "not a member")
		}
		result = m
	case "ListMemberships":
		result = []identity.Membership{m}
	default:
		return nil, status.Error(codes.Unimplemented, "test method")
	}
	raw, _ := json.Marshal(result)
	return wrapperspb.Bytes(raw), nil
}

type Client struct{ conn *grpc.ClientConn }

func Start(t *testing.T) *Client {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	s := grpc.NewServer()
	s.RegisterService(&grpc.ServiceDesc{ServiceName: "gateway.test.Identity", HandlerType: (*service)(nil), Methods: []grpc.MethodDesc{{MethodName: "Call", Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := new(wrapperspb.BytesValue)
		if e := dec(req); e != nil {
			return nil, e
		}
		return srv.(service).Call(ctx, req)
	}}}}, server{})
	go s.Serve(l)
	conn, e := grpc.NewClient(l.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { conn.Close(); s.Stop(); l.Close() })
	return &Client{conn}
}
func (c *Client) call(ctx context.Context, req command, out any) error {
	b, _ := json.Marshal(req)
	res := new(wrapperspb.BytesValue)
	if e := c.conn.Invoke(ctx, "/gateway.test.Identity/Call", wrapperspb.Bytes(b), res); e != nil {
		return e
	}
	return json.Unmarshal(res.Value, out)
}
func (c *Client) Ready(ctx context.Context) error {
	var v any
	return c.call(ctx, command{Method: "Ready"}, &v)
}
func (c *Client) UpsertMaxUser(ctx context.Context, u identity.MaxUser) (v identity.User, e error) {
	e = c.call(ctx, command{Method: "UpsertMaxUser", Max: u}, &v)
	return
}
func (c *Client) GetUserContext(ctx context.Context, id string) (v identity.UserContext, e error) {
	e = c.call(ctx, command{Method: "GetUserContext", User: id}, &v)
	return
}
func (c *Client) GetMembership(ctx context.Context, id, house string) (v identity.Membership, e error) {
	e = c.call(ctx, command{Method: "GetMembership", User: id, House: house}, &v)
	return
}
func (c *Client) ListMemberships(ctx context.Context, id string) (v []identity.Membership, e error) {
	e = c.call(ctx, command{Method: "ListMemberships", User: id}, &v)
	return
}
