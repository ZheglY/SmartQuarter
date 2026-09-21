package repository

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
)

type Queries interface {
	Issue(context.Context, string, bool) (domain.Issue, error)
	Attachment(context.Context, string, bool) (domain.Attachment, error)
	InsertAttachment(context.Context, domain.Attachment) error
	UpdateAttachment(context.Context, domain.Attachment) error
	InsertIssue(context.Context, domain.Issue) error
	ChangeStatus(context.Context, domain.Issue) error
	ListIssues(context.Context, domain.ListFilter) ([]domain.Issue, error)
	Attachments(context.Context, string) ([]domain.Attachment, error)
	Timeline(context.Context, string) ([]domain.TimelineEvent, error)
	Confirmed(context.Context, string, string) (bool, error)
	AddConfirmation(context.Context, domain.Confirmation) (int, error)
	LatestStatement(context.Context, string) (domain.StatementDraft, error)
	InsertStatement(context.Context, domain.StatementDraft) error
	AddTimeline(context.Context, domain.TimelineEvent) error
	AddOutbox(context.Context, domain.Event) error
}
type Store interface {
	Read() Queries
	WithinTx(context.Context, func(Queries) error) error
}
