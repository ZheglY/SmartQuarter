package domain

import "time"

// Role определяет роль пользователя в конкретном доме
type Role string

const (
	RoleUnspecified Role = "ROLE_UNSPECIFIED"
	RoleResident    Role = "RESIDENT"
	RoleChairman    Role = "CHAIRMAN"
	RoleAdmin       Role = "ADMIN"
)

// MembershipStatus определяет активность членства жителя в доме
type MembershipStatus string

const (
	MembershipStatusUnspecified MembershipStatus = "MEMBERSHIP_STATUS_UNSPECIFIED"
	MembershipStatusActive      MembershipStatus = "ACTIVE"
	MembershipStatusInactive    MembershipStatus = "INACTIVE"
)

// User представляет пользователя MAX
type User struct {
	ID             string
	MaxUserID      int64
	DisplayName    string
	Username       string
	DefaultHouseID *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// House представляет дом
type House struct {
	ID        string
	Name      string
	Address   string
	City      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Membership связывает пользователя с домо с определенной ролью и статусом
type Membership struct {
	ID        string
	UserID    string
	HouseID   string
	Role      Role
	Status    MembershipStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserContext агрегирует данные пользователя, его дома и членства
type UserContext struct {
	User           User
	Houses         []House
	Memberships    []Membership
	DefaultHouseID string
}
