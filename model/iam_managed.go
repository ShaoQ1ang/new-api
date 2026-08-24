package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ManagementSourceLocal = "LOCAL"
	ManagementSourceIAM   = "IAM"

	IAMIdentityStateActive   = "ACTIVE"
	IAMIdentityStateDisabled = "DISABLED"
	IAMIdentityStateDeleted  = "DELETED"

	IAMAPIKeyStateActive  = "ACTIVE"
	IAMAPIKeyStateRevoked = "REVOKED"
)

var (
	ErrIAMIdentityNotActive = errors.New("IAM identity is not active")
	ErrIAMVersionConflict   = errors.New("IAM lifecycle version conflict")
	ErrIAMAPIKeyNotFound    = errors.New("IAM API key is not found")
	ErrIAMInvalidInput      = errors.New("IAM control input is invalid")
	ErrIAMManagedResource   = errors.New("IAM managed resource can only be changed through new-api-control")
)

type IAMIdentityLink struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	IAMUserID        int64  `gorm:"not null;uniqueIndex" json:"iam_user_id"`
	NewAPIUserID     int    `gorm:"not null;uniqueIndex" json:"new_api_user_id"`
	OrganizationID   int64  `gorm:"not null;index" json:"organization_id"`
	OrganizationType int16  `gorm:"not null" json:"organization_type"`
	DesiredState     string `gorm:"type:varchar(16);not null;index" json:"desired_state"`
	LastVersion      int64  `gorm:"not null" json:"last_version"`
	LastEventID      int64  `gorm:"not null" json:"last_event_id"`
	SourceCreatedAt  int64  `gorm:"not null" json:"source_created_at"`
	CreatedAt        int64  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        int64  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (IAMIdentityLink) TableName() string { return "iam_identity_links" }

