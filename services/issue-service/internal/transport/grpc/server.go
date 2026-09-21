package grpc

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	pb "github.com/ZheglY/SmartQuarter/services/issue-service/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strings"
)

type Server struct {
	pb.UnimplementedIssueServiceServer
	service *usecase.Service
}

func NewServer(service *usecase.Service) *Server { return &Server{service: service} }
func (s *Server) CreateUpload(ctx context.Context, r *pb.CreateUploadRequest) (*pb.CreateUploadResponse, error) {
	v, e := s.service.CreateUpload(ctx, Actor(ctx), usecase.CreateUploadInput{HouseID: r.GetHouseId(), Filename: r.GetFilename(), MIMEType: r.GetMimeType(), SizeBytes: r.GetSizeBytes()})
	if e != nil {
		return nil, mapError(e)
	}
	return &pb.CreateUploadResponse{UploadId: v.ID, PutUrl: v.Signed.URL, ExpiresAt: timestamppb.New(v.Signed.ExpiresAt), RequiredHeaders: v.Signed.Headers}, nil
}
func (s *Server) CompleteUpload(ctx context.Context, r *pb.CompleteUploadRequest) (*pb.Attachment, error) {
	v, e := s.service.CompleteUpload(ctx, Actor(ctx), r.GetHouseId(), r.GetUploadId())
	if e != nil {
		return nil, mapError(e)
	}
	return attachment(v), nil
}
func (s *Server) CreateIssue(ctx context.Context, r *pb.CreateIssueRequest) (*pb.Issue, error) {
	v, e := s.service.CreateIssue(ctx, Actor(ctx), usecase.CreateIssueInput{HouseID: r.GetHouseId(), HouseAddressSnapshot: r.GetHouseAddressSnapshot(), Category: domain.Category(strings.TrimPrefix(r.GetCategory().String(), "ISSUE_CATEGORY_")), Description: r.GetDescription(), LocationText: r.GetLocationText(), AttachmentIDs: r.GetAttachmentIds()})
	if e != nil {
		return nil, mapError(e)
	}
	return issue(v), nil
}
func (s *Server) GetIssue(ctx context.Context, r *pb.GetIssueRequest) (*pb.IssueDetails, error) {
	v, e := s.service.GetIssue(ctx, Actor(ctx), r.GetHouseId(), r.GetIssueId())
	if e != nil {
		return nil, mapError(e)
	}
	result := &pb.IssueDetails{Issue: issue(v.Issue), ConfirmedByMe: v.ConfirmedByMe}
	for _, a := range v.Attachments {
		result.Attachments = append(result.Attachments, attachment(a))
	}
	for _, ev := range v.Timeline {
		result.Timeline = append(result.Timeline, &pb.TimelineEvent{Id: ev.ID, IssueId: ev.IssueID, Type: ev.Type, ActorUserId: ev.ActorUserID, PayloadJson: string(ev.Payload), CreatedAt: timestamppb.New(ev.CreatedAt)})
	}
	if v.LatestStatement != nil {
		result.LatestStatement = statement(*v.LatestStatement)
	}
	return result, nil
}
func (s *Server) ListIssues(ctx context.Context, r *pb.ListIssuesRequest) (*pb.ListIssuesResponse, error) {
	statuses := make([]domain.Status, 0, len(r.GetStatus()))
	for _, v := range r.GetStatus() {
		statuses = append(statuses, domain.Status(strings.TrimPrefix(v.String(), "ISSUE_STATUS_")))
	}
	v, e := s.service.ListIssues(ctx, Actor(ctx), usecase.ListIssuesInput{HouseID: r.GetHouseId(), Statuses: statuses, PageSize: int(r.GetPageSize()), PageToken: r.GetPageToken(), ChairmanQueue: r.GetChairmanQueue()})
	if e != nil {
		return nil, mapError(e)
	}
	result := &pb.ListIssuesResponse{NextPageToken: v.NextPageToken}
	for _, i := range v.Items {
		result.Items = append(result.Items, issue(i))
	}
	return result, nil
}
func (s *Server) ConfirmIssue(ctx context.Context, r *pb.ConfirmIssueRequest) (*pb.ConfirmIssueResponse, error) {
	n, e := s.service.ConfirmIssue(ctx, Actor(ctx), r.GetHouseId(), r.GetIssueId())
	if e != nil {
		return nil, mapError(e)
	}
	return &pb.ConfirmIssueResponse{ConfirmationCount: int32(n), ConfirmedByMe: true}, nil
}
func (s *Server) UpdateIssueStatus(ctx context.Context, r *pb.UpdateIssueStatusRequest) (*pb.Issue, error) {
	v, e := s.service.UpdateIssueStatus(ctx, Actor(ctx), r.GetHouseId(), r.GetIssueId(), domain.Status(strings.TrimPrefix(r.GetNewStatus().String(), "ISSUE_STATUS_")))
	if e != nil {
		return nil, mapError(e)
	}
	return issue(v), nil
}
func (s *Server) GenerateStatement(ctx context.Context, r *pb.GenerateStatementRequest) (*pb.StatementDraft, error) {
	v, e := s.service.GenerateStatement(ctx, Actor(ctx), r.GetHouseId(), r.GetIssueId(), r.GetChairmanNote())
	if e != nil {
		return nil, mapError(e)
	}
	return statement(v), nil
}
func (s *Server) GetStatement(ctx context.Context, r *pb.GetStatementRequest) (*pb.StatementDraft, error) {
	v, e := s.service.GetStatement(ctx, Actor(ctx), r.GetHouseId(), r.GetIssueId())
	if e != nil {
		return nil, mapError(e)
	}
	return statement(v), nil
}
func (s *Server) GetAttachmentDownloadURL(ctx context.Context, r *pb.GetAttachmentDownloadURLRequest) (*pb.GetAttachmentDownloadURLResponse, error) {
	v, e := s.service.GetAttachmentDownloadURL(ctx, Actor(ctx), r.GetHouseId(), r.GetAttachmentId())
	if e != nil {
		return nil, mapError(e)
	}
	return &pb.GetAttachmentDownloadURLResponse{Url: v.URL, ExpiresAt: timestamppb.New(v.ExpiresAt)}, nil
}
func issue(v domain.Issue) *pb.Issue {
	r := &pb.Issue{Id: v.ID, HouseId: v.HouseID, CreatedBy: v.CreatedBy, HouseAddressSnapshot: v.HouseAddressSnapshot, Category: pb.IssueCategory(pb.IssueCategory_value["ISSUE_CATEGORY_"+string(v.Category)]), Description: v.Description, LocationText: v.LocationText, Status: pb.IssueStatus(pb.IssueStatus_value["ISSUE_STATUS_"+string(v.Status)]), ConfirmationsCount: int32(v.ConfirmationsCount), CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt)}
	if v.ResolvedAt != nil {
		r.ResolvedAt = timestamppb.New(*v.ResolvedAt)
	}
	return r
}
func attachment(v domain.Attachment) *pb.Attachment {
	return &pb.Attachment{Id: v.ID, HouseId: v.HouseID, IssueId: v.IssueID, UploadedBy: v.UploadedBy, OriginalFilename: v.OriginalFilename, MimeType: v.MIMEType, SizeBytes: v.SizeBytes, Sha256: v.SHA256, Etag: v.ETag, Status: pb.AttachmentStatus(pb.AttachmentStatus_value["ATTACHMENT_STATUS_"+string(v.Status)]), UploadExpiresAt: timestamppb.New(v.UploadExpiresAt), CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt)}
}
func statement(v domain.StatementDraft) *pb.StatementDraft {
	return &pb.StatementDraft{Id: v.ID, IssueId: v.IssueID, Version: int32(v.Version), Status: v.Status, Body: v.Body, ChairmanNote: v.ChairmanNote, SourceSnapshotJson: string(v.SourceSnapshot), CreatedBy: v.CreatedBy, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt)}
}
