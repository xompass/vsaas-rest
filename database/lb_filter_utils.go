package database

import (
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/xompass/vsaas-rest/lbq"

	"github.com/go-errors/errors"

	"github.com/simplereach/timeutils"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var Operators = map[string]string{
	"eq":     "$eq",
	"neq":    "$ne",
	"gt":     "$gt",
	"gte":    "$gte",
	"lt":     "$lt",
	"lte":    "$lte",
	"inq":    "$in",
	"nin":    "$nin",
	"and":    "$and",
	"or":     "$or",
	"exists": "$exists",
}

const (
	DtObjectID = "ObjectID"
	DtDate     = "Date"
)

type MongoFilterOptions struct {
	Limit  *uint
	Skip   *uint
	Sort   any
	Fields map[string]bool
}

type MongoIncludes struct {
	Relation string
	Scope    lbq.Filter
}

type MongoFilter struct {
	Where   bson.M
	Options MongoFilterOptions
	Include []MongoIncludes
}

func adaptLoopbackFilter(filter lbq.Filter, schema *Schema) (MongoFilter, error) {
	where := filter.Where

	result := MongoFilter{}

	parsedWhere, err := buildWhere(where, "", schema.JSONFields)
	if err != nil {
		return result, err
	}

	if len(parsedWhere) == 0 && len(filter.Where) != 0 {
		return result, errors.New("invalid where parameter")
	}

	parsedSort := buildSort(filter.Order)

	if len(parsedSort) == 0 && len(filter.Order) != 0 {
		return result, errors.New("invalid order parameter")
	}

	result.Where = parsedWhere

	result.Options.Sort = parsedSort
	if filter.Limit != 0 {
		limit := uint(filter.Limit)
		result.Options.Limit = &limit
	}
	if filter.Skip != 0 {
		skip := uint(filter.Skip)
		result.Options.Skip = &skip
	}

	if len(filter.Fields) > 0 {
		projection := map[string]bool{}

		for key, val := range filter.Fields {
			if _, exists := getFieldIfExists(key, schema.JSONFields); exists {
				projection[key] = val
			}
		}

		for _, field := range schema.RequiredFilterFields {
			projection[field.JsonName] = true
		}

		for _, field := range schema.BannedFields {
			delete(projection, field.JsonName)
		}

		if len(projection) > 0 {
			result.Options.Fields = projection
		} else {
			result.Options.Fields = map[string]bool{"_id": true}
		}
	} else if len(schema.BannedFields) > 0 {
		projection := map[string]bool{}

		for _, field := range schema.BannedFields {
			projection[field.JsonName] = false
		}

		result.Options.Fields = projection
	}

	return result, nil
}

func buildSort(order []lbq.Order) bson.D {
	if order == nil {
		return bson.D{}
	}

	sort := bson.D{}
	for _, lbOrder := range order {
		if lbOrder.Direction == "DESC" {
			sort = append(sort, bson.E{Key: lbOrder.Field, Value: -1})
		} else {
			sort = append(sort, bson.E{Key: lbOrder.Field, Value: 1})
		}
	}

	return sort
}

func buildWhere(where lbq.Where, parentField string, fields map[string]*Field) (bson.M, error) {
	if where == nil {
		return bson.M{}, nil
	}

	if _, ok := where["$where"]; ok {
		return nil, errors.New("invalid where parameter. $where is not allowed")
	}

	query := bson.M{}

	like, hasLikeCond := where["like"]
	nLike, hasNLikeCond := where["nlike"]
	opts := where["options"]

	exists, hasExistsCond := where["exists"]

	switch {
	case hasExistsCond:
		if _, ok := exists.(bool); !ok {
			return nil, errors.New("invalid where parameter. exists must be boolean")
		}
		query["$exists"] = exists
	case hasLikeCond:
		query["$regex"] = like
		if opts != nil {
			query["$options"] = opts
		}
	case hasNLikeCond:
		regex := bson.M{"$regex": nLike}
		if opts != nil {
			regex["$options"] = opts
		}
		query["$not"] = regex
	default:
		for key, val := range where {
			if strings.HasPrefix(key, "$") {
				continue
			}

			_mongoOp, isOperator := Operators[key]
			var operatorName string
			var fieldName string
			var field *Field

			if isOperator {
				operatorName = _mongoOp
				/*_field, exists, err := getRootFieldIfExists(parentField, fields)
				if err != nil {
					return bson.M{}, err
				}*/
				_field, exists := getFieldIfExists(parentField, fields)
				if exists {
					field = _field
					fieldName = _field.BsonName
				}
			} else {
				/*_field, exists, err := getRootFieldIfExists(key, fields)
				if err != nil {
					return bson.M{}, err
				}*/

				_field, exists := getFieldIfExists(key, fields)
				if !exists {
					log.Println("field not exists", key)
					continue
				}
				field = _field
				fieldName = field.BsonName
				operatorName = fieldName
			}

			switch v := val.(type) {
			case lbq.AndOrCondition:
				arr := v
				barr := bson.A{}

				for _, el := range arr {
					whr, err := buildWhere(el, parentField, fields)
					if err != nil {
						return bson.M{}, err
					}
					if len(whr) > 0 {
						barr = append(barr, whr)
					}
				}

				if len(barr) == 0 {
					return bson.M{}, errors.New("invalid and/or condition")
				}

				query[operatorName] = barr
			case lbq.Where:
				whr, err := buildWhere(v, key, fields)
				if err != nil {
					return bson.M{}, err
				}
				if len(whr) > 0 {
					query[fieldName] = whr
				}
			default:
				switch field.DataType {
				case DtObjectID:
					if key == "inq" || key == "nin" {
						arr, err := getObjectIdArray(val)
						if err == nil {
							query[operatorName] = arr
						}
					} else {
						var oidVal any
						var err error
						if field.IsPointer {
							oidVal, err = getObjectIdOrNil(val)
						} else {
							oidVal, err = getObjectId(val)
						}

						if err == nil {
							query[operatorName] = oidVal
						}
					}
				case DtDate:
					if key == "inq" || key == "nin" {
						arr, err := getDateArray(val)
						if err == nil {
							query[operatorName] = arr
						}
					} else {
						var dateVal any
						var err error
						if field.IsPointer {
							dateVal, err = getDateOrNil(val)
						} else {
							dateVal, err = getDate(val)
						}
						if err == nil {
							query[operatorName] = dateVal
						}
					}
				default:
					query[operatorName] = val
				}
			}
		}
	}

	return query, nil
}

func getObjectIdArray(val any) ([]bson.ObjectID, error) {
	rv := reflect.ValueOf(val)
	if rv.Kind() == reflect.Slice {
		var arr []bson.ObjectID
		var valErr error
		for i := 0; i < rv.Len(); i++ {
			el := rv.Index(i).Interface()
			oid, err := getObjectId(el)
			if err != nil {
				valErr = err
				break
			}
			arr = append(arr, oid)
		}

		return arr, valErr
	} else {
		return nil, errors.New("invalid objectid collection")
	}
}

func getObjectIdOrNil(val any) (*bson.ObjectID, error) {
	if val == nil {
		return nil, nil
	}
	id, err := getObjectId(val)
	return &id, err
}

func getObjectId(val any) (bson.ObjectID, error) {
	switch v := val.(type) {
	case string:
		oid := v
		value, err := bson.ObjectIDFromHex(oid)
		return value, err
	case *string:
		oid := v
		value, err := bson.ObjectIDFromHex(*oid)
		return value, err
	case bson.ObjectID:
		return v, nil
	case *bson.ObjectID:
		oid, _ := val.(*bson.ObjectID)
		if oid == nil {
			return bson.ObjectID{}, errors.New("invalid ObjectID")
		}
		return *oid, nil
	default:
		return bson.ObjectID{}, errors.New("invalid ObjectID")
	}
}

// getDateArray returns a []time.Time from the given value.
func getDateArray(val any) ([]time.Time, error) {
	valArr, ok := val.([]any)
	if !ok {
		return nil, errors.New("invalid date collection")
	}

	var arr []time.Time
	var valErr error
	for _, s := range valArr {
		_date, err := getDate(s)
		if err != nil {
			valErr = err
			break
		}
		arr = append(arr, _date)
	}
	return arr, valErr
}

// getDateOrNil returns a time.Time or nil from the given value.
func getDateOrNil(val any) (*time.Time, error) {
	if val == nil {
		return nil, nil
	}

	value, err := getDate(val)
	return &value, err
}

// getDate returns a time.Time value from the given value.
func getDate(val any) (time.Time, error) {
	if val == nil {
		return time.Time{}, errors.New("invalid date")
	}

	switch v := val.(type) {
	case time.Time:
		return v, nil
	case *time.Time:
		return *v, nil
	case string:
		return timeutils.ParseDateString(v)
	case *string:
		return timeutils.ParseDateString(*v)
	case int64:
		return time.Unix(v, 0), nil
	case *int64:
		return time.Unix(*v, 0), nil
	default:
		return time.Time{}, errors.New("invalidate date format")
	}
}

/*func getRootFieldIfExists(fieldName string, fields map[string]*Field) (*Field, bool, error) {
	parts := strings.Split(fieldName, ".")
	if len(parts) > 1 {
		return nil, false, errors.New("can not query on nested fields")
	}

	rootField := parts[0]
	field, exists := fields[rootField]
	return field, exists, nil
}*/

func getFieldIfExists(fieldName string, fields map[string]*Field) (*Field, bool) {
	field, exists := fields[fieldName]
	if exists {
		return field, exists
	}

	parentField := fieldName
	for {
		lastDotIndex := strings.LastIndex(parentField, ".")
		if lastDotIndex == -1 {
			return nil, false
		}

		parentField = fieldName[0:lastDotIndex]
		field, exists = fields[parentField]
		if exists {
			return field, exists
		}
	}
}
