package database

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/xompass/vsaas-rest/helpers"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

type MongoConnectorOpts struct {
	options.ClientOptions
	Name     string
	Database string
}

type MongoConnector struct {
	ctx          context.Context
	client       *mongo.Client
	options      *MongoConnectorOpts
	indexManager *MongoIndexManager
}

/**
 * NewMongoConnector creates a new MongoDB connector.
 * It initializes the MongoDB client with the provided options and checks the connection.
 */
func NewMongoConnector(opts *MongoConnectorOpts) (*MongoConnector, error) {
	ctx := context.Background()
	connector := &MongoConnector{
		ctx:     ctx,
		options: opts,
	}

	err := connector.connect()
	if err != nil {
		return nil, err
	}

	if err := connector.Ping(); err != nil {
		return nil, err
	}

	return connector, nil
}

func NewDefaultMongoConnector() (*MongoConnector, error) {
	uri := helpers.GetEnv("MONGO_URI", "mongodb://localhost:27017")

	clientOptions := (&options.ClientOptions{}).ApplyURI(uri)

	conn, err := connstring.Parse(uri)
	if err != nil {
		return nil, err
	}

	dbName := conn.Database
	if dbName == "" {
		dbName = "test"
	}

	opts := MongoConnectorOpts{
		ClientOptions: *clientOptions,
		Name:          "mongodb",
		Database:      helpers.GetEnv("MONGO_DATABASE", dbName),
	}

	return NewMongoConnector(&opts)
}

/**
 * connect initializes the MongoDB client with the provided options.
 */
func (receiver *MongoConnector) connect() error {
	opts := receiver.options.ClientOptions

	client, err := mongo.Connect(&opts)

	if err != nil {
		return err
	}

	receiver.client = client
	receiver.indexManager = NewMongoIndexManager(receiver)
	return nil
}

/**
 * Ping checks the connection to the MongoDB server.
 */
func (receiver *MongoConnector) Ping() error {
	if receiver.client == nil {
		return errors.New("go_mongo_repository client not initialized")
	}
	return receiver.client.Ping(receiver.ctx, nil)
}

/**
 * Disconnect closes the connection to the MongoDB server.
 */
func (receiver *MongoConnector) Disconnect() error {
	if receiver.client == nil {
		return errors.New("go_mongo_repository client not initialized")
	}
	return receiver.client.Disconnect(receiver.ctx)
}

/**
 * GetDriver returns the underlying MongoDB client.
 */
func (receiver *MongoConnector) GetDriver() any {
	return receiver.client
}

func (receiver *MongoConnector) GetName() string {
	return receiver.options.Name
}

func (receiver *MongoConnector) GetDatabaseName() string {
	return receiver.options.Database
}

/**
 * GetOptions returns the options used to create the MongoDB connector.
 */
func (receiver *MongoConnector) GetOptions() MongoConnectorOpts {
	return *receiver.options
}

/**
 * GetIndexManager returns the index manager for this connector.
 */
func (receiver *MongoConnector) GetIndexManager() *MongoIndexManager {
	return receiver.indexManager
}
