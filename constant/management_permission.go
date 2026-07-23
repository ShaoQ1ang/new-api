package constant

const (
	PermissionSkillHubContentManage = "skill_hub.content.manage"
	PermissionSkillHubReportsManage = "skill_hub.reports.manage"
	PermissionChatModelsManage      = "chat_models.manage"
	PermissionClientReleasesManage  = "client_releases.manage"
	PermissionClientReleasesPublish = "client_releases.publish"
)

var managementPermissions = []string{
	PermissionSkillHubContentManage,
	PermissionSkillHubReportsManage,
	PermissionChatModelsManage,
	PermissionClientReleasesManage,
	PermissionClientReleasesPublish,
}

var managementPermissionSet = map[string]struct{}{
	PermissionSkillHubContentManage: {},
	PermissionSkillHubReportsManage: {},
	PermissionChatModelsManage:      {},
	PermissionClientReleasesManage:  {},
	PermissionClientReleasesPublish: {},
}

func IsValidManagementPermission(permission string) bool {
	_, ok := managementPermissionSet[permission]
	return ok
}

func AllManagementPermissions() []string {
	return append([]string(nil), managementPermissions...)
}
