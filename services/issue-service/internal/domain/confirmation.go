package domain

import "time"

type Confirmation struct {
	IssueID, UserID string
	CreatedAt       time.Time
}
