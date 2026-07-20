// Package storage 提供本地文件与阿里云 OSS 的统一对象存储能力。
package storage

import (
	"context"
	"io"
	"time"
)

// ObjectMeta 描述上传前约定及上传后校验的对象元数据。
type ObjectMeta struct {
	ContentType string
	Size        int64
	ETag        string
	Metadata    map[string]string
}

// SignedRequest 描述客户端可在短时间内执行的受限对象请求。
type SignedRequest struct {
	Method    string
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

// Provider 定义对象存储上传、校验、下载与删除能力。
type Provider interface {
	Name() string
	Put(ctx context.Context, objectKey string, body io.Reader, meta ObjectMeta) (ObjectMeta, error)
	PresignUpload(ctx context.Context, objectKey string, meta ObjectMeta, ttl time.Duration) (SignedRequest, error)
	Head(ctx context.Context, objectKey string) (ObjectMeta, error)
	PresignDownload(ctx context.Context, objectKey, downloadName string, ttl time.Duration) (SignedRequest, error)
	Delete(ctx context.Context, objectKey string) error
}
