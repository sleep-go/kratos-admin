package data

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
)

// CaptchaStore 使用 Redis 保存一次性图形验证码摘要。
type CaptchaStore struct {
	redis *redis.Client
}

// NewCaptchaStore 创建 Redis 图形验证码存储。
func NewCaptchaStore(resources *Data) *CaptchaStore {
	return &CaptchaStore{redis: resources.Redis}
}

// Put 保存验证码摘要及有效期。
func (s *CaptchaStore) Put(ctx context.Context, id, answerHash string, ttl time.Duration) error {
	return s.redis.Set(ctx, "auth:captcha:"+id, answerHash, ttl).Err()
}

// Take 原子读取并删除验证码摘要。
func (s *CaptchaStore) Take(ctx context.Context, id string) (string, error) {
	value, err := s.redis.GetDel(ctx, "auth:captcha:"+id).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

var _ bizauth.CaptchaStore = (*CaptchaStore)(nil)
