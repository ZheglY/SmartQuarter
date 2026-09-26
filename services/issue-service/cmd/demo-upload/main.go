// demo-upload uploads the checked-in demonstration images without modifying application data.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "demo upload:", err)
		os.Exit(1)
	}
}
func run() error {
	house := flag.String("house", "", "Existing house UUID")
	dir := flag.String("dir", "", "Directory containing the three PNG files")
	flag.Parse()
	namespace, err := uuid.Parse(*house)
	if err != nil || namespace == uuid.Nil || *dir == "" {
		return fmt.Errorf("house and dir are required")
	}
	for _, key := range []string{"S3_BUCKET", "S3_ENDPOINT", "S3_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
		if os.Getenv(key) == "" {
			return fmt.Errorf("missing %s", key)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("S3_REGION")))
	if err != nil {
		return err
	}
	cfg.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	cfg.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(os.Getenv("S3_ENDPOINT")); o.UsePathStyle = true })
	rows := []map[string]any{}
	for _, name := range []string{"uninvited-neighbor.png", "lobby-regatta.png", "bench-inspection.png"} {
		data, err := os.ReadFile(filepath.Join(*dir, name))
		if err != nil {
			return err
		}
		if len(data) < 8 || !bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
			return fmt.Errorf("invalid PNG: %s", name)
		}
		id := uuid.NewSHA1(namespace, []byte("demo-attachment-v1/"+name)).String()
		key := "houses/" + *house + "/attachments/" + id + "/original"
		hash := sha256.Sum256(data)
		result, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(os.Getenv("S3_BUCKET")), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String("image/png")})
		if err != nil {
			return err
		}
		check, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(os.Getenv("S3_BUCKET")), Key: aws.String(key)})
		if err != nil {
			return err
		}
		received, err := io.ReadAll(io.LimitReader(check.Body, int64(len(data))+1))
		check.Body.Close()
		if err != nil {
			return err
		}
		if !bytes.Equal(data, received) {
			return fmt.Errorf("uploaded content mismatch: %s", name)
		}
		rows = append(rows, map[string]any{"id": id, "filename": name, "object_key": key, "size_bytes": len(data), "sha256": hex.EncodeToString(hash[:]), "etag": aws.ToString(result.ETag)})
	}
	return json.NewEncoder(os.Stdout).Encode(rows)
}
