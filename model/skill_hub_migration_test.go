package model

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type legacySkillHubSkill struct {
	Id        int            `gorm:"primaryKey"`
	SkillID   string         `gorm:"column:skill_id;size:128;not null;uniqueIndex:uk_skill_hub_skill_id_delete_at,priority:1"`
	DeletedAt gorm.DeletedAt `gorm:"index;uniqueIndex:uk_skill_hub_skill_id_delete_at,priority:2"`
}

func (legacySkillHubSkill) TableName() string {
	return "skill_hub_skills"
}

type legacySkillHubTag struct {
	Id        int            `gorm:"primaryKey"`
	Name      string         `gorm:"size:64;not null;uniqueIndex:uk_skill_hub_tag_name_delete_at,priority:1"`
	DeletedAt gorm.DeletedAt `gorm:"index;uniqueIndex:uk_skill_hub_tag_name_delete_at,priority:2"`
}

func (legacySkillHubTag) TableName() string {
	return "skill_hub_tags"
}

func TestMigrateSkillHubSoftDeleteKeysBackfillsAndReplacesIndexes(t *testing.T) {
	db := openSkillHubMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&legacySkillHubSkill{}, &legacySkillHubTag{}))

	activeSkill := &legacySkillHubSkill{SkillID: "reusable-skill"}
	deletedSkill := &legacySkillHubSkill{SkillID: "reusable-skill"}
	require.NoError(t, db.Create(activeSkill).Error)
	require.NoError(t, db.Create(deletedSkill).Error)
	require.NoError(t, db.Delete(deletedSkill).Error)

	activeTag := &legacySkillHubTag{Name: "Code"}
	deletedTag := &legacySkillHubTag{Name: "Code"}
	require.NoError(t, db.Create(activeTag).Error)
	require.NoError(t, db.Create(deletedTag).Error)
	require.NoError(t, db.Delete(deletedTag).Error)

	require.NoError(t, migrateSkillHubSoftDeleteKeys(db))

	var skills []SkillHubSkill
	require.NoError(t, db.Unscoped().Order("id ASC").Find(&skills).Error)
	require.Len(t, skills, 2)
	assert.Zero(t, skills[0].DeleteKey)
	assert.EqualValues(t, deletedSkill.Id, skills[1].DeleteKey)

	var tags []SkillHubTag
	require.NoError(t, db.Unscoped().Order("id ASC").Find(&tags).Error)
	require.Len(t, tags, 2)
	assert.Zero(t, tags[0].DeleteKey)
	assert.EqualValues(t, deletedTag.Id, tags[1].DeleteKey)

	assert.True(t, db.Migrator().HasIndex(&SkillHubSkill{}, skillHubSkillUniqueIndex))
	assert.False(t, db.Migrator().HasIndex(&SkillHubSkill{}, skillHubSkillLegacyUniqueIndex))
	assert.True(t, db.Migrator().HasIndex(&SkillHubTag{}, skillHubTagUniqueIndex))
	assert.False(t, db.Migrator().HasIndex(&SkillHubTag{}, skillHubTagLegacyUniqueIndex))
}

func TestMigrateSkillHubSoftDeleteKeysRejectsActiveDuplicates(t *testing.T) {
	db := openSkillHubMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&legacySkillHubSkill{}))
	require.NoError(t, db.Create(&legacySkillHubSkill{SkillID: "duplicate-skill"}).Error)
	require.NoError(t, db.Create(&legacySkillHubSkill{SkillID: "duplicate-skill"}).Error)

	err := migrateSkillHubSoftDeleteKeys(db)
	require.Error(t, err)
	assert.ErrorContains(t, err, "clean duplicate values first")
	assert.False(t, db.Migrator().HasIndex(&SkillHubSkill{}, skillHubSkillUniqueIndex))
	assert.True(t, db.Migrator().HasIndex(&SkillHubSkill{}, skillHubSkillLegacyUniqueIndex))

	var count int64
	require.NoError(t, db.Table("skill_hub_skills").Count(&count).Error)
	assert.EqualValues(t, 2, count)
}

func openSkillHubMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "skill-hub-migration.db")),
		&gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		},
	)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})
	return db
}
