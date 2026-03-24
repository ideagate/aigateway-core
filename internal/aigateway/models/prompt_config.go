package models

import (
	"database/sql"
	"time"
)

// PromptConfig is a reusable prompt template that can be referenced when
// submitting bulk chat completions.
type PromptConfig struct {
	ID                 string          `gorm:"column:id;primaryKey;not null"`
	CreatedAt          time.Time       `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt          time.Time       `gorm:"column:updated_at;not null;autoUpdateTime"`
	Description        sql.NullString  `gorm:"column:description"`
	Model              sql.NullString  `gorm:"column:model"`
	SystemInstruction  sql.NullString  `gorm:"column:system_instruction"`
	JSONSchemaResponse sql.NullString  `gorm:"column:json_schema_response"`
	Temperature        sql.NullFloat64 `gorm:"column:temperature"`
	Metadata           []byte          `gorm:"column:metadata"`
}

func (PromptConfig) TableName() string {
	return "prompt_configs"
}
