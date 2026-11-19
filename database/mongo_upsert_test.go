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
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type upsertPlace struct {
	ID       bson.ObjectID `bson:"_id,omitempty"`
	Code     string        `bson:"code"`
	Name     string        `bson:"name"`
	Created  *time.Time    `bson:"created,omitempty"`
	Modified *time.Time    `bson:"modified,omitempty"`
	Deleted  *time.Time    `bson:"deleted"`
}

func (upsertPlace) GetTableName() string     { return "upsert_places" }
func (upsertPlace) GetModelName() string     { return "UpsertPlace" }
func (upsertPlace) GetConnectorName() string { return "mongodb" }
func (p upsertPlace) GetId() any             { return p.ID }

// testRepository is a repository of upsertPlace, with every managed field, on a database of its own
// in the MongoDB of VSAAS_REST_TEST_MONGO_URI.
func testRepository(t *testing.T) Repository[upsertPlace] {
	t.Helper()
	uri := os.Getenv("VSAAS_REST_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("VSAAS_REST_TEST_MONGO_URI is not set")
	}
	connector, err := NewMongoConnector(&MongoConnectorOpts{
		ClientOptions: *options.Client().ApplyURI(uri).SetTimeout(10 * time.Second),
		Name:          "mongodb",
		Database:      fmt.Sprintf("vsaas_rest_upsert_test_%d", time.Now().UnixNano()),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = connector.client.Database(connector.options.Database).Drop(context.Background())
		_ = connector.Disconnect()
	})
	ds := &Datasource{}
	require.NoError(t, ds.AddConnector(connector))
	repo, err := NewMongoRepository[upsertPlace](ds, RepositoryOptions{Created: true, Modified: true, Deleted: true})
	require.NoError(t, err)
	return repo
}

// FindOneOrCreate takes the model itself: it is inserted once, with the fields the repository
// manages, and found afterwards.
func TestMongoFindOneOrCreateInsertsTheModelOnce(t *testing.T) {
	repo := testRepository(t)
	ctx := context.Background()
	filter := func() *FilterBuilder { return NewFilter().WithWhere(NewWhere().Eq("code", "a")) }

	created, err := repo.FindOneOrCreate(ctx, filter(), upsertPlace{Code: "a", Name: "first"})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "first", created.Name)
	assert.NotNil(t, created.Created)
	assert.NotNil(t, created.Modified)

	found, err := repo.FindOneOrCreate(ctx, filter(), upsertPlace{Code: "a", Name: "second"})
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "first", found.Name)

	count, err := repo.Count(ctx, filter()) // Soft-delete aware: the inserted one has deleted: null
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

// An upsert that inserts gives the document its created date and soft-delete marker, so the
// repository finds it.
func TestMongoUpsertInsertsAFindableDocument(t *testing.T) {
	repo := testRepository(t)
	ctx := context.Background()
	filter := NewFilter().WithWhere(NewWhere().Eq("code", "b"))

	require.NoError(t, repo.Upsert(ctx, filter, bson.M{"$setOnInsert": bson.M{"code": "b", "name": "inserted"}}))
	stored, err := repo.FindOne(ctx, filter)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "inserted", stored.Name)
	assert.NotNil(t, stored.Created)
}
