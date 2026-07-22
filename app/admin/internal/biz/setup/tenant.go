package setup

import (
	"context"
	"errors"
	"fmt"
)

// 内置租户角色编码，租户开户后自动创建。
const builtinTenantAdminRoleCode = "tenant-admin"

// 内置租户角色名称。
const builtinTenantAdminRoleName = "租户管理员"

// TenantRole 描述待持久化的租户内置角色。
type TenantRole struct {
	TenantID  uint64
	Code      string
	Name      string
	DataScope uint8
	IsBuiltin bool
	Status    uint8
}

// TenantCasbinRule 描述待持久化的 Casbin 策略规则。
type TenantCasbinRule struct {
	Ptype string
	V0    string
	V1    string
	V2    string
	V3    string
}

// TenantRepository 定义租户开户默认初始化所需的仓储。
type TenantRepository interface {
	// FindRoleByTenantAndCode 按租户与角色编码查询角色，exists=false 表示不存在。
	FindRoleByTenantAndCode(ctx context.Context, tenantID uint64, code string) (roleID uint64, exists bool, err error)
	// CreateRole 创建角色并返回角色 ID。
	CreateRole(ctx context.Context, role TenantRole) (uint64, error)
	// CasbinRuleExists 查询 Casbin 规则是否已存在。
	CasbinRuleExists(ctx context.Context, ptype, v0, v1, v2, v3 string) (bool, error)
	// CreateCasbinRule 创建 Casbin 规则。
	CreateCasbinRule(ctx context.Context, rule TenantCasbinRule) error
}

// TenantProvisioner 负责租户开户后的默认初始化，幂等创建内置角色与通配策略。
type TenantProvisioner struct {
	repository TenantRepository
}

// NewTenantProvisioner 创建租户开户初始化器。
func NewTenantProvisioner(repository TenantRepository) *TenantProvisioner {
	return &TenantProvisioner{repository: repository}
}

// EnsureDefaults 幂等确保租户拥有内置 tenant-admin 角色及通配 Casbin 策略。
// D3 决策：仅创建默认角色，不创建默认管理员（管理员由平台管理员手动创建）。
func (p *TenantProvisioner) EnsureDefaults(ctx context.Context, tenantID uint64) error {
	if tenantID == 0 {
		return errors.New("租户 ID 不能为空")
	}
	roleID, exists, err := p.repository.FindRoleByTenantAndCode(ctx, tenantID, builtinTenantAdminRoleCode)
	if err != nil {
		return fmt.Errorf("查询租户内置角色失败: %w", err)
	}
	if !exists {
		roleID, err = p.repository.CreateRole(ctx, TenantRole{
			TenantID:  tenantID,
			Code:      builtinTenantAdminRoleCode,
			Name:      builtinTenantAdminRoleName,
			DataScope: 1,
			IsBuiltin: true,
			Status:    1,
		})
		if err != nil {
			return fmt.Errorf("创建租户内置角色失败: %w", err)
		}
	}
	policyExists, err := p.repository.CasbinRuleExists(ctx, "p", fmt.Sprint(tenantID), fmt.Sprint(roleID), "*", "*")
	if err != nil {
		return fmt.Errorf("查询租户通配策略失败: %w", err)
	}
	if policyExists {
		return nil
	}
	if err := p.repository.CreateCasbinRule(ctx, TenantCasbinRule{
		Ptype: "p",
		V0:    fmt.Sprint(tenantID),
		V1:    fmt.Sprint(roleID),
		V2:    "*",
		V3:    "*",
	}); err != nil {
		return fmt.Errorf("创建租户通配策略失败: %w", err)
	}
	return nil
}
