package data

import (
	"log"
	"os"

	"dps/internal/conf"
	"dps/internal/data/model"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProviderSet 是 data 层的 Wire Provider 集合。
var ProviderSet = wire.NewSet(NewData, NewAdminRepo)

// Data 持有数据库等存储客户端。
type Data struct {
	db *gorm.DB
}

// NewData 创建 Data 实例，初始化 GORM 数据库连接。
// DEPLOY_ENV=dev 时启用调试日志并自动迁移表结构。
func NewData(c *conf.Data) (*Data, func(), error) {
	db, err := gorm.Open(mysql.Open(c.Database.Source), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed opening connection to database: %v", err)
	}
	if os.Getenv("DEPLOY_ENV") == "dev" {
		db = db.Debug()
		if err = db.AutoMigrate(&model.Admin{}); err != nil {
			return nil, nil, err
		}
	}
	cleanup := func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}
	return &Data{db: db}, cleanup, nil
}
