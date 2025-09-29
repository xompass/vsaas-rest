package database

import (
	"strings"

	"maps"

	"github.com/bytedance/sonic"
	"github.com/go-errors/errors"
	"github.com/xompass/vsaas-rest/lbq"
)

const (
	FILTER_FIELD_EMPTY                    = "FILTER_FIELD_EMPTY"
	FILTER_INVALID_DIRECTION              = "FILTER_INVALID_DIRECTION"
	FILTER_WHERE_EMPTY                    = "FILTER_WHERE_EMPTY"
	FILTER_CANNOT_MIX_INCLUSION_EXCLUSION = "FILTER_CANNOT_MIX_INCLUSION_EXCLUSION"
	FILTER_WHERE_CANNOT_BE_NIL            = "FILTER_WHERE_CANNOT_BE_NIL"
	FILTER_VALUE_CANNOT_BE_NIL            = "FILTER_VALUE_CANNOT_BE_NIL"
)

type FilterBuilder struct {
	where   []lbq.Where
	fields  lbq.Fields
	limit   *uint
	skip    *uint
	order   []lbq.Order
	include []lbq.Include
	err     error
}

func NewFilter() *FilterBuilder {
	return &FilterBuilder{
		where:  []lbq.Where{},
		fields: lbq.Fields{},
		order:  []lbq.Order{},
	}
}

func (b *FilterBuilder) Fields(fields map[string]bool) *FilterBuilder {
	maps.Copy(b.fields, fields)
	return b
}

func (b *FilterBuilder) Limit(limit uint) *FilterBuilder {
	b.limit = &limit
	return b
}

func (b *FilterBuilder) Skip(skip uint) *FilterBuilder {
	b.skip = &skip
	return b
}

func (b *FilterBuilder) orderBy(field string, direction string) *FilterBuilder {
	if strings.TrimSpace(field) == "" {
		b.err = errors.New(FILTER_FIELD_EMPTY)
		return b
	}
	direction = strings.ToUpper(direction)
	if direction != "ASC" && direction != "DESC" {
		b.err = errors.New(FILTER_INVALID_DIRECTION)
		return b
	}
	b.order = append(b.order, lbq.Order{Field: field, Direction: direction})
	return b
}

func (b *FilterBuilder) OrderByAsc(field string) *FilterBuilder {
	return b.orderBy(field, "ASC")
}

func (b *FilterBuilder) OrderByDesc(field string) *FilterBuilder {
	return b.orderBy(field, "DESC")
}

func (b *FilterBuilder) Include(relation string, scope *lbq.Filter) *FilterBuilder {
	b.include = append(b.include, lbq.Include{Relation: relation, Scope: scope})
	return b
}

func (f *FilterBuilder) WithWhere(builder *WhereBuilder) *FilterBuilder {
	where, err := builder.Build()
	if err != nil {
		f.err = err
		return f
	}

	if len(where) == 0 {
		f.err = errors.New(FILTER_WHERE_EMPTY)
		return f
	}

	f.where = append(f.where, where)
	return f
}

func (b *FilterBuilder) Build() (*lbq.Filter, error) {
	if b.err != nil {
		return nil, b.err
	}

	var where lbq.Where

	if len(b.where) == 1 {
		where = b.where[0]
	} else if len(b.where) > 1 {
		where = lbq.Where{
			"and": lbq.AndOrCondition(b.where),
		}
	}

	if !isValidProjection(b.fields) {
		return nil, errors.New(FILTER_CANNOT_MIX_INCLUSION_EXCLUSION)
	}

	return &lbq.Filter{
		Where:   where,
		Fields:  b.fields,
		Order:   b.order,
		Limit:   derefUint(b.limit),
		Skip:    derefUint(b.skip),
		Include: b.include,
	}, nil
}

