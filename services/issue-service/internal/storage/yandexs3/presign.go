package yandexs3

import (
	"context"
	"strings"
	"time"

	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (c *Client) PresignUpload(ctx context.Context, key, mime string, size int64, ttl time.Duration) (result storage.SignedURL, err error) {
	defer func() { c.observe("presign_put", err) }()
	expires := time.Now().UTC().Add(ttl)
	request, e := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key), ContentType: aws.String(mime), ContentLength: aws.Int64(size), IfNoneMatch: aws.String("*")}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if e != nil {
		return result, translate(e)
	}
	headers := map[string]string{"Content-Type": mime, "If-None-Match": "*"}
	for k, v := range request.SignedHeader {
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Content-Length") {
			continue
		}
		headers[k] = strings.Join(v, ",")
	}
	return storage.SignedURL{URL: request.URL, Headers: headers, ExpiresAt: expires}, nil
}
func (c *Client) PresignDownload(ctx context.Context, key string, ttl time.Duration) (result storage.SignedURL, err error) {
	defer func() { c.observe("presign_get", err) }()
	expires := time.Now().UTC().Add(ttl)
	request, e := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if e != nil {
		return result, translate(e)
	}
	return storage.SignedURL{URL: request.URL, ExpiresAt: expires}, nil
}
