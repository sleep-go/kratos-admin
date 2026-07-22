package data

type managementResourceAdapter struct {
	Fields map[string]struct{}
}

func (a managementResourceAdapter) SupportsField(name string) bool {
	_, ok := a.Fields[name]
	return ok
}

var managementGenAdapters = map[string]managementResourceAdapter{
	"app-users":              {Fields: fieldSet("id", "username", "email", "phone", "display_name", "status", "mfa_enabled", "mfa_channel", "created_at", "updated_at")},
	"platform-admins":        {Fields: fieldSet("id", "username", "email", "phone", "display_name", "status", "is_super_admin", "mfa_enabled", "mfa_channel", "created_at", "updated_at")},
	"tenant-admins":          {Fields: fieldSet("id", "tenant_id", "username", "email", "phone", "display_name", "status", "mfa_enabled", "mfa_channel", "created_at", "updated_at")},
	"tenants":                {Fields: fieldSet("id", "code", "name", "status", "permission_version", "created_at", "updated_at")},
	"roles":                  {Fields: fieldSet("id", "tenant_id", "code", "name", "data_scope", "is_builtin", "status", "created_at", "updated_at")},
	"platform-roles":         {Fields: fieldSet("id", "tenant_id", "code", "name", "data_scope", "is_builtin", "status", "created_at", "updated_at")},
	"resources":              {Fields: fieldSet("id", "parent_id", "type", "scope_mask", "code", "name", "route_path", "component_key", "http_method", "api_path", "icon", "sort_order", "visible", "status")},
	"tenant-resources":       {Fields: fieldSet("id", "tenant_id", "resource_id", "created_by", "created_at")},
	"casbin-rules":           {Fields: fieldSet("id", "ptype", "v0", "v1", "v2", "v3", "v4", "v5")},
	"platform-casbin-rules":  {Fields: fieldSet("id", "ptype", "v0", "v1", "v2", "v3", "v4", "v5")},
	"login-logs":             {Fields: fieldSet("id", "tenant_id", "user_id", "identifier", "result", "reason", "ip", "user_agent", "request_id", "created_at")},
	"audit-logs":             {Fields: fieldSet("id", "event_id", "tenant_id", "user_id", "member_id", "action", "resource_type", "resource_id", "summary", "ip", "user_agent", "request_id", "created_at")},
	"api-logs":               {Fields: fieldSet("id", "tenant_id", "user_id", "request_id", "method", "route", "status_code", "duration_ms", "ip", "user_agent", "error_reason", "created_at")},
	"log-exports":            {Fields: fieldSet("id", "tenant_id", "user_id", "log_type", "status", "row_count", "file_id", "retry_count", "failure_reason", "created_at", "finished_at")},
	"platform-login-logs":   {Fields: fieldSet("id", "tenant_id", "user_id", "identifier", "result", "reason", "ip", "user_agent", "request_id", "created_at")},
	"platform-audit-logs":   {Fields: fieldSet("id", "event_id", "tenant_id", "user_id", "member_id", "action", "resource_type", "resource_id", "summary", "ip", "user_agent", "request_id", "created_at")},
	"platform-api-logs":     {Fields: fieldSet("id", "tenant_id", "user_id", "request_id", "method", "route", "status_code", "duration_ms", "ip", "user_agent", "error_reason", "created_at")},
	"platform-log-exports":  {Fields: fieldSet("id", "tenant_id", "user_id", "log_type", "status", "row_count", "file_id", "retry_count", "failure_reason", "created_at", "finished_at")},
	"settings":               {Fields: fieldSet("id", "tenant_id", "category", "setting_key", "value_type", "setting_value", "allow_tenant_override", "is_secret", "version", "updated_by", "created_at", "updated_at")},
	"dictionary-types":       {Fields: fieldSet("id", "tenant_id", "code", "name", "status", "created_at", "updated_at")},
	"dictionary-items":       {Fields: fieldSet("id", "tenant_id", "type_id", "item_value", "label", "sort_order", "status", "created_at", "updated_at")},
	"providers":              {Fields: fieldSet("id", "tenant_id", "provider_type", "provider_name", "display_name", "encrypted_config", "status", "is_default", "updated_by", "created_at", "updated_at")},
	"files":                  {Fields: fieldSet("id", "tenant_id", "uploader_id", "provider_name", "object_key", "original_name", "content_type", "size_bytes", "sha256", "status", "created_at")},
}

func managementAdapterFor(resource string) (managementResourceAdapter, bool) {
	adapter, ok := managementGenAdapters[resource]
	return adapter, ok
}
