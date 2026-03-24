package models

// MigrationModels returns all model pointers that should be migrated together.
func MigrationModels() []any {
	return []any{
		&BatchJob{},
		&PromptConfig{},
	}
}
