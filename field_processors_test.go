package rest

import (
	"log"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldProcessors_StringProcessing(t *testing.T) {
	t.Run("processStringValue with regular string", func(t *testing.T) {
		input := "hello world"
		value := reflect.ValueOf(&input).Elem()

		processStringValue(value, func(s string) string {
			return "processed_" + s
		})

		assert.Equal(t, "processed_hello world", input)
	})

	t.Run("processStringValue with pointer to string", func(t *testing.T) {
		str := "hello world"
		input := &str
		value := reflect.ValueOf(&input).Elem()

		processStringValue(value, func(s string) string {
			return "processed_" + s
		})

		assert.Equal(t, "processed_hello world", *input)
	})

	t.Run("processStringValue with nil pointer", func(t *testing.T) {
		var input *string = nil
		value := reflect.ValueOf(&input).Elem()

		// Should not panic
		processStringValue(value, func(s string) string {
			return "processed_" + s
		})

		assert.Nil(t, input)
	})
}

func TestTrimNormalizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"leading and trailing spaces", "  hello world  ", "hello world"},
		{"tabs and newlines", "\t\nhello world\n\t", "hello world"},
		{"only spaces", "   ", ""},
		{"no whitespace", "hello", "hello"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			trimNormalizer(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestLowercaseNormalizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"uppercase", "HELLO WORLD", "hello world"},
		{"mixed case", "HeLLo WoRLD", "hello world"},
		{"already lowercase", "hello world", "hello world"},
		{"with numbers", "Hello123World", "hello123world"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			lowercaseNormalizer(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestUppercaseNormalizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "hello world", "HELLO WORLD"},
		{"mixed case", "HeLLo WoRLD", "HELLO WORLD"},
		{"already uppercase", "HELLO WORLD", "HELLO WORLD"},
		{"with numbers", "hello123world", "HELLO123WORLD"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			uppercaseNormalizer(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestUnaccentNormalizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"french accents", "café", "cafe"},
		{"spanish accents", "niño", "nino"},
		{"german umlauts", "Müller", "Muller"},
		{"multiple accents", "résumé", "resume"},
		{"no accents", "hello", "hello"},
		{"mixed", "café niño", "cafe nino"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			unaccentNormalizer(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestUnicodeNormalizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"composed form", "é", "é"},                // Should remain NFC
		{"decomposed to composed", "e\u0301", "é"}, // e + combining acute -> é
		{"already normalized", "hello", "hello"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			unicodeNormalizer(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestHtmlSanitizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"script tag",
			"<script>alert('xss')</script>hello",
			"hello",
		},
		{
			"safe html preserved",
			"<b>bold</b> and <i>italic</i>",
			"<b>bold</b> and <i>italic</i>",
		},
		{
			"mixed dangerous and safe",
			"<b>Bold</b> <script>alert('bad')</script> <em>emphasis</em>",
			"<b>Bold</b>  <em>emphasis</em>",
		},
		{
			"no html",
			"plain text",
			"plain text",
		},
		{
			"empty string",
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			htmlSanitizer(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestAlphanumericSanitizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"special characters", "hello!@#world", "helloworld"},
		{"numbers and letters", "test123", "test123"},
		{"only special characters", "!@#$%", ""},
		{"mixed unicode", "café123!", "café123"},
		{"spaces", "hello world", "helloworld"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			alphanumericSanitizer(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestNumericSanitizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"letters and numbers", "abc123def456", "123456"},
		{"only numbers", "123456", "123456"},
		{"only letters", "abcdef", ""},
		{"special characters", "12!@#34", "1234"},
		{"mixed", "user123name456", "123456"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			value := reflect.ValueOf(&input).Elem()

			numericSanitizer(value)

			assert.Equal(t, tt.expected, input)
		})
	}
}

func TestParseTag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single tag", "trim", []string{"trim"}},
		{"multiple tags", "trim,lowercase,uppercase", []string{"trim", "lowercase", "uppercase"}},
		{"with spaces", "trim, lowercase, uppercase", []string{"trim", "lowercase", "uppercase"}},
		{"empty parts", "trim,,lowercase", []string{"trim", "lowercase"}},
		{"only commas", ",,", nil},
		{"empty string", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTag(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRemoveDiacritics(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple accent", "café", "cafe"},
		{"multiple accents", "résumé", "resume"},
		{"spanish", "niño", "nino"},
		{"german", "Müller", "Muller"},
		{"complex", "Åse Oyvind", "Ase Oyvind"},
		{"no diacritics", "hello world", "hello world"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeDiacritics(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeCheckers(t *testing.T) {
	t.Run("isStruct", func(t *testing.T) {
		type TestStruct struct{ Name string }

		assert.True(t, isStruct(reflect.TypeOf(TestStruct{})))
		assert.True(t, isStruct(reflect.TypeOf(&TestStruct{}))) // pointer to struct
		assert.False(t, isStruct(reflect.TypeOf("string")))
		assert.False(t, isStruct(reflect.TypeOf(123)))
	})

	t.Run("isSliceOrArray", func(t *testing.T) {
		assert.True(t, isSliceOrArray(reflect.TypeOf([]string{})))
		assert.True(t, isSliceOrArray(reflect.TypeOf([5]int{})))
		assert.False(t, isSliceOrArray(reflect.TypeOf("string")))
		assert.False(t, isSliceOrArray(reflect.TypeOf(map[string]int{})))
	})

	t.Run("isMap", func(t *testing.T) {
		assert.True(t, isMap(reflect.TypeOf(map[string]int{})))
		assert.False(t, isMap(reflect.TypeOf([]string{})))
		assert.False(t, isMap(reflect.TypeOf("string")))
	})

	t.Run("isDiveable", func(t *testing.T) {
		type TestStruct struct{ Name string }

		assert.True(t, isDiveable(reflect.TypeOf(TestStruct{})))
		assert.True(t, isDiveable(reflect.TypeOf(&TestStruct{})))
		assert.True(t, isDiveable(reflect.TypeOf([]string{})))
		assert.True(t, isDiveable(reflect.TypeOf(map[string]int{})))
		assert.False(t, isDiveable(reflect.TypeOf("string")))
		assert.False(t, isDiveable(reflect.TypeOf(123)))
	})
}

func TestBuildStructFields(t *testing.T) {
	type TestStruct struct {
		Name       string `json:"name" normalize:"trim,lowercase" validate:"required"`
		Email      string `json:"email" sanitize:"html" validate:"email"`
		Unexported string // Should be ignored
		NoTags     string `json:"no_tags"`
	}

	t.Run("build struct fields metadata", func(t *testing.T) {
		metadata, err := buildStructFields(reflect.TypeOf(TestStruct{}))

		assert.NoError(t, err)
		assert.True(t, metadata.hasValidate)
		assert.Len(t, metadata.fields, 2) // Only Name and Email should have processors

		// Find Name field
		var nameField *cachedStructField
		for _, field := range metadata.fields {
			if field.index[0] == 0 { // First field is Name
				nameField = &field
				break
			}
		}

		require.NotNil(t, nameField)
		assert.NotNil(t, nameField.normalize)
		assert.Len(t, nameField.normalize.funcs, 2) // trim and lowercase
		assert.False(t, nameField.normalize.dive)
	})

	type StructWithDive struct {
		Users []TestStruct `json:"users" normalize:"dive" sanitize:"dive"`
	}

	t.Run("build struct with dive", func(t *testing.T) {
		metadata, err := buildStructFields(reflect.TypeOf(StructWithDive{}))

		assert.NoError(t, err)
		assert.Len(t, metadata.fields, 1)

		field := metadata.fields[0]
		log.Println("Field metadata:", *field.normalize, *field.sanitize)
		assert.True(t, field.normalize.dive)
		assert.True(t, field.sanitize.dive)
	})

	type StructWithInvalidDive struct {
		Name string `json:"name" normalize:"dive,trim"` // Invalid: dive with other tags
	}

	t.Run("invalid dive combination", func(t *testing.T) {
		_, err := buildStructFields(reflect.TypeOf(StructWithInvalidDive{}))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dive' but is not diveable")
	})
}

func TestApplyProcessors(t *testing.T) {
	t.Run("apply to string value", func(t *testing.T) {
		input := "  hello  "
		value := reflect.ValueOf(&input).Elem()

		funcs := []fieldProcessorFunc{trimNormalizer, lowercaseNormalizer}
		err := applyProcessors(value, funcs)

		assert.NoError(t, err)
		assert.Equal(t, "hello", input)
	})

	t.Run("apply to pointer", func(t *testing.T) {
		str := "  HELLO  "
		input := &str
		value := reflect.ValueOf(&input).Elem()

		funcs := []fieldProcessorFunc{trimNormalizer, lowercaseNormalizer}
		err := applyProcessors(value, funcs)

		assert.NoError(t, err)
		assert.Equal(t, "hello", *input)
	})

	t.Run("invalid value", func(t *testing.T) {
		var value reflect.Value // Invalid/zero value

		err := applyProcessors(value, []fieldProcessorFunc{trimNormalizer})
		assert.NoError(t, err) // Should handle invalid values gracefully
	})
}
