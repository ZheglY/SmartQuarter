package usecase

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
)

func (s *Service) GetIssue(ctx context.Context, a domain.Actor, house, id string) (result domain.IssueDetails, err error) {
	if err = validate(ctx, a, house); err != nil {
		return result, err
	}
	err = s.store.WithinTx(ctx, func(q repository.Queries) error {
		// All mutators lock the issue, so details and counters form a consistent snapshot.
		issue, e := lookupIssue(ctx, q, a, id, true)
		if e != nil {
			return e
		}
		result.Issue = issue
		result.Attachments, e = q.Attachments(ctx, id)
		if e != nil {
			return e
		}
		result.ConfirmedByMe, e = q.Confirmed(ctx, id, a.UserID)
		if e != nil {
			return e
		}
		result.Timeline, e = q.Timeline(ctx, id)
		if e != nil {
			return e
		}
		if a.CanManage() {
			statement, e := q.LatestStatement(ctx, id)
			if e != nil && !errors.Is(e, domain.ErrNotFound) {
				return e
			}
			if e == nil {
				result.LatestStatement = &statement
			}
		}
		return nil
	})
	return result, err
}
