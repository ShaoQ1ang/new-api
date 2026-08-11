package model

import (
	"fmt"

	"gorm.io/gorm"
)

const (
	skillHubSkillLegacyUniqueIndex = "uk_skill_hub_skill_id_delete_at"
	skillHubSkillUniqueIndex       = "uk_skill_hub_skill_id_delete_key"
	skillHubTagLegacyUniqueIndex   = "uk_skill_hub_tag_name_delete_at"
	skillHubTagUniqueIndex         = "uk_skill_hub_tag_name_delete_key"
)

type skillHubSoftDeleteMigration struct {
	model       any
	table       string
	keyColumn   string
	keyLabel    string
	legacyIndex string
	uniqueIndex string
}

func migrateSkillHubSoftDeleteKeys(db *gorm.DB) error {
	migrations := []skillHubSoftDeleteMigration{
		{
			model:       &SkillHubSkill{},
			table:       "skill_hub_skills",
			keyColumn:   "skill_id",
			keyLabel:    "skill ids",
			legacyIndex: skillHubSkillLegacyUniqueIndex,
			uniqueIndex: skillHubSkillUniqueIndex,
		},
		{
			model:       &SkillHubTag{},
			table:       "skill_hub_tags",
			keyColumn:   "name",
			keyLabel:    "tag names",
			legacyIndex: skillHubTagLegacyUniqueIndex,
			uniqueIndex: skillHubTagUniqueIndex,
		},
	}

	for _, migration := range migrations {
		if err := migrateSkillHubSoftDeleteKey(db, migration); err != nil {
			return err
		}
	}
	return nil
}

func migrateSkillHubSoftDeleteKey(db *gorm.DB, migration skillHubSoftDeleteMigration) error {
	migrator := db.Migrator()
	if !migrator.HasTable(migration.model) {
		return nil
	}
	if !migrator.HasColumn(migration.model, "DeleteKey") {
		if err := migrator.AddColumn(migration.model, "DeleteKey"); err != nil {
			return fmt.Errorf("add %s.delete_key: %w", migration.table, err)
		}
	}

	if migrator.HasColumn(migration.model, "deleted_at") {
		if err := db.Table(migration.table).
			Where("deleted_at IS NOT NULL AND delete_key = ?", 0).
			UpdateColumn("delete_key", gorm.Expr("id")).Error; err != nil {
			return fmt.Errorf("backfill %s.delete_key: %w", migration.table, err)
		}
	}

	var duplicateKeys []string
	if err := db.Table(migration.table).
		Select(migration.keyColumn).
		Where("delete_key = ?", 0).
		Group(migration.keyColumn).
		Having("COUNT(*) > 1").
		Order(migration.keyColumn).
		Limit(10).
		Pluck(migration.keyColumn, &duplicateKeys).Error; err != nil {
		return fmt.Errorf("check duplicate %s: %w", migration.keyLabel, err)
	}
	if len(duplicateKeys) > 0 {
		return fmt.Errorf(
			"cannot enforce active Skill Hub %s uniqueness; clean duplicate values first: %v",
			migration.keyLabel,
			duplicateKeys,
		)
	}

	if !migrator.HasIndex(migration.model, migration.uniqueIndex) {
		if err := migrator.CreateIndex(migration.model, migration.uniqueIndex); err != nil {
			return fmt.Errorf("create index %s: %w", migration.uniqueIndex, err)
		}
	}
	if migrator.HasIndex(migration.model, migration.legacyIndex) {
		if err := migrator.DropIndex(migration.model, migration.legacyIndex); err != nil {
			return fmt.Errorf("drop index %s: %w", migration.legacyIndex, err)
		}
	}
	return nil
}
