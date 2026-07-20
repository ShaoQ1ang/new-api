package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PaymentMethodWechatPay   = "wechatpay_native"
	PaymentProviderWechatPay = "wechatpay"
)

var (
	ErrWechatPayAmountMismatch = errors.New("WeChat Pay amount mismatch")
	ErrWechatPayAmountInvalid  = errors.New("WeChat Pay amount is outside the supported range")
	ErrWechatPayQuotaOverflow  = errors.New("WeChat Pay quota would exceed the portable database range")
)

func WechatPayAmountFen(money float64) (int64, error) {
	if math.IsNaN(money) || math.IsInf(money, 0) || money <= 0 {
		return 0, ErrWechatPayAmountInvalid
	}
	fen := decimal.NewFromFloat(money).Mul(decimal.NewFromInt(100)).Round(0)
	if fen.LessThanOrEqual(decimal.Zero) || fen.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, ErrWechatPayAmountInvalid
	}
	return fen.IntPart(), nil
}

func WechatPayQuotaToAdd(amount int64) (int, error) {
	if amount <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || common.QuotaPerUnit <= 0 {
		return 0, ErrWechatPayQuotaOverflow
	}
	quota := decimal.NewFromInt(amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Truncate(0)
	// User.quota uses SQL INT. Apply the signed 32-bit ceiling everywhere,
	// including SQLite where INTEGER would otherwise accept an int64 value.
	if quota.LessThanOrEqual(decimal.Zero) || quota.GreaterThan(decimal.NewFromInt(math.MaxInt32)) {
		return 0, ErrWechatPayQuotaOverflow
	}
	return int(quota.IntPart()), nil
}

// CheckWechatPayQuotaCapacity prevents creating a provider order that cannot
// normally be credited. RechargeWechatPay repeats the bound check atomically
// because quota can still change between this preflight and settlement.
func CheckWechatPayQuotaCapacity(userID int, quotaToAdd int) error {
	if userID <= 0 || quotaToAdd <= 0 {
		return ErrWechatPayQuotaOverflow
	}
	var user User
	if err := DB.Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}
	if user.Quota < 0 || user.Quota > math.MaxInt32-quotaToAdd {
		return ErrWechatPayQuotaOverflow
	}
	return nil
}

func RechargeWechatPay(tradeNo string, paidFen int64, transactionID string, callerIP string) (bool, error) {
	if tradeNo == "" {
		return false, errors.New("missing trade number")
	}
	if paidFen <= 0 {
		return false, ErrWechatPayAmountMismatch
	}
	if transactionID == "" {
		return false, errors.New("missing provider transaction ID")
	}

	var (
		credited   bool
		quotaToAdd int
		topUp      TopUp
	)
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("trade_no = ?", tradeNo).
			First(&topUp).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTopUpNotFound
			}
			return err
		}
		if topUp.PaymentProvider != PaymentProviderWechatPay || topUp.PaymentMethod != PaymentMethodWechatPay {
			return ErrPaymentMethodMismatch
		}
		expectedFen, amountErr := WechatPayAmountFen(topUp.Money)
		if amountErr != nil || expectedFen != paidFen {
			return ErrWechatPayAmountMismatch
		}
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}
		if topUp.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}

		quotaToAdd, amountErr = WechatPayQuotaToAdd(topUp.Amount)
		if amountErr != nil {
			return amountErr
		}

		result := tx.Model(&TopUp{}).
			Where("trade_no = ? AND payment_provider = ? AND status = ?", tradeNo, PaymentProviderWechatPay, common.TopUpStatusPending).
			Updates(map[string]any{
				"status":        common.TopUpStatusSuccess,
				"complete_time": common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		userUpdate := tx.Model(&User{}).
			Where("id = ? AND quota >= 0 AND quota <= ?", topUp.UserId, math.MaxInt32-quotaToAdd).
			Update("quota", gorm.Expr("quota + ?", quotaToAdd))
		if userUpdate.Error != nil {
			return userUpdate.Error
		}
		if userUpdate.RowsAffected != 1 {
			return ErrWechatPayQuotaOverflow
		}
		credited = true
		return nil
	})
	if err != nil {
		return false, err
	}

	if err := DeleteWechatPayPendingTask(tradeNo); err != nil {
		common.SysLog("failed to delete WeChat Pay pending task: " + err.Error())
	}
	if !credited {
		return false, nil
	}

	// The database is authoritative. Invalidating avoids an asynchronous HINCR
	// racing an absolute cache refill and representing one top-up twice.
	if err := invalidateUserCache(topUp.UserId); err != nil {
		common.SysLog("failed to invalidate cached user after WeChat Pay: " + err.Error())
	}
	RecordTopupLog(topUp.UserId,
		fmt.Sprintf("WeChat Pay top-up successful, quota added: %v, amount: %.2f", logger.FormatQuota(quotaToAdd), topUp.Money),
		callerIP,
		topUp.PaymentMethod,
		PaymentMethodWechatPay,
		map[string]interface{}{
			"quota":                   quotaToAdd,
			"amount":                  topUp.Amount,
			"money":                   topUp.Money,
			"payment_method":          topUp.PaymentMethod,
			"payment_provider":        topUp.PaymentProvider,
			"callback_method":         PaymentMethodWechatPay,
			"provider_transaction_id": transactionID,
		})
	return true, nil
}
