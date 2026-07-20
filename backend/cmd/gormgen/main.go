// Command gormgen 基于迁移对应模型生成类型安全的 GORM Gen 查询代码。
package main

import (
	"gorm.io/gen"

	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
)

func main() {
	generator := gen.NewGenerator(gen.Config{
		OutPath:      "backend/internal/data/query",
		ModelPkgPath: "github.com/sleep-go/kratos-admin/backend/internal/data/model",
		Mode:         gen.WithDefaultQuery | gen.WithQueryInterface,
	})
	generator.ApplyBasic(
		model.Tenant{}, model.User{}, model.Department{}, model.Position{},
		model.TenantMember{}, model.MemberDepartment{}, model.Role{}, model.Resource{},
		model.TenantResource{}, model.CasbinRule{}, model.RoleScopeDepartment{},
		model.AuthSession{}, model.VerificationCode{}, model.LoginLog{}, model.AuditOutbox{},
		model.AuditLog{}, model.APIAccessLog{}, model.SystemSetting{}, model.DictionaryType{},
		model.DictionaryItem{}, model.ProviderConfig{}, model.File{}, model.FileReference{},
		model.FailedTask{}, model.LogExport{},
	)
	generator.Execute()
}
