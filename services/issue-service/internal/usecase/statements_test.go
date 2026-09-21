package usecase

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestStatements(t *testing.T) {
	a := actor()
	q := &issueQueries{issue: domain.Issue{ID: uuid.NewString(), HouseID: a.HouseID, CreatedAt: time.Now(), HouseAddressSnapshot: "Адрес", Description: "Описание", Category: domain.Safety}}
	svc := New(&fakeStore{Queries: q}, nil, Options{})
	if _, err := svc.GenerateStatement(context.Background(), a, a.HouseID, q.issue.ID, ""); !errors.Is(err, domain.ErrPermission) {
		t.Fatal("resident allowed")
	}
	a.Role = domain.Chairman
	if _, err := svc.GetStatement(context.Background(), a, a.HouseID, q.issue.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("missing draft not reported")
	}
	first, err := svc.GenerateStatement(context.Background(), a, a.HouseID, q.issue.ID, "Примечание")
	if err != nil || first.Version != 1 || len(first.SourceSnapshot) == 0 {
		t.Fatal(err)
	}
	second, err := svc.GenerateStatement(context.Background(), a, a.HouseID, q.issue.ID, "Примечание")
	if err != nil || second.Version != 2 || first.ID == second.ID || first.Body != second.Body || len(q.outbox) != 2 {
		t.Fatal("versioning failed", err)
	}
	latest, err := svc.GetStatement(context.Background(), a, a.HouseID, q.issue.ID)
	if err != nil || latest.ID != second.ID {
		t.Fatal(err)
	}
	q.err = domain.ErrUnavailable
	if _, err = svc.GenerateStatement(context.Background(), a, a.HouseID, q.issue.ID, ""); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatal("repository error swallowed")
	}
}
