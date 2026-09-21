package domain

import "github.com/google/uuid"

type Role string

const (
	Resident Role = "RESIDENT"
	Chairman Role = "CHAIRMAN"
	Admin    Role = "ADMIN"
)

type Actor struct {
	UserID  string
	HouseID string
	Role    Role
}

func ValidID(id string) bool {
	v, err := uuid.Parse(id)
	return err == nil && v != uuid.Nil && v.String() == id
}
func (a Actor) Validate() error {
	if !ValidID(a.UserID) || !ValidID(a.HouseID) {
		return ErrUnauthenticated
	}
	switch a.Role {
	case Resident, Chairman, Admin:
		return nil
	}
	return ErrPermission
}
func (a Actor) InHouse(house string) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if !ValidID(house) {
		return Fail(ErrInvalid, "invalid house_id")
	}
	if house != a.HouseID {
		return ErrPermission
	}
	return nil
}
func (a Actor) CanManage() bool { return a.Role == Chairman || a.Role == Admin }
func (a Actor) RequireManager() error {
	if err := a.Validate(); err != nil {
		return err
	}
	if !a.CanManage() {
		return ErrPermission
	}
	return nil
}
