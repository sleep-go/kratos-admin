// Package model 定义与 Goose 迁移保持一致的 GORM 数据模型。
package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Tenant 表示平台管理的租户。
type Tenant struct {
	ID                uint64         `gorm:"primaryKey"`
	Code              string         `gorm:"size:64;not null"`
	Name              string         `gorm:"size:128;not null"`
	Status            uint8          `gorm:"not null"`
	PermissionVersion uint64         `gorm:"not null"`
	CreatedBy         uint64         `gorm:"not null"`
	CreatedAt         time.Time      `gorm:"not null"`
	UpdatedAt         time.Time      `gorm:"not null"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

// TableName 返回租户表名。
func (Tenant) TableName() string { return "tenants" }

// User 表示跨租户共享的全局用户。
type User struct {
	ID                uint64  `gorm:"primaryKey"`
	Username          string  `gorm:"size:64;not null"`
	Email             *string `gorm:"size:191"`
	Phone             *string `gorm:"size:32"`
	PasswordHash      string  `gorm:"size:255;not null"`
	DisplayName       string  `gorm:"size:128;not null"`
	AvatarURL         *string `gorm:"type:text"`
	IsPlatformAdmin   bool    `gorm:"not null"`
	Status            uint8   `gorm:"not null"`
	FailedLoginCount  uint32  `gorm:"not null"`
	LockedUntil       *time.Time
	EmailVerifiedAt   *time.Time
	PhoneVerifiedAt   *time.Time
	MFAEnabled        bool           `gorm:"not null"`
	MFAChannel        string         `gorm:"size:16;not null;default:email"`
	PasswordChangedAt time.Time      `gorm:"not null"`
	CreatedAt         time.Time      `gorm:"not null"`
	UpdatedAt         time.Time      `gorm:"not null"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

// TableName 返回用户表名。
func (User) TableName() string { return "users" }

// Department 表示租户内的树形部门。
type Department struct {
	ID        uint64         `gorm:"primaryKey"`
	TenantID  uint64         `gorm:"not null"`
	ParentID  uint64         `gorm:"not null"`
	Name      string         `gorm:"size:128;not null"`
	Code      string         `gorm:"size:64;not null"`
	Path      string         `gorm:"size:1024;not null"`
	SortOrder int            `gorm:"not null"`
	Status    uint8          `gorm:"not null"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 返回部门表名。
func (Department) TableName() string { return "departments" }

// Position 表示租户内的岗位。
type Position struct {
	ID        uint64         `gorm:"primaryKey"`
	TenantID  uint64         `gorm:"not null"`
	Code      string         `gorm:"size:64;not null"`
	Name      string         `gorm:"size:128;not null"`
	SortOrder int            `gorm:"not null"`
	Status    uint8          `gorm:"not null"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 返回岗位表名。
func (Position) TableName() string { return "positions" }

// TenantMember 表示用户在租户中的成员身份。
type TenantMember struct {
	ID                  uint64         `gorm:"primaryKey"`
	TenantID            uint64         `gorm:"not null"`
	UserID              uint64         `gorm:"not null"`
	PrimaryDepartmentID uint64         `gorm:"not null"`
	PositionID          uint64         `gorm:"not null"`
	DisplayName         string         `gorm:"size:128;not null"`
	Status              uint8          `gorm:"not null"`
	IsTenantAdmin       bool           `gorm:"not null"`
	JoinedAt            time.Time      `gorm:"not null"`
	CreatedAt           time.Time      `gorm:"not null"`
	UpdatedAt           time.Time      `gorm:"not null"`
	DeletedAt           gorm.DeletedAt `gorm:"index"`
}

// TableName 返回租户成员表名。
func (TenantMember) TableName() string { return "tenant_members" }

// MemberDepartment 表示成员与部门的多对多关系。
type MemberDepartment struct {
	ID           uint64    `gorm:"primaryKey"`
	TenantID     uint64    `gorm:"not null"`
	MemberID     uint64    `gorm:"not null"`
	DepartmentID uint64    `gorm:"not null"`
	IsPrimary    bool      `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

// TableName 返回成员部门关系表名。
func (MemberDepartment) TableName() string { return "member_departments" }

// Role 表示租户或平台角色及其数据范围。
type Role struct {
	ID        uint64         `gorm:"primaryKey"`
	TenantID  uint64         `gorm:"not null"`
	Code      string         `gorm:"size:64;not null"`
	Name      string         `gorm:"size:128;not null"`
	DataScope uint8          `gorm:"not null"`
	IsBuiltin bool           `gorm:"not null"`
	Status    uint8          `gorm:"not null"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 返回角色表名。
func (Role) TableName() string { return "roles" }

// Resource 表示目录、菜单、按钮或 API 权限资源。
type Resource struct {
	ID           uint64         `gorm:"primaryKey"`
	ParentID     uint64         `gorm:"not null"`
	Type         uint8          `gorm:"not null"`
	Code         string         `gorm:"size:128;not null"`
	Name         string         `gorm:"size:128;not null"`
	RoutePath    string         `gorm:"size:255;not null"`
	ComponentKey string         `gorm:"size:128;not null"`
	HTTPMethod   string         `gorm:"size:16;not null"`
	APIPath      string         `gorm:"size:255;not null"`
	Icon         string         `gorm:"size:64;not null"`
	SortOrder    int            `gorm:"not null"`
	Visible      bool           `gorm:"not null"`
	Status       uint8          `gorm:"not null"`
	CreatedAt    time.Time      `gorm:"not null"`
	UpdatedAt    time.Time      `gorm:"not null"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// TableName 返回权限资源表名。
func (Resource) TableName() string { return "resources" }

// TenantResource 表示平台授予租户的可用功能。
type TenantResource struct {
	ID         uint64    `gorm:"primaryKey"`
	TenantID   uint64    `gorm:"not null"`
	ResourceID uint64    `gorm:"not null"`
	CreatedBy  uint64    `gorm:"not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

// TableName 返回租户资源授权表名。
func (TenantResource) TableName() string { return "tenant_resources" }

// CasbinRule 表示按租户域隔离的 Casbin 策略。
type CasbinRule struct {
	ID    uint64 `gorm:"primaryKey"`
	Ptype string `gorm:"size:16;not null"`
	V0    string `gorm:"size:191;not null"`
	V1    string `gorm:"size:191;not null"`
	V2    string `gorm:"size:191;not null"`
	V3    string `gorm:"size:191;not null"`
	V4    string `gorm:"size:191;not null"`
	V5    string `gorm:"size:191;not null"`
}

// TableName 返回 Casbin 策略表名。
func (CasbinRule) TableName() string { return "casbin_rules" }

// RoleScopeDepartment 表示角色的自定义部门数据范围。
type RoleScopeDepartment struct {
	ID           uint64    `gorm:"primaryKey"`
	TenantID     uint64    `gorm:"not null"`
	RoleID       uint64    `gorm:"not null"`
	DepartmentID uint64    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

// TableName 返回角色部门范围表名。
func (RoleScopeDepartment) TableName() string { return "role_scope_departments" }

// AuthSession 表示服务端可撤销的 refresh 会话。
type AuthSession struct {
	ID                string    `gorm:"type:char(36);primaryKey"`
	UserID            uint64    `gorm:"not null"`
	TenantID          uint64    `gorm:"not null"`
	MemberID          uint64    `gorm:"not null"`
	PermissionVersion uint64    `gorm:"not null"`
	RefreshJTIHash    string    `gorm:"type:char(64);not null"`
	DeviceName        string    `gorm:"size:128;not null"`
	UserAgent         string    `gorm:"size:512;not null"`
	IP                string    `gorm:"size:64;not null"`
	ExpiresAt         time.Time `gorm:"not null"`
	RevokedAt         *time.Time
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

// TableName 返回认证会话表名。
func (AuthSession) TableName() string { return "auth_sessions" }

// VerificationCode 表示 MFA 或密码重置验证码。
type VerificationCode struct {
	ID           uint64    `gorm:"primaryKey"`
	UserID       uint64    `gorm:"not null"`
	Target       string    `gorm:"size:191;not null"`
	Scene        string    `gorm:"size:32;not null"`
	Channel      string    `gorm:"size:16;not null"`
	CodeHash     string    `gorm:"type:char(64);not null"`
	AttemptCount uint32    `gorm:"not null"`
	ExpiresAt    time.Time `gorm:"not null"`
	ConsumedAt   *time.Time
	ContextData  datatypes.JSON
	CreatedAt    time.Time `gorm:"not null"`
}

// TableName 返回验证码表名。
func (VerificationCode) TableName() string { return "verification_codes" }

// LoginLog 表示认证安全日志。
type LoginLog struct {
	ID         uint64    `gorm:"primaryKey"`
	TenantID   uint64    `gorm:"not null"`
	UserID     uint64    `gorm:"not null"`
	Identifier string    `gorm:"size:191;not null"`
	Result     uint8     `gorm:"not null"`
	Reason     string    `gorm:"size:128;not null"`
	IP         string    `gorm:"size:64;not null"`
	UserAgent  string    `gorm:"size:512;not null"`
	RequestID  string    `gorm:"size:64;not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

// TableName 返回登录日志表名。
func (LoginLog) TableName() string { return "login_logs" }

// AuditOutbox 表示与业务事务同写的审计事件。
type AuditOutbox struct {
	ID            string         `gorm:"type:char(36);primaryKey"`
	TenantID      uint64         `gorm:"not null"`
	EventType     string         `gorm:"size:64;not null"`
	AggregateType string         `gorm:"size:64;not null"`
	AggregateID   string         `gorm:"size:64;not null"`
	Payload       datatypes.JSON `gorm:"type:json;not null"`
	Status        uint8          `gorm:"not null"`
	RetryCount    uint32         `gorm:"not null"`
	NextRetryAt   *time.Time
	DispatchedAt  *time.Time
	LastError     string `gorm:"size:1024;not null"`
	PublishedAt   *time.Time
	CreatedAt     time.Time `gorm:"not null"`
}

// TableName 返回审计 Outbox 表名。
func (AuditOutbox) TableName() string { return "audit_outbox" }

// AuditLog 表示 Worker 幂等生成的操作审计。
type AuditLog struct {
	ID           uint64         `gorm:"primaryKey"`
	EventID      string         `gorm:"type:char(36);not null"`
	TenantID     uint64         `gorm:"not null"`
	UserID       uint64         `gorm:"not null"`
	MemberID     uint64         `gorm:"not null"`
	Action       string         `gorm:"size:64;not null"`
	ResourceType string         `gorm:"size:64;not null"`
	ResourceID   string         `gorm:"size:64;not null"`
	Summary      string         `gorm:"size:512;not null"`
	BeforeData   datatypes.JSON `gorm:"type:json"`
	AfterData    datatypes.JSON `gorm:"type:json"`
	IP           string         `gorm:"size:64;not null"`
	UserAgent    string         `gorm:"size:512;not null"`
	RequestID    string         `gorm:"size:64;not null"`
	CreatedAt    time.Time      `gorm:"not null"`
}

// TableName 返回操作审计表名。
func (AuditLog) TableName() string { return "audit_logs" }

// APIAccessLog 表示 API 访问与异常日志。
type APIAccessLog struct {
	ID          uint64         `gorm:"primaryKey"`
	TenantID    uint64         `gorm:"not null"`
	UserID      uint64         `gorm:"not null"`
	RequestID   string         `gorm:"size:64;not null"`
	Method      string         `gorm:"size:16;not null"`
	Route       string         `gorm:"size:255;not null"`
	StatusCode  uint16         `gorm:"not null"`
	DurationMS  uint32         `gorm:"not null"`
	IP          string         `gorm:"size:64;not null"`
	UserAgent   string         `gorm:"size:512;not null"`
	RequestData datatypes.JSON `gorm:"type:json"`
	ErrorReason string         `gorm:"size:128;not null"`
	CreatedAt   time.Time      `gorm:"not null"`
}

// TableName 返回 API 访问日志表名。
func (APIAccessLog) TableName() string { return "api_access_logs" }

// SystemSetting 表示平台或租户作用域系统配置。
type SystemSetting struct {
	ID                  uint64         `gorm:"primaryKey"`
	TenantID            uint64         `gorm:"not null"`
	Category            string         `gorm:"size:64;not null"`
	SettingKey          string         `gorm:"size:128;not null"`
	ValueType           string         `gorm:"size:16;not null"`
	SettingValue        datatypes.JSON `gorm:"type:json;not null"`
	AllowTenantOverride bool           `gorm:"not null"`
	IsSecret            bool           `gorm:"not null"`
	Version             uint64         `gorm:"not null"`
	UpdatedBy           uint64         `gorm:"not null"`
	CreatedAt           time.Time      `gorm:"not null"`
	UpdatedAt           time.Time      `gorm:"not null"`
}

// TableName 返回系统配置表名。
func (SystemSetting) TableName() string { return "system_settings" }

// DictionaryType 表示平台或租户字典类型。
type DictionaryType struct {
	ID        uint64         `gorm:"primaryKey"`
	TenantID  uint64         `gorm:"not null"`
	Code      string         `gorm:"size:64;not null"`
	Name      string         `gorm:"size:128;not null"`
	Status    uint8          `gorm:"not null"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 返回字典类型表名。
func (DictionaryType) TableName() string { return "dictionary_types" }

// DictionaryItem 表示字典类型下的可选项。
type DictionaryItem struct {
	ID        uint64         `gorm:"primaryKey"`
	TenantID  uint64         `gorm:"not null"`
	TypeID    uint64         `gorm:"not null"`
	ItemValue string         `gorm:"size:191;not null"`
	Label     string         `gorm:"size:128;not null"`
	SortOrder int            `gorm:"not null"`
	Status    uint8          `gorm:"not null"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 返回字典项表名。
func (DictionaryItem) TableName() string { return "dictionary_items" }

// ProviderConfig 表示加密保存的邮件、短信或存储 Provider 配置。
type ProviderConfig struct {
	ID              uint64    `gorm:"primaryKey"`
	TenantID        uint64    `gorm:"not null"`
	ProviderType    string    `gorm:"size:32;not null"`
	ProviderName    string    `gorm:"size:64;not null"`
	DisplayName     string    `gorm:"size:128;not null"`
	EncryptedConfig string    `gorm:"type:text;not null"`
	Status          uint8     `gorm:"not null"`
	IsDefault       bool      `gorm:"not null"`
	UpdatedBy       uint64    `gorm:"not null"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

// TableName 返回 Provider 配置表名。
func (ProviderConfig) TableName() string { return "provider_configs" }

// File 表示租户文件的元数据。
type File struct {
	ID                   string `gorm:"type:char(36);primaryKey"`
	TenantID             uint64 `gorm:"not null"`
	UploaderMemberID     uint64 `gorm:"not null"`
	ProviderName         string `gorm:"size:64;not null"`
	ObjectKey            string `gorm:"size:512;not null"`
	OriginalName         string `gorm:"size:255;not null"`
	ContentType          string `gorm:"size:128;not null"`
	SizeBytes            uint64 `gorm:"not null"`
	SHA256               string `gorm:"type:char(64);not null"`
	ETag                 string `gorm:"column:etag;size:191;not null"`
	Status               uint8  `gorm:"not null"`
	CleanupDispatchedAt  *time.Time
	CleanupRetryCount    uint32 `gorm:"not null"`
	CleanupNextRetryAt   *time.Time
	CleanupFailureReason string         `gorm:"size:1024;not null"`
	CreatedAt            time.Time      `gorm:"not null"`
	UpdatedAt            time.Time      `gorm:"not null"`
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

// TableName 返回文件元数据表名。
func (File) TableName() string { return "files" }

// FileReference 表示文件与业务资源的引用关系。
type FileReference struct {
	ID           uint64    `gorm:"primaryKey"`
	TenantID     uint64    `gorm:"not null"`
	FileID       string    `gorm:"type:char(36);not null"`
	BusinessType string    `gorm:"size:64;not null"`
	BusinessID   string    `gorm:"size:64;not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

// TableName 返回文件引用表名。
func (FileReference) TableName() string { return "file_references" }

// FailedTask 表示超过重试次数的异步任务。
type FailedTask struct {
	ID             string         `gorm:"type:char(36);primaryKey"`
	TenantID       uint64         `gorm:"not null"`
	TaskType       string         `gorm:"size:64;not null"`
	PayloadVersion uint16         `gorm:"not null"`
	IdempotencyKey string         `gorm:"size:191;not null"`
	Payload        datatypes.JSON `gorm:"type:json;not null"`
	RetryCount     uint32         `gorm:"not null"`
	LastError      string         `gorm:"size:1024;not null"`
	NextRetryAt    *time.Time
	Status         uint8     `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

// TableName 返回失败任务表名。
func (FailedTask) TableName() string { return "failed_tasks" }

// LogExport 表示最多导出十万条日志的异步任务。
type LogExport struct {
	ID             string         `gorm:"type:char(36);primaryKey"`
	TenantID       uint64         `gorm:"not null"`
	UserID         uint64         `gorm:"not null"`
	MemberID       uint64         `gorm:"not null"`
	LogType        string         `gorm:"size:16;not null"`
	Keyword        string         `gorm:"size:191;not null"`
	Filters        datatypes.JSON `gorm:"type:json"`
	PayloadVersion uint16         `gorm:"not null"`
	IdempotencyKey string         `gorm:"size:191;not null"`
	Status         uint8          `gorm:"not null"`
	RowCount       uint32         `gorm:"not null"`
	FileID         string         `gorm:"type:char(36);not null"`
	RetryCount     uint32         `gorm:"not null"`
	NextRetryAt    *time.Time
	DispatchedAt   *time.Time
	FailureReason  string    `gorm:"size:1024;not null"`
	CreatedAt      time.Time `gorm:"not null"`
	StartedAt      *time.Time
	FinishedAt     *time.Time
	UpdatedAt      time.Time `gorm:"not null"`
}

// TableName 返回异步日志导出任务表名。
func (LogExport) TableName() string { return "log_exports" }
