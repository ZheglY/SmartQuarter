package usecase

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
)

func (s *Service) ConfirmIssue(ctx context.Context, a domain.Actor, house, id string) (count int, err error) {
	if err = validate(ctx, a, house); err != nil {
		return 0, err
	}
	err = s.store.WithinTx(ctx, func(q repository.Queries) error {
		issue, e := lookupIssue(ctx, q, a, id, true)
		if e != nil {
			return e
		}
		if issue.CreatedBy == a.UserID {
			return domain.Fail(domain.ErrPrecondition, "cannot confirm own issue")
		}
		now := s.now()
		count, e = q.AddConfirmation(ctx, domain.Confirmation{IssueID: id, UserID: a.UserID, CreatedAt: now})
		if e != nil {
			return e
		}
		return emit(ctx, q, issue, a, "issue.confirmed", now, map[string]interface{}{"confirmed_by": a.UserID, "confirmations_count": count})
	})
	return count, err
}