type IAMAPIKeyLink struct {
	ID                 uint   `gorm:"primaryKey" json:"id"`
	IAMAPIKeyID        int64  `gorm:"not null;uniqueIndex" json:"iam_api_key_id"`
	IAMUserID          int64  `gorm:"not null;index" json:"iam_user_id"`
	NewAPIUserID       int    `gorm:"not null;index" json:"new_api_user_id"`
	TokenID            int    `gorm:"not null;uniqueIndex" json:"token_id"`
	DesiredState       string `gorm:"type:varchar(16);not null;index" json:"desired_state"`
	LastVersion        int64  `gorm:"not null" json:"last_version"`
	LastEventID        int64  `gorm:"not null" json:"last_event_id"`
	RequestFingerprint string `gorm:"type:char(64);not null" json:"-"`
	CreatedAt          int64  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          int64  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (IAMAPIKeyLink) TableName() string { return "iam_api_key_links" }

type ApplyIAMIdentityInput struct {
	EventID          int64
	IAMUserID        int64
	OrganizationID   int64
	OrganizationType int16
	DisplayName      string
	DesiredState     string
	LifecycleVersion int64
	SourceCreatedAt  int64
}

type ApplyIAMIdentityResult struct {
	Applied        bool  `json:"applied"`
	AppliedVersion int64 `json:"applied_version"`
	NewAPIUserID   int   `json:"new_api_user_id"`
}

type CreateIAMAPIKeyInput struct {
	EventID            int64
	IAMAPIKeyID        int64
	IAMUserID          int64
	Version            int64
	Name               string
	ExpiresAt          int64
	ModelLimits        []string
	AllowedIPCIDRs     []string
	RequestFingerprint string
}

type IAMAPIKeyResult struct {
	Applied        bool   `json:"applied"`
	AppliedVersion int64  `json:"applied_version"`
	Secret         string `json:"secret,omitempty"`
	MaskedKey      string `json:"masked_key,omitempty"`
	DesiredState   string `json:"desired_state"`
}

func ValidateIAMIdentityInput(input ApplyIAMIdentityInput) error {
	if input.EventID <= 0 || input.IAMUserID <= 0 || input.OrganizationID <= 0 || input.LifecycleVersion <= 0 || input.SourceCreatedAt <= 0 {
		return fmt.Errorf("%w: identity identifiers and lifecycle version must be positive", ErrIAMInvalidInput)
	}
	if input.OrganizationType != 1 && input.OrganizationType != 2 {
		return fmt.Errorf("%w: unsupported organization type", ErrIAMInvalidInput)
	}
	switch input.DesiredState {
	case IAMIdentityStateActive, IAMIdentityStateDisabled, IAMIdentityStateDeleted:
		return nil
	default:
		return fmt.Errorf("%w: unsupported identity desired state", ErrIAMInvalidInput)
	}
}

func ApplyIAMIdentity(input ApplyIAMIdentityInput) (result ApplyIAMIdentityResult, err error) {
	if err = ValidateIAMIdentityInput(input); err != nil {
		return result, err
	}
	var invalidateUserID int
	err = DB.Transaction(func(tx *gorm.DB) error {
		var link IAMIdentityLink
		queryErr := lockForUpdate(tx).Where("iam_user_id = ?", input.IAMUserID).First(&link).Error
		if queryErr != nil && !errors.Is(queryErr, gorm.ErrRecordNotFound) {
			return queryErr
		}
		if queryErr == nil {
			result.AppliedVersion = link.LastVersion
			result.NewAPIUserID = link.NewAPIUserID
			if input.LifecycleVersion < link.LastVersion {
				return nil
			}
			if input.LifecycleVersion == link.LastVersion {
				if input.DesiredState == link.DesiredState && input.OrganizationID == link.OrganizationID &&
					input.OrganizationType == link.OrganizationType && input.SourceCreatedAt == link.SourceCreatedAt {
					return nil
				}
				return ErrIAMVersionConflict
			}
			// DELETED is a tombstone. A delayed or malformed future event must
			// never recreate access for the same IAM identity.
			if link.DesiredState == IAMIdentityStateDeleted && input.DesiredState != IAMIdentityStateDeleted {
				return ErrIAMVersionConflict
			}
		} else {
			user, createErr := createIAMManagedUser(tx, input)
			if createErr != nil {
				return createErr
			}
			link = IAMIdentityLink{
				IAMUserID: input.IAMUserID, NewAPIUserID: user.Id,
				OrganizationID: input.OrganizationID, OrganizationType: input.OrganizationType,
				DesiredState: input.DesiredState, LastVersion: input.LifecycleVersion,
				LastEventID: input.EventID, SourceCreatedAt: input.SourceCreatedAt,
			}
			if createErr = tx.Create(&link).Error; createErr != nil {
				return createErr
			}
			result = ApplyIAMIdentityResult{Applied: true, AppliedVersion: input.LifecycleVersion, NewAPIUserID: user.Id}
			invalidateUserID = user.Id
			return nil
		}

		status := common.UserStatusDisabled
		if input.DesiredState == IAMIdentityStateActive {
			status = common.UserStatusEnabled
		}
		updates := map[string]interface{}{
			"display_name": normalizeIAMDisplayName(input.DisplayName, input.IAMUserID),
			"status":       status,
		}
		if err := tx.Model(&User{}).Where("id = ? AND management_source = ?", link.NewAPIUserID, ManagementSourceIAM).Updates(updates).Error; err != nil {
			return err
		}
		if input.DesiredState == IAMIdentityStateDeleted {
			if err := revokeAllIAMAPIKeysTx(tx, input.IAMUserID, input.EventID, input.LifecycleVersion); err != nil {
				return err
			}
		} else {
			tokenStatus := common.TokenStatusDisabled
			if input.DesiredState == IAMIdentityStateActive {
				tokenStatus = common.TokenStatusEnabled
			}
			if err := tx.Model(&Token{}).
				Where("user_id = ? AND management_source = ? AND id IN (?)", link.NewAPIUserID, ManagementSourceIAM,
					tx.Model(&IAMAPIKeyLink{}).Select("token_id").Where("iam_user_id = ? AND desired_state = ?", input.IAMUserID, IAMAPIKeyStateActive)).
				Update("status", tokenStatus).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&link).Updates(map[string]interface{}{
			"organization_id": input.OrganizationID, "organization_type": input.OrganizationType,
			"desired_state": input.DesiredState, "last_version": input.LifecycleVersion,
			"last_event_id": input.EventID,
		}).Error; err != nil {
			return err
		}
		result = ApplyIAMIdentityResult{Applied: true, AppliedVersion: input.LifecycleVersion, NewAPIUserID: link.NewAPIUserID}
		invalidateUserID = link.NewAPIUserID
		return nil
	})
	if err == nil && invalidateUserID > 0 {
		if cacheErr := InvalidateUserCache(invalidateUserID); cacheErr != nil {
			common.SysError("failed to invalidate IAM managed user cache: " + cacheErr.Error())
		}
		if cacheErr := InvalidateUserTokensCache(invalidateUserID); cacheErr != nil {
			common.SysError("failed to invalidate IAM managed token cache: " + cacheErr.Error())
		}
	}
	return result, err
}

func createIAMManagedUser(tx *gorm.DB, input ApplyIAMIdentityInput) (*User, error) {
	password, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	password, err = common.Password2Hash(password + common.GetRandomString(16))
	if err != nil {
		return nil, err
	}
	status := common.UserStatusDisabled
	if input.DesiredState == IAMIdentityStateActive {
		status = common.UserStatusEnabled
	}
	quota := common.GetEnvOrDefault("IAM_MANAGED_USER_QUOTA", common.QuotaForNewUser)
	group := strings.TrimSpace(os.Getenv("IAM_MANAGED_USER_GROUP"))
	if group == "" {
		group = "default"
	}
	user := &User{
		Username: iamManagedUsername(input.IAMUserID), Password: password,
		DisplayName: normalizeIAMDisplayName(input.DisplayName, input.IAMUserID),
		Role:        common.RoleCommonUser, Status: status, Group: group, Quota: quota,
		ManagementSource: ManagementSourceIAM,
	}
	if err := tx.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func CreateIAMAPIKey(input CreateIAMAPIKeyInput) (result IAMAPIKeyResult, err error) {
	if input.EventID <= 0 || input.IAMAPIKeyID <= 0 || input.IAMUserID <= 0 || input.Version <= 0 {
		return result, fmt.Errorf("%w: API key identifiers and version must be positive", ErrIAMInvalidInput)
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 64 {
		return result, fmt.Errorf("%w: API key name must contain 1 to 64 characters", ErrIAMInvalidInput)
	}
	if err := validateIAMAPIKeyRestrictions(input.ModelLimits, input.AllowedIPCIDRs); err != nil {
		return result, err
	}
	if input.RequestFingerprint == "" {
		input.RequestFingerprint = fingerprintIAMAPIKeyRequest(input)
	}
	if len(input.RequestFingerprint) != 64 {
		return result, fmt.Errorf("%w: request fingerprint must be SHA-256", ErrIAMInvalidInput)
	}
	if _, decodeErr := hex.DecodeString(input.RequestFingerprint); decodeErr != nil {
		return result, fmt.Errorf("%w: request fingerprint must be SHA-256", ErrIAMInvalidInput)
	}
	var cacheKey string
	err = DB.Transaction(func(tx *gorm.DB) error {
		var identity IAMIdentityLink
		if err := lockForUpdate(tx).Where("iam_user_id = ?", input.IAMUserID).First(&identity).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIAMIdentityNotActive
			}
			return err
		}
		if identity.DesiredState != IAMIdentityStateActive {
			return ErrIAMIdentityNotActive
		}
		var link IAMAPIKeyLink
		queryErr := lockForUpdate(tx).Where("iam_api_key_id = ?", input.IAMAPIKeyID).First(&link).Error
		if queryErr == nil {
			if link.IAMUserID != input.IAMUserID || link.RequestFingerprint != input.RequestFingerprint {
				return ErrIAMVersionConflict
			}
			if input.Version != link.LastVersion || link.DesiredState != IAMAPIKeyStateActive {
				return ErrIAMVersionConflict
			}
			var token Token
			if err := tx.Unscoped().First(&token, link.TokenID).Error; err != nil {
				return err
			}
			result = IAMAPIKeyResult{Applied: false, AppliedVersion: link.LastVersion,
				Secret: "sk-" + token.Key, MaskedKey: MaskTokenKey("sk-" + token.Key), DesiredState: link.DesiredState}
			return nil
		}
		if !errors.Is(queryErr, gorm.ErrRecordNotFound) {
			return queryErr
		}
		key, err := common.GenerateKey()
		if err != nil {
			return err
		}
		expiredTime := int64(-1)
		if input.ExpiresAt > 0 {
			expiredTime = input.ExpiresAt
		}
		modelLimits := normalizeStringSet(input.ModelLimits)
		allowedIPs := normalizeStringSet(input.AllowedIPCIDRs)
		allowedIPText := strings.Join(allowedIPs, "\n")
		token := Token{
			UserId: identity.NewAPIUserID, Key: key, Name: input.Name,
			Status: common.TokenStatusEnabled, CreatedTime: time.Now().Unix(), ExpiredTime: expiredTime,
			UnlimitedQuota: true, ModelLimitsEnabled: len(modelLimits) > 0,
			ModelLimits: strings.Join(modelLimits, ","), AllowIps: &allowedIPText,
			ManagementSource: ManagementSourceIAM,
		}
		if err := tx.Create(&token).Error; err != nil {
			return err
		}
		link = IAMAPIKeyLink{
			IAMAPIKeyID: input.IAMAPIKeyID, IAMUserID: input.IAMUserID,
			NewAPIUserID: identity.NewAPIUserID, TokenID: token.Id,
			DesiredState: IAMAPIKeyStateActive, LastVersion: input.Version,
			LastEventID: input.EventID, RequestFingerprint: input.RequestFingerprint,
		}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
		cacheKey = key
		result = IAMAPIKeyResult{Applied: true, AppliedVersion: input.Version,
			Secret: "sk-" + key, MaskedKey: MaskTokenKey("sk-" + key), DesiredState: IAMAPIKeyStateActive}
		return nil
	})
	if err == nil && cacheKey != "" && common.RedisEnabled {
		_ = cacheDeleteToken(cacheKey)
	}
	return result, err
}

func RevokeIAMAPIKey(eventID, iamUserID, apiKeyID, version int64) (result IAMAPIKeyResult, err error) {
	if eventID <= 0 || iamUserID <= 0 || apiKeyID <= 0 || version <= 0 {
		return result, fmt.Errorf("%w: API key identifiers and version must be positive", ErrIAMInvalidInput)
	}
	var cacheKey string
	err = DB.Transaction(func(tx *gorm.DB) error {
		var link IAMAPIKeyLink
		if err := lockForUpdate(tx).Where("iam_api_key_id = ? AND iam_user_id = ?", apiKeyID, iamUserID).First(&link).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIAMAPIKeyNotFound
			}
			return err
		}
		result.AppliedVersion = link.LastVersion
		result.DesiredState = link.DesiredState
		if version < link.LastVersion {
			return nil
		}
		if version == link.LastVersion {
			if link.DesiredState == IAMAPIKeyStateRevoked && link.LastEventID == eventID {
				return nil
			}
			return ErrIAMVersionConflict
		}
		var token Token
		if err := tx.Unscoped().First(&token, link.TokenID).Error; err != nil {
			return err
		}
		cacheKey = token.Key
		if err := tx.Model(&Token{}).Unscoped().Where("id = ? AND management_source = ?", link.TokenID, ManagementSourceIAM).
			Update("status", common.TokenStatusDisabled).Error; err != nil {
			return err
		}
		if err := tx.Model(&link).Updates(map[string]interface{}{
			"desired_state": IAMAPIKeyStateRevoked, "last_version": version, "last_event_id": eventID,
		}).Error; err != nil {
			return err
		}
		result = IAMAPIKeyResult{Applied: true, AppliedVersion: version, DesiredState: IAMAPIKeyStateRevoked,
			MaskedKey: MaskTokenKey("sk-" + token.Key)}
		return nil
	})
	if err == nil && cacheKey != "" && common.RedisEnabled {
		_ = cacheDeleteToken(cacheKey)
	}
	return result, err
}

