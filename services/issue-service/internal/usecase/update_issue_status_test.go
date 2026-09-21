package usecase

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/google/uuid"
	"testing"
)

func TestUpdateStatus(t *testing.T) {
	a := actor()
	q := &issueQueries{issue: domain.Issue{ID: uuid.NewString(), HouseID: a.HouseID, Status: domain.Detected}}
	svc := New(&fakeStore{Queries: q}, nil, Options{})
	if _, err := svc.UpdateIssueStatus(context.Background(), a, a.HouseID, q.issue.ID, domain.Resolved); !errors.Is(err, domain.ErrPermission) {
		t.Fatal("resident accepted")
	}
	a.Role = domain.Chairman
	if _, err := svc.UpdateIssueStatus(context.Background(), a, a.HouseID, q.issue.ID, domain.MarkedSent); !errors.Is(err, domain.ErrPrecondition) {
		t.Fatal("illegal transition accepted")
	}
	result, err := svc.UpdateIssueStatus(context.Background(), a, a.HouseID, q.issue.ID, domain.Resolved)
	if err != nil || result.ResolvedAt == nil || len(q.outbox) != 1 {
		t.Fatal("early close failed", err)
	}
	a.Role = domain.Admin
	result, err = svc.UpdateIssueStatus(context.Background(), a, a.HouseID, q.issue.ID, domain.Confirming)
	if err != nil || result.ResolvedAt != nil {
		t.Fatal("reopen failed", err)
	}
}
