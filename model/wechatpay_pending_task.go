package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type WechatPayPendingTask struct {
	Id          int    `json:"id"`
	TradeNo     string `json:"trade_no" gorm:"uniqueIndex;type:varchar(64);index"`
	NextQueryAt int64  `json:"next_query_at" gorm:"index"`
	ExpireAt    int64  `json:"expire_at" gorm:"index"`
	LeaseOwner  string `json:"lease_owner" gorm:"type:varchar(64);index"`
	LeaseUntil  int64  `json:"lease_until" gorm:"index"`
	RetryCount  int    `json:"retry_count"`
	LastQueryAt int64  `json:"last_query_at"`
	LastError   string `json:"last_error" gorm:"type:text"`
	CreateTime  int64  `json:"create_time"`
	UpdateTime  int64  `json:"update_time"`
}

func CreateWechatPayTopUpWithPendingTask(topUp *TopUp, nextQueryAt int64, expireAt int64) error {
	if topUp == nil || topUp.TradeNo == "" {
		return gorm.ErrInvalidData
	}
	now := time.Now().Unix()
	if nextQueryAt <= 0 {
		nextQueryAt = now
	}
	if expireAt <= now {
		return gorm.ErrInvalidData
	}
	if topUp.CreateTime == 0 {
		topUp.CreateTime = now
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(topUp).Error; err != nil {
			return err
		}
		return tx.Create(&WechatPayPendingTask{
			TradeNo:     topUp.TradeNo,
			NextQueryAt: nextQueryAt,
			ExpireAt:    expireAt,
			CreateTime:  now,
			UpdateTime:  now,
		}).Error
	})
}

func ClaimDueWechatPayPendingTasks(now int64, leaseOwner string, leaseUntil int64, limit int) ([]*WechatPayPendingTask, error) {
	if leaseOwner == "" || leaseUntil <= now {
		return nil, gorm.ErrInvalidData
	}
	if limit <= 0 {
		limit = 100
	}
	var candidates []*WechatPayPendingTask
	if err := DB.Where("next_query_at <= ? AND (lease_until = 0 OR lease_until <= ?)", now, now).
		Order("id asc").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return nil, err
	}

	claimed := make([]*WechatPayPendingTask, 0, len(candidates))
	for _, task := range candidates {
		result := DB.Model(&WechatPayPendingTask{}).
			Where("id = ? AND next_query_at <= ? AND (lease_until = 0 OR lease_until <= ?)", task.Id, now, now).
			Updates(map[string]any{
				"lease_owner":   leaseOwner,
				"lease_until":   leaseUntil,
				"last_query_at": now,
				"update_time":   now,
			})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 1 {
			task.LeaseOwner = leaseOwner
			task.LeaseUntil = leaseUntil
			task.LastQueryAt = now
			claimed = append(claimed, task)
		}
	}
	return claimed, nil
}

func DeleteWechatPayPendingTask(tradeNo string) error {
	if tradeNo == "" {
		return nil
	}
	return DB.Where("trade_no = ?", tradeNo).Delete(&WechatPayPendingTask{}).Error
}

func DeleteClaimedWechatPayPendingTask(tradeNo string, leaseOwner string) error {
	if tradeNo == "" || leaseOwner == "" {
		return gorm.ErrInvalidData
	}
	result := DB.Where("trade_no = ? AND lease_owner = ?", tradeNo, leaseOwner).Delete(&WechatPayPendingTask{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func UpdateWechatPayPendingTaskRetry(tradeNo string, leaseOwner string, nextQueryAt int64, lastError string) error {
	if tradeNo == "" {
		return nil
	}
	now := time.Now().Unix()
	lastError = truncateWechatPayPendingTaskError(lastError)
	result := DB.Model(&WechatPayPendingTask{}).
		Where("trade_no = ? AND lease_owner = ?", tradeNo, leaseOwner).
		Updates(map[string]any{
			"next_query_at": nextQueryAt,
			"last_error":    lastError,
			"update_time":   now,
			"retry_count":   gorm.Expr("retry_count + ?", 1),
			"lease_owner":   "",
			"lease_until":   0,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func RescheduleWechatPayPendingTask(tradeNo string, leaseOwner string, nextQueryAt int64, lastState string) error {
	if tradeNo == "" || leaseOwner == "" {
		return gorm.ErrInvalidData
	}
	now := time.Now().Unix()
	result := DB.Model(&WechatPayPendingTask{}).
		Where("trade_no = ? AND lease_owner = ?", tradeNo, leaseOwner).
		Updates(map[string]any{
			"next_query_at": nextQueryAt,
			"last_error":    truncateWechatPayPendingTaskError(lastState),
			"update_time":   now,
			"retry_count":   0,
			"lease_owner":   "",
			"lease_until":   0,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func truncateWechatPayPendingTaskError(value string) string {
	const maxRunes = 1000
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return value
}
