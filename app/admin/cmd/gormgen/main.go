// Command gormgen 基于迁移对应模型生成类型安全的 GORM Gen 查询代码。
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gorm.io/gen"

	"github.com/sleep-go/kratos-admin/internal/data/model"
)

type genOptions struct {
	OutPath string
}

func main() {
	if err := newRootCommand(run).ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(run func(context.Context, genOptions) error) *cobra.Command {
	options := genOptions{OutPath: "internal/data/query"}
	cmd := &cobra.Command{
		Use:           "admin-gormgen",
		Short:         "生成 GORM Gen 类型安全查询代码",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), options)
		},
	}
	cmd.Flags().StringVar(&options.OutPath, "out-path", options.OutPath, "查询代码输出目录")
	return cmd
}

func run(_ context.Context, options genOptions) error {
	generator := gen.NewGenerator(gen.Config{
		OutPath:      options.OutPath,
		ModelPkgPath: "github.com/sleep-go/kratos-admin/internal/data/model",
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
	return nil
}
