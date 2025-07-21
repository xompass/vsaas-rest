package rest

import (
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs for different scenarios
type SimpleTestStruct struct {
	Name  string `json:"name" normalize:"trim,lowercase" validate:"required,min=2"`
	Email string `json:"email" sanitize:"html" normalize:"trim" validate:"email"`
}

type NestedTestStruct struct {
	User    SimpleTestStruct             `json:"user" normalize:"dive" sanitize:"dive" validate:"dive"`
	Users   []SimpleTestStruct           `json:"users" normalize:"dive" sanitize:"dive" validate:"dive"`
	UserMap map[string]*SimpleTestStruct `json:"userMap" normalize:"dive" sanitize:"dive" validate:"dive"`
	Company string                       `json:"company" normalize:"trim,uppercase"`
}

type SliceTestStruct struct {
	Users []SimpleTestStruct `json:"users" normalize:"dive" sanitize:"dive" validate:"dive"`
	Tags  []string           `json:"tags" normalize:"dive,trim"`
}

type MapTestStruct struct {
	UserMap map[string]SimpleTestStruct `json:"userMap" normalize:"dive" sanitize:"dive" validate:"dive"`
	MetaMap map[string]string           `json:"metaMap" sanitize:"dive,alphanumeric"`
}

type InterfaceTestStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (its *InterfaceTestStruct) Normalize(ctx *EndpointContext) error {
	its.Name = "normalized_" + its.Name
	return nil
}

func (its *InterfaceTestStruct) Sanitize(ctx *EndpointContext) error {
	its.Name = "sanitized_" + its.Name
	return nil
}

func (its *InterfaceTestStruct) Validate(ctx *EndpointContext) error {
	if its.Age < 0 {
		return fmt.Errorf("age cannot be negative")
	}
	return nil
}

// Helper function to create a test EndpointContext
func createTestEndpointContext() *EndpointContext {
	app := &RestApp{
		ValidatorInstance: validator.New(),
	}

	return &EndpointContext{
		App: app,
	}
}

func TestProcessStruct_SimpleNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    SimpleTestStruct
		expected SimpleTestStruct
	}{
		{
			name: "trim and lowercase",
			input: SimpleTestStruct{
				Name:  "  JOHN DOE  ",
				Email: "  test@EXAMPLE.com  ",
			},
			expected: SimpleTestStruct{
				Name:  "john doe",
				Email: "test@EXAMPLE.com", // Only trim, no lowercase for email
			},
		},
		{
			name: "empty values",
			input: SimpleTestStruct{
				Name:  "",
				Email: "",
			},
			expected: SimpleTestStruct{
				Name:  "",
				Email: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			err := processStruct(&input, "normalize")

			assert.NoError(t, err)
			assert.Equal(t, tt.expected.Name, input.Name)
			assert.Equal(t, tt.expected.Email, input.Email)
		})
	}
}

func TestProcessStruct_SimpleSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    SimpleTestStruct
		expected SimpleTestStruct
	}{
		{
			name: "html sanitization",
			input: SimpleTestStruct{
				Name:  "John",
				Email: "<script>alert('xss')</script>test@example.com",
			},
			expected: SimpleTestStruct{
				Name:  "John",
				Email: "test@example.com", // HTML tags should be stripped
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			err := processStruct(&input, "sanitize")

			assert.NoError(t, err)
			assert.Equal(t, tt.expected.Email, input.Email)
		})
	}
}

func TestProcessStruct_NestedStructures(t *testing.T) {
	t.Run("nested struct with dive", func(t *testing.T) {
		input := NestedTestStruct{
			User: SimpleTestStruct{
				Name:  "  john doe  ",
				Email: "  test@example.com  ",
			},
			UserMap: map[string]*SimpleTestStruct{
				"jane": {Name: "  jane doe  ", Email: "  jane@example.com  "},
			},
			Users: []SimpleTestStruct{
				{Name: "  john smith  ", Email: "  john.smith@example.com  "},
			},
			Company: "  tech corp  ",
		}

		err := processStruct(&input, "normalize")

		assert.NoError(t, err)
		assert.Equal(t, "john doe", input.User.Name)
		assert.Equal(t, "test@example.com", input.User.Email)
		assert.Equal(t, "TECH CORP", input.Company)
		assert.Equal(t, "jane doe", input.UserMap["jane"].Name)
		assert.Equal(t, "jane@example.com", input.UserMap["jane"].Email)
	})
}

func TestProcessStruct_Slices(t *testing.T) {
	t.Run("slice with dive", func(t *testing.T) {
		input := SliceTestStruct{
			Users: []SimpleTestStruct{
				{Name: "  JOHN  ", Email: "  john@example.com  "},
				{Name: "  JANE  ", Email: "  jane@example.com  "},
			},
			Tags: []string{"  golang  ", "  REST  "},
		}

		err := processStruct(&input, "normalize")

		assert.NoError(t, err)
		/* assert.Equal(t, "john", input.Users[0].Name)
		assert.Equal(t, "jane", input.Users[1].Name) */
		assert.Equal(t, "golang", input.Tags[0])
		assert.Equal(t, "REST", input.Tags[1])
	})
}

