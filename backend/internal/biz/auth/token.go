package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType 表示 JWT 的业务用途。
type TokenType string

const (
	// TokenTypeAccess 表示用于调用业务 API 的短期令牌。
	TokenTypeAccess TokenType = "access"
	// TokenTypeRefresh 表示用于轮换令牌的长期令牌。
	TokenTypeRefresh TokenType = "refresh"
)

// TokenSubject 描述签发令牌所需的认证上下文。
type TokenSubject struct {
	UserID            uint64
	TenantID          uint64
	MemberID          uint64
	SessionID         string
	PermissionVersion uint64
}

// TokenClaims 描述 Kratos Admin JWT 中的业务声明。
type TokenClaims struct {
	UserID            uint64    `json:"uid"`
	TenantID          uint64    `json:"tid"`
	MemberID          uint64    `json:"mid"`
	SessionID         string    `json:"sid"`
	PermissionVersion uint64    `json:"pv"`
	TokenType         TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// TokenPair 表示同一会话签发的 access 与 refresh JWT。
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessJTI        string
	RefreshJTI       string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// TokenManager 负责 Ed25519 JWT 的签发与验证。
type TokenManager struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// NewTokenManager 创建 JWT 管理器。
func NewTokenManager(privateKey ed25519.PrivateKey, accessTTL, refreshTTL time.Duration, now func() time.Time) *TokenManager {
	if now == nil {
		now = time.Now
	}
	return &TokenManager{
		privateKey: privateKey,
		publicKey:  privateKey.Public().(ed25519.PublicKey),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        now,
	}
}

// Issue 为认证上下文签发一对具有独立 jti 的令牌。
func (m *TokenManager) Issue(subject TokenSubject) (TokenPair, error) {
	if subject.UserID == 0 || subject.SessionID == "" {
		return TokenPair{}, errors.New("令牌用户和会话不能为空")
	}
	accessToken, accessJTI, accessExpiresAt, err := m.sign(subject, TokenTypeAccess, m.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refreshToken, refreshJTI, refreshExpiresAt, err := m.sign(subject, TokenTypeRefresh, m.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessJTI:        accessJTI,
		RefreshJTI:       refreshJTI,
		AccessExpiresAt:  accessExpiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

// Parse 验证签名、时效、签发者和令牌类型，并返回业务声明。
func (m *TokenManager) Parse(tokenString string, expectedType TokenType) (*TokenClaims, error) {
	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodEdDSA {
				return nil, errors.New("JWT签名算法无效")
			}
			return m.publicKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer("kratos-admin"),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return nil, fmt.Errorf("验证JWT失败: %w", err)
	}
	if !token.Valid || claims.TokenType != expectedType {
		return nil, errors.New("JWT类型无效")
	}
	return claims, nil
}

// HashJTI 返回可安全持久化的 JWT ID 摘要。
func HashJTI(jti string) string {
	digest := sha256.Sum256([]byte(jti))
	return hex.EncodeToString(digest[:])
}

func (m *TokenManager) sign(subject TokenSubject, tokenType TokenType, ttl time.Duration) (string, string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(ttl)
	jti, err := randomTokenID()
	if err != nil {
		return "", "", time.Time{}, err
	}
	claims := TokenClaims{
		UserID:            subject.UserID,
		TenantID:          subject.TenantID,
		MemberID:          subject.MemberID,
		SessionID:         subject.SessionID,
		PermissionVersion: subject.PermissionVersion,
		TokenType:         tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "kratos-admin",
			Subject:   strconv.FormatUint(subject.UserID, 10),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(m.privateKey)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("签发JWT失败: %w", err)
	}
	return signed, jti, expiresAt, nil
}

func randomTokenID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("生成JWT ID失败: %w", err)
	}
	return hex.EncodeToString(value), nil
}
