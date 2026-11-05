package database

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"maps"
	"sort"
	"strings"

	"github.com/go-errors/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoIndexManager manages indexes for MongoDB collections
type MongoIndexManager struct {
	connector *MongoConnector
	ctx       context.Context
}

// NewMongoIndexManager creates a new MongoDB index manager
func NewMongoIndexManager(connector *MongoConnector) *MongoIndexManager {
	return &MongoIndexManager{
		connector: connector,
		ctx:       context.Background(),
	}
}

// EnsureIndexes creates the indexes defined in the model
func (m *MongoIndexManager) EnsureIndexes(model IModel) error {
	// Check if model implements MongoIndexableModel
	indexableModel, ok := model.(MongoIndexableModel)
	if !ok {
		// Model doesn't define indexes, skip
		return nil
	}

	indexes := indexableModel.DefineMongoIndexes()
	if len(indexes) == 0 {
		return nil
	}

	collection := m.getCollection(model)

	// First, compare indexes and log warnings
	warnings, err := m.CompareIndexes(model)
	if err != nil {
		log.Printf("Warning: Could not compare indexes for %s: %v", model.GetModelName(), err)
	}

	// Only log if there are new indexes or differences
	hasNewIndexes := false
	hasDifferences := false
	// The indexes whose keys changed in code: MongoDB refuses another index with the same name,
	// so they are dropped and created again.
	recreate := map[string]bool{}
	for _, warning := range warnings {
		switch warning.Type {
		case IndexWarningMissingInDB:
			hasNewIndexes = true
			log.Printf("[%s] New index for %s: %s", warning.Type, model.GetModelName(), warning.Message)
		case IndexWarningDifferent:
			hasDifferences = true
			log.Printf("[%s] Index difference for %s: %s", warning.Type, model.GetModelName(), warning.Message)
			if warning.Details["keysDiffer"] == true {
				recreate[warning.Details["indexName"].(string)] = true
			}
		}
		// Silently ignore IndexWarningMissingInCode unless there are other changes
	}

	// One by one: an index that cannot be created (its data does not fit it) does not stop the rest.
	var names []string
	var errs []error
	for _, idx := range indexes {
		indexModel, err := m.convertToMongoIndexModel(idx)
		if err != nil {
			errs = append(errs, errors.Errorf("failed to convert index %s: %v", idx.Name, err))
			continue
		}
		if recreate[idx.Name] {
			if err := collection.Indexes().DropOne(m.ctx, idx.Name); err != nil {
				errs = append(errs, errors.Errorf("failed to drop index %s to recreate it: %v", idx.Name, err))
				continue
			}
			log.Printf("Recreating index %s of %s with its new keys", idx.Name, model.GetModelName())
		}
		name, err := collection.Indexes().CreateOne(m.ctx, indexModel)
		if err != nil {
			errs = append(errs, errors.Errorf("failed to create index %s: %v", idx.Name, err))
			continue
		}
		names = append(names, name)
	}

	// Only log success if there were new indexes or differences
	if hasNewIndexes || hasDifferences {
		log.Printf("Successfully ensured %d indexes for %s: %v", len(names), model.GetModelName(), names)
	}
	if len(errs) > 0 {
		return errors.Errorf("failed to create indexes for %s: %v", model.GetModelName(), stderrors.Join(errs...))
	}
	return nil
}

// ListIndexes returns all index names for a model's collection
func (m *MongoIndexManager) ListIndexes(model IModel) ([]string, error) {
	collection := m.getCollection(model)

	cursor, err := collection.Indexes().List(m.ctx)
	if err != nil {
		return nil, errors.Errorf("failed to list indexes: %v", err)
	}
	defer cursor.Close(m.ctx)

	var indexes []string
	for cursor.Next(m.ctx) {
		var index bson.M
		if err := cursor.Decode(&index); err != nil {
			return nil, errors.Errorf("failed to decode index: %v", err)
		}

		if name, ok := index["name"].(string); ok {
			indexes = append(indexes, name)
		}
	}

	if err := cursor.Err(); err != nil {
		return nil, errors.Errorf("cursor error: %v", err)
	}

	return indexes, nil
}

