package data

import "testing"

func TestManagementGenRegistryCoversResources(t *testing.T) {
	for resource, definition := range managementResources {
		registered, ok := managementAdapterFor(resource)
		if !ok {
			t.Fatalf("资源 %s 未注册 Gen 适配器", resource)
		}
		for _, column := range definition.columns {
			if !registered.SupportsField(column) {
				t.Fatalf("资源 %s 缺少字段 %s", resource, column)
			}
		}
	}
}