func GetIAMAPIKeyState(iamUserID, apiKeyID int64) (IAMAPIKeyResult, error) {
	var link IAMAPIKeyLink
	if err := DB.Where("iam_api_key_id = ? AND iam_user_id = ?", apiKeyID, iamUserID).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return IAMAPIKeyResult{}, ErrIAMAPIKeyNotFound
		}
		return IAMAPIKeyResult{}, err
	}
	var token Token
	if err := DB.Unscoped().First(&token, link.TokenID).Error; err != nil {
		return IAMAPIKeyResult{}, err
	}
	return IAMAPIKeyResult{AppliedVersion: link.LastVersion, DesiredState: link.DesiredState,
		MaskedKey: MaskTokenKey("sk-" + token.Key)}, nil
}

func ResolveActiveIAMIdentity(iamUserID, minimumVersion int64) (IAMIdentityLink, error) {
	var link IAMIdentityLink
	if err := DB.Where("iam_user_id = ?", iamUserID).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return link, ErrIAMIdentityNotActive
		}
		return link, err
	}
	if link.DesiredState != IAMIdentityStateActive || link.LastVersion < minimumVersion {
		return link, ErrIAMIdentityNotActive
	}
	return link, nil
}

func revokeAllIAMAPIKeysTx(tx *gorm.DB, iamUserID, eventID, _ int64) error {
	var links []IAMAPIKeyLink
	if err := lockForUpdate(tx).Where("iam_user_id = ? AND desired_state = ?", iamUserID, IAMAPIKeyStateActive).Find(&links).Error; err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}
	tokenIDs := make([]int, 0, len(links))
	for _, link := range links {
		tokenIDs = append(tokenIDs, link.TokenID)
	}
	if err := tx.Model(&Token{}).Unscoped().Where("id IN ? AND management_source = ?", tokenIDs, ManagementSourceIAM).
		Update("status", common.TokenStatusDisabled).Error; err != nil {
		return err
	}
	return tx.Model(&IAMAPIKeyLink{}).Where("iam_user_id = ? AND desired_state = ?", iamUserID, IAMAPIKeyStateActive).
		Updates(map[string]interface{}{"desired_state": IAMAPIKeyStateRevoked, "last_event_id": eventID}).Error
}

