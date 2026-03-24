package models

const (
	// TokenJobTypeBatchJob is the job_type value used for batch jobs.
	TokenJobTypeBatchJob = "batch_job"

	// TokenTypeInput represents the prompt / input token count.
	TokenTypeInput = "input"
	// TokenTypeOutput represents the candidates / output token count.
	TokenTypeOutput = "output"
	// TokenTypeTotal represents the combined total token count.
	TokenTypeTotal = "total"
)

// TokenJob records the token breakdown for a single job.
// The composite primary key is (JobType, JobID, TokenType).
type TokenJob struct {
	JobType    string `gorm:"column:job_type;primaryKey;not null"`
	JobID      string `gorm:"column:job_id;primaryKey;not null"`
	TokenType  string `gorm:"column:token_type;primaryKey;not null"`
	TokenCount int64  `gorm:"column:token_count;not null"`
}

func (TokenJob) TableName() string {
	return "token_jobs"
}