// CompareIndexes compares defined indexes with existing ones
func (m *MongoIndexManager) CompareIndexes(model IModel) ([]IndexWarning, error) {
	indexableModel, ok := model.(MongoIndexableModel)
	if !ok {
		return nil, nil
	}

	definedIndexes := indexableModel.DefineMongoIndexes()
	collection := m.getCollection(model)

	// Get existing indexes from DB
	cursor, err := collection.Indexes().List(m.ctx)
	if err != nil {
		return nil, errors.Errorf("failed to list indexes: %v", err)
	}
	defer cursor.Close(m.ctx)

	existingIndexes := make(map[string]bson.M)
	for cursor.Next(m.ctx) {
		var index bson.M
		if err := cursor.Decode(&index); err != nil {
			return nil, errors.Errorf("failed to decode index: %v", err)
		}

		if name, ok := index["name"].(string); ok {
			existingIndexes[name] = index
		}
	}

	if err := cursor.Err(); err != nil {
		return nil, errors.Errorf("cursor error: %v", err)
	}

	// Compare indexes
	var warnings []IndexWarning
	definedIndexMap := make(map[string]MongoIndexDefinition)

	// Build map of defined indexes
	for _, idx := range definedIndexes {
		definedIndexMap[idx.Name] = idx
	}

	// Check for indexes in DB but not in code
	for name, dbIndex := range existingIndexes {
		// Skip the default _id index
		if name == "_id_" {
			continue
		}

		if _, exists := definedIndexMap[name]; !exists {
			warnings = append(warnings, IndexWarning{
				Type:    IndexWarningMissingInCode,
				Message: fmt.Sprintf("Index '%s' exists in database but is not defined in code", name),
				Details: map[string]any{
					"indexName": name,
					"dbIndex":   dbIndex,
				},
			})
		}
	}

	// Check for indexes in code but not in DB
	for _, idx := range definedIndexes {
		if _, exists := existingIndexes[idx.Name]; !exists {
			warnings = append(warnings, IndexWarning{
				Type:    IndexWarningMissingInDB,
				Message: fmt.Sprintf("Index '%s' is defined in code but does not exist in database", idx.Name),
				Details: map[string]any{
					"indexName":  idx.Name,
					"definition": idx,
				},
			})
		} else {
			// Index exists in both, compare details
			dbIndex := existingIndexes[idx.Name]
			if diff := m.compareIndexDetails(idx, dbIndex); diff != "" {
				warnings = append(warnings, IndexWarning{
					Type:    IndexWarningDifferent,
					Message: fmt.Sprintf("Index '%s' differs: %s", idx.Name, diff),
					Details: map[string]any{
						"indexName":  idx.Name,
						"difference": diff,
						"keysDiffer": keysDiffer(idx, dbIndex),
						"defined":    idx,
						"existing":   dbIndex,
					},
				})
			}
		}
	}

	return warnings, nil
}

// getCollection gets the MongoDB collection for a model
func (m *MongoIndexManager) getCollection(model IModel) *mongo.Collection {
	client := m.connector.client
	database := client.Database(m.connector.options.Database)
	return database.Collection(model.GetTableName())
}

