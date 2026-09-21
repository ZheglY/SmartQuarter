package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"io"
	"sort"
	"time"
)

type ListIssuesInput struct {
	HouseID       string
	Statuses      []domain.Status
	PageSize      int
	PageToken     string
	ChairmanQueue bool
}
type IssuePage struct {
	Items         []domain.Issue
	NextPageToken string
}
type cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
	Filter    string    `json:"filter"`
}

func (s *Service) ListIssues(ctx context.Context, a domain.Actor, in ListIssuesInput) (result IssuePage, err error) {
	if err = validate(ctx, a, in.HouseID); err != nil {
		return result, err
	}
	if in.ChairmanQueue {
		if err = a.RequireManager(); err != nil {
			return result, err
		}
	}
	if in.PageSize < 0 || in.PageSize > s.options.MaxPageSize || len(in.Statuses) > 7 || len(in.PageToken) > 4096 {
		return result, domain.ErrInvalid
	}
	size := in.PageSize
	if size == 0 {
		size = 20
		if size > s.options.MaxPageSize {
			size = s.options.MaxPageSize
		}
	}
	statuses := append([]domain.Status(nil), in.Statuses...)
	sort.Slice(statuses, func(i, j int) bool { return statuses[i] < statuses[j] })
	for i, status := range statuses {
		if !status.Valid() || (i > 0 && statuses[i-1] == status) {
			return result, domain.ErrInvalid
		}
	}
	filterData, _ := json.Marshal(struct {
		House    string
		Statuses []domain.Status
		Chairman bool
	}{a.HouseID, statuses, in.ChairmanQueue})
	hash := sha256.Sum256(filterData)
	fingerprint := hex.EncodeToString(hash[:])
	filter := domain.ListFilter{HouseID: a.HouseID, Statuses: statuses, Limit: size + 1, ChairmanQueue: in.ChairmanQueue}
	if in.PageToken != "" {
		raw, e := base64.RawURLEncoding.DecodeString(in.PageToken)
		if e != nil {
			return result, domain.ErrInvalid
		}
		var c cursor
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&c) != nil || decoder.Decode(&struct{}{}) != io.EOF || !domain.ValidID(c.ID) || c.CreatedAt.IsZero() || c.Filter != fingerprint {
			return result, domain.ErrInvalid
		}
		filter.BeforeTime = &c.CreatedAt
		filter.BeforeID = c.ID
	}
	items, err := s.store.Read().ListIssues(ctx, filter)
	if err != nil {
		return result, err
	}
	if len(items) > size {
		items = items[:size]
		last := items[len(items)-1]
		raw, e := json.Marshal(cursor{CreatedAt: last.CreatedAt, ID: last.ID, Filter: fingerprint})
		if e != nil {
			return result, e
		}
		result.NextPageToken = base64.RawURLEncoding.EncodeToString(raw)
	}
	result.Items = items
	return result, nil
}
