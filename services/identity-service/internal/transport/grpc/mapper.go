package grpc

import (
	"errors"
	"gogle.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/domain"
	// Поправить
	identity v1 "github.com/ZheglY/SmartQuarter/services/identity-service/internal/gen/smartquarter/identity/v1"
)

// toProtouser конвертирует доменого пользователя в Protobuf User
func toProtoUser(u domain.User) *identityv1.User {
	return &identityv1.User{
		Id: u.ID,
		MaxUserId: u.MaxUserID,
		DisplayName: u.DisplayName,
		Username: u.Username,
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}

// toProtoHouse конвертирует дом в Protobuf House
func toProtoHouse(h domain.House) *identityv1.House {
	return &identityv1.House{
		Id:        h.ID,
		Name:      h.Name,
		Address:   h.Address,
		City:      h.City,
		CreatedAt: timestamppb.New(h.CreatedAt),
		UpdatedAt: timestamppb.New(h.UpdatedAt),
	}
}

// toProtomembership конвертирует членство в доме в Protobuf Membership
func toProtoMembership(m domain.Membership) *identityv1.Membership {
	return &identityv1.Membership{
		Id:        m.ID,
		UserId:    m.UserID,
		HouseId:   m.HouseID,
		Role:      toProtoRole(m.Role),
		Status:    toProtoMembershipStatus(m.Status),
		CreatedAt: timestamppb.New(m.CreatedAt),
	}
}

// toProtoRole переводит строковую роль в Protobuf Role enum
func toProtoRole(r domain.Role) identityv1.Role {
	switch r {
	case domain.RoleResident:
		return identityv1.Role_ROLE_RESIDENT
	case domain.RoleChairman: 
		return identityv1.Role_ROLE_CHAIRMAN
	case domain.RoleAdmin:
		return identityv1.Role_ROLE_ADMIN
	default:
		return identityv1.Role_ROLE_UNSPECIFIED
	}
}

// toProtoMembershipStatus переводит строковый статус членства в Protobuf MembershipStatus enum
func toProtoMembershipStatus(s domain.MembershipStatus) identityv1.MembershipStatus {
	switch s {
	case domain.MembershipStatusActive:
		return identityv1.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE
	case domain.MembershipStatusInactive:
		return identityv1.MembershipStatus_MEMBERSHIP_STATUS_INACTIVE
	default:
		return identityv1.MembershipStatus_MEMBERSHIP_STATU_UNSPECIFIED
	}
}

// mapError транслирует ошибки домена в gRPC статус-коды
func mapError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Errorf(codes.InvalidArgument, "%s", err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, "user not found")
	case errors.Is(err, domain.ErrHouseNotFound):
		return status.Error(codes.NotFound, "house not found")
	case errors.Is(err, domain.ErrMembershipNotFound):
		return status.Error(codes.NotFound, "membership not found")
	case errors.Is(err, domain.ErrMembershipInactive):
		return status.Error(codes.PermissionDenied, "membership is inactive")
	default:
		return status.Errorf(codes.Internal, "internal error: %s", err.Error())
	}
}