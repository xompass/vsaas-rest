package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xompass/vsaas-rest/lbq"
)

func TestFilterBuilder_MergeWith_BasicMerge(t *testing.T) {
	// Create first filter
	filter1 := NewFilter().
		WithWhere(NewWhere().Eq("status", "active")).
		Limit(10).
		OrderByAsc("name")

	// Create second filter
	filter2 := NewFilter().
		WithWhere(NewWhere().Eq("type", "user")).
		Skip(5).
		OrderByDesc("created_at")

	// Merge
	merged := filter1.MergeWith(filter2)

	// Build and check result
	result, err := merged.Build()
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check WHERE is combined with AND
	expectedWhere := lbq.Where{
		"and": lbq.AndOrCondition{
			lbq.Where{"status": "active"},
			lbq.Where{"type": "user"},
		},
	}
	assert.Equal(t, expectedWhere, result.Where)

	// Check limit and skip
	assert.Equal(t, uint(10), result.Limit)
	assert.Equal(t, uint(5), result.Skip)

	// Check order (should be overwritten)
	assert.Len(t, result.Order, 1)
	assert.Equal(t, "created_at", result.Order[0].Field)
	assert.Equal(t, "DESC", result.Order[0].Direction)
}

func TestFilterBuilder_MergeWith_OrOperator(t *testing.T) {
	filter1 := NewFilter().WithWhere(NewWhere().Eq("status", "active"))
	filter2 := NewFilter().WithWhere(NewWhere().Eq("status", "pending"))

	config := &MergeConfig{WhereOperator: "or"}
	merged := filter1.MergeWith(filter2, config)

	result, err := merged.Build()
	assert.NoError(t, err)

	expectedWhere := lbq.Where{
		"or": lbq.AndOrCondition{
			lbq.Where{"status": "active"},
			lbq.Where{"status": "pending"},
		},
	}
	assert.Equal(t, expectedWhere, result.Where)
}

func TestFilterBuilder_MergeWith_FieldConflicts(t *testing.T) {
	filter1 := NewFilter().Fields(map[string]bool{"name": true, "email": true})
	filter2 := NewFilter().Fields(map[string]bool{"name": false, "phone": true})

	// Should error by default due to field conflict
	merged := filter1.MergeWith(filter2)
	_, err := merged.Build()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "field projection conflict")

	// Should work with AllowFieldConflicts but still fail due to mixing inclusion/exclusion
	config := &MergeConfig{AllowFieldConflicts: true}
	merged = filter1.MergeWith(filter2, config)
	_, err = merged.Build()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), FILTER_CANNOT_MIX_INCLUSION_EXCLUSION)

	// Test successful merge without mixing inclusion/exclusion
	filter3 := NewFilter().Fields(map[string]bool{"name": true, "email": true})
	filter4 := NewFilter().Fields(map[string]bool{"phone": true, "address": true})

	merged = filter3.MergeWith(filter4)
	result, err := merged.Build()
	assert.NoError(t, err)

	expected := lbq.Fields{"name": true, "email": true, "phone": true, "address": true}
	assert.Equal(t, expected, result.Fields)
}

func TestFilterBuilder_MergeWith_MaxLimit(t *testing.T) {
	filter1 := NewFilter().Limit(5)
	filter2 := NewFilter().Limit(100)

	// Without max limit - should use filter2's limit
	merged := filter1.MergeWith(filter2)
	result, err := merged.Build()
	assert.NoError(t, err)
	assert.Equal(t, uint(100), result.Limit)

	// With max limit - should be capped
	maxLimit := uint(50)
	config := &MergeConfig{MaxLimit: &maxLimit}
	merged = filter1.MergeWith(filter2, config)
	result, err = merged.Build()
	assert.NoError(t, err)
	assert.Equal(t, uint(50), result.Limit)
}

func TestFilterBuilder_MergeWith_Include(t *testing.T) {
	filter1 := NewFilter().Include("users", nil)
	filter2 := NewFilter().Include("posts", nil).Include("comments", nil)

	merged := filter1.MergeWith(filter2)
	result, err := merged.Build()
	assert.NoError(t, err)

	assert.Len(t, result.Include, 3)
	relations := make([]string, len(result.Include))
	for i, inc := range result.Include {
		relations[i] = inc.Relation
	}
	assert.Contains(t, relations, "users")
	assert.Contains(t, relations, "posts")
	assert.Contains(t, relations, "comments")
}

