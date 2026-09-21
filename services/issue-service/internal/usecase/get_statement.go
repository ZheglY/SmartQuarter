package usecase

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
)

func (s *Service) GetStatement(ctx context.Context, a domain.Actor, house, id string) (domain.StatementDraft, error) {
	if err := validate(ctx, a, house); err != nil {
		return domain.StatementDraft{}, err
	}
	if err := a.RequireManager(); err != nil {
		return domain.StatementDraft{}, err
	}
	if _, err := lookupIssue(ctx, s.store.Read(), a, id, false); err != nil {
		return domain.StatementDraft{}, err
	}
	return s.store.Read().LatestStatement(ctx, id)
}
