package service

import (
	"context"
	"fmt"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
	managementbiz "github.com/sleep-go/kratos-admin/internal/biz/management"
)

// GetEffectiveSettings 返回按代码默认、平台默认和租户覆盖解析后的有效设置。
func (s *ManagementService) GetEffectiveSettings(ctx context.Context, request *v1.GetEffectiveSettingsRequest) (*v1.GetEffectiveSettingsResponse, error) {
	scope, err := managementScope(ctx, "settings")
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, scope, "settings", "list"); err != nil {
		return nil, err
	}
	rows, err := s.repository.EffectiveSettings(ctx, scope, request.GetCategory())
	if err != nil {
		return nil, mapManagementError(err)
	}
	items := make([]*structpb.Struct, 0, len(rows))
	for _, row := range rows {
		item, conversionErr := structpb.NewStruct(normalizeStructValues(row))
		if conversionErr != nil {
			return nil, kratoserrors.InternalServer("MANAGEMENT_ENCODE_FAILED", "有效配置响应编码失败")
		}
		items = append(items, item)
	}
	return &v1.GetEffectiveSettingsResponse{Items: items}, nil
}

// TestProviderConnection 使用已保存的加密配置验证 Provider 连接。
func (s *ManagementService) TestProviderConnection(ctx context.Context, request *v1.TestProviderConnectionRequest) (*v1.TestProviderConnectionResponse, error) {
	scope, err := managementScope(ctx, "providers")
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, scope, "providers", "update"); err != nil {
		return nil, err
	}
	if request.GetId() == 0 {
		return nil, kratoserrors.BadRequest("MANAGEMENT_ID_REQUIRED", "Provider ID不能为空")
	}
	if err := s.repository.TestProviderConnection(ctx, scope, request.GetId()); err != nil {
		return nil, kratoserrors.BadRequest("PROVIDER_CONNECTION_FAILED", err.Error())
	}
	return &v1.TestProviderConnectionResponse{Success: true, Message: "连接测试成功"}, nil
}

// UpdateRoleAuthorization 在单个事务内替换角色资源授权与数据范围。
func (s *ManagementService) UpdateRoleAuthorization(ctx context.Context, request *v1.UpdateRoleAuthorizationRequest) (*v1.UpdateRoleAuthorizationResponse, error) {
	scope, err := managementScope(ctx, "roles")
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, scope, "roles", "update"); err != nil {
		return nil, err
	}
	if request.GetRoleId() == 0 {
		return nil, kratoserrors.BadRequest("ROLE_ID_REQUIRED", "角色ID不能为空")
	}
	grants := make([]managementbiz.RoleGrant, 0, len(request.GetGrants()))
	for _, grant := range request.GetGrants() {
		grants = append(grants, managementbiz.RoleGrant{ResourceCode: grant.GetResourceCode(), Actions: grant.GetActions()})
	}
	if err := s.repository.UpdateRoleAuthorization(ctx, scope, request.GetRoleId(), request.GetDataScope(), grants, request.GetDepartmentIds()); err != nil {
		return nil, mapManagementError(err)
	}
	return &v1.UpdateRoleAuthorizationResponse{RoleId: request.GetRoleId()}, nil
}

// UpdateTenantFeatures 在单个事务内替换目标租户功能授权。
func (s *ManagementService) UpdateTenantFeatures(ctx context.Context, request *v1.UpdateTenantFeaturesRequest) (*v1.UpdateTenantFeaturesResponse, error) {
	scope, err := managementScope(ctx, "tenant-resources")
	if err != nil {
		return nil, err
	}
	if request.GetTenantId() == 0 {
		return nil, kratoserrors.BadRequest("TENANT_ID_REQUIRED", "租户ID不能为空")
	}
	if err := s.repository.UpdateTenantFeatures(ctx, scope, request.GetTenantId(), request.GetResourceIds()); err != nil {
		return nil, mapManagementError(err)
	}
	return &v1.UpdateTenantFeaturesResponse{TenantId: request.GetTenantId()}, nil
}

// ManagementService 实现统一的后台资源管理 API。
type ManagementService struct {
	v1.UnimplementedManagementServiceServer
	repository  managementbiz.Repository
	permissions managementbiz.PermissionChecker
}

// NewManagementService 创建后台资源管理服务。
func NewManagementService(repository managementbiz.Repository, checkers ...managementbiz.PermissionChecker) *ManagementService {
	service := &ManagementService{repository: repository}
	if len(checkers) > 0 {
		service.permissions = checkers[0]
	}
	return service
}

