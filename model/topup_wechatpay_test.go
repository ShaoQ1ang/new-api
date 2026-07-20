package model

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createWechatPayTopUpForTest(t *testing.T, tradeNo string, userID int, provider string, money float64) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: tradeNo + "-user",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, CreateWechatPayTopUpWithPendingTask(&TopUp{
		UserId:          userID,
		Amount:          10,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   PaymentMethodWechatPay,
		PaymentProvider: provider,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}, time.Now().Unix(), time.Now().Add(10*time.Minute).Unix()))
}

func TestRechargeWechatPayIsIdempotent(t *testing.T) {
	truncateTables(t)
	createWechatPayTopUpForTest(t, "wx-idempotent", 701, PaymentProviderWechatPay, 72.55)

	credited, err := RechargeWechatPay("wx-idempotent", 7255, "provider-txn-1", "127.0.0.1")
	require.NoError(t, err)
	require.True(t, credited)

	credited, err = RechargeWechatPay("wx-idempotent", 7255, "provider-txn-1", "127.0.0.1")
	require.NoError(t, err)
	require.False(t, credited)

	stored := GetTopUpByTradeNo("wx-idempotent")
	require.NotNil(t, stored)
	require.Equal(t, common.TopUpStatusSuccess, stored.Status)
	require.Equal(t, int(10*common.QuotaPerUnit), getUserQuotaForPaymentGuardTest(t, 701))

	var taskCount int64
	require.NoError(t, DB.Model(&WechatPayPendingTask{}).Where("trade_no = ?", "wx-idempotent").Count(&taskCount).Error)
	require.Zero(t, taskCount)
}

func TestRechargeWechatPayRejectsAmountMismatch(t *testing.T) {
	truncateTables(t)
	createWechatPayTopUpForTest(t, "wx-amount-mismatch", 702, PaymentProviderWechatPay, 72.55)

	credited, err := RechargeWechatPay("wx-amount-mismatch", 7254, "provider-txn-2", "127.0.0.1")
	require.ErrorIs(t, err, ErrWechatPayAmountMismatch)
	require.False(t, credited)
	require.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "wx-amount-mismatch"))
	require.Zero(t, getUserQuotaForPaymentGuardTest(t, 702))
}

func TestRechargeWechatPayRejectsProviderMismatch(t *testing.T) {
	truncateTables(t)
	createWechatPayTopUpForTest(t, "wx-provider-mismatch", 703, PaymentProviderAlipay, 72.55)

	credited, err := RechargeWechatPay("wx-provider-mismatch", 7255, "provider-txn-3", "127.0.0.1")
	require.ErrorIs(t, err, ErrPaymentMethodMismatch)
	require.False(t, credited)
	require.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "wx-provider-mismatch"))
	require.Zero(t, getUserQuotaForPaymentGuardTest(t, 703))
}

func TestWechatPayAmountAndQuotaBounds(t *testing.T) {
	for _, money := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, -1, math.MaxFloat64} {
		_, err := WechatPayAmountFen(money)
		require.ErrorIs(t, err, ErrWechatPayAmountInvalid)
	}

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = float64(math.MaxInt32)

	quota, err := WechatPayQuotaToAdd(1)
	require.NoError(t, err)
	require.Equal(t, math.MaxInt32, quota)
	_, err = WechatPayQuotaToAdd(2)
	require.ErrorIs(t, err, ErrWechatPayQuotaOverflow)
}

func TestWechatPayQuotaCapacityIsCheckedBeforeAndDuringSettlement(t *testing.T) {
	truncateTables(t)
	createWechatPayTopUpForTest(t, "wx-quota-capacity", 704, PaymentProviderWechatPay, 72.55)

	quotaToAdd, err := WechatPayQuotaToAdd(10)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 704).Update("quota", math.MaxInt32-quotaToAdd).Error)
	require.NoError(t, CheckWechatPayQuotaCapacity(704, quotaToAdd))

	require.NoError(t, DB.Model(&User{}).Where("id = ?", 704).Update("quota", math.MaxInt32-quotaToAdd+1).Error)
	require.ErrorIs(t, CheckWechatPayQuotaCapacity(704, quotaToAdd), ErrWechatPayQuotaOverflow)

	credited, err := RechargeWechatPay("wx-quota-capacity", 7255, "provider-txn-capacity", "127.0.0.1")
	require.ErrorIs(t, err, ErrWechatPayQuotaOverflow)
	require.False(t, credited)
	require.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "wx-quota-capacity"))
	require.Equal(t, math.MaxInt32-quotaToAdd+1, getUserQuotaForPaymentGuardTest(t, 704))
}

func TestWechatPayPendingTaskLeaseHasSingleOwner(t *testing.T) {
	truncateTables(t)
	createWechatPayTopUpForTest(t, "wx-task-lease", 705, PaymentProviderWechatPay, 10)
	now := time.Now().Unix() + 1

	first, err := ClaimDueWechatPayPendingTasks(now, "owner-a", now+120, 10)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, "owner-a", first[0].LeaseOwner)

	second, err := ClaimDueWechatPayPendingTasks(now, "owner-b", now+120, 10)
	require.NoError(t, err)
	require.Empty(t, second)
	require.ErrorIs(t, UpdateWechatPayPendingTaskRetry("wx-task-lease", "owner-b", now+2, "lost lease"), gorm.ErrRecordNotFound)
	require.NoError(t, UpdateWechatPayPendingTaskRetry("wx-task-lease", "owner-a", now+2, "retry"))

	second, err = ClaimDueWechatPayPendingTasks(now+2, "owner-b", now+122, 10)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, "owner-b", second[0].LeaseOwner)
	require.ErrorIs(t, DeleteClaimedWechatPayPendingTask("wx-task-lease", "owner-a"), gorm.ErrRecordNotFound)
	require.NoError(t, RescheduleWechatPayPendingTask("wx-task-lease", "owner-b", now+3, "USERPAYING"))

	third, err := ClaimDueWechatPayPendingTasks(now+3, "owner-c", now+123, 10)
	require.NoError(t, err)
	require.Len(t, third, 1)
	require.Zero(t, third[0].RetryCount)
	require.NoError(t, DeleteClaimedWechatPayPendingTask("wx-task-lease", "owner-c"))
}

func TestWechatPayPendingTaskErrorIsBounded(t *testing.T) {
	longError := " " + strings.Repeat("错", 1200) + " "
	truncated := truncateWechatPayPendingTaskError(longError)
	require.Equal(t, 1000, len([]rune(truncated)))
	require.NotEmpty(t, truncated)
}
