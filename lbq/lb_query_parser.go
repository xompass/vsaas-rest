package lbq

import (
	"log"
	"strings"

	"github.com/go-errors/errors"
	"github.com/valyala/fastjson"
)

var filterPool fastjson.ParserPool
var wherePool fastjson.ParserPool
var fieldsPool fastjson.ParserPool
var orderPool fastjson.ParserPool
var includePool fastjson.ParserPool

var operators = map[string]bool{
	"eq":     true,
	"neq":    true,
	"gt":     true,
	"gte":    true,
	"lt":     true,
	"lte":    true,
	"inq":    true,
	"nin":    true,
	"and":    true,
	"or":     true,
	"like":   true,
	"nlike":  true,
	"exists": true,
} // @name Operator

type AndOrCondition []Where

type Where map[string]interface{} // @name Where

type Fields map[string]bool // @name Fields

type Order struct {
	Field     string `json:"field,omitempty"`
	Direction string `json:"Direction,omitempty"`
} // @name Order

type Filter struct {
	Fields  Fields    `json:"fields,omitempty"`
	Limit   uint      `json:"limit,omitempty"`
	Order   []Order   `json:"order,omitempty"`
	Skip    uint      `json:"skip,omitempty"`
	Where   Where     `json:"where,omitempty"`
	Include []Include `json:"include,omitempty"`
} // @name Filter

type Include struct {
	Relation string  `json:"relation,omitempty"`
	Scope    *Filter `json:"scope,omitempty"`
} // @name Include

func parseWhereValue(where *fastjson.Value) (Where, error) {
	if where == nil {
		return nil, nil
	}

	if where.Type() != fastjson.TypeObject {
		return nil, errors.New("invalid where filter")
	}

	val, _ := where.Object()

	var nestedError error = nil

	likeCond := val.Get("like")
	nlikeCond := val.Get("nlike")
	opts := val.Get("options")

	if likeCond != nil {
		return Where{
			"like":    getRawValue(likeCond),
			"options": getRawValue(opts),
		}, nil
	}

	if nlikeCond != nil {
		return Where{
			"nlike":   getRawValue(nlikeCond),
			"options": getRawValue(opts),
		}, nil
	}

	result := Where{}
	val.Visit(func(key []byte, v *fastjson.Value) {
		keyStr := string(key)

		// Prevent ma
		if strings.HasPrefix(keyStr, "$") {
			nestedError = errors.Errorf("invalid use of operator or field: %s", keyStr)
			return
		}

		valueType := v.Type()

		switch {
		case keyStr == "and" || keyStr == "or":
			if valueType != fastjson.TypeArray {
				nestedError = errors.New("invalid query")
				return
			}
			andOr := AndOrCondition{}
			arr, _ := v.Array()
			for _, nested := range arr {
				cond, err := parseWhereValue(nested)
				if err != nil {
					nestedError = err
				}
				andOr = append(andOr, cond)
			}
			result[keyStr] = andOr
		case valueType == fastjson.TypeObject:
			lbWhere, err := parseWhereValue(v)
			if err != nil {
				nestedError = err
			}
			result[keyStr] = lbWhere
		default:
			_, isOp := operators[keyStr]
			if isOp && (keyStr == "inq" || keyStr == "nin") && valueType != fastjson.TypeArray {
				nestedError = errors.New("invalid query")
				return
			}
			value := getRawValue(v)
			if isOp {
				result[keyStr] = value
			} else {
				result[keyStr] = Where{
					"eq": value,
				}

			}
		}
	})

	return result, nestedError
}

func getRawValue(v *fastjson.Value) interface{} {
	if v == nil {
		return nil
	}
	valueType := v.Type()
	switch valueType {
	case fastjson.TypeString:
		return string(v.GetStringBytes())
	case fastjson.TypeNumber:
		return v.GetFloat64()
	case fastjson.TypeNull:
		return nil
	case fastjson.TypeFalse:
		return false
	case fastjson.TypeTrue:
		return true
	case fastjson.TypeArray:
		arr := v.GetArray()
		var value []interface{}
		for _, current := range arr {
			value = append(value, getRawValue(current))
		}

		return value
	case fastjson.TypeObject:
	default:
		log.Println(valueType.String())
	}

	return nil
}

func parseOrderValue(order *fastjson.Value) ([]Order, error) {
	switch order.Type() { //nolint:exhaustive
	case fastjson.TypeString:
		lbOrder, err := parseOrderStr(string(order.GetStringBytes()))
		if err != nil {
			return nil, err
		}

		return []Order{lbOrder}, nil
	case fastjson.TypeArray:
		arr := order.GetArray()
		var result []Order
		for _, value := range arr {
			if value.Type() != fastjson.TypeString {
				return nil, errors.New("invalid order param")
			}
			lbOrder, err := parseOrderStr(string(value.GetStringBytes()))
			if err != nil {
				return nil, err
			}

			result = append(result, lbOrder)
		}
		return result, nil
	case fastjson.TypeObject:
	default:
		return nil, errors.New("invalid order param")
	}

	return nil, errors.New("invalid order param")
}

func parseOrderStr(orderStr string) (Order, error) {
	sort := strings.Split(strings.TrimSpace(orderStr), " ")
	if len(sort) != 2 {
		return Order{}, errors.New("invalid order param")
	}

	field := sort[0]
	direction := strings.ToUpper(sort[1])
	if direction != "ASC" && direction != "DESC" {
		return Order{}, errors.New("invalid order param")
	}

	return Order{
		Field:     field,
		Direction: direction,
	}, nil
}

