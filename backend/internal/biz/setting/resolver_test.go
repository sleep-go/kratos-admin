package setting

import (
	"encoding/json"
	"testing"
)

func TestResolveUsesTenantOverrideOnlyWhenPlatformAllowsIt(t *testing.T) {
	codeDefault := json.RawMessage(`"Kratos Admin"`)
	platform := &Value{Raw: json.RawMessage(`"平台名称"`), AllowTenantOverride: true}
	tenant := &Value{Raw: json.RawMessage(`"租户名称"`)}

	resolved := Resolve(codeDefault, platform, tenant)
	if resolved.Source != SourceTenant || string(resolved.Raw) != `"租户名称"` {
		t.Fatalf("Resolve() = %+v", resolved)
	}

	platform.AllowTenantOverride = false
	resolved = Resolve(codeDefault, platform, tenant)
	if resolved.Source != SourcePlatform || string(resolved.Raw) != `"平台名称"` {
		t.Fatalf("Resolve() = %+v", resolved)
	}
}

func TestResolveFallsBackFromPlatformToCodeDefault(t *testing.T) {
	codeDefault := json.RawMessage(`30`)
	resolved := Resolve(codeDefault, nil, nil)
	if resolved.Source != SourceCode || string(resolved.Raw) != `30` {
		t.Fatalf("Resolve() = %+v", resolved)
	}
}
