//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/repository/postgres"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/usecase"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Block while reading the S3 body, so the test can inspect database availability.
type pausedObjects struct {
	storage.ObjectStorage
	data    []byte
	entered chan struct{}
	release chan struct{}
}

func (o *pausedObjects) HeadObject(context.Context, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{Size: int64(len(o.data)), ContentType: "image/png", ETag: "test-etag"}, nil
}
func (o *pausedObjects) GetObject(ctx context.Context, _ string, etag string) (io.ReadCloser, error) {
	if etag != "test-etag" {
		return nil, domain.ErrPrecondition
	}
	return &pausedBody{ctx: ctx, reader: bytes.NewReader(o.data), entered: o.entered, release: o.release}, nil
}

type pausedBody struct {
	ctx     context.Context
	reader  *bytes.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *pausedBody) Read(p []byte) (int, error) {
	b.once.Do(func() { b.entered <- struct{}{} })
	select {
	case <-b.release:
		return b.reader.Read(p)
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	}
}
func (b *pausedBody) Close() error { return nil }

func TestCompleteUploadReleasesDatabaseDuringS3(t *testing.T) {
	base, _ := database(t)
	config := base.Config()
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := postgres.New(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	actor := domain.Actor{HouseID: uuid.NewString(), UserID: uuid.NewString(), Role: domain.Resident}
	now := time.Now().UTC()
	objects := &pausedObjects{data: photo(), entered: make(chan struct{}, 2), release: make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(objects.release) }) }
	// Unblock workers before closing the pool, including when an assertion fails.
	defer unblock()
	a := domain.Attachment{ID: uuid.NewString(), HouseID: actor.HouseID, UploadedBy: actor.UserID, ObjectKey: "concurrency/" + uuid.NewString(), OriginalFilename: "photo.png", MIMEType: "image/png", SizeBytes: int64(len(objects.data)), Status: domain.Uploading, UploadExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now}
	if err = store.Read().InsertAttachment(ctx, a); err != nil {
		t.Fatal(err)
	}
	service := usecase.New(store, objects, usecase.Options{})
	type outcome struct {
		attachment domain.Attachment
		err        error
	}
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() { v, e := service.CompleteUpload(ctx, actor, actor.HouseID, a.ID); results <- outcome{v, e} }()
	}
	wait, cancelWait := context.WithTimeout(ctx, 3*time.Second)
	defer cancelWait()
	for i := 0; i < 2; i++ {
		select {
		case <-objects.entered:
		case <-wait.Done():
			t.Fatal("concurrent upload could not start S3 reading with a one-connection pool")
		}
	}
	probe, stopProbe := context.WithTimeout(ctx, time.Second)
	defer stopProbe()
	if err = pool.Ping(probe); err != nil {
		t.Fatal("S3 read is occupying the database connection")
	}
	tx, err := pool.Begin(probe)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(probe, "SELECT id FROM attachments WHERE id=$1 FOR UPDATE NOWAIT", a.ID)
	_ = tx.Rollback(context.Background())
	if err != nil {
		t.Fatal("S3 read is holding the attachment row lock", err)
	}
	unblock()
	hash := sha256.Sum256(objects.data)
	for i := 0; i < 2; i++ {
		select {
		case result := <-results:
			if result.err != nil || result.attachment.Status != domain.Ready || result.attachment.SHA256 != hex.EncodeToString(hash[:]) {
				t.Fatalf("concurrent completion failed: %v", result.err)
			}
		case <-ctx.Done():
			t.Fatal("completion timed out")
		}
	}
	saved, err := store.Read().Attachment(ctx, a.ID, false)
	if err != nil || saved.Status != domain.Ready || saved.SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatal("verified metadata not persisted", err)
	}
}