func parseFieldsValue(v *fastjson.Value) (map[string]bool, error) {
	fields := map[string]bool{}
	switch v.Type() { //nolint:exhaustive
	case fastjson.TypeArray:
		arr := v.GetArray()

		for _, value := range arr {
			if value.Type() != fastjson.TypeString {
				return nil, errors.New("invalid fields param")
			}
			fields[string(value.GetStringBytes())] = true
		}
	case fastjson.TypeObject:
		obj := v.GetObject()
		obj.Visit(func(key []byte, v *fastjson.Value) {
			prop := string(key)
			switch v.Type() { //nolint:exhaustive
			case fastjson.TypeFalse:
				fields[prop] = false
			case fastjson.TypeTrue:
				fields[prop] = true
			}
		})
	default:
		return nil, errors.New("invalid fields param")
	}
	return fields, nil
}

func parseIncludeValue(include *fastjson.Value) ([]Include, error) {
	if include == nil {
		return nil, nil
	}

	var result []Include
	switch include.Type() { //nolint:exhaustive
	case fastjson.TypeString:
		relations := strings.Split(string(include.GetStringBytes()), ",")
		for _, relation := range relations {
			result = append(result, Include{
				Relation: relation,
			})
		}
	case fastjson.TypeObject:
		obj, _ := include.Object()
		relationName := obj.Get("relation")
		scope := obj.Get("scope")
		if relationName == nil || relationName.Type() != fastjson.TypeString {
			return nil, errors.New("invalid relation name")
		}
		var scopeValue *Filter
		var err error
		if scope != nil {
			if scope.Type() != fastjson.TypeObject {
				err = errors.New("invalid relation scope")
			} else {
				scopeValue, err = parseFilterValue(scope)
			}
		}

		if err != nil {
			return nil, err
		}

		result = append(result, Include{
			Relation: string(relationName.GetStringBytes()),
			Scope:    scopeValue,
		})
	case fastjson.TypeArray:
		arr, _ := include.Array()
		for _, value := range arr {
			includes, err := parseIncludeValue(value)
			if err != nil {
				return nil, err
			}
			if includes != nil {
				result = append(result, includes...)
			}
		}
	}

	return result, nil
}

func parseFilterValue(parsedFilter *fastjson.Value) (*Filter, error) {
	if parsedFilter.Type() != fastjson.TypeObject {
		return nil, errors.New("invalid filter")
	}
	whereValue := parsedFilter.Get("where")
	filter := &Filter{}
	if whereValue != nil {
		lbWhere, err := parseWhereValue(whereValue)
		if err != nil {
			return nil, err
		}
		filter.Where = lbWhere
	}

	orderValue := parsedFilter.Get("order")
	if orderValue != nil {
		lbOrder, err := parseOrderValue(orderValue)
		if err != nil {
			return nil, err
		}

		filter.Order = lbOrder
	}

	fieldsValue := parsedFilter.Get("fields")
	if fieldsValue != nil {
		fields, err := parseFieldsValue(fieldsValue)
		if err != nil {
			return nil, err
		}

		filter.Fields = fields
	}

	limitValue := parsedFilter.Get("limit")
	if limitValue != nil {
		limit := limitValue.GetUint()
		if limit > 0 {
			filter.Limit = limit
		}
	}

	skipValue := parsedFilter.Get("skip")
	if skipValue != nil {
		skip := skipValue.GetUint()
		if skip > 0 {
			filter.Skip = skip
		}
	}

	includeValue := parsedFilter.Get("include")
	if includeValue != nil {
		includes, err := parseIncludeValue(includeValue)
		if err != nil {
			return nil, err
		}

		filter.Include = includes
	}
	return filter, nil
}

func ParseWhere(f string) (Where, error) {
	parser := wherePool.Get()
	parsed, err := parser.Parse(f)
	if err != nil {
		return nil, errors.New("cannot parse where query")
	}
	wherePool.Put(parser)
	return parseWhereValue(parsed)
}

func ParseOrder(f string) ([]Order, error) {
	parser := orderPool.Get()
	parsed, err := parser.Parse(f)
	if err != nil {
		return nil, errors.New("cannot parse order query")
	}

	orderPool.Put(parser)
	return parseOrderValue(parsed)
}

func ParseFields(f string) (map[string]bool, error) {
	parser := fieldsPool.Get()
	parsed, err := parser.Parse(f)
	if err != nil {
		return nil, errors.New("cannot parse fields query")
	}

	fieldsPool.Put(parser)
	return parseFieldsValue(parsed)
}

func ParseInclude(f string) ([]Include, error) {
	parser := includePool.Get()
	parsed, err := parser.Parse(f)
	if err != nil {
		return nil, errors.New("cannot parse includes")
	}
	includePool.Put(parser)
	return parseIncludeValue(parsed)
}

func ParseFilter(f string) (filter *Filter, err error) {
	parser := filterPool.Get()
	parsed, err := parser.Parse(f)

	if err != nil {
		return nil, errors.New("cannot parse filter")
	}

	filter, err = parseFilterValue(parsed)
	if err != nil {
		return nil, err
	}

	filterPool.Put(parser)
	return filter, nil
}
