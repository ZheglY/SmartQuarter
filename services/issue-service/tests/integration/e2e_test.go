//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	pb "github.com/ZheglY/SmartQuarter/services/issue-service/internal/gen/smartquarter/issue/v1"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/observability"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/outbox"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	transport "github.com/ZheglY/SmartQuarter/services/issue-service/internal/transport/grpc"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/usecase"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func client(t *testing.T, server pb.IssueServiceServer) pb.IssueServiceClient {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(transport.Unary(zap.NewNop(), observability.NewMetrics(), 10*time.Second)))
	pb.RegisterIssueServiceServer(s, server)
	go func() { _ = s.Serve(listener) }()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(); s.Stop(); listener.Close() })
	return pb.NewIssueServiceClient(conn)
}
func actorContext(house, user, role string) context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-house-id", house, "x-actor-user-id", user, "x-actor-role", role, "x-request-id", uuid.NewString()))
}
func requireCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("code=%s want=%s (%v)", status.Code(err), want, err)
	}
}
func photo() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	return b.Bytes()
}
func TestEndToEndFiveRuns(t *testing.T) {
	pool, store := database(t)
	objects := objects(t)
	ctx := context.Background()
	c := client(t, transport.NewServer(usecase.New(store, objects, usecase.Options{})))
	stream := redis.NewClient(&redis.Options{Addr: "localhost:16379", MaxRetries: -1})
	defer stream.Close()
	if err := stream.Ping(ctx).Err(); err != nil {
		t.Fatal("local test Redis unavailable")
	}
	streamName := "issue-test:" + uuid.NewString()
	defer stream.Del(ctx, streamName)
	publisher := outbox.New(store, stream, streamName, 100, time.Millisecond, observability.NewMetrics(), zap.NewNop())
	for run := 0; run < 5; run++ {
		t.Run(fmt.Sprint(run+1), func(t *testing.T) {
			house, author, neighbor := uuid.NewString(), uuid.NewString(), uuid.NewString()
			resident := actorContext(house, author, "RESIDENT")
			other := actorContext(house, neighbor, "RESIDENT")
			chairman := actorContext(house, uuid.NewString(), "CHAIRMAN")
			foreignHouse := uuid.NewString()
			foreign := actorContext(foreignHouse, uuid.NewString(), "ADMIN")
			data := photo()
			var headers metadata.MD
			upload, err := c.CreateUpload(resident, &pb.CreateUploadRequest{HouseId: house, Filename: "photo.png", MimeType: "image/png", SizeBytes: int64(len(data))}, grpc.Header(&headers))
			requireCode(t, err, codes.OK)
			sent, _ := metadata.FromOutgoingContext(resident)
			if headers.Get("x-request-id")[0] != sent.Get("x-request-id")[0] {
				t.Fatal("request id not propagated")
			}
			key := "houses/" + house + "/attachments/" + upload.UploadId + "/original"
			defer objects.DeleteObject(ctx, key)
			_, err = c.CompleteUpload(resident, &pb.CompleteUploadRequest{HouseId: house, UploadId: upload.UploadId})
			requireCode(t, err, codes.FailedPrecondition)
			if code := signedPut(t, storage.SignedURL{URL: upload.PutUrl, Headers: upload.RequiredHeaders}, data); code != 200 {
				t.Fatalf("PUT=%d", code)
			}
			attachment, err := c.CompleteUpload(resident, &pb.CompleteUploadRequest{HouseId: house, UploadId: upload.UploadId})
			requireCode(t, err, codes.OK)
			sum := sha256.Sum256(data)
			if attachment.Sha256 != hex.EncodeToString(sum[:]) || attachment.Status != pb.AttachmentStatus_ATTACHMENT_STATUS_READY {
				t.Fatal("verification not persisted")
			}
			_, err = c.CompleteUpload(resident, &pb.CompleteUploadRequest{HouseId: house, UploadId: upload.UploadId})
			requireCode(t, err, codes.OK)
			_, err = c.CompleteUpload(other, &pb.CompleteUploadRequest{HouseId: house, UploadId: upload.UploadId})
			requireCode(t, err, codes.PermissionDenied)
			input := &pb.CreateIssueRequest{HouseId: house, HouseAddressSnapshot: "Тестовый дом 1", Category: pb.IssueCategory_ISSUE_CATEGORY_SAFETY, Description: "Открытый люк", LocationText: "У подъезда", AttachmentIds: []string{upload.UploadId}}
			_, err = c.CreateIssue(other, input)
			requireCode(t, err, codes.PermissionDenied)
			created, err := c.CreateIssue(resident, input)
			requireCode(t, err, codes.OK)
			_, err = c.CreateIssue(resident, input)
			requireCode(t, err, codes.FailedPrecondition)
			_, err = c.GetIssue(foreign, &pb.GetIssueRequest{HouseId: foreignHouse, IssueId: created.Id})
			requireCode(t, err, codes.PermissionDenied)
			_, err = c.ConfirmIssue(resident, &pb.ConfirmIssueRequest{HouseId: house, IssueId: created.Id})
			requireCode(t, err, codes.FailedPrecondition)
			results := make(chan error, 2)
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, e := c.ConfirmIssue(other, &pb.ConfirmIssueRequest{HouseId: house, IssueId: created.Id})
					results <- e
				}()
			}
			wg.Wait()
			close(results)
			success, duplicate := 0, 0
			for e := range results {
				switch status.Code(e) {
				case codes.OK:
					success++
				case codes.AlreadyExists:
					duplicate++
				default:
					t.Fatal(e)
				}
			}
			if success != 1 || duplicate != 1 {
				t.Fatal("duplicate confirmation not atomic")
			}
			_, err = c.GenerateStatement(resident, &pb.GenerateStatementRequest{HouseId: house, IssueId: created.Id})
			requireCode(t, err, codes.PermissionDenied)
			_, err = c.UpdateIssueStatus(resident, &pb.UpdateIssueStatusRequest{HouseId: house, IssueId: created.Id, NewStatus: pb.IssueStatus_ISSUE_STATUS_CONFIRMING})
			requireCode(t, err, codes.PermissionDenied)
			_, err = c.UpdateIssueStatus(chairman, &pb.UpdateIssueStatusRequest{HouseId: house, IssueId: created.Id, NewStatus: pb.IssueStatus_ISSUE_STATUS_MARKED_SENT})
			requireCode(t, err, codes.FailedPrecondition)
			drafts := make(chan *pb.StatementDraft, 4)
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					v, e := c.GenerateStatement(chairman, &pb.GenerateStatementRequest{HouseId: house, IssueId: created.Id, ChairmanNote: "Просим устранить"})
					if e != nil {
						t.Error(e)
						return
					}
					drafts <- v
				}()
			}
			wg.Wait()
			close(drafts)
			versions := map[int32]bool{}
			body := ""
			for d := range drafts {
				if versions[d.Version] {
					t.Fatal("duplicate statement version")
				}
				versions[d.Version] = true
				if body != "" && body != d.Body {
					t.Fatal("non deterministic statement")
				}
				body = d.Body
			}
			if len(versions) != 4 || !versions[4] {
				t.Fatal("statement versions not serialized")
			}
			latest, err := c.GetStatement(chairman, &pb.GetStatementRequest{HouseId: house, IssueId: created.Id})
			requireCode(t, err, codes.OK)
			if latest.Version != 4 {
				t.Fatal("not latest statement")
			}
			details, err := c.GetIssue(other, &pb.GetIssueRequest{HouseId: house, IssueId: created.Id})
			requireCode(t, err, codes.OK)
			if !details.ConfirmedByMe || details.Issue.ConfirmationsCount != 1 || details.LatestStatement != nil || len(details.Attachments) != 1 || len(details.Timeline) != 6 {
				t.Fatal("incorrect resident issue details")
			}
			page, err := c.ListIssues(chairman, &pb.ListIssuesRequest{HouseId: house, ChairmanQueue: true, PageSize: 1})
			requireCode(t, err, codes.OK)
			if len(page.Items) != 1 || page.Items[0].Id != created.Id {
				t.Fatal("chairman queue missing issue")
			}
			download, err := c.GetAttachmentDownloadURL(other, &pb.GetAttachmentDownloadURLRequest{HouseId: house, AttachmentId: upload.UploadId})
			requireCode(t, err, codes.OK)
			response, err := (&http.Client{Timeout: 5 * time.Second}).Get(download.Url)
			if err != nil {
				t.Fatal("signed GET failed")
			}
			got, e := io.ReadAll(response.Body)
			response.Body.Close()
			if e != nil || response.StatusCode != 200 || !bytes.Equal(got, data) {
				t.Fatal("download differs")
			}
			for _, target := range []pb.IssueStatus{2, 3, 4, 5, 6, 7} {
				updated, e := c.UpdateIssueStatus(chairman, &pb.UpdateIssueStatusRequest{HouseId: house, IssueId: created.Id, NewStatus: target})
				requireCode(t, e, codes.OK)
				if updated.Status != target {
					t.Fatal("wrong status")
				}
				if target == 7 && updated.ResolvedAt == nil {
					t.Fatal("missing resolved_at")
				}
			}
			page, err = c.ListIssues(chairman, &pb.ListIssuesRequest{HouseId: house, ChairmanQueue: true})
			requireCode(t, err, codes.OK)
			if len(page.Items) != 0 {
				t.Fatal("resolved issue still queued")
			}
			var count int
			if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE aggregate_id=$1", created.Id).Scan(&count); err != nil || count != 12 {
				t.Fatalf("outbox count=%d err=%v", count, err)
			}
			for i := 0; i < 10; i++ {
				n, e := store.PublishBatch(ctx, 100, publisher.Publish)
				if e != nil {
					t.Fatal(e)
				}
				if n == 0 {
					break
				}
			}
			var pending int
			if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE aggregate_id=$1 AND published_at IS NULL", created.Id).Scan(&pending); err != nil || pending != 0 {
				t.Fatal("unpublished flow events")
			}
			messages, e := stream.XRange(ctx, streamName, "-", "+").Result()
			if e != nil {
				t.Fatal(e)
			}
			seen := 0
			for _, m := range messages {
				var envelope domain.Event
				raw, ok := m.Values["data"].(string)
				if !ok || json.Unmarshal([]byte(raw), &envelope) != nil {
					t.Fatal("invalid stream envelope")
				}
				var p struct {
					IssueID string `json:"issue_id"`
				}
				if json.Unmarshal(envelope.Payload, &p) != nil {
					t.Fatal("bad payload")
				}
				if p.IssueID == created.Id {
					seen++
					if envelope.ID == "" || envelope.Version != 1 || envelope.Producer != "issue-service" {
						t.Fatal("bad envelope")
					}
				}
			}
			if seen != 12 {
				t.Fatalf("delivered %d events want12", seen)
			}
		})
	}
	t.Run("invalid requests", func(t *testing.T) {
		house := uuid.NewString()
		a := actorContext(house, uuid.NewString(), "RESIDENT")
		_, e := c.ListIssues(context.Background(), &pb.ListIssuesRequest{HouseId: house})
		requireCode(t, e, codes.Unauthenticated)
		_, e = c.GetIssue(a, &pb.GetIssueRequest{HouseId: house, IssueId: "bad"})
		requireCode(t, e, codes.InvalidArgument)
		_, e = c.GetIssue(a, &pb.GetIssueRequest{HouseId: house, IssueId: uuid.NewString()})
		requireCode(t, e, codes.NotFound)
		_, e = c.ListIssues(a, &pb.ListIssuesRequest{HouseId: house, Status: []pb.IssueStatus{999}})
		requireCode(t, e, codes.InvalidArgument)
		_, e = c.CreateIssue(a, &pb.CreateIssueRequest{HouseId: house, Category: 999})
		requireCode(t, e, codes.InvalidArgument)
		_, e = c.ListIssues(a, &pb.ListIssuesRequest{HouseId: house, ChairmanQueue: true})
		requireCode(t, e, codes.PermissionDenied)
		canceled, cancel := context.WithCancel(a)
		cancel()
		_, e = c.ListIssues(canceled, &pb.ListIssuesRequest{HouseId: house})
		requireCode(t, e, codes.Canceled)
		expired, stop := context.WithDeadline(a, time.Now().Add(-time.Second))
		defer stop()
		_, e = c.ListIssues(expired, &pb.ListIssuesRequest{HouseId: house})
		requireCode(t, e, codes.DeadlineExceeded)
	})
}

