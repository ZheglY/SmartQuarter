package yandexs3

import (
	"context"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
)

func (c *Client) HeadObject(ctx context.Context, key string) (result storage.ObjectInfo, err error) {
	defer func() { c.observe("head", err) }()
	output, e := c.api.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	if e != nil {
		return result, translate(e)
	}
	return storage.ObjectInfo{Size: aws.ToInt64(output.ContentLength), ContentType: aws.ToString(output.ContentType), ETag: aws.ToString(output.ETag)}, nil
}
func (c *Client) GetObject(ctx context.Context, key, etag string) (body io.ReadCloser, err error) {
	defer func() { c.observe("get", err) }()
	output, e := c.api.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key), IfMatch: aws.String(etag)})
	if e != nil {
		return nil, translate(e)
	}
	return output.Body, nil
}
func (c *Client) DeleteObject(ctx context.Context, key string) (err error) {
	defer func() { c.observe("delete", err) }()
	_, err = c.api.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	return translate(err)
}
