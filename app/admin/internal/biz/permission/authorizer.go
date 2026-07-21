package permission

import (
	"context"
	"fmt"
	"strconv"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

const casbinModel = `[request_definition]
r = dom, sub, obj, act

[policy_definition]
p = dom, sub, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.dom == p.dom && g(r.sub, p.sub, r.dom) && r.obj == p.obj && r.act == p.act
`

// Authorizer 使用 Casbin domain 模型实施租户隔离的功能权限。
type Authorizer struct {
	enforcer *casbin.SyncedEnforcer
}

// NewAuthorizer 创建空策略的多租户权限执行器。
func NewAuthorizer() (*Authorizer, error) {
	permissionModel, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("解析Casbin模型失败: %w", err)
	}
	enforcer, err := casbin.NewSyncedEnforcer(permissionModel)
	if err != nil {
		return nil, fmt.Errorf("创建Casbin执行器失败: %w", err)
	}
	return &Authorizer{enforcer: enforcer}, nil
}

// AddRoleForMember 在指定租户域内为成员分配角色。
func (a *Authorizer) AddRoleForMember(_ context.Context, tenantID, memberID, roleID uint64) error {
	_, err := a.enforcer.AddGroupingPolicy(id(memberID), id(roleID), id(tenantID))
	if err != nil {
		return fmt.Errorf("添加成员角色策略失败: %w", err)
	}
	return nil
}

// GrantRole 在指定租户域内为角色授予资源动作。
func (a *Authorizer) GrantRole(_ context.Context, tenantID, roleID uint64, resource, action string) error {
	_, err := a.enforcer.AddPolicy(id(tenantID), id(roleID), resource, action)
	if err != nil {
		return fmt.Errorf("添加角色权限策略失败: %w", err)
	}
	return nil
}

// Enforce 判断成员是否可以在租户域内执行资源动作。
func (a *Authorizer) Enforce(_ context.Context, tenantID, memberID uint64, resource, action string) (bool, error) {
	allowed, err := a.enforcer.Enforce(id(tenantID), id(memberID), resource, action)
	if err != nil {
		return false, fmt.Errorf("执行权限校验失败: %w", err)
	}
	return allowed, nil
}

func id(value uint64) string {
	return strconv.FormatUint(value, 10)
}
