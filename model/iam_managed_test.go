package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupIAMManagedTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &IAMIdentityLink{}, &IAMAPIKeyLink{}))
	DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		DB = previousDB
		common.RedisEnabled = previousRedis
		_ = sqlDB.Close()
	})
}

func TestIAMIdentityDeletionIsAnIrreversibleTombstone(t *testing.T) {
	setupIAMManagedTestDB(t)
	base := ApplyIAMIdentityInput{EventID: 100, IAMUserID: 10, OrganizationID: 20,
		OrganizationType: 1, DisplayName: "Alice", DesiredState: IAMIdentityStateActive,
		LifecycleVersion: 1, SourceCreatedAt: 1}
	created, err := ApplyIAMIdentity(base)
	require.NoError(t, err)
	require.True(t, created.Applied)

	deleted := base
	deleted.EventID = 101
	deleted.LifecycleVersion = 2
	deleted.DesiredState = IAMIdentityStateDeleted
	_, err = ApplyIAMIdentity(deleted)
	require.NoError(t, err)

	resurrect := base
	resurrect.EventID = 102
	resurrect.LifecycleVersion = 3
	_, err = ApplyIAMIdentity(resurrect)
	require.ErrorIs(t, err, ErrIAMVersionConflict)

	var link IAMIdentityLink
	require.NoError(t, DB.Where("iam_user_id = ?", base.IAMUserID).First(&link).Error)
	require.Equal(t, IAMIdentityStateDeleted, link.DesiredState)
	require.EqualValues(t, 2, link.LastVersion)
}

func TestIdentityDeletionDoesNotOverwriteAPIKeyVersion(t *testing.T) {
	setupIAMManagedTestDB(t)
	identity := ApplyIAMIdentityInput{EventID: 500, IAMUserID: 13, OrganizationID: 23,
		OrganizationType: 1, DisplayName: "Dave", DesiredState: IAMIdentityStateActive,
		LifecycleVersion: 1, SourceCreatedAt: 1}
	_, err := ApplyIAMIdentity(identity)
	require.NoError(t, err)
	_, err = CreateIAMAPIKey(CreateIAMAPIKeyInput{EventID: 501, IAMAPIKeyID: 502, IAMUserID: 13,
		Version: 1, Name: "managed", RequestFingerprint: strings.Repeat("c", 64)})
	require.NoError(t, err)

	identity.EventID = 503
	identity.LifecycleVersion = 7
	identity.DesiredState = IAMIdentityStateDeleted
	_, err = ApplyIAMIdentity(identity)
	require.NoError(t, err)
	state, err := GetIAMAPIKeyState(13, 502)
	require.NoError(t, err)
	require.EqualValues(t, 1, state.AppliedVersion)
	require.Equal(t, IAMAPIKeyStateRevoked, state.DesiredState)

	revoked, err := RevokeIAMAPIKey(504, 13, 502, 2)
	require.NoError(t, err)
	require.EqualValues(t, 2, revoked.AppliedVersion)
}

func TestIAMAPIKeyCreationIsIdempotentAndSecretIsStable(t *testing.T) {
	setupIAMManagedTestDB(t)
	identity := ApplyIAMIdentityInput{EventID: 200, IAMUserID: 11, OrganizationID: 21,
		OrganizationType: 1, DisplayName: "Bob", DesiredState: IAMIdentityStateActive,
		LifecycleVersion: 1, SourceCreatedAt: 1}
	_, err := ApplyIAMIdentity(identity)
	require.NoError(t, err)

	input := CreateIAMAPIKeyInput{EventID: 201, IAMAPIKeyID: 301, IAMUserID: 11,
		Version: 1, Name: "desktop", RequestFingerprint: strings.Repeat("a", 64)}
	first, err := CreateIAMAPIKey(input)
	require.NoError(t, err)
	require.NotEmpty(t, first.Secret)

	replay, err := CreateIAMAPIKey(input)
	require.NoError(t, err)
	require.False(t, replay.Applied)
	require.Equal(t, first.Secret, replay.Secret)

	var tokenCount int64
	require.NoError(t, DB.Model(&Token{}).Count(&tokenCount).Error)
	require.EqualValues(t, 1, tokenCount)
}

func TestIAMAPIKeyCannotBeRecreatedAfterRevocation(t *testing.T) {
	setupIAMManagedTestDB(t)
	identity := ApplyIAMIdentityInput{EventID: 210, IAMUserID: 15, OrganizationID: 25,
		OrganizationType: 1, DisplayName: "Erin", DesiredState: IAMIdentityStateActive,
		LifecycleVersion: 1, SourceCreatedAt: 1}
	_, err := ApplyIAMIdentity(identity)
	require.NoError(t, err)
	input := CreateIAMAPIKeyInput{EventID: 211, IAMAPIKeyID: 311, IAMUserID: 15,
		Version: 1, Name: "desktop", RequestFingerprint: strings.Repeat("d", 64)}
	_, err = CreateIAMAPIKey(input)
	require.NoError(t, err)
	_, err = RevokeIAMAPIKey(212, 15, 311, 2)
	require.NoError(t, err)

	input.EventID = 213
	input.Version = 2
	_, err = CreateIAMAPIKey(input)
	require.ErrorIs(t, err, ErrIAMVersionConflict)
}

func TestLocalTokenManagementCannotMutateOrExportIAMKeys(t *testing.T) {
	setupIAMManagedTestDB(t)
	identity := ApplyIAMIdentityInput{EventID: 400, IAMUserID: 12, OrganizationID: 22,
		OrganizationType: 1, DisplayName: "Carol", DesiredState: IAMIdentityStateActive,
		LifecycleVersion: 1, SourceCreatedAt: 1}
	created, err := ApplyIAMIdentity(identity)
	require.NoError(t, err)
	_, err = CreateIAMAPIKey(CreateIAMAPIKeyInput{EventID: 401, IAMAPIKeyID: 402, IAMUserID: 12,
		Version: 1, Name: "managed", RequestFingerprint: strings.Repeat("b", 64)})
	require.NoError(t, err)

	var link IAMAPIKeyLink
	require.NoError(t, DB.Where("iam_api_key_id = ?", 402).First(&link).Error)
	var token Token
	require.NoError(t, DB.First(&token, link.TokenID).Error)
	token.Name = "tampered"
	require.ErrorIs(t, token.Update(), ErrIAMManagedResource)
	require.ErrorIs(t, token.Delete(), ErrIAMManagedResource)
	_, err = BatchDeleteTokens([]int{token.Id}, created.NewAPIUserID)
	require.ErrorIs(t, err, ErrIAMManagedResource)
	keys, err := GetTokenKeysByIds([]int{token.Id}, created.NewAPIUserID)
	require.NoError(t, err)
	require.Empty(t, keys)
	require.ErrorIs(t, (&Token{UserId: created.NewAPIUserID, Name: "local", Key: "local-key"}).Insert(), ErrIAMManagedResource)
}
