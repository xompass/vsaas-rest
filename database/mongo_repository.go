package database

import (
	"context"

	"github.com/go-errors/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	ID             = "id"
	SET            = "$set"
	AND            = "$and"
	CREATED        = "created"
	MODIFIED       = "modified"
	DELETED        = "deleted"
	CURRENT_DATE   = "$currentDate"
	SET_ON_INSERT  = "$setOnInsert"
	TYPE           = "$type"
	COMMAND_PREFIX = "$"
	NO_DOCUMENTS   = "no documents founds"
	MIXED_UPDATE   = "the update has a mix between fields and commands"
)

type MongoRepository[T IModel] struct {
	Options    RepositoryOptions
	collection *mongo.Collection
	schema     *Schema
	connector  *MongoConnector
	datasource *Datasource
}

func NewMongoRepository[T IModel](ds *Datasource, options RepositoryOptions) (Repository[T], error) {
	var instance T
	collectionName := instance.GetTableName()

	schema := NewSchema(instance)

	err := ds.RegisterModel(instance)
	if err != nil {
		return nil, err
	}

	tmp, err := ds.GetModelConnector(instance)
	if err != nil {
		return nil, err
	}

	connector, ok := tmp.(*MongoConnector)
	if !ok {
		return nil, errors.Errorf("the connector for model %s is not a MongoConnector", instance.GetModelName())
	}

	if connector == nil {
		return nil, errors.New("connector is nil")
	}

	connectorOpts := connector.GetOptions()
	client, ok := connector.GetDriver().(*mongo.Client)
	if !ok {
		return nil, errors.New("the MongoDB client is not initialized correctly")
	}

	databaseName := connectorOpts.Database
	if databaseName == "" {
		return nil, errors.New("database name is required")
	}

	repository := &MongoRepository[T]{
		Options:    options,
		collection: client.Database(databaseName).Collection(collectionName),
		schema:     schema,
		connector:  connector,
		datasource: ds,
	}

	RegisterDatasourceRepository(ds, instance, repository)

	return repository, nil
}

func (repository *MongoRepository[T]) GetCollection() *mongo.Collection {
	return repository.collection
}

func (repository *MongoRepository[T]) GetSchema() *Schema {
	return repository.schema
}

func (repository *MongoRepository[T]) GetConnector() Connector {
	return repository.connector
}

func (repository *MongoRepository[T]) Find(ctx context.Context, filterBuilder *FilterBuilder) ([]T, error) {
	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}
	query, parsedFilter, _, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return nil, err
	}

	findOpts := options.Find()
	if parsedFilter.Options.Sort != nil {
		findOpts.SetSort(parsedFilter.Options.Sort)
	}
	if parsedFilter.Options.Limit != nil {
		limit := int64(*parsedFilter.Options.Limit)
		findOpts.SetLimit(limit)
	}
	if parsedFilter.Options.Skip != nil {
		skip := int64(*parsedFilter.Options.Skip)
		findOpts.SetSkip(skip)
	}
	if parsedFilter.Options.Fields != nil {
		findOpts.SetProjection(parsedFilter.Options.Fields)
	}

	cursor, err := repository.collection.Find(ctx, query, findOpts)

	if err != nil {
		return nil, err
	}

	var receiver []T
	if err = cursor.All(ctx, &receiver); err != nil {
		return nil, err
	}

	if receiver == nil {
		return []T{}, nil
	}
	return receiver, nil
}

func (repository *MongoRepository[T]) FindOne(ctx context.Context, filterBuilder *FilterBuilder) (*T, error) {
	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	query, parsedFilter, lbFilter, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return nil, err
	}

	receiver := new(T)

	findOneOptions := options.FindOne()
	if parsedFilter.Options.Sort != nil {
		findOneOptions.SetSort(parsedFilter.Options.Sort)
	}

	if parsedFilter.Options.Skip != nil {
		skip := int64(*parsedFilter.Options.Skip)
		findOneOptions.SetSkip(skip)
	}

	if parsedFilter.Options.Fields != nil {
		findOneOptions.SetProjection(parsedFilter.Options.Fields)
	}

	result := repository.collection.FindOne(ctx, query, findOneOptions)

	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, result.Err()
	}

	err = result.Decode(receiver)

	if err != nil {
		return nil, err
	}

	// Resolve includes if any
	repository.resolveIncludes(ctx, receiver, lbFilter.Include)

	return receiver, err
}

