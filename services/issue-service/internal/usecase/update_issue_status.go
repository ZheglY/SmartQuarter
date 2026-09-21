package usecase

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
)

func (s *Service) UpdateIssueStatus(ctx context.Context, a domain.Actor, house, id string, target domain.Status) (result domain.Issue, err error) {
	if err = validate(ctx, a, house); err != nil {
		return result, err
	}
	if err = a.RequireManager(); err != nil {
		return result, err
	}
	if !target.Valid() {
		return result, domain.ErrInvalid
	}
	err = s.store.WithinTx(ctx, func(q repository.Queries) error {
		issue, e := lookupIssue(ctx, q, a, id, true)
		if e != nil {
			return e
		}
		previous := issue.Status
		if e = domain.Transition(previous, target, a.Role); e != nil {
			return e
		}
		now := s.now()
		issue.Status = target
		issue.UpdatedAt = now
		issue.ResolvedAt = nil
		if target == domain.Resolved {
			issue.ResolvedAt = &now
		}
		if e = q.ChangeStatus(ctx, issue); e != nil {
			return e
		}
		if e = emit(ctx, q, issue, a, "issue.status_changed", now, map[string]interface{}{"from": previous, "to": target}); e != nil {
			return e
		}
		result = issue
		return nil
	})
	return result, err
}
