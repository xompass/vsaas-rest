package rest

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
)

// bindFormToStruct intelligently binds form data to struct, handling both regular JSON and multipart forms
func bindFormToStruct(ec *EndpointContext, form any) error {
	contentType := ec.EchoCtx.Request().Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") && ec.FormValues != nil {
		return bindMultipartFormValues(ec, form)
	}

	return ec.EchoCtx.Bind(form)
}

// bindMultipartFormValues implements a robust form binding that handles complex data types
// including JSON strings, arrays, and nested structures
func bindMultipartFormValues(ec *EndpointContext, target any) error {
	if target == nil {
		return nil
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to a struct")
	}

	rv = rv.Elem() // dereference pointer
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get the field name from JSON tag, fallback to struct field name
		fieldName := getFieldName(fieldType)
		if fieldName == "-" {
			continue // Skip fields marked with json:"-"
		}

		// Get form values for this field
		values, exists := ec.FormValues[fieldName]
		if !exists || len(values) == 0 {
			continue
		}

		// Set the field value based on its type
		if err := setAdvancedFieldValue(field, fieldType, values); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldName, err)
		}
	}

	return nil
}

// getFieldName extracts the field name from JSON tag or returns the struct field name
func getFieldName(fieldType reflect.StructField) string {
	jsonTag := fieldType.Tag.Get("json")
	if jsonTag == "" {
		return fieldType.Name
	}

	// Extract field name from json tag (e.g., "name,omitempty" -> "name")
	if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
		return jsonTag[:commaIdx]
	}
	return jsonTag
}

// setAdvancedFieldValue sets a field value with support for complex types including JSON strings
func setAdvancedFieldValue(field reflect.Value, fieldType reflect.StructField, values []string) error {
	if len(values) == 0 {
		return nil
	}

	// For most types, use the first value
	value := values[0]

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value == "" {
			return nil
		}
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %w", err)
		}
		field.SetInt(intVal)
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if value == "" {
			return nil
		}
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value: %w", err)
		}
		field.SetUint(uintVal)
		return nil

	case reflect.Float32, reflect.Float64:
		if value == "" {
			return nil
		}
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value: %w", err)
		}
		field.SetFloat(floatVal)
		return nil

	case reflect.Bool:
		if value == "" {
			return nil
		}
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %w", err)
		}
		field.SetBool(boolVal)
		return nil

	case reflect.Slice:
		return setSliceValue(field, values)

	case reflect.Map, reflect.Struct:
		// Try to parse as JSON for complex types
		return setJSONValue(field, value)

	case reflect.Interface:
		// For interface{} types, try to parse as JSON first, fallback to string
		return setInterfaceValue(field, value)

	case reflect.Ptr:
		// Handle pointer types
		if value == "" {
			return nil
		}
		elemType := field.Type().Elem()
		newVal := reflect.New(elemType)
		if err := setAdvancedFieldValue(newVal.Elem(), fieldType, values); err != nil {
			return err
		}
		field.Set(newVal)
		return nil

	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
}

// setSliceValue handles slice types including string arrays
func setSliceValue(field reflect.Value, values []string) error {
	elemType := field.Type().Elem()
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))

	for i, value := range values {
		elem := slice.Index(i)
		if err := setSingleValue(elem, elemType, value); err != nil {
			return fmt.Errorf("error setting slice element %d: %w", i, err)
		}
	}

	field.Set(slice)
	return nil
}

// setSingleValue sets a single value for slice elements
func setSingleValue(elem reflect.Value, elemType reflect.Type, value string) error {
	switch elemType.Kind() {
	case reflect.String:
		elem.SetString(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		elem.SetInt(intVal)
		return nil
	// Add more types as needed
	default:
		return fmt.Errorf("unsupported slice element type: %s", elemType.Kind())
	}
}

// setJSONValue attempts to parse a string as JSON and set it to the field
func setJSONValue(field reflect.Value, value string) error {
	if value == "" {
		return nil
	}

	// Create a new instance of the field type
	newVal := reflect.New(field.Type())

	// Try to unmarshal JSON into it
	if err := sonic.Unmarshal([]byte(value), newVal.Interface()); err != nil {
		// For complex types (maps, structs), JSON parsing failure should be an error
		// For other types, we might want to be more lenient
		if field.Kind() == reflect.Map || field.Kind() == reflect.Struct {
			return fmt.Errorf("invalid JSON value for %s field: %w", field.Kind(), err)
		}
		return fmt.Errorf("invalid JSON value: %w", err)
	}

	// Set the field to the unmarshaled value
	field.Set(newVal.Elem())
	return nil
}

// setInterfaceValue handles interface{} types with smart type detection
func setInterfaceValue(field reflect.Value, value string) error {
	if value == "" {
		return nil
	}

	// Try to parse as JSON first (for complex types)
	var jsonVal interface{}
	if err := sonic.Unmarshal([]byte(value), &jsonVal); err == nil {
		field.Set(reflect.ValueOf(jsonVal))
		return nil
	}

	// Try to parse as number
	if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
		field.Set(reflect.ValueOf(intVal))
		return nil
	}

	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		field.Set(reflect.ValueOf(floatVal))
		return nil
	}

	// Try to parse as boolean
	if boolVal, err := strconv.ParseBool(value); err == nil {
		field.Set(reflect.ValueOf(boolVal))
		return nil
	}

	// Fallback to string
	field.Set(reflect.ValueOf(value))
	return nil
}
