package domain

import "time"

type Category string

const (
	Safety         Category = "SAFETY"
	Cleanliness    Category = "CLEANLINESS"
	Utilities      Category = "UTILITIES"
	Infrastructure Category = "INFRASTRUCTURE"
	Other          Category = "OTHER"
)

func (c Category) Valid() bool {
	switch c {
	case Safety, Cleanliness, Utilities, Infrastructure, Other:
		return true
	}
	return false
}

type Issue struct {
	ID, HouseID, CreatedBy, HouseAddressSnapshot string
	Category                                     Category
	Description, LocationText                    string
	Status                                       Status
	ConfirmationsCount                           int
	CreatedAt, UpdatedAt                         time.Time
	ResolvedAt                                   *time.Time
}
type IssueDetails struct {
	Issue           Issue
	Attachments     []Attachment
	ConfirmedByMe   bool
	Timeline        []TimelineEvent
	LatestStatement *StatementDraft
}
type ListFilter struct {
	HouseID       string
	Statuses      []Status
	Limit         int
	BeforeTime    *time.Time
	BeforeID      string
	ChairmanQueue bool
}