// ListResources 按可信租户边界分页查询白名单资源。
func (s *ManagementService) ListResources(ctx context.Context, request *v1.ListResourcesRequest) (*v1.ListResourcesResponse, error) {
	scope, err := managementScope(ctx, request.GetResource())
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, scope, request.GetResource(), "list"); err != nil {
		return nil, err
	}
	page, pageSize := normalizePage(request.GetPage(), request.GetPageSize())
	filters := make(map[string]string, len(request.GetFilters()))
	for key, value := range request.GetFilters() {
		if key != "tenant_id" && key != "user_id" && key != "member_id" && key != "data_scope" {
			filters[key] = value
		}
	}
	rows, total, err := s.repository.List(ctx, scope, request.GetResource(), managementbiz.PageQuery{
		Page: page, PageSize: pageSize, Keyword: request.GetKeyword(), Sort: request.GetSort(), Filters: filters,
	})
	if err != nil {
		return nil, mapManagementError(err)
	}
	items := make([]*structpb.Struct, 0, len(rows))
	for _, row := range rows {
		item, conversionErr := structpb.NewStruct(normalizeStructValues(row))
		if conversionErr != nil {
			return nil, kratoserrors.InternalServer("MANAGEMENT_ENCODE_FAILED", "资源响应编码失败")
		}
		items = append(items, item)
	}
	return &v1.ListResourcesResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// CreateResource 在可信租户边界内创建白名单资源并写入审计 Outbox。
func (s *ManagementService) CreateResource(ctx context.Context, request *v1.CreateResourceRequest) (*v1.CreateResourceResponse, error) {
	scope, err := managementScope(ctx, request.GetResource())
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, scope, request.GetResource(), "create"); err != nil {
		return nil, err
	}
	id, err := s.repository.Create(ctx, scope, request.GetResource(), request.GetData().AsMap())
	if err != nil {
		return nil, mapManagementError(err)
	}
	return &v1.CreateResourceResponse{Id: id}, nil
}

// UpdateResource 在可信租户边界内更新白名单资源并写入审计 Outbox。
func (s *ManagementService) UpdateResource(ctx context.Context, request *v1.UpdateResourceRequest) (*v1.UpdateResourceResponse, error) {
	scope, err := managementScope(ctx, request.GetResource())
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, scope, request.GetResource(), "update"); err != nil {
		return nil, err
	}
	if request.GetId() == 0 {
		return nil, kratoserrors.BadRequest("MANAGEMENT_ID_REQUIRED", "资源ID不能为空")
	}
	if err := s.repository.Update(ctx, scope, request.GetResource(), request.GetId(), request.GetData().AsMap()); err != nil {
		return nil, mapManagementError(err)
	}
	return &v1.UpdateResourceResponse{Id: request.GetId()}, nil
}

// DeleteResource 在可信租户边界内逻辑删除白名单资源并写入审计 Outbox。
func (s *ManagementService) DeleteResource(ctx context.Context, request *v1.DeleteResourceRequest) (*v1.DeleteResourceResponse, error) {
	scope, err := managementScope(ctx, request.GetResource())
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, scope, request.GetResource(), "delete"); err != nil {
		return nil, err
	}
	if request.GetId() == 0 {
		return nil, kratoserrors.BadRequest("MANAGEMENT_ID_REQUIRED", "资源ID不能为空")
	}
	if err := s.repository.Delete(ctx, scope, request.GetResource(), request.GetId()); err != nil {
		return nil, mapManagementError(err)
	}
	return &v1.DeleteResourceResponse{Id: request.GetId()}, nil
}

func (s *ManagementService) authorize(ctx context.Context, scope managementbiz.Scope, resource, action string) error {
	if scope.PlatformAdmin || s.permissions == nil {
		return nil
	}
	allowed, err := s.permissions.Allowed(ctx, scope, resource, action)
	if err != nil {
		return kratoserrors.InternalServer("PERMISSION_CHECK_FAILED", "权限校验失败")
	}
	if !allowed {
		return kratoserrors.Forbidden("PERMISSION_DENIED", "没有执行该操作的权限")
	}
	return nil
}

func managementScope(ctx context.Context, resource string) (managementbiz.Scope, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok {
		return managementbiz.Scope{}, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	platformAdmin := claims.PlatformAdmin
	if (resource == "tenants" || resource == "resources" || resource == "tenant-resources") && !platformAdmin {
		return managementbiz.Scope{}, kratoserrors.Forbidden("PLATFORM_ADMIN_REQUIRED", "该资源仅限平台管理员")
	}
	return managementbiz.Scope{
		TenantID: claims.TenantID, UserID: claims.UserID, MemberID: claims.MemberID, PlatformAdmin: platformAdmin,
	}, nil
}

func normalizePage(page, pageSize uint32) (uint32, uint32) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func normalizeStructValues(row map[string]any) map[string]any {
	result := make(map[string]any, len(row))
	for key, value := range row {
		switch typed := value.(type) {
		case time.Time:
			result[key] = typed.UTC().Format(time.RFC3339Nano)
		case []byte:
			result[key] = string(typed)
		case nil, string, bool, float64:
			result[key] = typed
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			result[key] = fmt.Sprint(typed)
		default:
			result[key] = fmt.Sprint(typed)
		}
	}
	return result
}

func mapManagementError(err error) error {
	return kratoserrors.BadRequest("MANAGEMENT_REQUEST_INVALID", err.Error())
}

var _ v1.ManagementServiceServer = (*ManagementService)(nil)
