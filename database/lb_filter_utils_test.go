package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xompass/vsaas-rest/lbq"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// An empty inq/nin list matches nothing (or everything); it must reach Mongo as [], never as
// null, which Mongo refuses ("$in needs an array").
func TestBuildWhere_EmptyListsStayArrays(t *testing.T) {
	fields := map[string]*Field{
		"id":      {BsonName: "_id", DataType: DtObjectID},
		"created": {BsonName: "created", DataType: DtDate},
	}

	query, _, err := buildWhere(lbq.Where{
		"id":      lbq.Where{"inq": []bson.ObjectID{}},
		"created": lbq.Where{"nin": []any{}},
	}, "", fields)

	assert.NoError(t, err)
	assert.Equal(t, bson.M{
		"_id":     bson.M{"$in": []bson.ObjectID{}},
		"created": bson.M{"$nin": []time.Time{}},
	}, query)
}
