package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	WalletCallbackStatusReservePending = "reserve_pending"
	WalletCallbackStatusReserved       = "reserved"
	WalletCallbackStatusConfirmPending = "confirm_pending"
	WalletCallbackStatusConfirmed      = "confirmed"
	WalletCallbackStatusCancelPending  = "cancel_pending"
	WalletCallbackStatusCancelled      = "cancelled"
	WalletCallbackStatusRejected       = "rejected"
)

type WalletUsageCallback struct {
	ID                int64   `json:"id" gorm:"primaryKey"`
	APIRequestID      string  `json:"api_request_id" gorm:"type:varchar(128);not null;uniqueIndex"`
	APIPlatformUserID int     `json:"api_platform_user_id" gorm:"column:user_id;not null;index"`
	BusinessOrderNo   *string `json:"business_order_no" gorm:"type:varchar(128)"`
	UsageAtMS         int64   `json:"usage_at_ms" gorm:"not null"`
	ModelName         string  `json:"model_name" gorm:"type:varchar(255)"`
	APIKeyName        string  `json:"api_key_name" gorm:"type:varchar(255)"`

	ReservedQuota  int64  `json:"reserved_quota" gorm:"not null"`
	ReservedAmount int64  `json:"reserved_amount" gorm:"not null"`
	FinalQuota     *int64 `json:"final_quota"`
	FinalAmount    *int64 `json:"final_amount"`
	ExchangeRate   string `json:"exchange_rate" gorm:"type:decimal(20,8);not null"`

	Status        string `json:"status" gorm:"type:varchar(32);not null;index:idx_wallet_callback_due,priority:1"`
	NextRetryAtMS int64  `json:"next_retry_at_ms" gorm:"not null;index:idx_wallet_callback_due,priority:2"`
	RetryCount    int    `json:"retry_count" gorm:"not null"`
	FailureCode   string `json:"failure_code" gorm:"type:varchar(64);not null;default:''"`
	LastError     string `json:"last_error" gorm:"type:text"`
	ReservedAtMS  int64  `json:"reserved_at_ms" gorm:"not null"`
	FinalizedAtMS int64  `json:"finalized_at_ms" gorm:"not null"`
	CreatedAtMS   int64  `json:"created_at_ms" gorm:"not null"`
	UpdatedAtMS   int64  `json:"updated_at_ms" gorm:"not null"`
}

func CreateWalletUsageCallback(callback *WalletUsageCallback) error {
	if callback == nil || callback.APIRequestID == "" || callback.APIPlatformUserID <= 0 || callback.ReservedAmount <= 0 {
		return gorm.ErrInvalidData
	}
	nowMS := time.Now().UnixMilli()
	if callback.UsageAtMS <= 0 {
		callback.UsageAtMS = nowMS
	}
	callback.Status = WalletCallbackStatusReservePending
	if callback.NextRetryAtMS <= 0 {
		callback.NextRetryAtMS = nowMS
	}
	callback.CreatedAtMS = nowMS
	callback.UpdatedAtMS = nowMS
	return DB.Create(callback).Error
}

func PrepareWalletUsageConfirm(apiRequestID string, finalQuota int64, finalAmount int64, protectedUntilMS int64) error {
	if apiRequestID == "" || finalQuota < 0 || finalAmount < 0 || protectedUntilMS <= 0 {
		return gorm.ErrInvalidData
	}
	nowMS := time.Now().UnixMilli()
	return DB.Model(&WalletUsageCallback{}).
		Where("api_request_id = ? AND status NOT IN ?", apiRequestID, []string{WalletCallbackStatusConfirmed, WalletCallbackStatusCancelled, WalletCallbackStatusRejected}).
		Updates(map[string]any{
			"final_quota":      finalQuota,
			"final_amount":     finalAmount,
			"status":           WalletCallbackStatusConfirmPending,
			"next_retry_at_ms": protectedUntilMS,
			"failure_code":     "",
			"last_error":       "",
			"updated_at_ms":    nowMS,
		}).Error
}

func PrepareWalletUsageCancel(apiRequestID string, protectedUntilMS int64) error {
	if apiRequestID == "" || protectedUntilMS <= 0 {
		return gorm.ErrInvalidData
	}
	nowMS := time.Now().UnixMilli()
	return DB.Model(&WalletUsageCallback{}).
		Where("api_request_id = ? AND status NOT IN ?", apiRequestID, []string{WalletCallbackStatusConfirmed, WalletCallbackStatusCancelled, WalletCallbackStatusRejected}).
		Updates(map[string]any{
			"status":           WalletCallbackStatusCancelPending,
			"next_retry_at_ms": protectedUntilMS,
			"failure_code":     "",
			"last_error":       "",
			"updated_at_ms":    nowMS,
		}).Error
}