// convertToMongoIndexModel converts our IndexDefinition to MongoDB's IndexModel
func (m *MongoIndexManager) convertToMongoIndexModel(idx MongoIndexDefinition) (mongo.IndexModel, error) {
	// Build keys document
	keys := bson.D{}
	for _, field := range idx.Fields {
		keys = append(keys, bson.E{Key: field.Name, Value: field.key()})
	}

	// Build options
	opts := options.Index()
	opts.SetName(idx.Name)

	if idx.Unique {
		opts.SetUnique(true)
	}

	// Note: Background option was deprecated and removed in MongoDB driver v2
	// Indexes are now built in the foreground by default for better consistency

	if idx.Sparse {
		opts.SetSparse(true)
	}

	if idx.ExpireAfterSeconds != nil {
		opts.SetExpireAfterSeconds(*idx.ExpireAfterSeconds)
	}

	if idx.PartialFilter != nil {
		opts.SetPartialFilterExpression(idx.PartialFilter)
	}

	if idx.Hidden {
		opts.SetHidden(true)
	}

	if idx.Weights != nil {
		opts.SetWeights(idx.Weights)
	}

	if idx.DefaultLanguage != "" {
		opts.SetDefaultLanguage(idx.DefaultLanguage)
	}

	if idx.LanguageOverride != "" {
		opts.SetLanguageOverride(idx.LanguageOverride)
	}

	if idx.TextVersion != nil {
		opts.SetTextVersion(*idx.TextVersion)
	}

	if idx.SphereVersion != nil {
		opts.SetSphereVersion(*idx.SphereVersion)
	}

	if idx.Bits != nil {
		opts.SetBits(*idx.Bits)
	}

	if idx.Max != nil {
		opts.SetMax(*idx.Max)
	}

	if idx.Min != nil {
		opts.SetMin(*idx.Min)
	}

	if idx.BucketSize != nil {
		// Note: BucketSize is deprecated in newer MongoDB versions
		// but we keep it for compatibility
	}

	if idx.StorageEngine != nil {
		opts.SetStorageEngine(idx.StorageEngine)
	}

	if idx.WildcardProjection != nil {
		opts.SetWildcardProjection(idx.WildcardProjection)
	}

	if idx.Collation != nil {
		collation := &options.Collation{
			Locale:          idx.Collation.Locale,
			CaseLevel:       idx.Collation.CaseLevel,
			CaseFirst:       idx.Collation.CaseFirst,
			Strength:        idx.Collation.Strength,
			NumericOrdering: idx.Collation.NumericOrdering,
			Alternate:       idx.Collation.Alternate,
			MaxVariable:     idx.Collation.MaxVariable,
			Backwards:       idx.Collation.Backwards,
		}
		opts.SetCollation(collation)
	}

	return mongo.IndexModel{
		Keys:    keys,
		Options: opts,
	}, nil
}

// compareIndexDetails compares the details of a defined index vs existing one
func (m *MongoIndexManager) compareIndexDetails(defined MongoIndexDefinition, existing bson.M) string {
	var differences []string

	if keysDiffer(defined, existing) {
		differences = append(differences, "keys differ")
	}

	// Compare unique constraint
	if unique, ok := existing["unique"].(bool); ok {
		if unique != defined.Unique {
			differences = append(differences, "unique constraint differs")
		}
	}

	// Compare sparse
	if sparse, ok := existing["sparse"].(bool); ok {
		if sparse != defined.Sparse {
			differences = append(differences, "sparse option differs")
		}
	}

	// Compare TTL
	if expireAfter, ok := existing["expireAfterSeconds"].(int32); ok {
		if defined.ExpireAfterSeconds == nil || *defined.ExpireAfterSeconds != expireAfter {
			differences = append(differences, "TTL differs")
		}
	} else if defined.ExpireAfterSeconds != nil {
		differences = append(differences, "TTL not set in DB")
	}

	if len(differences) == 0 {
		return ""
	}

	sort.Strings(differences)
	return strings.Join(differences, ", ")
}

// indexKeys is the keys document MongoDB keeps for the defined index: the fields of a text index
// become "_fts"/"_ftsx" (its weights list them), the rest stay as they are.
func indexKeys(fields []IndexField) map[string]any {
	keys := make(map[string]any, len(fields))
	for _, field := range fields {
		if field.Type == MongoIndexTypeText {
			keys["_fts"], keys["_ftsx"] = string(MongoIndexTypeText), 1
			continue
		}
		keys[field.Name] = field.key()
	}
	return keys
}

// storedKeys is the keys document of an existing index, its numbers as int.
func storedKeys(existing bson.M) (map[string]any, bool) {
	keys := map[string]any{}
	add := func(key string, value any) {
		switch number := value.(type) {
		case int32:
			value = int(number)
		case int64:
			value = int(number)
		case float64:
			value = int(number)
		}
		keys[key] = value
	}
	switch document := existing["key"].(type) {
	case bson.M:
		for key, value := range document {
			add(key, value)
		}
	case bson.D:
		for _, element := range document {
			add(element.Key, element.Value)
		}
	default:
		return nil, false
	}
	return keys, true
}

// keysDiffer reports whether the existing index has other keys (fields or types) than the defined one.
func keysDiffer(defined MongoIndexDefinition, existing bson.M) bool {
	stored, ok := storedKeys(existing)
	return ok && !maps.Equal(stored, indexKeys(defined.Fields))
}
