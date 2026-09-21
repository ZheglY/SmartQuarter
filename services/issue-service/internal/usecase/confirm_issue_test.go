package usecase

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/google/uuid"
	"testing"
)

func TestConfirmIssue(t *testing.T) {
	a := actor()
	q := &issueQueries{issue: domain.Issue{ID: uuid.NewString(), HouseID: a.HouseID, CreatedBy: uuid.NewString(), Status: domain.Detected}}
	svc := New(&fakeStore{Queries: q}, nil, Options{})
	count, err := svc.ConfirmIssue(context.Background(), a, a.HouseID, q.issue.ID)
	if err != nil || count != 1 || len(q.outbox) != 1 || len(q.timeline) != 1 {
		t.Fatal(err)
	}
	q.duplicate = true
	if _, err = svc.ConfirmIssue(context.Background(), a, a.HouseID, q.issue.ID); !errors.Is(err, domain.ErrExists) {
		t.Fatal("duplicate allowed")
	}
	q.duplicate = false
	q.issue.CreatedBy = a.UserID
	if _, err = svc.ConfirmIssue(context.Background(), a, a.HouseID, q.issue.ID); !errors.Is(err, domain.ErrPrecondition) {
		t.Fatal("own confirmation allowed")
	}
	q.issue.HouseID = uuid.NewString()
	if _, err = svc.ConfirmIssue(context.Background(), a, a.HouseID, q.issue.ID); !errors.Is(err, domain.ErrPermission) {
		t.Fatal("wrong house accepted")
	}
	q.err = domain.ErrUnavailable
	if _, err = svc.ConfirmIssue(context.Background(), a, a.HouseID, q.issue.ID); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatal("database error lost")
	}
}
