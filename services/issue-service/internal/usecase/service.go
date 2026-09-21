package usecase

import (
	"context"
	"encoding/json"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/google/uuid"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Options struct {
	MaxUploadSize          int64
	MaxPageSize            int
	UploadTTL, DownloadTTL time.Duration
	Now                    func() time.Time
}
type Service struct {
	store   repository.Store
	objects storage.ObjectStorage
	options Options
}

func New(store repository.Store, objects storage.ObjectStorage, o Options) *Service {
	if o.MaxUploadSize <= 0 {
		o.MaxUploadSize = 10 << 20
	}
	if o.MaxPageSize <= 0 {
		o.MaxPageSize = 100
	}
	if o.UploadTTL <= 0 {
		o.UploadTTL = 10 * time.Minute
	}
	if o.DownloadTTL <= 0 {
		o.DownloadTTL = 5 * time.Minute
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{store: store, objects: objects, options: o}
}
func (s *Service) now() time.Time { return s.options.Now().UTC().Truncate(time.Microsecond) }
func validate(ctx context.Context, a domain.Actor, house string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.InHouse(house)
}
func validText(value string, max int, required bool) bool {
	return utf8.ValidString(value) && len(value) <= max && (!required || strings.TrimSpace(value) != "") && !strings.ContainsRune(value, 0)
}
func validFilename(v string) bool {
	if !validText(v, 255, true) || strings.ContainsAny(v, "/\\") || v == "." || v == ".." {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func emit(ctx context.Context, q repository.Queries, issue domain.Issue, a domain.Actor, kind string, now time.Time, extra map[string]interface{}) error {
	payload := map[string]interface{}{"issue_id": issue.ID, "house_id": issue.HouseID, "created_by": issue.CreatedBy}
	for k, v := range extra {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err = q.AddTimeline(ctx, domain.TimelineEvent{ID: uuid.NewString(), IssueID: issue.ID, Type: kind, ActorUserID: a.UserID, Payload: data, CreatedAt: now}); err != nil {
		return err
	}
	return q.AddOutbox(ctx, domain.Event{ID: uuid.NewString(), AggregateType: "issue", AggregateID: issue.ID, Type: kind, Version: 1, Payload: data, OccurredAt: now, Producer: "issue-service"})
}
func lookupIssue(ctx context.Context, q repository.Queries, a domain.Actor, id string, lock bool) (domain.Issue, error) {
	if !domain.ValidID(id) {
		return domain.Issue{}, domain.ErrInvalid
	}
	i, err := q.Issue(ctx, id, lock)
	if err != nil {
		return i, err
	}
	return i, a.InHouse(i.HouseID)
}
