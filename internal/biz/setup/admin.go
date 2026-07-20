// Package setup 提供部署后显式执行的幂等初始化能力。
package setup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
)

var (
	// ErrAdminNotFound 表示指定用户名尚不存在。
	ErrAdminNotFound = errors.New("初始化管理员不存在")
	// ErrUsernameOccupied 表示用户名已被普通账号占用。
	ErrUsernameOccupied = errors.New("用户名已被非平台管理员占用")
)

// Admin 表示待持久化的平台超级管理员。
type Admin struct {
	Username      string
	Email         string
	Phone         string
	DisplayName   string
	PasswordHash  string
	PlatformAdmin bool
}

// AdminInput 描述初始化命令接收的管理员资料。
type AdminInput struct {
	Username    string
	Email       string
	Phone       string
	DisplayName string
	Password    string
}

// AdminRepository 定义幂等初始化所需的用户仓储。
type AdminRepository interface {
	FindByUsername(ctx context.Context, username string) (*Admin, error)
	Create(ctx context.Context, admin Admin) error
}

// AdminInitializer 负责安全创建首个平台超级管理员。
type AdminInitializer struct {
	repository AdminRepository
	hasher     *bizauth.PasswordHasher
}

// NewAdminInitializer 创建平台管理员初始化器。
func NewAdminInitializer(repository AdminRepository, hasher *bizauth.PasswordHasher) *AdminInitializer {
	return &AdminInitializer{repository: repository, hasher: hasher}
}

// Ensure 幂等确保管理员存在；密码只由命令输入，不写入迁移 SQL。
func (i *AdminInitializer) Ensure(ctx context.Context, input AdminInput) (bool, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.Username == "" || len(input.Password) < 12 {
		return false, errors.New("用户名不能为空且密码长度不能少于12位")
	}
	existing, err := i.repository.FindByUsername(ctx, input.Username)
	if err == nil {
		if existing.PlatformAdmin {
			return false, nil
		}
		return false, ErrUsernameOccupied
	}
	if !errors.Is(err, ErrAdminNotFound) {
		return false, fmt.Errorf("检查平台管理员失败: %w", err)
	}
	hash, err := i.hasher.Hash(input.Password)
	if err != nil {
		return false, fmt.Errorf("生成管理员密码摘要失败: %w", err)
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Username
	}
	err = i.repository.Create(ctx, Admin{
		Username: input.Username, Email: input.Email, Phone: input.Phone,
		DisplayName: input.DisplayName, PasswordHash: hash, PlatformAdmin: true,
	})
	if err != nil {
		return false, fmt.Errorf("创建平台管理员失败: %w", err)
	}
	return true, nil
}
