package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestSpecialIndexesUseTheirTypeAsKey(t *testing.T) {
	manager := &MongoIndexManager{}
	cases := []struct {
		index MongoIndexDefinition
		keys  bson.D
	}{
		{NewMongo2DSphereIndex("location"), bson.D{{Key: "location", Value: "2dsphere"}}},
		{NewMongoHashedIndex("code"), bson.D{{Key: "code", Value: "hashed"}}},
		{NewMongoTextIndex("search", []string{"name", "notes"}), bson.D{{Key: "name", Value: "text"}, {Key: "notes", Value: "text"}}},
		{NewMongoSimpleIndex("name", false), bson.D{{Key: "name", Value: 1}}},
		{NewMongoCompoundIndex("projectId_1_created_-1", []IndexField{{Name: "projectId", Order: 1}, {Name: "created", Order: -1}}, false),
			bson.D{{Key: "projectId", Value: 1}, {Key: "created", Value: -1}}},
		// A special type next to ordinary fields (a compound geospatial index).
		{NewMongoCompoundIndex("projectId_1_location_2dsphere", []IndexField{{Name: "projectId", Order: 1}, {Name: "location", Type: MongoIndexType2DSphere}}, false),
			bson.D{{Key: "projectId", Value: 1}, {Key: "location", Value: "2dsphere"}}},
	}
	for _, c := range cases {
		model, err := manager.convertToMongoIndexModel(c.index)
		require.NoError(t, err)
		assert.Equal(t, c.keys, model.Keys, c.index.Name)
	}
}

func TestIndexKeysAreComparedWithTheirTypes(t *testing.T) {
	sphere := NewMongo2DSphereIndex("location")
	// What the earlier versions created for it: an ascending index with its name.
	assert.True(t, keysDiffer(sphere, bson.M{"key": bson.M{"location": int32(1)}}))
	assert.False(t, keysDiffer(sphere, bson.M{"key": bson.M{"location": "2dsphere"}}))
	assert.False(t, keysDiffer(sphere, bson.M{"key": bson.D{{Key: "location", Value: "2dsphere"}}}))

	compound := NewMongoCompoundIndex("a_1_b_-1", []IndexField{{Name: "a", Order: 1}, {Name: "b", Order: -1}}, false)
	assert.False(t, keysDiffer(compound, bson.M{"key": bson.M{"a": int32(1), "b": int64(-1)}}))
	assert.True(t, keysDiffer(compound, bson.M{"key": bson.M{"a": int32(1), "b": int32(1)}}))
	assert.True(t, keysDiffer(compound, bson.M{"key": bson.M{"a": int32(1)}}))

	// MongoDB keeps a text index's fields in its weights.
	text := NewMongoTextIndex("search", []string{"name", "notes"})
	assert.False(t, keysDiffer(text, bson.M{"key": bson.M{"_fts": "text", "_ftsx": int32(1)}}))
	assert.True(t, keysDiffer(text, bson.M{"key": bson.M{"name": int32(1), "notes": int32(1)}}))

	assert.Equal(t, "keys differ", (&MongoIndexManager{}).compareIndexDetails(sphere, bson.M{"key": bson.M{"location": int32(1)}}))
}

type geoPlace struct {
	ID       bson.ObjectID `bson:"_id"`
	Name     string        `bson:"name"`
	Location any           `bson:"location,omitempty"`
}

func (geoPlace) GetTableName() string     { return "geo_places" }
func (geoPlace) GetModelName() string     { return "GeoPlace" }
func (geoPlace) GetConnectorName() string { return "mongodb" }
func (p geoPlace) GetId() any             { return p.ID }
func (geoPlace) DefineMongoIndexes() []MongoIndexDefinition {
	return []MongoIndexDefinition{
		NewMongo2DSphereIndex("location"),
		NewMongoSimpleIndex("name", false),
	}
}

