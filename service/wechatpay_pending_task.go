package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	wechatPayPendingTickInterval  = 30 * time.Second
	wechatPayPendingQueryDelay    = 30 * time.Second
	wechatPayPendingBatchSize     = 100
	wechatPayPendingLease         = 2 * time.Minute
	wechatPayPendingMaxRetryDelay = 30 * time.Minute
)

var (
	wechatPayPendingOnce    sync.Once
	wechatPayPendingRunning atomic.Bool
	wechatPayPendingOwner   = common.GetRandomString(32)
)

func StartWechatPayPendingTopUpTask() {
	wechatPayPendingOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			ticker := time.NewTicker(wechatPayPendingTickInterval)
			defer ticker.Stop()
			runWechatPayPendingTopUpTaskOnce()
			for range ticker.C {
				runWechatPayPendingTopUpTaskOnce()
			}
		})
	})
}

func NextWechatPayPendingQueryTime(base time.Time) int64 {
	return base.Add(wechatPayPendingQueryDelay).Unix()
}

func NextWechatPayPendingRetryTime(base time.Time, retryCount int) int64 {
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > 6 {
		retryCount = 6
	}
	delay := wechatPayPendingQueryDelay * time.Duration(1<<retryCount)
	if delay > wechatPayPendingMaxRetryDelay {
		delay = wechatPayPendingMaxRetryDelay
	}
	return base.Add(delay).Unix()
}

func runWechatPayPendingTopUpTaskOnce() {
	if !wechatPayPendingRunning.CompareAndSwap(false, true) {
		return
	}
	defer wechatPayPendingRunning.Store(false)

	config := setting.GetWechatPayConfig()
	if !setting.WechatPayConfigComplete(config) {
		return
	}

	// Claim only the task that is about to run. Claiming a large batch before
	// sequential network requests could let later leases expire in the queue.
	for processed := 0; processed < wechatPayPendingBatchSize; processed++ {
		now := time.Now()
		tasks, err := model.ClaimDueWechatPayPendingTasks(
			now.Unix(),
			wechatPayPendingOwner,
			now.Add(wechatPayPendingLease).Unix(),
			1,
		)
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("WeChat Pay pending query failed: %v", err))
			return
		}
		if len(tasks) == 0 {
			return
		}
		handleWechatPayPendingTopUpTask(context.Background(), config, now, tasks[0])
	}
}

func handleWechatPayPendingTopUpTask(ctx context.Context, config setting.WechatPayConfig, now time.Time, task *model.WechatPayPendingTask) {
	topUp := model.GetTopUpByTradeNo(task.TradeNo)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderWechatPay || topUp.Status != common.TopUpStatusPending {
		_ = model.DeleteClaimedWechatPayPendingTask(task.TradeNo, task.LeaseOwner)
		return
	}

	transaction, err := QueryWechatPayOrder(ctx, config, task.TradeNo)
	if err != nil {
		_ = model.UpdateWechatPayPendingTaskRetry(task.TradeNo, task.LeaseOwner, NextWechatPayPendingRetryTime(now, task.RetryCount), err.Error())
		logger.LogWarn(ctx, fmt.Sprintf("WeChat Pay order query failed trade_no=%s error=%v", task.TradeNo, err))
		return
	}
	state := ""
	if transaction != nil && transaction.TradeState != nil {
		state = *transaction.TradeState
	}
	switch MapWechatPayTradeState(state) {
	case common.TopUpStatusSuccess:
		if _, err := SettleWechatPayTransaction(config, transaction, "system/wechatpay-pending-task"); err != nil {
			_ = model.UpdateWechatPayPendingTaskRetry(task.TradeNo, task.LeaseOwner, NextWechatPayPendingRetryTime(now, task.RetryCount), err.Error())
			logger.LogWarn(ctx, fmt.Sprintf("WeChat Pay pending settlement failed trade_no=%s error=%v", task.TradeNo, err))
		}
	case common.TopUpStatusPending:
		expiresAt := time.Unix(task.ExpireAt, 0)
		if task.ExpireAt <= 0 {
			// Compatibility fallback for rows created before expire_at existed.
			expiresAt = time.Unix(topUp.CreateTime, 0).Add(time.Duration(config.OrderExpireMinutes) * time.Minute)
		}
		if now.Before(expiresAt) {
			_ = model.RescheduleWechatPayPendingTask(task.TradeNo, task.LeaseOwner, NextWechatPayPendingQueryTime(now), state)
			return
		}
		if err := CloseWechatPayOrder(ctx, config, task.TradeNo); err != nil {
			_ = model.UpdateWechatPayPendingTaskRetry(task.TradeNo, task.LeaseOwner, NextWechatPayPendingRetryTime(now, task.RetryCount), err.Error())
			return
		}
		if err := model.UpdatePendingTopUpStatus(task.TradeNo, model.PaymentProviderWechatPay, common.TopUpStatusExpired); err != nil &&
			!errors.Is(err, model.ErrTopUpNotFound) && !errors.Is(err, model.ErrTopUpStatusInvalid) {
			_ = model.UpdateWechatPayPendingTaskRetry(task.TradeNo, task.LeaseOwner, NextWechatPayPendingRetryTime(now, task.RetryCount), err.Error())
			return
		}
		_ = model.DeleteClaimedWechatPayPendingTask(task.TradeNo, task.LeaseOwner)
	case common.TopUpStatusExpired, common.TopUpStatusFailed:
		targetStatus := MapWechatPayTradeState(state)
		if err := model.UpdatePendingTopUpStatus(task.TradeNo, model.PaymentProviderWechatPay, targetStatus); err != nil &&
			!errors.Is(err, model.ErrTopUpNotFound) && !errors.Is(err, model.ErrTopUpStatusInvalid) {
			_ = model.UpdateWechatPayPendingTaskRetry(task.TradeNo, task.LeaseOwner, NextWechatPayPendingRetryTime(now, task.RetryCount), err.Error())
			return
		}
		_ = model.DeleteClaimedWechatPayPendingTask(task.TradeNo, task.LeaseOwner)
	}
}