func (b *FilterBuilder) FromLBFilter(filter *lbq.Filter) *FilterBuilder {
	if filter == nil {
		return b
	}

	b.where = []lbq.Where{filter.Where}
	b.fields = filter.Fields
	b.limit = &filter.Limit
	b.skip = &filter.Skip
	b.order = filter.Order
	b.include = filter.Include

	if !isValidProjection(b.fields) {
		b.err = errors.New(FILTER_CANNOT_MIX_INCLUSION_EXCLUSION)
	}

	return b
}

func (b *FilterBuilder) Reset() *FilterBuilder {
	b.where = []lbq.Where{}
	b.fields = lbq.Fields{}
	b.limit = nil
	b.skip = nil
	b.order = []lbq.Order{}
	b.include = []lbq.Include{}
	b.err = nil
	return b
}

func (b *FilterBuilder) Clone() *FilterBuilder {
	clone := &FilterBuilder{
		where:   make([]lbq.Where, len(b.where)),
		fields:  make(lbq.Fields),
		order:   make([]lbq.Order, len(b.order)),
		include: make([]lbq.Include, len(b.include)),
		err:     b.err,
	}

	copy(clone.where, b.where)
	copy(clone.order, b.order)
	copy(clone.include, b.include)
	maps.Copy(clone.fields, b.fields)

	if b.limit != nil {
		limit := *b.limit
		clone.limit = &limit
	}
	if b.skip != nil {
		skip := *b.skip
		clone.skip = &skip
	}

	return clone
}

func (b *FilterBuilder) Page(page, size uint) *FilterBuilder {
	if page > 0 && size > 0 {
		b.Skip((page - 1) * size)
		b.Limit(size)
	}
	return b
}

func (f *FilterBuilder) ToJSON() (string, error) {
	filter, err := f.Build()
	if err != nil {
		return "", errors.Errorf(`{"error": "%s"}`, err.Error())
	}
	data, _ := sonic.MarshalIndent(filter, "", "  ")
	return string(data), nil
}

/************************
 * Where Builder
 ************************/

type WhereBuilder struct {
	conditions []lbq.Where
	err        error
}

func NewWhere() *WhereBuilder {
	return &WhereBuilder{}
}

func (b *WhereBuilder) Eq(field string, value any, strict ...bool) *WhereBuilder {
	if len(strict) > 0 && strict[0] {
		return b.Raw(lbq.Where{field: lbq.Where{"eq": value}})
	}

	return b.Raw(lbq.Where{field: value})
}

func (b *WhereBuilder) Neq(field string, value any) *WhereBuilder {
	return b.Raw(lbq.Where{field: lbq.Where{"neq": value}})
}

func (b *WhereBuilder) In(field string, values any) *WhereBuilder {
	if err := validateFieldAndValue(field, values); err != nil {
		b.err = err
		return b
	}
	return b.Raw(lbq.Where{field: lbq.Where{"inq": values}})
}

func (b *WhereBuilder) Nin(field string, values any) *WhereBuilder {
	if err := validateFieldAndValue(field, values); err != nil {
		b.err = err
		return b
	}

	return b.Raw(lbq.Where{field: lbq.Where{"nin": values}})
}

func (b *WhereBuilder) Between(field string, min any, max any, exclusive bool) *WhereBuilder {
	lowOp := "gte"
	highOp := "lte"
	if exclusive {
		lowOp = "gt"
		highOp = "lt"
	}

	return b.Raw(lbq.Where{
		"and": lbq.AndOrCondition{
			{field: lbq.Where{lowOp: min}},
			{field: lbq.Where{highOp: max}},
		},
	})
}

func (b *WhereBuilder) Gt(field string, value any) *WhereBuilder {
	if err := validateFieldAndValue(field, value); err != nil {
		b.err = err
		return b
	}
	return b.Raw(lbq.Where{field: lbq.Where{"gt": value}})
}

