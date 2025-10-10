package database

import "time"

// MongoIndexDefinition represents a MongoDB index with all possible options
type MongoIndexDefinition struct {
	IndexDefinition

	// MongoDB-specific options
	Background         bool             // Create index in background
	Sparse             bool             // Only index documents that have the indexed field
	ExpireAfterSeconds *int32           // TTL in seconds (for TTL indexes)
	Unique             bool             // Enforce uniqueness
	PartialFilter      map[string]any   // Partial filter expression
	Collation          *MongoCollation  // Collation options
	Weights            map[string]int32 // Text search weights (for text indexes)
	DefaultLanguage    string           // Default language for text indexes
	LanguageOverride   string           // Field name containing language override
	TextVersion        *int32           // Text index version
	SphereVersion      *int32           // 2dsphere index version
	Bits               *int32           // Precision for geohash (for 2d indexes)
	Max                *float64         // Max boundary for 2d indexes
	Min                *float64         // Min boundary for 2d indexes
	BucketSize         *int32           // Bucket size for geoHaystack indexes
	StorageEngine      map[string]any   // Storage engine options
	Hidden             bool             // Hide index from query planner
	WildcardProjection map[string]any   // Wildcard index projection
}

// MongoCollation represents collation options for MongoDB
type MongoCollation struct {
	Locale          string
	CaseLevel       bool
	CaseFirst       string
	Strength        int
	NumericOrdering bool
	Alternate       string
	MaxVariable     string
	Backwards       bool
}

// MongoIndexType represents special MongoDB index types
type MongoIndexType string

const (
	MongoIndexTypeText     MongoIndexType = "text"     // Full text search
	MongoIndexType2D       MongoIndexType = "2d"       // 2D geospatial
	MongoIndexType2DSphere MongoIndexType = "2dsphere" // 2D sphere geospatial
	MongoIndexTypeHashed   MongoIndexType = "hashed"   // Hashed index
	MongoIndexTypeWildcard MongoIndexType = "wildcard" // Wildcard index
)

// Helper constructors for common MongoDB index types

// NewMongoSimpleIndex creates a simple ascending index on a single field
func NewMongoSimpleIndex(fieldName string, unique bool) MongoIndexDefinition {
	return MongoIndexDefinition{
		IndexDefinition: IndexDefinition{
			Name:   fieldName + "_1",
			Fields: []IndexField{{Name: fieldName, Order: 1}},
			Unique: unique,
		},
		Unique: unique,
	}
}

// NewMongoCompoundIndex creates a compound index on multiple fields
func NewMongoCompoundIndex(name string, fields []IndexField, unique bool) MongoIndexDefinition {
	return MongoIndexDefinition{
		IndexDefinition: IndexDefinition{
			Name:   name,
			Fields: fields,
			Unique: unique,
		},
		Unique: unique,
	}
}

// NewMongoTextIndex creates a full-text search index
func NewMongoTextIndex(name string, fields []string) MongoIndexDefinition {
	indexFields := make([]IndexField, len(fields))
	for i, field := range fields {
		indexFields[i] = IndexField{Name: field, Order: 1} // text indexes use special "text" order in MongoDB
	}

	return MongoIndexDefinition{
		IndexDefinition: IndexDefinition{
			Name:   name,
			Fields: indexFields,
		},
	}
}

// NewMongoTTLIndex creates a simple TTL (Time To Live) index on a single date field
// Note: TTL indexes in MongoDB must include a date field, but can be compound indexes
func NewMongoTTLIndex(fieldName string, expireAfter time.Duration) MongoIndexDefinition {
	seconds := int32(expireAfter.Seconds())
	return MongoIndexDefinition{
		IndexDefinition: IndexDefinition{
			Name:   fieldName + "_ttl",
			Fields: []IndexField{{Name: fieldName, Order: 1}},
		},
		ExpireAfterSeconds: &seconds,
	}
}

// NewMongoCompoundTTLIndex creates a compound TTL index
// The first field MUST be a date field for TTL to work properly
// Additional fields can be used for better query performance
func NewMongoCompoundTTLIndex(name string, fields []IndexField, expireAfter time.Duration) MongoIndexDefinition {
	seconds := int32(expireAfter.Seconds())
	return MongoIndexDefinition{
		IndexDefinition: IndexDefinition{
			Name:   name,
			Fields: fields,
		},
		ExpireAfterSeconds: &seconds,
	}
}

// NewMongo2DSphereIndex creates a 2dsphere geospatial index
func NewMongo2DSphereIndex(fieldName string) MongoIndexDefinition {
	return MongoIndexDefinition{
		IndexDefinition: IndexDefinition{
			Name:   fieldName + "_2dsphere",
			Fields: []IndexField{{Name: fieldName, Order: 1}},
		},
	}
}

// NewMongoHashedIndex creates a hashed index
func NewMongoHashedIndex(fieldName string) MongoIndexDefinition {
	return MongoIndexDefinition{
		IndexDefinition: IndexDefinition{
			Name:   fieldName + "_hashed",
			Fields: []IndexField{{Name: fieldName, Order: 1}},
		},
	}
}

// WithBackground sets the background option
func (idx MongoIndexDefinition) WithBackground(background bool) MongoIndexDefinition {
	idx.Background = background
	return idx
}

// WithSparse sets the sparse option
func (idx MongoIndexDefinition) WithSparse(sparse bool) MongoIndexDefinition {
	idx.Sparse = sparse
	return idx
}

// WithPartialFilter sets a partial filter expression
func (idx MongoIndexDefinition) WithPartialFilter(filter map[string]any) MongoIndexDefinition {
	idx.PartialFilter = filter
	return idx
}

// WithHidden sets the hidden option
func (idx MongoIndexDefinition) WithHidden(hidden bool) MongoIndexDefinition {
	idx.Hidden = hidden
	return idx
}

// WithWeights sets text search weights
func (idx MongoIndexDefinition) WithWeights(weights map[string]int32) MongoIndexDefinition {
	idx.Weights = weights
	return idx
}

// WithDefaultLanguage sets the default language for text indexes
func (idx MongoIndexDefinition) WithDefaultLanguage(language string) MongoIndexDefinition {
	idx.DefaultLanguage = language
	return idx
}

// WithTTL sets the TTL (Time To Live) for the index
// The index must include a date field for TTL to work
func (idx MongoIndexDefinition) WithTTL(expireAfter time.Duration) MongoIndexDefinition {
	seconds := int32(expireAfter.Seconds())
	idx.ExpireAfterSeconds = &seconds
	return idx
}

// WithCollation sets collation options for the index
func (idx MongoIndexDefinition) WithCollation(collation *MongoCollation) MongoIndexDefinition {
	idx.Collation = collation
	return idx
}

// WithStorageEngine sets storage engine specific options
func (idx MongoIndexDefinition) WithStorageEngine(options map[string]any) MongoIndexDefinition {
	idx.StorageEngine = options
	return idx
}

// WithWildcardProjection sets the wildcard projection for wildcard indexes
func (idx MongoIndexDefinition) WithWildcardProjection(projection map[string]any) MongoIndexDefinition {
	idx.WildcardProjection = projection
	return idx
}