func TestProcessStruct_Maps(t *testing.T) {
	t.Run("map with dive", func(t *testing.T) {
		input := MapTestStruct{
			UserMap: map[string]SimpleTestStruct{
				"user1": {Name: "  JOHN  ", Email: "  john@example.com  "},
			},
			MetaMap: map[string]string{
				"key1": "value123!@#",
			},
		}

		err := processStruct(&input, "normalize")
		assert.NoError(t, err)
		assert.Equal(t, "john", input.UserMap["user1"].Name)

		err = processStruct(&input, "sanitize")
		assert.NoError(t, err)
		assert.Equal(t, "value123", input.MetaMap["key1"]) // alphanumeric sanitization
	})
}

func TestProcessStruct_ErrorCases(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		err := processStruct(nil, "normalize")
		assert.NoError(t, err) // Should handle nil gracefully
	})

	t.Run("invalid operator", func(t *testing.T) {
		input := SimpleTestStruct{}
		err := processStruct(&input, "invalid_operator")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid operator")
	})

	t.Run("non-pointer input", func(t *testing.T) {
		input := SimpleTestStruct{}
		err := processStruct(input, "normalize")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected a non-nil pointer")
	})

	t.Run("non-struct input", func(t *testing.T) {
		input := "not a struct"
		err := processStruct(&input, "normalize")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected a struct")
	})
}

func TestEndpointContext_NormalizeStruct(t *testing.T) {
	ctx := createTestEndpointContext()

	t.Run("interface implementation", func(t *testing.T) {
		input := &InterfaceTestStruct{Name: "test", Age: 25}

		err := normalizeStruct(ctx, input)

		assert.NoError(t, err)
		assert.Equal(t, "normalized_test", input.Name)
	})

	t.Run("tags processing", func(t *testing.T) {
		input := &SimpleTestStruct{
			Name:  "  TEST  ",
			Email: "  test@example.com  ",
		}

		err := normalizeStruct(ctx, input)

		assert.NoError(t, err)
		assert.Equal(t, "test", input.Name)
		assert.Equal(t, "test@example.com", input.Email)
	})

	t.Run("nil input", func(t *testing.T) {
		err := ctx.NormalizeStruct(nil)
		assert.NoError(t, err)
	})
}

func TestEndpointContext_SanitizeStruct(t *testing.T) {
	ctx := createTestEndpointContext()

	t.Run("interface implementation", func(t *testing.T) {
		input := &InterfaceTestStruct{Name: "test", Age: 25}

		err := sanitizeStruct(ctx, input)

		log.Println("Sanitized InterfaceTestStruct:", input.Name)

		assert.NoError(t, err)
		assert.Equal(t, "sanitized_test", input.Name)
	})

	t.Run("tags processing", func(t *testing.T) {
		input := &SimpleTestStruct{
			Email: "<script>alert('xss')</script>test@example.com",
		}

		err := sanitizeStruct(ctx, input)

		assert.NoError(t, err)
		assert.Equal(t, "test@example.com", input.Email)
	})
}

func TestEndpointContext_ValidateStruct(t *testing.T) {
	ctx := createTestEndpointContext()

	t.Run("valid struct", func(t *testing.T) {
		input := &SimpleTestStruct{
			Name:  "John",
			Email: "john@example.com",
		}

		err := ctx.ValidateStruct(input)
		assert.NoError(t, err)
	})

	t.Run("invalid struct", func(t *testing.T) {
		input := &SimpleTestStruct{
			Name:  "J", // Too short
			Email: "invalid-email",
		}

		err := ctx.ValidateStruct(input)
		assert.Error(t, err)
	})
}

func TestValidateAny(t *testing.T) {
	ctx := createTestEndpointContext()

	t.Run("interface validation", func(t *testing.T) {
		input := &InterfaceTestStruct{Name: "test", Age: -1}

		err := validateAny(ctx, input)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "age cannot be negative")
	})

	t.Run("struct with validation tags", func(t *testing.T) {
		input := &SimpleTestStruct{
			Name:  "J", // Too short
			Email: "invalid",
		}

		err := validateAny(ctx, input)
		assert.Error(t, err)
	})

	t.Run("slice validation", func(t *testing.T) {
		input := []*SimpleTestStruct{
			{Name: "John", Email: "john@example.com"},
			{Name: "J", Email: "invalid"}, // Invalid
		}

		err := validateAny(ctx, input)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation error at index 1")
	})

	t.Run("map validation", func(t *testing.T) {
		input := map[string]*SimpleTestStruct{
			"user1": {Name: "John", Email: "john@example.com"},
			"user2": {Name: "J", Email: "invalid"}, // Invalid
		}

		err := validateAny(ctx, input)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation error at key")
	})

	t.Run("nil input", func(t *testing.T) {
		err := validateAny(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot validate nil value")
	})
}

