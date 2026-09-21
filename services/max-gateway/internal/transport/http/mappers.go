package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/issue/v1"
)

var categories = map[pb.IssueCategory]string{1: "SAFETY", 2: "CLEANLINESS", 3: "UTILITIES", 4: "INFRASTRUCTURE", 5: "OTHER"}
var statuses = map[pb.IssueStatus]string{1: "DETECTED", 2: "CONFIRMING", 3: "READY_FOR_APPEAL", 4: "HANDED_TO_CHAIRMAN", 5: "MARKED_SENT", 6: "WAITING_RESULT", 7: "RESOLVED"}
var attachmentStatuses = map[pb.AttachmentStatus]string{1: "UPLOADING", 2: "READY", 3: "ATTACHED", 4: "REJECTED", 5: "EXPIRED"}
var categoriesReverse = func() map[string]pb.IssueCategory {
	m := map[string]pb.IssueCategory{}
	for k, v := range categories {
		m[v] = k
	}
	return m
}()
var statusesReverse = func() map[string]pb.IssueStatus {
	m := map[string]pb.IssueStatus{}
	for k, v := range statuses {
		m[v] = k
	}
	return m
}()
var invalidUpstream = errors.New("invalid upstream response")

func timestamp(t *timestamppb.Timestamp) *string {
	if t == nil {
		return nil
	}
	if t.CheckValid() != nil {
		panic(invalidUpstream)
	}
	v := t.AsTime().UTC().Format(time.RFC3339Nano)
	return &v
}
func object(s string) (map[string]any, error) {
	var v map[string]any
	if json.Unmarshal([]byte(s), &v) != nil || v == nil {
		return nil, invalidUpstream
	}
	return v, nil
}
func (a *API) mapped(w http.ResponseWriter, r *http.Request, code int, f func() (any, error)) {
	v, e := f()
	if e != nil {
		a.fail(w, r, 502, "INVALID_UPSTREAM_RESPONSE", "invalid upstream response")
		return
	}
	write(w, code, v)
}

type IssueDTO struct {
	ID                   string  `json:"id"`
	HouseID              string  `json:"house_id"`
	CreatedBy            string  `json:"created_by"`
	HouseAddressSnapshot string  `json:"house_address_snapshot"`
	Category             string  `json:"category"`
	Description          string  `json:"description"`
	LocationText         string  `json:"location_text"`
	Status               string  `json:"status"`
	ConfirmationsCount   int32   `json:"confirmations_count"`
	CreatedAt            *string `json:"created_at"`
	UpdatedAt            *string `json:"updated_at"`
	ResolvedAt           *string `json:"resolved_at"`
}
type AttachmentDTO struct {
	ID               string  `json:"id"`
	HouseID          string  `json:"house_id"`
	IssueID          *string `json:"issue_id"`
	UploadedBy       string  `json:"uploaded_by"`
	OriginalFilename string  `json:"original_filename"`
	MIMEType         string  `json:"mime_type"`
	SizeBytes        int64   `json:"size_bytes"`
	SHA256           string  `json:"sha256"`
	ETag             string  `json:"etag"`
	Status           string  `json:"status"`
	UploadExpiresAt  *string `json:"upload_expires_at"`
	CreatedAt        *string `json:"created_at"`
	UpdatedAt        *string `json:"updated_at"`
}
type StatementDTO struct {
	ID             string         `json:"id"`
	IssueID        string         `json:"issue_id"`
	Version        int32          `json:"version"`
	Status         string         `json:"status"`
	Body           string         `json:"body"`
	ChairmanNote   string         `json:"chairman_note"`
	SourceSnapshot map[string]any `json:"source_snapshot"`
	CreatedBy      string         `json:"created_by"`
	CreatedAt      *string        `json:"created_at"`
	UpdatedAt      *string        `json:"updated_at"`
}

