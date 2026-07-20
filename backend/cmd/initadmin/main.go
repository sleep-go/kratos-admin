// Command initadmin 幂等创建首个平台超级管理员。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/internal/biz/setup"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("初始化平台管理员失败: %v", err)
	}
}

func run(ctx context.Context) error {
	username := flag.String("username", "admin", "平台管理员用户名")
	displayName := flag.String("display-name", "超级管理员", "平台管理员显示名称")
	email := flag.String("email", "", "平台管理员邮箱")
	phone := flag.String("phone", "", "平台管理员手机号")
	flag.Parse()
	password := os.Getenv("KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD")
	if password == "" {
		return fmt.Errorf("必须通过 KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD 提供初始密码")
	}
	cfg, err := conf.LoadFromEnv()
	if err != nil {
		return err
	}
	db, err := data.OpenMySQL(ctx, cfg.Data.MySQLDSN)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	initializer := setup.NewAdminInitializer(data.NewAdminRepository(db), bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()))
	created, err := initializer.Ensure(ctx, setup.AdminInput{
		Username: *username, DisplayName: *displayName, Email: *email, Phone: *phone, Password: password,
	})
	if err != nil {
		return err
	}
	if created {
		log.Printf("平台管理员 %s 创建成功", *username)
	} else {
		log.Printf("平台管理员 %s 已存在，无需重复创建", *username)
	}
	return nil
}
