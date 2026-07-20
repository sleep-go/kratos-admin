package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sleep-go/kratos-admin/backend/internal/provider/message"
)

const (
	verificationTTL        = 5 * time.Minute
	verificationResendWait = time.Minute
	verificationMaxAttempt = 5
)

var (
	// ErrVerificationInvalid 表示验证码错误、过期、已消费或挑战不存在。
	ErrVerificationInvalid = errors.New("验证码错误或已过期")
	// ErrVerificationRateLimited 表示同一目标六十秒内重复发送。
	ErrVerificationRateLimited = errors.New("验证码发送过于频繁")
	// ErrVerificationTarget 表示用户没有可用的邮件或短信目标。
	ErrVerificationTarget = errors.New("验证码接收目标不可用")
)

// VerificationRecord 描述一次邮件或短信验证码挑战。
type VerificationRecord struct {
	ID           uint64
	UserID       uint64
	Target       string
	Scene        string
	Channel      string
	CodeHash     string
	AttemptCount uint32
	ExpiresAt    time.Time
	ContextData  json.RawMessage
}

// VerificationRepository 定义验证码挑战和密码重置持久化能力。
type VerificationRepository interface {
	CreateVerification(ctx context.Context, record VerificationRecord, resendWait time.Duration) (uint64, error)
	ConsumeVerification(ctx context.Context, challengeID uint64, scene, codeHash string, now time.Time, maxAttempts uint32) (VerificationRecord, error)
	UpdatePassword(ctx context.Context, userID uint64, passwordHash string, changedAt time.Time) error
}

// VerificationChallenge 描述可返回给客户端的验证码挑战。
type VerificationChallenge struct {
	ID        uint64
	ExpiresAt time.Time
}

// VerificationUsecase 实施验证码有效期、防重发、失败上限和密码重置规则。
type VerificationUsecase struct {
	repository VerificationRepository
	users      UserRepository
	hasher     *PasswordHasher
	senders    map[string]message.Sender
	secret     []byte
	now        func() time.Time
}

// NewVerificationUsecase 创建邮件短信验证码用例。
func NewVerificationUsecase(repository VerificationRepository, users UserRepository, hasher *PasswordHasher, secret []byte, senders []message.Sender, now func() time.Time) (*VerificationUsecase, error) {
	if len(secret) < 16 {
		return nil, errors.New("验证码摘要密钥至少需要16字节")
	}
	if now == nil {
		now = time.Now
	}
	registry := make(map[string]message.Sender, len(senders))
	for _, sender := range senders {
		registry[sender.Channel()] = sender
	}
	return &VerificationUsecase{repository: repository, users: users, hasher: hasher, senders: registry, secret: append([]byte(nil), secret...), now: now}, nil
}

// ForgotPassword 创建不泄露账号存在性的密码重置挑战。
func (u *VerificationUsecase) ForgotPassword(ctx context.Context, identifier, channel string) (VerificationChallenge, error) {
	user, err := u.users.FindByIdentifier(ctx, identifier)
	if err != nil {
		id, randomErr := randomChallengeID()
		return VerificationChallenge{ID: id, ExpiresAt: u.now().UTC().Add(verificationTTL)}, randomErr
	}
	target := verificationTarget(*user, channel)
	if target == "" {
		return VerificationChallenge{}, ErrVerificationTarget
	}
	return u.issue(ctx, *user, target, "password_reset", channel, nil)
}

func randomChallengeID() (uint64, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, err
	}
	id := binary.BigEndian.Uint64(raw[:])
	if id == 0 {
		id = 1
	}
	return id, nil
}

// ResetPassword 消费验证码、更新密码并撤销该用户全部现有会话。
func (u *VerificationUsecase) ResetPassword(ctx context.Context, challengeID uint64, code, newPassword string) error {
	if challengeID == 0 || code == "" {
		return ErrVerificationInvalid
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	record, err := u.repository.ConsumeVerification(ctx, challengeID, "password_reset", u.codeHash(code), u.now().UTC(), verificationMaxAttempt)
	if err != nil {
		return err
	}
	passwordHash, err := u.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	return u.repository.UpdatePassword(ctx, record.UserID, passwordHash, u.now().UTC())
}

// IssueMFA 创建包含设备上下文的登录 MFA 挑战。
func (u *VerificationUsecase) IssueMFA(ctx context.Context, user User, input LoginInput) (VerificationChallenge, error) {
	channel := user.MFAChannel
	target := verificationTarget(user, channel)
	if target == "" {
		return VerificationChallenge{}, ErrVerificationTarget
	}
	contextData, _ := json.Marshal(LoginInput{
		TenantID: input.TenantID, DeviceName: input.DeviceName, IP: input.IP, UserAgent: input.UserAgent,
	})
	return u.issue(ctx, user, target, "mfa_login", channel, contextData)
}

// VerifyMFA 消费登录 MFA 验证码并返回用户与原始设备上下文。
func (u *VerificationUsecase) VerifyMFA(ctx context.Context, challengeID uint64, code string) (User, LoginInput, error) {
	record, err := u.repository.ConsumeVerification(ctx, challengeID, "mfa_login", u.codeHash(code), u.now().UTC(), verificationMaxAttempt)
	if err != nil {
		return User{}, LoginInput{}, err
	}
	user, err := u.users.FindByID(ctx, record.UserID)
	if err != nil {
		return User{}, LoginInput{}, err
	}
	var input LoginInput
	if err := json.Unmarshal(record.ContextData, &input); err != nil {
		return User{}, LoginInput{}, fmt.Errorf("解析MFA设备上下文失败: %w", err)
	}
	return *user, input, nil
}

func (u *VerificationUsecase) issue(ctx context.Context, user User, target, scene, channel string, contextData json.RawMessage) (VerificationChallenge, error) {
	sender := u.senders[channel]
	if sender == nil {
		return VerificationChallenge{}, ErrVerificationTarget
	}
	code, err := randomDigits(6)
	if err != nil {
		return VerificationChallenge{}, err
	}
	now := u.now().UTC()
	record := VerificationRecord{UserID: user.ID, Target: target, Scene: scene, Channel: channel, CodeHash: u.codeHash(code), ExpiresAt: now.Add(verificationTTL), ContextData: contextData}
	id, err := u.repository.CreateVerification(ctx, record, verificationResendWait)
	if err != nil {
		return VerificationChallenge{}, err
	}
	if err := sender.SendCode(ctx, message.CodeMessage{Target: target, Scene: scene, Code: code, Minutes: 5}); err != nil {
		return VerificationChallenge{}, err
	}
	return VerificationChallenge{ID: id, ExpiresAt: record.ExpiresAt}, nil
}

func (u *VerificationUsecase) codeHash(code string) string {
	mac := hmac.New(sha256.New, u.secret)
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func verificationTarget(user User, channel string) string {
	switch channel {
	case "email":
		return user.Email
	case "sms":
		return user.Phone
	default:
		return ""
	}
}
