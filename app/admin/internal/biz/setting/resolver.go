// Package setting 提供平台与租户系统配置的三级解析规则。
package setting

import "encoding/json"

// Source 表示最终配置值来源。
type Source string

const (
	// SourceCode 表示代码内置安全默认值。
	SourceCode Source = "code"
	// SourcePlatform 表示平台配置值。
	SourcePlatform Source = "platform"
	// SourceTenant 表示租户覆盖值。
	SourceTenant Source = "tenant"
)

// Value 表示某一作用域保存的配置值及覆盖策略。
type Value struct {
	Raw                 json.RawMessage
	AllowTenantOverride bool
}

// ResolvedValue 表示完成三级解析后的配置值和来源。
type ResolvedValue struct {
	Raw    json.RawMessage
	Source Source
}

// Resolve 按代码默认值、平台默认值、允许的租户覆盖值顺序解析最终配置。
func Resolve(codeDefault json.RawMessage, platform, tenant *Value) ResolvedValue {
	resolved := ResolvedValue{Raw: codeDefault, Source: SourceCode}
	if platform == nil {
		return resolved
	}
	resolved = ResolvedValue{Raw: platform.Raw, Source: SourcePlatform}
	if platform.AllowTenantOverride && tenant != nil {
		resolved = ResolvedValue{Raw: tenant.Raw, Source: SourceTenant}
	}
	return resolved
}
