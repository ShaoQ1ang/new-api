package controller

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPostSetupCreatesRootWhenIAMManagedUserAlreadyExists(t *testing.T) {
	previousDB := model.DB
	previousSetup := constant.Setup
	previousSelfUseMode := operation_setting.SelfUseModeEnabled
	previousDemoSite := operation_setting.DemoSiteEnabled
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Option{}, &model.Setup{}))
	model.DB = db
	constant.Setup = false
	t.Cleanup(func() {
		model.DB = previousDB
		constant.Setup = previousSetup
		operation_setting.SelfUseModeEnabled = previousSelfUseMode
		operation_setting.DemoSiteEnabled = previousDemoSite
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	require.NoError(t, db.Create(&model.User{
		Username: "iam_existing", Password: "disabled", DisplayName: "IAM User",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		AffCode: "iam_existing", ManagementSource: model.ManagementSourceIAM,
	}).Error)

	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("POST", "/api/setup", bytes.NewBufferString(
		`{"username":"admin","password":"password123","confirmPassword":"password123","SelfUseModeEnabled":false,"DemoSiteEnabled":false}`,
	))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request

	PostSetup(context)

	require.Equal(t, 200, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var root model.User
	require.NoError(t, db.Where("role = ?", common.RoleRootUser).First(&root).Error)
	require.NotEmpty(t, root.AffCode)
	require.NotEqual(t, "iam_existing", root.AffCode)
}
