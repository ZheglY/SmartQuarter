package usecase

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/google/uuid"
	"sort"
)

type CreateIssueInput struct {
	HouseID, HouseAddressSnapshot string
	Category                      domain.Category
	Description, LocationText     string
	AttachmentIDs                 []string
}

func (s *Service) CreateIssue(ctx context.Context, a domain.Actor, in CreateIssueInput) (result domain.Issue, err error) {
	if err = validate(ctx, a, in.HouseID); err != nil {
		return result, err
	}
	if !in.Category.Valid() || !validText(in.HouseAddressSnapshot, 1000, true) || !validText(in.Description, 8000, true) || !validText(in.LocationText, 1000, false) || len(in.AttachmentIDs) == 0 || len(in.AttachmentIDs) > 10 {
		return result, domain.ErrInvalid
	}
	ids := append([]string(nil), in.AttachmentIDs...)
	sort.Strings(ids)
	for i, id := range ids {
		if !domain.ValidID(id) || (i > 0 && ids[i-1] == id) {
			return result, domain.ErrInvalid
		}
	}
	now := s.now()
	result = domain.Issue{ID: uuid.NewString(), HouseID: a.HouseID, CreatedBy: a.UserID, HouseAddressSnapshot: in.HouseAddressSnapshot, Category: in.Category, Description: in.Description, LocationText: in.LocationText, Status: domain.Detected, CreatedAt: now, UpdatedAt: now}
	err = s.store.WithinTx(ctx, func(q repository.Queries) error {
		attachments := make([]domain.Attachment, 0, len(ids))
		// Stable lock order prevents deadlocks when requests share attachments.
		for _, id := range ids {
			attachment, e := q.Attachment(ctx, id, true)
			if e != nil {
				return e
			}
			if e = attachment.OwnedBy(a); e != nil {
				return e
			}
			if attachment.Status != domain.Ready || attachment.IssueID != "" {
				return domain.ErrPrecondition
			}
			attachments = append(attachments, attachment)
		}
		if e := q.InsertIssue(ctx, result); e != nil {
			return e
		}
		for _, attachment := range attachments {
			if e := attachment.Transition(domain.Attached); e != nil {
				return e
			}
			attachment.IssueID = result.ID
			attachment.UpdatedAt = now
			if e := q.UpdateAttachment(ctx, attachment); e != nil {
				return e
			}
		}
		return emit(ctx, q, result, a, "issue.created", now, nil)
	})
	return result, err
}
