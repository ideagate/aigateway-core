package models

import (
	"math/rand"
	"time"

	"gorm.io/gorm"
)

const (
	BatchJobStatusPending    = "pending"
	BatchJobStatusSubmitted  = "submitted"
	BatchJobStatusProcessing = "processing"
	BatchJobStatusCompleted  = "completed"
	BatchJobStatusFailed     = "failed"

	alphanum = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// newBatchJobID returns a sortable string ID in the form yymmddhhmmss<3 random alphanumeric chars>.
// Example: 260323142530a1b
func newBatchJobID() string {
	t := time.Now()
	suffix := make([]byte, 3)
	for i := range suffix {
		suffix[i] = alphanum[rand.Intn(len(alphanum))]
	}
	return t.Format("060102150405") + string(suffix)
}

type BatchJob struct {
	ID                         string     `gorm:"column:id;primaryKey;not null"`
	RequestProto               []byte     `gorm:"column:request_proto;not null"`
	LengthData                 int        `gorm:"column:length_data;not null;default:0"`
	Status                     string     `gorm:"column:status;not null;default:'pending'"`
	ReferenceID                *string    `gorm:"column:reference_id"`
	Provider                   string     `gorm:"column:provider;not null"`
	CreatedTimestamp           time.Time  `gorm:"column:created_timestamp;not null;default:now();type:timestamptz"`
	SubmittedTimestamp         *time.Time `gorm:"column:submitted_timestamp;type:timestamptz"`
	FinishedTimestamp          *time.Time `gorm:"column:finished_timestamp;type:timestamptz"`
	ResultsJson                []byte     `gorm:"column:results_json"`
	WebhookResultURL           *string    `gorm:"column:webhook_result_url"`
	SendWebhookResultTimestamp *time.Time `gorm:"column:send_webhook_result_timestamp;type:timestamptz"`
}

func (BatchJob) TableName() string {
	return "batch_jobs"
}

// BeforeCreate is a GORM hook that auto-generates a sortable ID before every insert.
func (b *BatchJob) BeforeCreate(_ *gorm.DB) error {
	if b.ID == "" {
		b.ID = newBatchJobID()
	}
	return nil
}
