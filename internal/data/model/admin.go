package model

import "time"

// Admin 管理员表的持久化对象（PO）。
type Admin struct {
	ID         int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Name       string    `gorm:"column:name;type:varchar(255);default:''"`
	Email      string    `gorm:"column:email;type:varchar(255);default:''"`
	Avatar     string    `gorm:"column:avatar;type:varchar(500);default:''"`
	Access     string    `gorm:"column:access;type:varchar(50);default:''"`
	Password   string    `gorm:"column:password;type:varchar(255);default:''"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateTime time.Time `gorm:"column:update_time"`
}

// TableName 指定表名。
func (Admin) TableName() string {
	return "admins"
}
