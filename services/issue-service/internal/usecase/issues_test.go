package usecase

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/google/uuid"
	"testing"
	"time"
)

type issueQueries struct {
	uploadQueries
	issue     domain.Issue
	items     []domain.Issue
	latest    domain.StatementDraft
	duplicate bool
	timeline  []domain.TimelineEvent
	outbox    []domain.Event
}

func (q *issueQueries) Issue(context.Context, string, bool) (domain.Issue, error) {
	return q.issue, q.err
}
func (q *issueQueries) InsertIssue(_ context.Context, i domain.Issue) error {
	q.issue = i
	return q.err
}
func (q *issueQueries) ChangeStatus(_ context.Context, i domain.Issue) error {
	q.issue = i
	return q.err
}
func (q *issueQueries) ListIssues(context.Context, domain.ListFilter) ([]domain.Issue, error) {
	return q.items, q.err
}
func (q *issueQueries) Attachments(context.Context, string) ([]domain.Attachment, error) {
	return []domain.Attachment{q.attachment}, q.err
}
func (q *issueQueries) Timeline(context.Context, string) ([]domain.TimelineEvent, error) {
	return q.timeline, q.err
}
func (q *issueQueries) Confirmed(context.Context, string, string) (bool, error) {
	return q.duplicate, q.err
}
func (q *issueQueries) AddConfirmation(context.Context, domain.Confirmation) (int, error) {
	if q.duplicate {
		return 0, domain.ErrExists
	}
	q.issue.ConfirmationsCount++
	return q.issue.ConfirmationsCount, q.err
}
func (q *issueQueries) LatestStatement(context.Context, string) (domain.StatementDraft, error) {
	if q.err != nil {
		return q.latest, q.err
	}
	if q.latest.ID == "" {
		return q.latest, domain.ErrNotFound
	}
	return q.latest, nil
}
func (q *issueQueries) InsertStatement(_ context.Context, s domain.StatementDraft) error {
	q.latest = s
	return q.err
}
func (q *issueQueries) AddTimeline(_ context.Context, e domain.TimelineEvent) error {
	q.timeline = append(q.timeline, e)
	return q.err
}
func (q *issueQueries) AddOutbox(_ context.Context, e domain.Event) error {
	q.outbox = append(q.outbox, e)
	return q.err
}
func TestCreateAndReadIssue(t *testing.T) {
	a := actor()
	id := uuid.NewString()
	ready := domain.Attachment{ID: id, HouseID: a.HouseID, UploadedBy: a.UserID, Status: domain.Ready}
	q := &issueQueries{uploadQueries: uploadQueries{attachment: ready}}
	svc := New(&fakeStore{Queries: q}, nil, Options{})
	input := CreateIssueInput{HouseID: a.HouseID, HouseAddressSnapshot: "Дом 1", Category: domain.Safety, Description: "Открытый люк", AttachmentIDs: []string{id}}
	issue, err := svc.CreateIssue(context.Background(), a, input)
	if err != nil || issue.Status != domain.Detected || q.attachment.IssueID != issue.ID || len(q.timeline) != 1 || len(q.outbox) != 1 {
		t.Fatal("transaction not assembled", err)
	}
	detail, err := svc.GetIssue(context.Background(), a, a.HouseID, issue.ID)
	if err != nil || len(detail.Attachments) != 1 || len(detail.Timeline) != 1 {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		modify func()
		want   error
	}{
		{"not READY", func() { q.attachment.Status = domain.Uploading }, domain.ErrPrecondition},
		{"already attached", func() { q.attachment.IssueID = uuid.NewString() }, domain.ErrPrecondition},
		{"wrong owner", func() { q.attachment.UploadedBy = uuid.NewString() }, domain.ErrPermission},
		{"wrong house", func() { q.attachment.HouseID = uuid.NewString() }, domain.ErrPermission},
		{"database down", func() { q.err = domain.ErrUnavailable }, domain.ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			q.attachment = ready
			q.err = nil
			test.modify()
			_, err := svc.CreateIssue(context.Background(), a, input)
			if !errors.Is(err, test.want) {
				t.Fatal(err)
			}
		})
	}
	q.err = nil
	q.issue.HouseID = uuid.NewString()
	if _, err = svc.GetIssue(context.Background(), a, a.HouseID, issue.ID); !errors.Is(err, domain.ErrPermission) {
		t.Fatal("foreign issue visible")
	}
}
func TestCursorValidation(t *testing.T) {
	a := actor()
	q := &issueQueries{items: []domain.Issue{{ID: uuid.NewString(), CreatedAt: time.Now()}, {ID: uuid.NewString(), CreatedAt: time.Now()}}}
	svc := New(&fakeStore{Queries: q}, nil, Options{MaxPageSize: 10})
	page, err := svc.ListIssues(context.Background(), a, ListIssuesInput{HouseID: a.HouseID, PageSize: 1})
	if err != nil || len(page.Items) != 1 || page.NextPageToken == "" {
		t.Fatal(err)
	}
	if _, err = svc.ListIssues(context.Background(), a, ListIssuesInput{HouseID: a.HouseID, PageSize: 1, PageToken: page.NextPageToken, Statuses: []domain.Status{domain.Resolved}}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("cross-filter cursor accepted")
	}
	if _, err = svc.ListIssues(context.Background(), a, ListIssuesInput{HouseID: a.HouseID, ChairmanQueue: true}); !errors.Is(err, domain.ErrPermission) {
		t.Fatal("resident queue accepted")
	}
	if _, err = svc.ListIssues(context.Background(), a, ListIssuesInput{HouseID: a.HouseID, PageToken: "not-json"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("malformed cursor accepted")
	}
}