func MarkWalletUsageReserved(apiRequestID string, completed bool) error {
	if apiRequestID == "" {
		return gorm.ErrInvalidData
	}
	nowMS := time.Now().UnixMilli()
	if err := DB.Model(&WalletUsageCallback{}).
		Where("api_request_id = ?", apiRequestID).
		Updates(map[string]any{
			"reserved_at_ms": nowMS,
			"failure_code":   "",
			"last_error":     "",
			"updated_at_ms":  nowMS,
		}).Error; err != nil || !completed {
		return err
	}
	return DB.Model(&WalletUsageCallback{}).
		Where("api_request_id = ? AND status = ?", apiRequestID, WalletCallbackStatusReservePending).
		Updates(map[string]any{
			"status":           WalletCallbackStatusReserved,
			"next_retry_at_ms": int64(0),
			"updated_at_ms":    nowMS,
		}).Error
}

func MarkWalletUsageFinalized(apiRequestID string, status string) error {
	if apiRequestID == "" || (status != WalletCallbackStatusConfirmed && status != WalletCallbackStatusCancelled) {
		return gorm.ErrInvalidData
	}
	nowMS := time.Now().UnixMilli()
	pendingStatus := WalletCallbackStatusConfirmPending
	if status == WalletCallbackStatusCancelled {
		pendingStatus = WalletCallbackStatusCancelPending
	}
	return DB.Model(&WalletUsageCallback{}).
		Where("api_request_id = ? AND status = ?", apiRequestID, pendingStatus).
		Updates(map[string]any{
			"status":           status,
			"next_retry_at_ms": int64(0),
			"failure_code":     "",
			"last_error":       "",
			"finalized_at_ms":  nowMS,
			"updated_at_ms":    nowMS,
		}).Error
}

func MarkWalletUsageRetry(apiRequestID string, expectedStatus string, nextRetryAtMS int64, failureCode string, lastError string) error {
	if apiRequestID == "" || expectedStatus == "" {
		return gorm.ErrInvalidData
	}
	return DB.Model(&WalletUsageCallback{}).
		Where("api_request_id = ? AND status = ?", apiRequestID, expectedStatus).
		Updates(map[string]any{
			"next_retry_at_ms": nextRetryAtMS,
			"failure_code":     failureCode,
			"last_error":       lastError,
			"retry_count":      gorm.Expr("retry_count + ?", 1),
			"updated_at_ms":    time.Now().UnixMilli(),
		}).Error
}

func MarkWalletUsageRejected(apiRequestID string, expectedStatus string, failureCode string, lastError string) error {
	if apiRequestID == "" || expectedStatus == "" || failureCode == "" {
		return gorm.ErrInvalidData
	}
	nowMS := time.Now().UnixMilli()
	return DB.Model(&WalletUsageCallback{}).
		Where("api_request_id = ? AND status = ?", apiRequestID, expectedStatus).
		Updates(map[string]any{
			"status":           WalletCallbackStatusRejected,
			"next_retry_at_ms": int64(0),
			"failure_code":     failureCode,
			"last_error":       lastError,
			"finalized_at_ms":  nowMS,
			"updated_at_ms":    nowMS,
		}).Error
}

func MarkWalletUsageCancelPendingAfterReserveFailure(apiRequestID string, failureCode string, lastError string) error {
	if apiRequestID == "" {
		return gorm.ErrInvalidData
	}
	nowMS := time.Now().UnixMilli()
	return DB.Model(&WalletUsageCallback{}).
		Where("api_request_id = ? AND status = ?", apiRequestID, WalletCallbackStatusReservePending).
		Updates(map[string]any{
			"status":           WalletCallbackStatusCancelPending,
			"next_retry_at_ms": nowMS,
			"failure_code":     failureCode,
			"last_error":       lastError,
			"retry_count":      gorm.Expr("retry_count + ?", 1),
			"updated_at_ms":    nowMS,
		}).Error
}

func GetWalletUsageCallback(apiRequestID string) (*WalletUsageCallback, error) {
	var callback WalletUsageCallback
	err := DB.Where("api_request_id = ?", apiRequestID).First(&callback).Error
	return &callback, err
}

func GetDueWalletUsageCallbacks(nowMS int64, limit int) ([]*WalletUsageCallback, error) {
	if limit <= 0 {
		limit = 100
	}
	var callbacks []*WalletUsageCallback
	err := DB.Where("status IN ? AND next_retry_at_ms <= ?", []string{
		WalletCallbackStatusReservePending,
		WalletCallbackStatusConfirmPending,
		WalletCallbackStatusCancelPending,
	}, nowMS).
		Order("id asc").
		Limit(limit).
		Find(&callbacks).Error
	return callbacks, err
}

func ClaimWalletUsageCallback(id int64, expectedStatus string, dueAtMS int64, leaseUntilMS int64) (bool, error) {
	if id <= 0 || expectedStatus == "" || leaseUntilMS <= dueAtMS {
		return false, gorm.ErrInvalidData
	}
	result := DB.Model(&WalletUsageCallback{}).
		Where("id = ? AND status = ? AND next_retry_at_ms <= ?", id, expectedStatus, dueAtMS).
		Updates(map[string]any{
			"next_retry_at_ms": leaseUntilMS,
			"updated_at_ms":    time.Now().UnixMilli(),
		})
	return result.RowsAffected == 1, result.Error
}