func TestFilterBuilder_MergeWith_WithErrors(t *testing.T) {
	// Create filter with error - using empty field should cause validation error
	whereBuilder := NewWhere().Eq("", "value") // Empty field should cause error
	filter1 := NewFilter().WithWhere(whereBuilder)

	filter2 := NewFilter().WithWhere(NewWhere().Eq("status", "active"))

	// Merge should propagate error from filter1
	merged := filter1.MergeWith(filter2)
	assert.NotNil(t, merged.err, "Expected error to be propagated from filter1")

	_, err := merged.Build()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), FILTER_FIELD_EMPTY)

	// Test the other direction too - error in second filter
	filter3 := NewFilter().WithWhere(NewWhere().Eq("status", "active"))
	filter4 := NewFilter().WithWhere(NewWhere().Eq("   ", "value")) // Whitespace-only field

	merged2 := filter3.MergeWith(filter4)
	assert.NotNil(t, merged2.err, "Expected error to be propagated from filter4")
}

func TestFilterBuilder_MergeWith_NilCases(t *testing.T) {
	filter1 := NewFilter().WithWhere(NewWhere().Eq("status", "active"))

	// Merge with nil should return clone of filter1
	merged := filter1.MergeWith(nil)
	result1, err1 := filter1.Build()
	result2, err2 := merged.Build()
	assert.NoError(t, err1)
	assert.NoError(t, err2)

	// Compare the important fields instead of the entire structs
	assert.Equal(t, result1.Where, result2.Where)
	assert.Equal(t, result1.Fields, result2.Fields)
	assert.Equal(t, result1.Limit, result2.Limit)
	assert.Equal(t, result1.Skip, result2.Skip)
	assert.Equal(t, result1.Order, result2.Order)

	// Nil merge with filter should return clone of filter
	var nilFilter *FilterBuilder
	merged = nilFilter.MergeWith(filter1)
	result3, err3 := merged.Build()
	assert.NoError(t, err3)
	assert.Equal(t, result1.Where, result3.Where)
	assert.Equal(t, result1.Fields, result3.Fields)
	assert.Equal(t, result1.Limit, result3.Limit)
	assert.Equal(t, result1.Skip, result3.Skip)
	assert.Equal(t, result1.Order, result3.Order)
}

func TestFilterBuilder_MergeWith_ComplexWhere(t *testing.T) {
	// Create complex WHERE conditions
	filter1 := NewFilter().
		WithWhere(NewWhere().Eq("status", "active")).
		WithWhere(NewWhere().Gt("age", 18))

	filter2 := NewFilter().
		WithWhere(NewWhere().In("role", []string{"admin", "user"}))

	merged := filter1.MergeWith(filter2)
	result, err := merged.Build()
	assert.NoError(t, err)

	// Should combine all WHERE conditions with AND
	expectedWhere := lbq.Where{
		"and": lbq.AndOrCondition{
			lbq.Where{
				"and": lbq.AndOrCondition{
					lbq.Where{"status": "active"},
					lbq.Where{"age": lbq.Where{"gt": 18}},
				},
			},
			lbq.Where{"role": lbq.Where{"inq": []string{"admin", "user"}}},
		},
	}
	assert.Equal(t, expectedWhere, result.Where)
}

func TestFilterBuilder_MergeWith_AllowNilValues(t *testing.T) {
	// Test that nil values are allowed (for cases like {"deleted": null})
	filter1 := NewFilter().WithWhere(NewWhere().Eq("deleted", nil))
	filter2 := NewFilter().WithWhere(NewWhere().Eq("status", "active"))

	merged := filter1.MergeWith(filter2)
	result, err := merged.Build()
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check WHERE contains both conditions
	expectedWhere := lbq.Where{
		"and": lbq.AndOrCondition{
			lbq.Where{"deleted": nil},
			lbq.Where{"status": "active"},
		},
	}
	assert.Equal(t, expectedWhere, result.Where)

	// Test IsNull and IsNotNull work correctly
	filter3 := NewFilter().WithWhere(NewWhere().IsNull("deleted"))
	filter4 := NewFilter().WithWhere(NewWhere().IsNotNull("created_at"))

	merged2 := filter3.MergeWith(filter4)
	result2, err2 := merged2.Build()
	assert.NoError(t, err2)
	assert.NotNil(t, result2)
}