type failingStore struct{ repository.Store }
type failingQueries struct{ repository.Queries }

func (f failingStore) WithinTx(ctx context.Context, fn func(repository.Queries) error) error {
	return f.Store.WithinTx(ctx, func(q repository.Queries) error { return fn(failingQueries{q}) })
}
func (f failingQueries) AddOutbox(context.Context, domain.Event) error { return domain.ErrUnavailable }
func TestBusinessRollback(t *testing.T) {
	pool, store := database(t)
	ctx := context.Background()
	house, user := uuid.NewString(), uuid.NewString()
	now := time.Now().UTC()
	a := domain.Attachment{ID: uuid.NewString(), HouseID: house, UploadedBy: user, ObjectKey: "rollback/" + uuid.NewString(), OriginalFilename: "x.png", MIMEType: "image/png", SizeBytes: 1, Status: domain.Ready, SHA256: fmt.Sprintf("%064d", 0), ETag: "etag", CreatedAt: now, UpdatedAt: now, UploadExpiresAt: now.Add(time.Minute)}
	a.Status = domain.Uploading
	if err := store.Read().InsertAttachment(ctx, a); err != nil {
		t.Fatal(err)
	}
	a.Status = domain.Ready
	if err := store.Read().UpdateAttachment(ctx, a); err != nil {
		t.Fatal(err)
	}
	service := usecase.New(failingStore{store}, nil, usecase.Options{})
	_, err := service.CreateIssue(ctx, domain.Actor{HouseID: house, UserID: user, Role: domain.Resident}, usecase.CreateIssueInput{HouseID: house, HouseAddressSnapshot: "House", Category: domain.Safety, Description: "Problem", AttachmentIDs: []string{a.ID}})
	if err == nil {
		t.Fatal("outbox failure ignored")
	}
	saved, err := store.Read().Attachment(ctx, a.ID, false)
	if err != nil || saved.Status != domain.Ready || saved.IssueID != "" {
		t.Fatal("attachment not rolled back", err)
	}
	for _, table := range []string{"issues", "timeline_events", "outbox_events"} {
		var n int
		query := "SELECT COUNT(*) FROM issues WHERE house_id=$1"
		if table == "timeline_events" {
			query = "SELECT COUNT(*) FROM timeline_events WHERE payload->>'house_id'=$1"
		}
		if table == "outbox_events" {
			query = "SELECT COUNT(*) FROM outbox_events WHERE payload->>'house_id'=$1"
		}
		if err = pool.QueryRow(ctx, query, house).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s leaked after rollback: %d %v", table, n, err)
		}
	}
}

type panickingServer struct {
	pb.UnimplementedIssueServiceServer
}

func (panickingServer) ListIssues(context.Context, *pb.ListIssuesRequest) (*pb.ListIssuesResponse, error) {
	panic("secret internal details")
}
func TestGRPCPanicRecovery(t *testing.T) {
	c := client(t, panickingServer{})
	ctx := actorContext(uuid.NewString(), uuid.NewString(), "RESIDENT")
	_, err := c.ListIssues(ctx, &pb.ListIssuesRequest{})
	requireCode(t, err, codes.Internal)
	if status.Convert(err).Message() != "internal error" {
		t.Fatal("panic leaked")
	}
}
