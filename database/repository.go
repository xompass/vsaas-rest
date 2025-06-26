package database

import (
	"context"
)

type Repository[T IModel] interface {
	// GetSchema returns the schema of the model used by this repository.
	GetSchema() *Schema

	// GetConnector returns the connector used by this repository.
	// This is useful for accessing the underlying database connection.
	// It is typically used for advanced operations that are not covered by the repository methods.
	GetConnector() Connector

	// Find retrieves all documents matching the filter.
	// If no documents match, it returns an empty slice.
	// If an error occurs, it returns an error.
	Find(ctx context.Context, filter *FilterBuilder) ([]T, error)

	// FindOne retrieves a single document matching the filter.
	// If multiple documents match, it returns the first one found.
	// If no documents match, it returns an error.
	FindOne(ctx context.Context, filter *FilterBuilder) (*T, error)

	// FindById retrieves a single document by its ID.
	// If the document does not exist, it returns an error.
	FindById(ctx context.Context, id any, filter *FilterBuilder) (*T, error)

	// Insert inserts a new document into the collection.
	// It returns the inserted document's ID or an error if the operation fails.
	Insert(ctx context.Context, doc T) (any, error)

	// Create inserts a new document into the collection and returns the created document.
	Create(ctx context.Context, doc T) (*T, error)

	// FindOneOrCreate finds a document matching the filter or creates a new one if it does not exist.
	FindOneOrCreate(ctx context.Context, filter *FilterBuilder, doc T) (*T, error)

	// Upsert updates a document matching the filter or inserts a new one if it does not exist.
	Upsert(ctx context.Context, filter *FilterBuilder, update any) error

	// UpdateOne updates a single document matching the filter.
	UpdateOne(ctx context.Context, filter *FilterBuilder, update any) error

	// UpdateById updates a single document by its ID.
	UpdateById(ctx context.Context, id any, update any) error

	// FindOneAndUpdate finds a single document matching the filter and updates it.
	FindOneAndUpdate(ctx context.Context, filter *FilterBuilder, update any) (*T, error)

	// UpdateMany updates all documents matching the filter.
	UpdateMany(ctx context.Context, filter *FilterBuilder, update any) (int64, error)

	// Count returns the number of documents matching the filter.
	Count(ctx context.Context, filter *FilterBuilder) (int64, error)

	// Exists checks if a document with the given ID exists in the collection.
	Exists(ctx context.Context, id any) (bool, error)

	// DeleteOne deletes a single document matching the filter.
	DeleteOne(ctx context.Context, filter *FilterBuilder) error

	// DeleteById deletes a single document by its ID.
	DeleteById(ctx context.Context, id any) error

	// DeleteMany deletes all documents matching the filter.
	DeleteMany(ctx context.Context, filter *FilterBuilder) (int64, error)
}