func iamManagedUsername(iamUserID int64) string {
	return "iam_" + strconv.FormatInt(iamUserID, 36)
}

func normalizeIAMDisplayName(displayName string, iamUserID int64) string {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = iamManagedUsername(iamUserID)
	}
	runes := []rune(displayName)
	if len(runes) > 20 {
		displayName = string(runes[:20])
	}
	return displayName
}

func validateIAMAPIKeyRestrictions(models, cidrs []string) error {
	if len(models) > 256 || len(cidrs) > 64 {
		return fmt.Errorf("%w: too many API key restrictions", ErrIAMInvalidInput)
	}
	for _, value := range models {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 128 || strings.ContainsAny(value, "\x00\r\n,") {
			return fmt.Errorf("%w: model restriction is invalid", ErrIAMInvalidInput)
		}
	}
	for _, value := range cidrs {
		if _, err := netip.ParsePrefix(strings.TrimSpace(value)); err != nil {
			return fmt.Errorf("%w: IP restriction is invalid", ErrIAMInvalidInput)
		}
	}
	return nil
}

func normalizeStringSet(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func fingerprintIAMAPIKeyRequest(input CreateIAMAPIKeyInput) string {
	data := strings.Join([]string{
		strconv.FormatInt(input.IAMAPIKeyID, 10), strconv.FormatInt(input.IAMUserID, 10),
		input.Name, strconv.FormatInt(input.ExpiresAt, 10), strings.Join(normalizeStringSet(input.ModelLimits), ","),
		strings.Join(normalizeStringSet(input.AllowedIPCIDRs), ","),
	}, "\x00")
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func IsIAMManagedUsername(username string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(username)), "iam_")
}

func ValidateLocalUserManagement(user User) error {
	if user.ManagementSource == ManagementSourceIAM {
		return errors.New("IAM managed users can only be changed through new-api-control")
	}
	if IsIAMManagedUsername(user.Username) {
		return fmt.Errorf("username prefix iam_ is reserved")
	}
	return nil
}
