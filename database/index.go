package database

// IndexField represents a field in an index
type IndexField struct {
	Name  string // Field name
	Order int    // 1 for ascending, -1 for descending
}

// IndexDefinition is a generic, database-agnostic representation of an index
type IndexDefinition struct {
	Name   string       // Index name
	Fields []IndexField // Fields that compose the index
	Unique bool         // Whether the index is unique
}

// MongoIndexableModel defines models that can specify MongoDB indexes
type MongoIndexableModel interface {
	DefineMongoIndexes() []MongoIndexDefinition
}

// PostgresIndexableModel defines models that can specify Postgres indexes (future)
type PostgresIndexableModel interface {
	DefinePostgresIndexes() []PostgresIndexDefinition
}

// PostgresIndexDefinition placeholder for future Postgres support
type PostgresIndexDefinition struct {
	IndexDefinition
	// Postgres-specific options will go here
}

// IndexManager is a generic interface for managing database indexes
type IndexManager interface {
	// EnsureIndexes creates the indexes for a given model
	EnsureIndexes(model IModel) error

	// ListIndexes returns all indexes for a given model's collection/table
	ListIndexes(model IModel) ([]string, error)

	// CompareIndexes compares defined indexes vs existing ones and returns warnings
	CompareIndexes(model IModel) ([]IndexWarning, error)
}

// IndexWarning represents a discrepancy between defined and actual indexes
type IndexWarning struct {
	Type    IndexWarningType
	Message string
	Details map[string]interface{}
}

type IndexWarningType string

const (
	IndexWarningMissingInCode IndexWarningType = "missing_in_code" // Index exists in DB but not in code
	IndexWarningMissingInDB   IndexWarningType = "missing_in_db"   // Index defined in code but not in DB
	IndexWarningDifferent     IndexWarningType = "different"       // Index exists in both but with different options
)
