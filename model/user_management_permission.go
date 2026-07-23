package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidManagementPermission            = errors.New("invalid management permission")
	ErrManagementPermissionsRequireCommonUser = errors.New("management permissions can only be assigned to common users")
	ErrManagementPermissionGrantorNotRoot     = errors.New("management permissions can only be assigned by an enabled root user")
)

type UserManagementPermission struct {
	Id          int    `json:"-" gorm:"primaryKey"`
	UserId      int    `json:"user_id" gorm:"column:user_id;not null;uniqueIndex:uk_user_management_permission,priority:1;index:idx_user_management_permission_user"`
	Permission  string `json:"permission" gorm:"type:varchar(64);not null;uniqueIndex:uk_user_management_permission,priority:2"`
	GrantedBy   int    `json:"granted_by" gorm:"column:granted_by;not null"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint;not null"`
}

type UserManagementPermissionState struct {
	UserId    int
	Role      int
	Status    int
	Assigned  []string
	Effective []string
}

func normalizeManagementPermissions(permissions []string) ([]string, error) {
	if len(permissions) > len(constant.AllManagementPermissions()) {
		return nil, ErrInvalidManagementPermission
	}

	requested := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if !constant.IsValidManagementPermission(permission) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidManagementPermission, permission)
		}
		requested[permission] = struct{}{}
	}

	normalized := make([]string, 0, len(requested))
	for _, permission := range constant.AllManagementPermissions() {
		if _, ok := requested[permission]; ok {
			normalized = append(normalized, permission)
		}
	}
	return normalized, nil
}

func lockUserForUpdateTx(tx *gorm.DB, userId int, unscoped bool) (*User, error) {
	if userId <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	query := tx
	if unscoped {
		query = query.Unscoped()
	}
	if tx.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var user User
	if err := query.Select("id", "role", "status").Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func getAssignedUserManagementPermissionsTx(tx *gorm.DB, userId int) ([]string, error) {
	var rows []UserManagementPermission
	if err := tx.Where("user_id = ?", userId).Find(&rows).Error; err != nil {
		return nil, err
	}
	permissions := make([]string, 0, len(rows))
	for _, row := range rows {
		permissions = append(permissions, row.Permission)
	}
	return normalizeManagementPermissions(permissions)
}

func GetUserManagementPermissionState(userId int) (*UserManagementPermissionState, error) {
	var user User
	if err := DB.Select("id", "role", "status").Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}
	assigned, err := getAssignedUserManagementPermissionsTx(DB, userId)
	if err != nil {
		return nil, err
	}
	effective := assigned
	if user.Status != common.UserStatusEnabled {
		effective = []string{}
	} else if user.Role >= common.RoleAdminUser {
		effective = constant.AllManagementPermissions()
	}
	return &UserManagementPermissionState{
		UserId:    user.Id,
		Role:      user.Role,
		Status:    user.Status,
		Assigned:  assigned,
		Effective: effective,
	}, nil
}

func GetEffectiveUserManagementPermissions(userId int) ([]string, error) {
	state, err := GetUserManagementPermissionState(userId)
	if err != nil {
		return nil, err
	}
	return state.Effective, nil
}

func IsEnabledRootUser(userId int) (bool, error) {
	var user User
	if err := DB.Select("id", "role", "status").Where("id = ?", userId).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return user.Role == common.RoleRootUser && user.Status == common.UserStatusEnabled, nil
}

func hasAnyManagementPermissionTx(tx *gorm.DB, userId int, lockUser bool, permissions ...string) (bool, error) {
	normalized, err := normalizeManagementPermissions(permissions)
	if err != nil {
		return false, err
	}
	if len(normalized) == 0 {
		return false, nil
	}

	query := tx.Select("id", "role", "status").Where("id = ?", userId)
	if lockUser && tx.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var user User
	if err := query.First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if user.Status != common.UserStatusEnabled {
		return false, nil
	}
	if user.Role >= common.RoleAdminUser {
		return true, nil
	}

	var count int64
	if err := tx.Model(&UserManagementPermission{}).
		Where("user_id = ? AND permission IN ?", userId, normalized).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func HasAnyManagementPermission(userId int, permissions ...string) (bool, error) {
	return hasAnyManagementPermissionTx(DB, userId, false, permissions...)
}

func ReplaceUserManagementPermissions(userId int, permissions []string, grantedBy int) ([]string, []string, error) {
	normalized, err := normalizeManagementPermissions(permissions)
	if err != nil {
		return nil, nil, err
	}
	if grantedBy <= 0 {
		return nil, nil, errors.New("invalid permission grantor")
	}

	var previous []string
	err = DB.Transaction(func(tx *gorm.DB) error {
		user, err := lockUserForUpdateTx(tx, userId, false)
		if err != nil {
			return err
		}
		if user.Role != common.RoleCommonUser {
			return ErrManagementPermissionsRequireCommonUser
		}
		if grantedBy == userId {
			return ErrManagementPermissionGrantorNotRoot
		}
		grantor, err := lockUserForUpdateTx(tx, grantedBy, false)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrManagementPermissionGrantorNotRoot
			}
			return err
		}
		if grantor.Role != common.RoleRootUser || grantor.Status != common.UserStatusEnabled {
			return ErrManagementPermissionGrantorNotRoot
		}

		previous, err = getAssignedUserManagementPermissionsTx(tx, userId)
		if err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userId).Delete(&UserManagementPermission{}).Error; err != nil {
			return err
		}
		if len(normalized) == 0 {
			return nil
		}

		createdTime := common.GetTimestamp()
		rows := make([]UserManagementPermission, 0, len(normalized))
		for _, permission := range normalized {
			rows = append(rows, UserManagementPermission{
				UserId:      userId,
				Permission:  permission,
				GrantedBy:   grantedBy,
				CreatedTime: createdTime,
			})
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return previous, normalized, nil
}

func deleteUserManagementPermissionsTx(tx *gorm.DB, userId int) error {
	return tx.Where("user_id = ?", userId).Delete(&UserManagementPermission{}).Error
}

func UpdateUserRoleAndClearManagementPermissions(userId int, role int) error {
	if role != common.RoleCommonUser && role != common.RoleAdminUser {
		return errors.New("invalid target role")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if _, err := lockUserForUpdateTx(tx, userId, false); err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).Update("role", role).Error; err != nil {
			return err
		}
		return deleteUserManagementPermissionsTx(tx, userId)
	})
}
