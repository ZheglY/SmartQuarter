package usecase

import (
	"context"
	"errors"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/statement"
	"github.com/google/uuid"
)

func (s *Service) GenerateStatement(ctx context.Context, a domain.Actor, house, id, note string) (result domain.StatementDraft, err error) {
	if err = validate(ctx, a, house); err != nil {
		return result, err
	}
	if err = a.RequireManager(); err != nil {
		return result, err
	}
	if !validText(note, 4000, false) {
		return result, domain.ErrInvalid
	}
	err = s.store.WithinTx(ctx, func(q repository.Queries) error {
		issue, e := lookupIssue(ctx, q, a, id, true)
		if e != nil {
			return e
		}
		latest, e := q.LatestStatement(ctx, id)
		if e != nil && !errors.Is(e, domain.ErrNotFound) {
			return e
		}
		if latest.Version >= 2147483647 {
			return domain.ErrLimit
		}
		body, snapshot, e := statement.Generate(issue, note)
		if e != nil {
			return e
		}
		now := s.now()
		result = domain.StatementDraft{ID: uuid.NewString(), IssueID: id, Version: latest.Version + 1, Status: "DRAFT", Body: body, ChairmanNote: note, SourceSnapshot: snapshot, CreatedBy: a.UserID, CreatedAt: now, UpdatedAt: now}
		if e = q.InsertStatement(ctx, result); e != nil {
			return e
		}
		return emit(ctx, q, issue, a, "statement.generated", now, map[string]interface{}{"statement_id": result.ID, "version": result.Version})
	})
	return result, err
}
