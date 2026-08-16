package entity

const (
	RequestStatusSubmitted  = "submitted"
	RequestStatusQueued     = "queued"
	RequestStatusProcessing = "processing"
	RequestStatusCompleted  = "completed"
	RequestStatusFailed     = "failed"
	RequestStatusCanceled   = "canceled"
)

type AigcRequest struct {
	ID              int64  `json:"-" gorm:"primaryKey"`
	IdempotencyKey  string `json:"idempotency_key" gorm:"type:varchar(191);not null;uniqueIndex:uk_aigc_request_user_key,priority:2"`
	GenerationID    string `json:"generation_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	UserID          int    `json:"-" gorm:"not null;uniqueIndex:uk_aigc_request_user_key,priority:1;index"`
	TokenID         int    `json:"-" gorm:"not null;index"`
	GroupName       string `json:"-" gorm:"type:varchar(64);not null"`
	PublicModelID   string `json:"model" gorm:"type:varchar(128);not null;index"`
	UpstreamModelID string `json:"-" gorm:"type:varchar(255);not null"`
	ModelType       string `json:"type" gorm:"type:varchar(16);not null;index"`
	Mode            string `json:"mode" gorm:"type:varchar(40);not null"`
	ConfigVersion   int    `json:"config_version" gorm:"not null"`
	Status          string `json:"status" gorm:"type:varchar(24);not null;index"`
	Progress        int    `json:"progress" gorm:"not null"`
	NativeTaskID    string `json:"-" gorm:"type:varchar(191);not null"`
	RequestDigest   string `json:"-" gorm:"type:varchar(64);not null"`
	RequestJSON     string `json:"-" gorm:"type:text;not null"`
	ResultJSON      string `json:"-" gorm:"type:text;not null"`
	ErrorCode       string `json:"error_code,omitempty" gorm:"type:varchar(64);not null"`
	ErrorMessage    string `json:"error_message,omitempty" gorm:"type:text;not null"`
	CreatedTime     int64  `json:"created_at" gorm:"not null"`
	UpdatedTime     int64  `json:"updated_at" gorm:"not null"`
	FinishedTime    int64  `json:"finished_at,omitempty" gorm:"not null"`
}

type RequestStateUpdate struct {
	Status       string
	Progress     int
	NativeTaskID string
	ResultJSON   string
	ErrorCode    string
	ErrorMessage string
}

func (AigcRequest) TableName() string {
	return "aigc_requests"
}
