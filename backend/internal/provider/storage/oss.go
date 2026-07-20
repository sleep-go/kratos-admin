package storage

import (
	"context"
	"errors"
	"io"
	"mime"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

// OSSConfig 描述阿里云 OSS Provider 所需配置。
type OSSConfig struct {
	Region          string
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
}

// OSSProvider 使用阿里云 OSS Go SDK V2 实现私有对象操作。
type OSSProvider struct {
	client *oss.Client
	bucket string
}

// NewOSSProvider 创建阿里云 OSS Provider。
func NewOSSProvider(config OSSConfig) (*OSSProvider, error) {
	if config.Region == "" || config.Bucket == "" || config.AccessKeyID == "" || config.AccessKeySecret == "" {
		return nil, errors.New("OSS region、bucket 和访问密钥不能为空")
	}
	credentialProvider := credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.AccessKeySecret)
	if config.SecurityToken != "" {
		credentialProvider = credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.AccessKeySecret, config.SecurityToken)
	}
	sdkConfig := oss.LoadDefaultConfig().WithRegion(config.Region).WithCredentialsProvider(credentialProvider)
	if config.Endpoint != "" {
		sdkConfig.WithEndpoint(config.Endpoint)
	}
	return &OSSProvider{client: oss.NewClient(sdkConfig), bucket: config.Bucket}, nil
}

// Name 返回 Provider 稳定名称。
func (p *OSSProvider) Name() string { return "aliyun-oss" }

// Put 由服务端写入异步任务生成的私有对象，并禁止覆盖同名对象。
func (p *OSSProvider) Put(ctx context.Context, objectKey string, body io.Reader, meta ObjectMeta) (ObjectMeta, error) {
	forbidOverwrite := "true"
	result, err := p.client.PutObject(ctx, &oss.PutObjectRequest{
		Bucket: &p.bucket, Key: &objectKey, Body: body, ContentLength: &meta.Size,
		ContentType: &meta.ContentType, Metadata: meta.Metadata, ForbidOverwrite: &forbidOverwrite,
	})
	if err != nil {
		return ObjectMeta{}, err
	}
	if result.ETag != nil {
		meta.ETag = *result.ETag
	}
	return meta, nil
}

// PresignUpload 生成带大小、类型和元数据约束的 OSS V4 PUT URL。
func (p *OSSProvider) PresignUpload(ctx context.Context, objectKey string, meta ObjectMeta, ttl time.Duration) (SignedRequest, error) {
	forbidOverwrite := "true"
	request := &oss.PutObjectRequest{
		Bucket: &p.bucket, Key: &objectKey, ContentLength: &meta.Size, ContentType: &meta.ContentType,
		Metadata: meta.Metadata, ForbidOverwrite: &forbidOverwrite,
	}
	result, err := p.client.Presign(ctx, request, func(options *oss.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return SignedRequest{}, err
	}
	return SignedRequest{Method: result.Method, URL: result.URL, Headers: result.SignedHeaders, ExpiresAt: result.Expiration}, nil
}

// Head 查询 OSS 对象元数据，用于上传完成后的服务端确认。
func (p *OSSProvider) Head(ctx context.Context, objectKey string) (ObjectMeta, error) {
	result, err := p.client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: &p.bucket, Key: &objectKey})
	if err != nil {
		return ObjectMeta{}, err
	}
	contentType := ""
	if result.ContentType != nil {
		contentType = *result.ContentType
	}
	etag := ""
	if result.ETag != nil {
		etag = *result.ETag
	}
	return ObjectMeta{ContentType: contentType, Size: result.ContentLength, ETag: etag, Metadata: result.Metadata}, nil
}

// PresignDownload 生成私有对象的短期 GET URL。
func (p *OSSProvider) PresignDownload(ctx context.Context, objectKey, downloadName string, ttl time.Duration) (SignedRequest, error) {
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": downloadName})
	request := &oss.GetObjectRequest{Bucket: &p.bucket, Key: &objectKey, ResponseContentDisposition: &disposition}
	result, err := p.client.Presign(ctx, request, func(options *oss.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return SignedRequest{}, err
	}
	return SignedRequest{Method: result.Method, URL: result.URL, Headers: result.SignedHeaders, ExpiresAt: result.Expiration}, nil
}

// Delete 删除 OSS 对象。
func (p *OSSProvider) Delete(ctx context.Context, objectKey string) error {
	_, err := p.client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: &p.bucket, Key: &objectKey})
	return err
}

// TestConnection 读取 Bucket 信息以验证地址、凭证与访问权限。
func (p *OSSProvider) TestConnection(ctx context.Context) error {
	_, err := p.client.GetBucketInfo(ctx, &oss.GetBucketInfoRequest{Bucket: &p.bucket})
	return err
}

var _ Provider = (*OSSProvider)(nil)
