package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type IModel interface {
	GetTableName() string
	GetModelName() string
	GetConnectorName() string
	GetId() any
}

type BeforeCreateHook interface {
	BeforeCreate() error
}

type BeforeUpdateHook interface {
	BeforeUpdate() error
}

type BeforeDeleteHook interface {
	BeforeDelete() error
}

type ModelRelation struct {
	Name   string `json:"name"`
	IsList bool   `json:"isList"`

	Resolve func(ctx context.Context, ds *Datasource) (any, error)
	Set     func(value any) error
}

type IRelationalModel interface {
	Relations() map[string]ModelRelation
}

type MongoDate struct {
	time.Time
}

var dateFormat = "2006-01-02T15:04:05.000Z"

func (date *MongoDate) UnmarshalBSONValue(t bson.Type, data []byte) error {
	switch t {
	case bson.TypeDateTime:
		// Caso normal: fecha guardada como DateTime BSON
		if len(data) < 8 {
			return fmt.Errorf("invalid DateTime data length")
		}

		// Leer los 8 bytes como int64 (little-endian)
		milliseconds := int64(data[0]) | int64(data[1])<<8 | int64(data[2])<<16 | int64(data[3])<<24 |
			int64(data[4])<<32 | int64(data[5])<<40 | int64(data[6])<<48 | int64(data[7])<<56

		*date = MongoDate{time.Unix(0, milliseconds*int64(time.Millisecond))}
		return nil

	case bson.TypeInt64:
		// Caso problemático: fecha guardada como número (milliseconds)
		if len(data) < 8 {
			return fmt.Errorf("invalid Int64 data length")
		}

		// Leer los 8 bytes como int64 (little-endian)
		milliseconds := int64(data[0]) | int64(data[1])<<8 | int64(data[2])<<16 | int64(data[3])<<24 |
			int64(data[4])<<32 | int64(data[5])<<40 | int64(data[6])<<48 | int64(data[7])<<56

		*date = MongoDate{time.Unix(0, milliseconds*int64(time.Millisecond))}
		return nil

	case bson.TypeInt32:
		// Caso adicional: fecha guardada como int32 (seconds desde epoch)
		if len(data) < 4 {
			return fmt.Errorf("invalid Int32 data length")
		}

		// Leer los 4 bytes como int32 (little-endian)
		seconds := int32(data[0]) | int32(data[1])<<8 | int32(data[2])<<16 | int32(data[3])<<24

		*date = MongoDate{time.Unix(int64(seconds), 0)}
		return nil

	default:
		return fmt.Errorf("cannot unmarshal %v into MongoDate", t)
	}
}

func (date MongoDate) MarshalBSONValue() (bson.Type, []byte, error) {
	milliseconds := date.Time.UnixNano() / int64(time.Millisecond)

	// Convertir int64 a bytes (little-endian)
	data := make([]byte, 8)
	data[0] = byte(milliseconds)
	data[1] = byte(milliseconds >> 8)
	data[2] = byte(milliseconds >> 16)
	data[3] = byte(milliseconds >> 24)
	data[4] = byte(milliseconds >> 32)
	data[5] = byte(milliseconds >> 40)
	data[6] = byte(milliseconds >> 48)
	data[7] = byte(milliseconds >> 56)

	return bson.TypeDateTime, data, nil
}

func (date *MongoDate) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", date.Time.Format(dateFormat))
	return []byte(stamp), nil
}