func (repository *MongoRepository[T]) FindById(ctx context.Context, id any, filterBuilder *FilterBuilder) (*T, error) {
	if id == nil {
		return nil, errors.New("id cannot be nil")
	}

	var filterClone *FilterBuilder
	if filterBuilder == nil {
		filterClone = NewFilter()
	} else {
		filterClone = filterBuilder.Clone()
	}

	newWhere := NewWhere().Eq(ID, id)
	filterClone.WithWhere(newWhere)

	return repository.FindOne(ctx, filterClone)
}

func (repository *MongoRepository[T]) Insert(ctx context.Context, doc T) (any, error) {
	if hook, ok := any(&doc).(BeforeCreateHook); ok {
		if err := hook.BeforeCreate(); err != nil {
			return nil, err
		}
	}

	document, err := repository.prepareInsertDocument(doc)
	if err != nil {
		return nil, err
	}

	insertedResult, err := repository.collection.InsertOne(ctx, document)

	if err != nil {
		return nil, err
	}

	return insertedResult.InsertedID, nil
}

func (repository *MongoRepository[T]) Create(ctx context.Context, doc T) (*T, error) {
	insertedID, err := repository.Insert(ctx, doc)
	if err != nil {
		return nil, err
	}

	return repository.FindById(ctx, insertedID, NewFilter())
}

func (repository *MongoRepository[T]) FindOneOrCreate(ctx context.Context, filterBuilder *FilterBuilder, doc T) (*T, error) {
	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	upsert := true
	after := options.After

	return repository.applyFindOneAndUpdate(ctx, filterBuilder, bson.M{
		"$setOnInsert": doc,
	}, &options.FindOneAndUpdateOptions{Upsert: &upsert, ReturnDocument: &after})
}

func (repository *MongoRepository[T]) Upsert(ctx context.Context, filterBuilder *FilterBuilder, update any) error {
	if update == nil {
		return errors.New("update cannot be nil")
	}

	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}
	query, _, _, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return err
	}

	upsert := true
	fixedUpdate, err := repository.prepareUpdateDocument(update, UpdateOptions{}, UpdateOptions{})
	if err != nil {
		return err
	}

	updateOptions := options.UpdateOne()
	updateOptions.SetUpsert(upsert)

	_, err = repository.collection.UpdateOne(ctx, query, fixedUpdate, updateOptions)
	if err != nil {
		return err
	}

	return nil
}

func (repository *MongoRepository[T]) UpdateOne(ctx context.Context, filterBuilder *FilterBuilder, update any) error {
	if update == nil {
		return errors.New("update cannot be nil")
	}

	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	query, _, _, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return err
	}

	fixedUpdate, err := repository.prepareUpdateDocument(update, UpdateOptions{}, UpdateOptions{})
	if err != nil {
		return err
	}

	_, err = repository.collection.UpdateOne(ctx, query, fixedUpdate)
	if err != nil {
		return err
	}

	return nil
}

func (repository *MongoRepository[T]) UpdateById(ctx context.Context, id any, update any) error {
	if id == nil {
		return errors.New("id cannot be nil")
	}

	if update == nil {
		return errors.New("update cannot be nil")
	}

	filter := NewFilter().
		WithWhere(NewWhere().Eq(ID, id))
	return repository.UpdateOne(ctx, filter, update)
}

func (repository *MongoRepository[T]) FindOneAndUpdate(ctx context.Context, filterBuilder *FilterBuilder, update any) (*T, error) {
	return repository.applyFindOneAndUpdate(ctx, filterBuilder, update)
}

