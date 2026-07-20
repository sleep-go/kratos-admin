package permission

import (
	"reflect"
	"testing"
)

func TestResolveDataScope(t *testing.T) {
	tests := []struct {
		name  string
		roles []RoleDataScope
		want  QueryDataScope
	}{
		{
			name:  "全部数据优先于其他范围",
			roles: []RoleDataScope{{Type: DataScopeSelf}, {Type: DataScopeAll}},
			want:  QueryDataScope{All: true},
		},
		{
			name: "部门范围与自定义部门取并集",
			roles: []RoleDataScope{
				{Type: DataScopeDepartment, PrimaryDepartmentID: 10},
				{Type: DataScopeCustom, DepartmentIDs: []uint64{20, 10}},
			},
			want: QueryDataScope{DepartmentIDs: []uint64{10, 20}},
		},
		{
			name:  "本部门及下级保留层级扩展标记",
			roles: []RoleDataScope{{Type: DataScopeDepartmentTree, PrimaryDepartmentID: 10}},
			want:  QueryDataScope{DepartmentIDs: []uint64{10}, DescendantRootIDs: []uint64{10}},
		},
		{
			name: "层级部门不会错误扩展自定义部门",
			roles: []RoleDataScope{
				{Type: DataScopeDepartmentTree, PrimaryDepartmentID: 10},
				{Type: DataScopeCustom, DepartmentIDs: []uint64{20}},
			},
			want: QueryDataScope{DepartmentIDs: []uint64{10, 20}, DescendantRootIDs: []uint64{10}},
		},
		{
			name:  "只有本人权限时限制创建人",
			roles: []RoleDataScope{{Type: DataScopeSelf}},
			want:  QueryDataScope{SelfOnly: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDataScope(tt.roles)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveDataScope() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
