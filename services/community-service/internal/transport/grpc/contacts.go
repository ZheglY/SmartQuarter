package grpc

import (
	"context"
	"encoding/json"
	"github.com/ZheglY/SmartQuarter/services/community-service/internal/contacts"
	pb "github.com/ZheglY/SmartQuarter/services/community-service/internal/gen/community/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (h *CommunityHandler) contact(ctx context.Context, op, house, id string, input *pb.ServiceContactInput, archived bool, out proto.Message) error {
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get("x-house-id")) != 1 || len(md.Get("x-actor-user-id")) != 1 || len(md.Get("x-actor-role")) != 1 || md.Get("x-house-id")[0] != house {
		return status.Error(codes.PermissionDenied, "trusted actor context required")
	}
	user, role, e := extractContext(ctx, house)
	if e != nil {
		return e
	}
	if role != "CHAIRMAN" && role != "ADMIN" && role != "RESIDENT" {
		return status.Error(codes.PermissionDenied, "active membership required")
	}
	if h.Contacts == nil {
		return status.Error(codes.Unavailable, "contacts unavailable")
	}
	var in *contacts.Input
	if input != nil {
		raw, e := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(input)
		if e != nil {
			return status.Error(codes.InvalidArgument, "invalid contact")
		}
		in = new(contacts.Input)
		if json.Unmarshal(raw, in) != nil {
			return status.Error(codes.InvalidArgument, "invalid contact")
		}
	}
	raw, e := h.Contacts.Execute(ctx, op, user, role, house, id, in, archived)
	if e != nil {
		return e
	}
	if (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, out) != nil {
		return status.Error(codes.Internal, "invalid contact response")
	}
	return nil
}
func (h *CommunityHandler) CreateServiceContact(ctx context.Context, r *pb.CreateServiceContactRequest) (*pb.ServiceContact, error) {
	out := new(pb.ServiceContact)
	e := h.contact(ctx, "Create", r.HouseId, "", r.Contact, false, out)
	return out, e
}
func (h *CommunityHandler) GetServiceContact(ctx context.Context, r *pb.GetServiceContactRequest) (*pb.ServiceContact, error) {
	out := new(pb.ServiceContact)
	e := h.contact(ctx, "Get", r.HouseId, r.Id, nil, false, out)
	return out, e
}
func (h *CommunityHandler) UpdateServiceContact(ctx context.Context, r *pb.UpdateServiceContactRequest) (*pb.ServiceContact, error) {
	out := new(pb.ServiceContact)
	e := h.contact(ctx, "Update", r.HouseId, r.Id, r.Contact, false, out)
	return out, e
}
func (h *CommunityHandler) ArchiveServiceContact(ctx context.Context, r *pb.ArchiveServiceContactRequest) (*pb.ServiceContact, error) {
	out := new(pb.ServiceContact)
	e := h.contact(ctx, "Archive", r.HouseId, r.Id, nil, false, out)
	return out, e
}
func (h *CommunityHandler) ListServiceContacts(ctx context.Context, r *pb.ListServiceContactsRequest) (*pb.ListServiceContactsResponse, error) {
	out := new(pb.ListServiceContactsResponse)
	e := h.contact(ctx, "List", r.HouseId, "", nil, r.IncludeArchived, out)
	return out, e
}
