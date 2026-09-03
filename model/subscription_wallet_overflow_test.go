package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserActiveSubscriptionsAllowWalletOverflow(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()

	require.NoError(t, DB.Create(&UserSubscription{
		UserId:              1001,
		PlanId:              1,
		Status:              "active",
		EndTime:             now + 3600,
		AllowWalletOverflow: common.GetPointer(false),
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:              1002,
		PlanId:              1,
		Status:              "active",
		EndTime:             now + 3600,
		AllowWalletOverflow: common.GetPointer(true),
	}).Error)

	allowed, err := UserActiveSubscriptionsAllowWalletOverflow(1001)
	require.NoError(t, err)
	assert.False(t, allowed)

	allowed, err = UserActiveSubscriptionsAllowWalletOverflow(1002)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestCreateUserSubscriptionSnapshotsWalletOverflow(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&User{Id: 1101, Username: "wallet-overflow-user"}).Error)

	allowOverflow := false
	plan := &SubscriptionPlan{
		Id:                  1102,
		Title:               "Strict subscription",
		PriceAmount:         10,
		DurationUnit:        SubscriptionDurationMonth,
		DurationValue:       1,
		AllowWalletOverflow: &allowOverflow,
	}
	require.NoError(t, DB.Create(plan).Error)

	subscription, err := CreateUserSubscriptionFromPlanTx(DB, 1101, plan, "admin")
	require.NoError(t, err)
	require.NotNil(t, subscription.AllowWalletOverflow)
	assert.False(t, *subscription.AllowWalletOverflow)
}
