package domain

import (
	"encoding/json"
	"time"
)

type StatementDraft struct {
	ID, IssueID                string
	Version                    int
	Status, Body, ChairmanNote string
	SourceSnapshot             json.RawMessage
	CreatedBy                  string
	CreatedAt, UpdatedAt       time.Time
}