func issueDTO(v *pb.Issue) (IssueDTO, error) {
	var d IssueDTO
	if v == nil {
		return d, invalidUpstream
	}
	d.ID = v.Id
	d.HouseID = v.HouseId
	d.CreatedBy = v.CreatedBy
	d.HouseAddressSnapshot = v.HouseAddressSnapshot
	d.Description = v.Description
	d.LocationText = v.LocationText
	d.ConfirmationsCount = v.ConfirmationsCount
	d.CreatedAt = timestamp(v.CreatedAt)
	d.UpdatedAt = timestamp(v.UpdatedAt)
	d.ResolvedAt = timestamp(v.ResolvedAt)
	d.Category = categories[v.Category]
	d.Status = statuses[v.Status]
	if d.Category == "" || d.Status == "" {
		return d, invalidUpstream
	}
	return d, nil
}
func attachmentDTO(v *pb.Attachment) (AttachmentDTO, error) {
	var d AttachmentDTO
	if v == nil {
		return d, invalidUpstream
	}
	d.ID = v.Id
	d.HouseID = v.HouseId
	d.UploadedBy = v.UploadedBy
	d.OriginalFilename = v.OriginalFilename
	d.MIMEType = v.MimeType
	d.SizeBytes = v.SizeBytes
	d.SHA256 = v.Sha256
	d.ETag = v.Etag
	d.UploadExpiresAt = timestamp(v.UploadExpiresAt)
	d.CreatedAt = timestamp(v.CreatedAt)
	d.UpdatedAt = timestamp(v.UpdatedAt)
	if v.IssueId != "" {
		d.IssueID = &v.IssueId
	}
	d.Status = attachmentStatuses[v.Status]
	if d.Status == "" {
		return d, invalidUpstream
	}
	return d, nil
}
func statementDTO(v *pb.StatementDraft) (StatementDTO, error) {
	var d StatementDTO
	if v == nil {
		return d, invalidUpstream
	}
	d.ID = v.Id
	d.IssueID = v.IssueId
	d.Version = v.Version
	d.Status = v.Status
	d.Body = v.Body
	d.ChairmanNote = v.ChairmanNote
	d.CreatedBy = v.CreatedBy
	d.CreatedAt = timestamp(v.CreatedAt)
	d.UpdatedAt = timestamp(v.UpdatedAt)
	var e error
	d.SourceSnapshot, e = object(v.SourceSnapshotJson)
	if e != nil {
		return d, e
	}
	return d, nil
}

type TimelineDTO struct {
	ID          string         `json:"id"`
	IssueID     string         `json:"issue_id"`
	Type        string         `json:"type"`
	ActorUserID string         `json:"actor_user_id"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   *string        `json:"created_at"`
}
type DetailsDTO struct {
	Issue           IssueDTO        `json:"issue"`
	Attachments     []AttachmentDTO `json:"attachments"`
	ConfirmedByMe   bool            `json:"confirmed_by_me"`
	Timeline        []TimelineDTO   `json:"timeline"`
	LatestStatement *StatementDTO   `json:"latest_statement"`
}

func detailsDTO(v *pb.IssueDetails) (DetailsDTO, error) {
	var d DetailsDTO
	if v == nil {
		return d, invalidUpstream
	}
	var e error
	d.Issue, e = issueDTO(v.Issue)
	if e != nil {
		return d, e
	}
	d.ConfirmedByMe = v.ConfirmedByMe
	d.Attachments = []AttachmentDTO{}
	d.Timeline = []TimelineDTO{}
	for _, a := range v.Attachments {
		x, e := attachmentDTO(a)
		if e != nil {
			return d, e
		}
		d.Attachments = append(d.Attachments, x)
	}
	for _, t := range v.Timeline {
		if t == nil {
			return d, invalidUpstream
		}
		p, e := object(t.PayloadJson)
		if e != nil {
			return d, e
		}
		d.Timeline = append(d.Timeline, TimelineDTO{t.Id, t.IssueId, t.Type, t.ActorUserId, p, timestamp(t.CreatedAt)})
	}
	if v.LatestStatement != nil {
		s, e := statementDTO(v.LatestStatement)
		if e != nil {
			return d, e
		}
		d.LatestStatement = &s
	}
	return d, nil
}
