//go:build container

package container

import (
	"bytes"
	"context"
	pb "github.com/ZheglY/SmartQuarter/services/issue-service/internal/gen/smartquarter/issue/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"image"
	"image/png"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestRunningContainer(t *testing.T) {
	conn, err := grpc.NewClient("localhost:18082", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	house, user := uuid.NewString(), uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-house-id", house, "x-actor-user-id", user, "x-actor-role", "RESIDENT"))
	client := pb.NewIssueServiceClient(conn)
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	upload, err := client.CreateUpload(ctx, &pb.CreateUploadRequest{HouseId: house, Filename: "container.png", MimeType: "image/png", SizeBytes: int64(buffer.Len())})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", upload.PutUrl, bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatal("invalid presigned URL")
	}
	for k, v := range upload.RequiredHeaders {
		req.Header.Set(k, v)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal("container public signed URL unreachable")
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("PUT status=%d", response.StatusCode)
	}
	attachment, err := client.CompleteUpload(ctx, &pb.CompleteUploadRequest{HouseId: house, UploadId: upload.UploadId})
	if err != nil {
		t.Fatal(err)
	}
	created, err := client.CreateIssue(ctx, &pb.CreateIssueRequest{HouseId: house, HouseAddressSnapshot: "Container test", Category: pb.IssueCategory_ISSUE_CATEGORY_SAFETY, Description: "Synthetic test", AttachmentIds: []string{attachment.Id}})
	if err != nil {
		t.Fatal(err)
	}
	found, err := client.GetIssue(ctx, &pb.GetIssueRequest{HouseId: house, IssueId: created.Id})
	if err != nil || len(found.GetAttachments()) != 1 {
		t.Fatal("container issue flow failed", err)
	}
	for _, path := range []string{"/livez", "/readyz", "/metrics"} {
		r, e := (&http.Client{Timeout: 5 * time.Second}).Get("http://localhost:18083" + path)
		if e != nil {
			t.Fatal("technical endpoint unavailable")
		}
		data, e := io.ReadAll(r.Body)
		r.Body.Close()
		if e != nil || r.StatusCode != 200 {
			t.Fatalf("%s failed", path)
		}
		if path == "/metrics" && !bytes.Contains(data, []byte("grpc_requests_total")) {
			t.Fatal("RPC metrics missing")
		}
	}
	t.Log("container upload, issue and readiness succeeded")
}