func (b *WhereBuilder) Lt(field string, value any) *WhereBuilder {
	if err := validateFieldAndValue(field, value); err != nil {
		b.err = err
		return b
	}
	return b.Raw(lbq.Where{field: lbq.Where{"lt": value}})
}

func (b *WhereBuilder) Lte(field string, value any) *WhereBuilder {
	if err := validateFieldAndValue(field, value); err != nil {
		b.err = err
		return b
	}
	return b.Raw(lbq.Where{field: lbq.Where{"lte": value}})
}

func (b *WhereBuilder) Gte(field string, value any) *WhereBuilder {
	if err := validateFieldAndValue(field, value); err != nil {
		b.err = err
		return b
	}
	return b.Raw(lbq.Where{field: lbq.Where{"gte": value}})
}

func (b *WhereBuilder) Like(field string, pattern string, options ...string) *WhereBuilder {
	where := lbq.Where{"like": pattern}
	if len(options) > 0 {
		where["options"] = options[0]
	}

	return b.Raw(lbq.Where{field: where})
}

func (b *WhereBuilder) IsNull(field string) *WhereBuilder {
	return b.Raw(lbq.Where{field: lbq.Where{"eq": nil}})
}

func (b *WhereBuilder) IsNotNull(field string) *WhereBuilder {
	return b.Raw(lbq.Where{field: lbq.Where{"neq": nil}})
}

func (b *WhereBuilder) Raw(w lbq.Where) *WhereBuilder {
	if b.err != nil {
		return b
	}
	if len(w) == 0 {
		b.err = errors.New("raw where condition cannot be empty")
		return b
	}
	b.conditions = append(b.conditions, w)
	return b
}

func (b *WhereBuilder) Or(builders ...*WhereBuilder) *WhereBuilder {
	var ors []lbq.Where
	for _, sub := range builders {
		w, e := sub.Build()
		if e != nil {
			b.err = e
		}
		if len(w) > 0 {
			ors = append(ors, w)
		}
	}
	if len(ors) > 0 {
		b.conditions = append(b.conditions, lbq.Where{"or": lbq.AndOrCondition(ors)})
	}
	return b
}

func (b *WhereBuilder) And(builders ...*WhereBuilder) *WhereBuilder {
	var flat []lbq.Where

	for _, sub := range builders {
		w, e := sub.Build()
		if e != nil {
			b.err = e
			return b
		}

		// Detectar si ya es un "and" y aplanar
		if inner, ok := w["and"]; ok {
			if conds, ok := inner.(lbq.AndOrCondition); ok {
				flat = append(flat, conds...)
				continue
			}
		}

		if len(w) > 0 {
			flat = append(flat, w)
		}
	}

	if len(flat) > 0 {
		b.conditions = append(b.conditions, lbq.Where{"and": lbq.AndOrCondition(flat)})
	}
	return b
}

func (b *WhereBuilder) Build() (lbq.Where, error) {
	if b == nil {
		return nil, errors.New(FILTER_WHERE_CANNOT_BE_NIL)
	}

	if b.err != nil {
		return nil, b.err
	}
	switch len(b.conditions) {
	case 0:
		return lbq.Where{}, nil
	case 1:
		return b.conditions[0], nil
	default:
		return lbq.Where{"and": lbq.AndOrCondition(b.conditions)}, nil
	}
}

func derefUint(p *uint) uint {
	if p == nil {
		return 0
	}
	return *p
}

func isValidProjection(fields map[string]bool) bool {
	hasTrue := false
	hasFalse := false
	for key, val := range fields {
		if key == "_id" {
			continue
		}
		if val {
			hasTrue = true
		} else {
			hasFalse = true
		}
	}
	return !(hasTrue && hasFalse)
}

func validateFieldAndValue(field string, value any) error {
	if strings.TrimSpace(field) == "" {
		return errors.New(FILTER_FIELD_EMPTY)
	}
	if value == nil {
		return errors.New(FILTER_VALUE_CANNOT_BE_NIL)
	}
	return nil
}
