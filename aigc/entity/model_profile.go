package entity

const (
	ModelStatusDraft = iota
	ModelStatusPublished
	ModelStatusDisabled
)

type ModelProfile struct {
	ID            int64  `json:"id" gorm:"primaryKey"`
	PublicModelID string `json:"public_model_id" gorm:"type:varchar(128);not null;uniqueIndex"`
	DisplayName   string `json:"display_name" gorm:"type:varchar(128);not null"`
	ModelType     string `json:"model_type" gorm:"type:varchar(16);not null;index"`
	Description   string `json:"description,omitempty" gorm:"type:text"`
	Status        int    `json:"status" gorm:"not null;index"`
	GroupsJSON    string `json:"-" gorm:"type:text;not null"`
	ConfigJSON    string `json:"-" gorm:"type:text;not null"`
	ConfigVersion int    `json:"config_version" gorm:"not null"`
	CreatedTime   int64  `json:"created_time" gorm:"not null"`
	UpdatedTime   int64  `json:"updated_time" gorm:"not null"`
}

func (ModelProfile) TableName() string {
	return "aigc_model_profiles"
}
