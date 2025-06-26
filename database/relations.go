package database

import (
	"context"
)

type IRelation interface {
	ResolveForMany(ctx context.Context, ds *Datasource, docs []IModel) (any, error) // Resolve the relation for multiple documents
	ResolveForOne(ctx context.Context, ds *Datasource, doc IModel) (any, error)     // Resolve the relation for a single document
	Set(value any) error                                                            // Set the value of the relation
	validate() error                                                                // Validate the relation configuration
}

type RelationHasOne struct {
	Model       IModel   // Model for the relation
	Key         []string // Keys for the relation, e.g. ["userId", "userType"]
	TargetModel IModel   // Target model for the relation
	ForeignKey  []string // Keys in the target model that point to the source model, e.g. ["id", "type"]
	Set         func(any) error
}