// testIndexManager connects to the MongoDB of VSAAS_REST_TEST_MONGO_URI, on a database of its own
// that is dropped at the end.
func testIndexManager(t *testing.T) *MongoIndexManager {
	t.Helper()
	uri := os.Getenv("VSAAS_REST_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("VSAAS_REST_TEST_MONGO_URI is not set")
	}
	connector, err := NewMongoConnector(&MongoConnectorOpts{
		ClientOptions: *options.Client().ApplyURI(uri).SetTimeout(10 * time.Second),
		Name:          "mongodb",
		Database:      fmt.Sprintf("vsaas_rest_index_test_%d", time.Now().UnixNano()),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = connector.client.Database(connector.options.Database).Drop(context.Background())
		_ = connector.Disconnect()
	})
	return connector.GetIndexManager()
}

func storedIndexKeys(t *testing.T, manager *MongoIndexManager, name string) bson.M {
	t.Helper()
	cursor, err := manager.getCollection(geoPlace{}).Indexes().List(context.Background())
	require.NoError(t, err)
	var indexes []bson.M
	require.NoError(t, cursor.All(context.Background(), &indexes))
	for _, index := range indexes {
		if index["name"] == name {
			keys, _ := storedKeys(index)
			return bson.M(keys)
		}
	}
	return nil
}

func TestMongoA2DSphereIndexIsGeospatialAndReplacesTheAscendingOne(t *testing.T) {
	manager := testIndexManager(t)
	ctx := context.Background()
	collection := manager.getCollection(geoPlace{})

	// The index an earlier version created with the 2dsphere's name: ascending.
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "location", Value: 1}}, Options: options.Index().SetName("location_2dsphere"),
	})
	require.NoError(t, err)
	near := bson.ObjectID{1}
	_, err = collection.InsertMany(ctx, []any{
		geoPlace{ID: near, Name: "near", Location: bson.M{"type": "Point", "coordinates": bson.A{-70.6, -33.4}}},
		geoPlace{ID: bson.ObjectID{2}, Name: "far", Location: bson.M{"type": "Point", "coordinates": bson.A{-70.7, -33.5}}},
		geoPlace{ID: bson.ObjectID{3}, Name: "nowhere"}, // Without a location: not indexed, not refused
	})
	require.NoError(t, err)

	require.NoError(t, manager.EnsureIndexes(geoPlace{}))
	assert.Equal(t, bson.M{"location": "2dsphere"}, storedIndexKeys(t, manager, "location_2dsphere"))

	// $nearSphere needs a geospatial index: it fails on the ascending one.
	var found []geoPlace
	cursor, err := collection.Find(ctx, bson.M{"location": bson.M{"$nearSphere": bson.M{
		"$geometry": bson.M{"type": "Point", "coordinates": bson.A{-70.6, -33.4}}, "$maxDistance": 500,
	}}})
	require.NoError(t, err)
	require.NoError(t, cursor.All(ctx, &found))
	require.Len(t, found, 1)
	assert.Equal(t, near, found[0].ID)

	// Once right, it is left as it is.
	warnings, err := manager.CompareIndexes(geoPlace{})
	require.NoError(t, err)
	for _, warning := range warnings {
		assert.NotEqual(t, IndexWarningDifferent, warning.Type, warning.Message)
	}
	require.NoError(t, manager.EnsureIndexes(geoPlace{}))
}

func TestMongoAnIndexTheDataRefusesDoesNotStopTheOthers(t *testing.T) {
	manager := testIndexManager(t)
	ctx := context.Background()
	collection := manager.getCollection(geoPlace{})
	_, err := collection.InsertOne(ctx, geoPlace{ID: bson.ObjectID{1}, Name: "broken",
		Location: bson.M{"type": "Point", "coordinates": bson.A{-70.6, 200.0}}}) // Latitude out of range
	require.NoError(t, err)

	err = manager.EnsureIndexes(geoPlace{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "location_2dsphere")
	assert.Nil(t, storedIndexKeys(t, manager, "location_2dsphere"))
	assert.Equal(t, bson.M{"name": 1}, storedIndexKeys(t, manager, "name_1"))
}
