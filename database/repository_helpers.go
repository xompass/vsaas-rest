package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-errors/errors"
	"github.com/xompass/vsaas-rest/lbq"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (repository *MongoRepository[T]) fixQuery(query bson.M) bson.M {
	if repository.Options.Deleted {
		query = getSoftDeleteQuery(query)
	}

	return query
}

func (repository *MongoRepository[T]) prepareUpdateDocument(update any, updateDeleted UpdateOptions, setCreated UpdateOptions) (bson.M, error) {
	document, err := toBsonMap(update)
	if err != nil {
		return nil, err
	}

	hasFields := false
	hasCommands := false
	for key := range document {
		if strings.HasPrefix(key, COMMAND_PREFIX) {
			hasCommands = true
		} else {
			hasFields = true
		}
	}

	if hasFields && hasCommands {
		return bson.M{}, errors.New(MIXED_UPDATE)
	}

	var newUpdate bson.M
	var bsonSet bson.M

	if hasCommands {
		set, ok := document[SET]
		newUpdate = document
		if ok {
			switch set := set.(type) {
			case bson.M:
				bsonSet = set
			case bson.D:
				// Transform to bson.M
				bsonSet = bson.M{}
				for _, elem := range set {
					bsonSet[elem.Key] = elem.Value
				}
			default:
				_json, err := sonic.Marshal(set)
				if err != nil {
					return nil, errors.New(fmt.Sprintf("invalid $set value: %T", set))
				}

				err = sonic.Unmarshal(_json, &bsonSet)
				if err != nil {
					return nil, errors.New(fmt.Sprintf("invalid $set value: %T", set))
				}
			}
		} else {
			bsonSet = bson.M{}
		}
	}

	if hasFields {
		newUpdate = bson.M{}
		bsonSet = document
	}

	// Remove created, deleted and modified fields from update. This is managed by the repository
	if repository.Options.Created {
		delete(bsonSet, CREATED)
	}

	if repository.Options.Modified {
		delete(bsonSet, MODIFIED)
	}

	if repository.Options.Deleted {
		delete(bsonSet, DELETED)
	}

	if len(bsonSet) > 0 {
		newUpdate[SET] = bsonSet
	}

	if repository.Options.Modified || repository.Options.Created || repository.Options.Deleted {
		currentDate, ok := document[CURRENT_DATE]
		var bsonCurrentDate bson.M
		if ok {
			bsonCurrentDate, ok = currentDate.(bson.M)
			if !ok {
				return nil, errors.New("invalid $currentDate value")
			}
		} else {
			bsonCurrentDate = bson.M{}
		}

		// The MODIFIED date is set
		if repository.Options.Modified {
			bsonCurrentDate[MODIFIED] = true
		}

		if repository.Options.Deleted {
			// The DELETED date is set if required
			if updateDeleted.Update {
				bsonCurrentDate[DELETED] = true
			} else {
				delete(bsonCurrentDate, DELETED)
			}
		}

		if repository.Options.Created {
			// The CREATED date is set if required
			if setCreated.Update && !setCreated.Insert {
				bsonCurrentDate[CREATED] = true
			} else {
				delete(bsonCurrentDate, CREATED)
			}
		}

		if len(bsonCurrentDate) > 0 {
			newUpdate[CURRENT_DATE] = bsonCurrentDate
		} else {
			delete(newUpdate, CURRENT_DATE)
		}
	}

	if repository.Options.Created && setCreated.Insert || repository.Options.Deleted && setCreated.Insert {
		temp, ok := newUpdate[SET_ON_INSERT]
		var setOnInsert bson.M
		if ok {
			setOnInsert, ok = temp.(bson.M)
			if !ok {
				return nil, errors.New("invalid $setOnInsert value")
			}
		} else {
			setOnInsert = bson.M{}
		}

		// The created date is set if required
		if repository.Options.Created && setCreated.Insert {
			setOnInsert[CREATED] = time.Now()
		}

		// Deleted date is set to nil if required
		if repository.Options.Deleted && setCreated.Insert {
			setOnInsert[DELETED] = nil
		}

		if len(setOnInsert) > 0 {
			newUpdate[SET_ON_INSERT] = setOnInsert
		} else {
			delete(newUpdate, SET_ON_INSERT)
		}
	}

	if len(newUpdate) == 0 {
		return nil, errors.New("the update document is empty")
	}

	return newUpdate, nil
}

func (repository *MongoRepository[T]) prepareInsertDocument(doc any) (bson.M, error) {
	document, err := toBsonMap(doc)
	if err != nil {
		return nil, err
	}

	if repository.Options.Created {
		document[CREATED] = time.Now()
	}

	if repository.Options.Modified {
		document[MODIFIED] = time.Now()
	}

	if repository.Options.Deleted {
		document[DELETED] = nil
	}

	return document, nil
}

func getSoftDeleteQuery(query bson.M) bson.M {
	return bson.M{
		AND: []any{
			query,
			bson.M{DELETED: bson.M{TYPE: 10}},
		},
	}
}

func toBsonMap(v any) (doc bson.M, err error) {
	if v == nil {
		return bson.M{}, nil
	}

	if bsonMap, ok := v.(bson.M); ok {
		return bsonMap, nil
	}

	data, err := bson.Marshal(v)
	if err != nil {
		return
	}

	err = bson.Unmarshal(data, &doc)
	return doc, err
}

func (repository *MongoRepository[T]) buildQuery(filterBuilder FilterBuilder) (bson.M, MongoFilter, *lbq.Filter, error) {
	filter, err := filterBuilder.Build()
	if err != nil {
		return nil, MongoFilter{}, nil, err
	}

	parsedFilter, err := adaptLoopbackFilter(*filter, repository.schema)
	if err != nil {
		return nil, MongoFilter{}, nil, err
	}

	query := repository.fixQuery(parsedFilter.Where)

	return query, parsedFilter, filter, nil
}

func (repository *MongoRepository[T]) resolveIncludes(ctx context.Context, doc *T, includes []lbq.Include) error {
	// TODO: Implement a way to resolve includes
	return nil
}
