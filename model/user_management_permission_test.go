package model

import (
	"errors"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupManagementPermissionTestDB(t *testing.T) {
	t.Helper()

	previousDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}, &UserManagementPermission{}, &ClientRelease{}))

	DB = db
	t.Cleanup(func() {
		DB = previousDB
		_ = sqlDB.Close()
	})
}

func createManagementPermissionTestUser(t *testing.T, username string, role int) *User {
	t.Helper()
	user := &User{
		Username: username,
		Password: "test-password",
		AffCode:  username + "-aff",
		Role:     role,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func createManagementPermissionTestRoot(t *testing.T, username string) *User {
	t.Helper()
	return createManagementPermissionTestUser(t, username, common.RoleRootUser)
}

func TestReplaceUserManagementPermissions(t *testing.T) {
	setupManagementPermissionTestDB(t)
	user := createManagementPermissionTestUser(t, "permission-user", common.RoleCommonUser)
	root := createManagementPermissionTestRoot(t, "permission-root")

	previous, current, err := ReplaceUserManagementPermissions(user.Id, []string{
		constant.PermissionClientReleasesPublish,
		constant.PermissionSkillHubContentManage,
		constant.PermissionClientReleasesPublish,
	}, root.Id)
	require.NoError(t, err)
	require.Empty(t, previous)
	require.Equal(t, []string{
		constant.PermissionSkillHubContentManage,
		constant.PermissionClientReleasesPublish,
	}, current)

	state, err := GetUserManagementPermissionState(user.Id)
	require.NoError(t, err)
	require.Equal(t, current, state.Assigned)
	require.Equal(t, current, state.Effective)

	_, _, err = ReplaceUserManagementPermissions(user.Id, []string{"unknown.manage"}, root.Id)
	require.ErrorIs(t, err, ErrInvalidManagementPermission)

	state, err = GetUserManagementPermissionState(user.Id)
	require.NoError(t, err)
	require.Equal(t, current, state.Assigned, "invalid replacement must not mutate existing grants")
}

func TestManagementPermissionsRequireEnabledRootGrantor(t *testing.T) {
	setupManagementPermissionTestDB(t)
	user := createManagementPermissionTestUser(t, "grant-target", common.RoleCommonUser)
	admin := createManagementPermissionTestUser(t, "grant-admin", common.RoleAdminUser)
	root := createManagementPermissionTestRoot(t, "disabled-grant-root")

	_, _, err := ReplaceUserManagementPermissions(user.Id, []string{
		constant.PermissionChatModelsManage,
	}, admin.Id)
	require.ErrorIs(t, err, ErrManagementPermissionGrantorNotRoot)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", root.Id).Update("status", common.UserStatusDisabled).Error)
	_, _, err = ReplaceUserManagementPermissions(user.Id, []string{
		constant.PermissionChatModelsManage,
	}, root.Id)
	require.ErrorIs(t, err, ErrManagementPermissionGrantorNotRoot)

	state, err := GetUserManagementPermissionState(user.Id)
	require.NoError(t, err)
	require.Empty(t, state.Assigned)
}

func TestEnabledRootCheckUsesCanonicalUserState(t *testing.T) {
	setupManagementPermissionTestDB(t)
	root := createManagementPermissionTestRoot(t, "canonical-root")
	admin := createManagementPermissionTestUser(t, "canonical-admin", common.RoleAdminUser)

	allowed, err := IsEnabledRootUser(root.Id)
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = IsEnabledRootUser(admin.Id)
	require.NoError(t, err)
	require.False(t, allowed)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", root.Id).Update("status", common.UserStatusDisabled).Error)
	allowed, err = IsEnabledRootUser(root.Id)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestManagementPermissionEffectiveAccess(t *testing.T) {
	setupManagementPermissionTestDB(t)
	commonUser := createManagementPermissionTestUser(t, "common-user", common.RoleCommonUser)
	adminUser := createManagementPermissionTestUser(t, "admin-user", common.RoleAdminUser)
	root := createManagementPermissionTestRoot(t, "effective-root")

	_, _, err := ReplaceUserManagementPermissions(commonUser.Id, []string{
		constant.PermissionChatModelsManage,
	}, root.Id)
	require.NoError(t, err)

	allowed, err := HasAnyManagementPermission(commonUser.Id, constant.PermissionChatModelsManage)
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = HasAnyManagementPermission(commonUser.Id, constant.PermissionSkillHubReportsManage)
	require.NoError(t, err)
	require.False(t, allowed)

	allowed, err = HasAnyManagementPermission(adminUser.Id, constant.PermissionSkillHubReportsManage)
	require.NoError(t, err)
	require.True(t, allowed, "administrator permissions are implicit")

	require.NoError(t, DB.Model(&User{}).Where("id = ?", commonUser.Id).Update("status", common.UserStatusDisabled).Error)
	allowed, err = HasAnyManagementPermission(commonUser.Id, constant.PermissionChatModelsManage)
	require.NoError(t, err)
	require.False(t, allowed, "disabled users must never retain effective permissions")
}

func TestRoleChangeAndPermissionReplacementAreAtomic(t *testing.T) {
	setupManagementPermissionTestDB(t)
	user := createManagementPermissionTestUser(t, "concurrent-user", common.RoleCommonUser)
	root := createManagementPermissionTestRoot(t, "concurrent-root")

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var replaceErr error
	var roleErr error

	go func() {
		defer wg.Done()
		<-start
		_, _, replaceErr = ReplaceUserManagementPermissions(user.Id, []string{
			constant.PermissionClientReleasesManage,
		}, root.Id)
	}()
	go func() {
		defer wg.Done()
		<-start
		roleErr = UpdateUserRoleAndClearManagementPermissions(user.Id, common.RoleAdminUser)
	}()

	close(start)
	wg.Wait()
	require.NoError(t, roleErr)
	if replaceErr != nil {
		require.True(t, errors.Is(replaceErr, ErrManagementPermissionsRequireCommonUser))
	}

	state, err := GetUserManagementPermissionState(user.Id)
	require.NoError(t, err)
	require.Equal(t, common.RoleAdminUser, state.Role)
	require.Empty(t, state.Assigned, "role transition must not leave stale explicit grants")
	require.Equal(t, constant.AllManagementPermissions(), state.Effective)
}

func TestDeleteUserClearsManagementPermissions(t *testing.T) {
	setupManagementPermissionTestDB(t)
	user := createManagementPermissionTestUser(t, "deleted-user", common.RoleCommonUser)
	root := createManagementPermissionTestRoot(t, "deleted-root")
	_, _, err := ReplaceUserManagementPermissions(user.Id, []string{
		constant.PermissionSkillHubReportsManage,
	}, root.Id)
	require.NoError(t, err)

	require.NoError(t, user.Delete())

	var count int64
	require.NoError(t, DB.Model(&UserManagementPermission{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestClientReleasePublishedMutationsRequirePublishPermission(t *testing.T) {
	setupManagementPermissionTestDB(t)
	manager := createManagementPermissionTestUser(t, "release-manager", common.RoleCommonUser)
	publisher := createManagementPermissionTestUser(t, "release-publisher", common.RoleCommonUser)
	root := createManagementPermissionTestRoot(t, "release-root")
	_, _, err := ReplaceUserManagementPermissions(manager.Id, []string{
		constant.PermissionClientReleasesManage,
	}, root.Id)
	require.NoError(t, err)
	_, _, err = ReplaceUserManagementPermissions(publisher.Id, []string{
		constant.PermissionClientReleasesManage,
		constant.PermissionClientReleasesPublish,
	}, root.Id)
	require.NoError(t, err)

	release := &ClientRelease{
		Version:   "1.2.3",
		Platform:  "windows",
		Arch:      "x64",
		Channel:   "stable",
		FileName:  "client.exe",
		ObjectKey: "client-releases/client.exe",
		Size:      1024,
		SHA512:    "sha512",
		Status:    ClientReleaseStatusDraft,
	}
	require.NoError(t, release.Insert())

	draftUpdate := *release
	draftUpdate.ReleaseNotes = "draft update"
	draftUpdate.Status = ClientReleaseStatusPublished
	_, err = draftUpdate.UpdateReturningPreviousObjectKey(manager.Id)
	require.NoError(t, err)

	current, err := GetClientReleaseByID(release.Id)
	require.NoError(t, err)
	require.Equal(t, ClientReleaseStatusDraft, current.Status, "metadata request must not publish a draft")

	current, err = UpdateClientReleaseStatus(release.Id, ClientReleaseStatusPublished, publisher.Id)
	require.NoError(t, err)
	require.Equal(t, ClientReleaseStatusPublished, current.Status)

	publishedUpdate := *current
	publishedUpdate.ReleaseNotes = "unauthorized update"
	_, err = publishedUpdate.UpdateReturningPreviousObjectKey(manager.Id)
	require.ErrorIs(t, err, ErrClientReleasePublishPermissionRequired)

	current, err = GetClientReleaseByID(release.Id)
	require.NoError(t, err)
	require.NotEqual(t, "unauthorized update", current.ReleaseNotes)

	_, err = DeleteClientRelease(release.Id, manager.Id)
	require.ErrorIs(t, err, ErrClientReleasePublishPermissionRequired)

	publishedUpdate = *current
	publishedUpdate.ReleaseNotes = "authorized update"
	_, err = publishedUpdate.UpdateReturningPreviousObjectKey(publisher.Id)
	require.NoError(t, err)

	_, _, err = ReplaceUserManagementPermissions(publisher.Id, []string{
		constant.PermissionClientReleasesManage,
	}, root.Id)
	require.NoError(t, err)

	publishedUpdate.ReleaseNotes = "update after revocation"
	_, err = publishedUpdate.UpdateReturningPreviousObjectKey(publisher.Id)
	require.ErrorIs(t, err, ErrClientReleasePublishPermissionRequired)

	_, err = DeleteClientRelease(release.Id, publisher.Id)
	require.ErrorIs(t, err, ErrClientReleasePublishPermissionRequired)

	objectKey, err := DeleteClientRelease(release.Id, root.Id)
	require.NoError(t, err)
	require.Equal(t, release.ObjectKey, objectKey)
}