func (repository *MongoRepository[T]) applyFindOneAndUpdate(ctx context.Context, filterBuilder *FilterBuilder, update any, opts ...*options.FindOneAndUpdateOptions) (*T, error) {
	if update == nil {
		return nil, errors.New("update cannot be nil")
	}

	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	query, _, filter, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return nil, err
	}

	var updateOptions *options.FindOneAndUpdateOptions
	setCreated := false
	if len(opts) > 0 {
		updateOptions = opts[0]
		setCreated = *updateOptions.Upsert
	} else {
		updateOptions = &options.FindOneAndUpdateOptions{}
	}

	updateOptions.Projection = filter.Fields
	if updateOptions.ReturnDocument == nil {
		afterUpdate := options.After
		updateOptions.ReturnDocument = &afterUpdate
	}

	fixedUpdate, err := repository.prepareUpdateDocument(update, UpdateOptions{}, UpdateOptions{Insert: setCreated})

	if err != nil {
		return nil, err
	}

	receiver := new(T)

	cmdOpts := options.FindOneAndUpdate()

	if updateOptions.Sort != nil {
		cmdOpts.SetSort(updateOptions.Sort)
	}
	if updateOptions.ReturnDocument != nil {
		cmdOpts.SetReturnDocument(*updateOptions.ReturnDocument)
	}
	if updateOptions.Projection != nil {
		cmdOpts.SetProjection(updateOptions.Projection)
	}
	if updateOptions.Upsert != nil {
		cmdOpts.SetUpsert(*updateOptions.Upsert)
	}

	if updateOptions.Collation != nil {
		cmdOpts.SetCollation(updateOptions.Collation)
	}

	if updateOptions.Hint != nil {
		cmdOpts.SetHint(updateOptions.Hint)
	}

	if updateOptions.ArrayFilters != nil {
		cmdOpts.SetArrayFilters(updateOptions.ArrayFilters)
	}

	if updateOptions.BypassDocumentValidation != nil {
		cmdOpts.SetBypassDocumentValidation(*updateOptions.BypassDocumentValidation)
	}

	result := repository.collection.FindOneAndUpdate(ctx, query, fixedUpdate, cmdOpts)

	if err := result.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	err = result.Decode(receiver)
	if err != nil {
		return nil, err
	}

	return receiver, nil
}

func (repository *MongoRepository[T]) UpdateMany(ctx context.Context, filterBuilder *FilterBuilder, update any) (int64, error) {
	if update == nil {
		return 0, errors.New("update cannot be nil")
	}

	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	query, _, _, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return 0, err
	}

	fixedUpdate, err := repository.prepareUpdateDocument(update, UpdateOptions{}, UpdateOptions{})
	if err != nil {
		return 0, err
	}

	result, err := repository.collection.UpdateMany(ctx, query, fixedUpdate)
	if err != nil {
		return 0, err
	}

	return result.ModifiedCount, nil
}

func (repository *MongoRepository[T]) Count(ctx context.Context, filterBuilder *FilterBuilder) (int64, error) {
	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}
	query, _, _, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return 0, err
	}

	return repository.collection.CountDocuments(ctx, query)
}

func (repository *MongoRepository[T]) Exists(ctx context.Context, id any) (bool, error) {
	if id == nil {
		return false, errors.New("id cannot be nil")
	}

	filter := NewFilter().
		WithWhere(NewWhere().Eq(ID, id)).
		Fields(map[string]bool{
			"_id": true,
		})

	doc, err := repository.FindOne(ctx, filter)
	if err != nil {
		return false, err
	}

	if doc != nil {
		return true, nil
	}

	return false, nil
}

func (repository *MongoRepository[T]) DeleteOne(ctx context.Context, filterBuilder *FilterBuilder) error {
	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	query, _, _, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return err
	}

	if repository.Options.Deleted {
		result, err := repository.collection.UpdateOne(ctx, query, bson.M{CURRENT_DATE: bson.M{DELETED: true}})
		if err != nil {
			return err
		}
		if result.MatchedCount == 0 {
			return errors.New(NO_DOCUMENTS)
		}
		return nil
	}

	result, err := repository.collection.DeleteOne(ctx, query)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return errors.New(NO_DOCUMENTS)
	}

	return nil
}

func (repository *MongoRepository[T]) DeleteById(ctx context.Context, id any) error {
	if id == nil {
		return errors.New("id cannot be nil")
	}

	filterBuilder := NewFilter().
		WithWhere(NewWhere().Eq(ID, id))

	return repository.DeleteOne(ctx, filterBuilder)
}

func (repository *MongoRepository[T]) DeleteMany(ctx context.Context, filterBuilder *FilterBuilder) (int64, error) {
	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	query, _, _, err := repository.buildQuery(*filterBuilder)
	if err != nil {
		return 0, err
	}

	if repository.Options.Deleted {
		result, err := repository.collection.UpdateMany(ctx, query, bson.M{CURRENT_DATE: bson.M{DELETED: true}})
		if err != nil {
			return 0, err
		}
		return result.ModifiedCount, nil
	}

	result, err := repository.collection.DeleteMany(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}
