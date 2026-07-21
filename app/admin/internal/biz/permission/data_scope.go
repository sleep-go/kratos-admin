// Package permission 提供 Casbin 功能权限之外的数据范围计算能力。
package permission

import "sort"

// DataScopeType 表示角色的数据可见范围。
type DataScopeType uint8

const (
	// DataScopeAll 表示租户内全部数据。
	DataScopeAll DataScopeType = 1
	// DataScopeDepartmentTree 表示本部门及其全部下级部门。
	DataScopeDepartmentTree DataScopeType = 2
	// DataScopeDepartment 表示仅本部门。
	DataScopeDepartment DataScopeType = 3
	// DataScopeSelf 表示仅当前成员创建的数据。
	DataScopeSelf DataScopeType = 4
	// DataScopeCustom 表示角色配置的自定义部门集合。
	DataScopeCustom DataScopeType = 5
)

// RoleDataScope 描述单个有效角色的数据范围配置。
type RoleDataScope struct {
	Type                DataScopeType
	PrimaryDepartmentID uint64
	DepartmentIDs       []uint64
}

// QueryDataScope 描述仓储查询实际应用的数据过滤范围。
type QueryDataScope struct {
	All               bool
	SelfOnly          bool
	DepartmentIDs     []uint64
	DescendantRootIDs []uint64
}

// ResolveDataScope 将多个角色的数据范围合并为最小限制的有效范围。
func ResolveDataScope(roles []RoleDataScope) QueryDataScope {
	departmentSet := make(map[uint64]struct{})
	descendantRootSet := make(map[uint64]struct{})
	result := QueryDataScope{SelfOnly: true}
	for _, role := range roles {
		switch role.Type {
		case DataScopeAll:
			return QueryDataScope{All: true}
		case DataScopeDepartmentTree:
			result.SelfOnly = false
			addDepartment(departmentSet, role.PrimaryDepartmentID)
			addDepartment(descendantRootSet, role.PrimaryDepartmentID)
		case DataScopeDepartment:
			result.SelfOnly = false
			addDepartment(departmentSet, role.PrimaryDepartmentID)
		case DataScopeCustom:
			result.SelfOnly = false
			for _, departmentID := range role.DepartmentIDs {
				addDepartment(departmentSet, departmentID)
			}
		case DataScopeSelf:
			// 仅本人不会收窄其他角色已经授予的部门范围。
		}
	}
	if len(departmentSet) == 0 {
		return result
	}
	result.DepartmentIDs = make([]uint64, 0, len(departmentSet))
	for departmentID := range departmentSet {
		result.DepartmentIDs = append(result.DepartmentIDs, departmentID)
	}
	sort.Slice(result.DepartmentIDs, func(i, j int) bool {
		return result.DepartmentIDs[i] < result.DepartmentIDs[j]
	})
	if len(descendantRootSet) > 0 {
		result.DescendantRootIDs = make([]uint64, 0, len(descendantRootSet))
		for departmentID := range descendantRootSet {
			result.DescendantRootIDs = append(result.DescendantRootIDs, departmentID)
		}
		sort.Slice(result.DescendantRootIDs, func(i, j int) bool {
			return result.DescendantRootIDs[i] < result.DescendantRootIDs[j]
		})
	}
	return result
}

func addDepartment(departments map[uint64]struct{}, departmentID uint64) {
	if departmentID != 0 {
		departments[departmentID] = struct{}{}
	}
}