func TestIsValidable(t *testing.T) {
	t.Run("struct with validate tags", func(t *testing.T) {
		input := &SimpleTestStruct{}
		result := isValidable(input)
		log.Println("IsValidable result:", result)
		assert.True(t, result)
	})

	t.Run("struct without validate tags", func(t *testing.T) {
		type NoValidationStruct struct {
			Name string `json:"name"`
		}
		input := &NoValidationStruct{}
		result := isValidable(input)
		assert.False(t, result)
	})

	t.Run("non-struct", func(t *testing.T) {
		input := "not a struct"
		result := isValidable(input)
		assert.False(t, result)
	})

	t.Run("nil input", func(t *testing.T) {
		result := isValidable(nil)
		assert.False(t, result)
	})
}

func TestRegisterStruct(t *testing.T) {
	t.Run("valid struct", func(t *testing.T) {
		err := registerStruct(SimpleTestStruct{})
		assert.NoError(t, err)
	})

	t.Run("nil input", func(t *testing.T) {
		err := registerStruct(nil)
		assert.NoError(t, err)
	})

	t.Run("non-struct", func(t *testing.T) {
		err := registerStruct("not a struct")
		assert.Error(t, err)
	})
}

func TestCustomProcessors(t *testing.T) {
	// Test custom normalizer registration
	t.Run("register custom normalizer", func(t *testing.T) {
		customNorm := func(v reflect.Value) {
			processStringValue(v, func(s string) string {
				return "custom_" + s
			})
		}

		err := RegisterBodyNormalizer("custom", customNorm)
		assert.NoError(t, err)

		// Test duplicate registration
		err = RegisterBodyNormalizer("custom", customNorm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "normalizer already exists")

		// Test nil function
		err = RegisterBodyNormalizer("nil_test", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "normalizer function cannot be nil")
	})

	// Test custom sanitizer registration
	t.Run("register custom sanitizer", func(t *testing.T) {
		customSan := func(v reflect.Value) {
			processStringValue(v, func(s string) string {
				return "clean_" + s
			})
		}

		err := RegisterBodySanitizer("custom", customSan)
		assert.NoError(t, err)

		// Test duplicate registration
		err = RegisterBodySanitizer("custom", customSan)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sanitizer already exists")
	})
}

func TestGetProcessors(t *testing.T) {
	t.Run("get normalizers", func(t *testing.T) {
		normalizers := GetBodyNormalizers()
		assert.NotNil(t, normalizers)
		assert.Contains(t, normalizers, "trim")
		assert.Contains(t, normalizers, "lowercase")
		assert.Contains(t, normalizers, "uppercase")
	})

	t.Run("get sanitizers", func(t *testing.T) {
		sanitizers := GetBodySanitizers()
		assert.NotNil(t, sanitizers)
		assert.Contains(t, sanitizers, "html")
		assert.Contains(t, sanitizers, "alphanumeric")
		assert.Contains(t, sanitizers, "numeric")
	})
}

func TestSpecificNormalizers(t *testing.T) {
	tests := []struct {
		name      string
		processor func(reflect.Value)
		input     string
		expected  string
	}{
		{"trim", trimNormalizer, "  hello world  ", "hello world"},
		{"lowercase", lowercaseNormalizer, "HELLO WORLD", "hello world"},
		{"uppercase", uppercaseNormalizer, "hello world", "HELLO WORLD"},
		{"unicode", unicodeNormalizer, "café", "café"}, // NFC normalization
		{"unaccent", unaccentNormalizer, "café", "cafe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			tt.processor(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestSpecificSanitizers(t *testing.T) {
	tests := []struct {
		name      string
		processor func(reflect.Value)
		input     string
		expected  string
	}{
		{"html", htmlSanitizer, "<script>alert('xss')</script>hello", "hello"},
		{"alphanumeric", alphanumericSanitizer, "hello123!@#world", "hello123world"},
		{"numeric", numericSanitizer, "abc123def456", "123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			tt.processor(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

// CombinedStruct for testing interface + tags combination
type CombinedStruct struct {
	Name string `json:"name" normalize:"uppercase"`
}

// Implement interfaces
func (cs *CombinedStruct) Normalize(ctx *EndpointContext) error {
	cs.Name = "interface_" + cs.Name
	return ctx.NormalizeStruct(cs)
}

func (cs *CombinedStruct) Sanitize(ctx *EndpointContext) error {
	cs.Name = "clean_" + cs.Name
	return ctx.SanitizeStruct(cs)
}

func TestCombinedInterfaceAndTags(t *testing.T) {
	ctx := createTestEndpointContext()

	t.Run("both interface and tags execute", func(t *testing.T) {
		input := &CombinedStruct{Name: "test"}

		// First sanitize (interface only in this case)
		err := sanitizeStruct(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, "clean_test", input.Name)

		// Reset and test normalize (interface + tags)
		input.Name = "test"
		err = normalizeStruct(ctx, input)
		require.NoError(t, err)
		// Should have both interface processing and tag processing
		assert.Equal(t, "INTERFACE_TEST", input.Name)
	})
}
