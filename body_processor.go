package rest

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
)

// Validable is an interface that defines a method for validating an endpoint context
type Validable interface {
	Validate(ctx *EndpointContext) error
}

// Sanitizeable is an interface that defines a method for sanitizing an endpoint context
type Sanitizeable interface {
	Sanitize(ctx *EndpointContext) error
}

// Normalizeable is an interface that defines a method for normalizing an endpoint context
type Normalizeable interface {
	Normalize(ctx *EndpointContext) error
}

// RegisterBodyNormalizer permite registrar nuevos normalizadores personalizados
func RegisterBodyNormalizer(name string, fn fieldProcessorFunc) error {
	if fn == nil {
		return errors.New("normalizer function cannot be nil")
	}

	if _, exists := operators["normalize"][name]; exists {
		return errors.New("normalizer already exists")
	}

	operators["normalize"][name] = fn
	return nil
}

// RegisterBodySanitizer permite registrar nuevos sanitizadores personalizados
func RegisterBodySanitizer(name string, fn fieldProcessorFunc) error {
	if fn == nil {
		return errors.New("sanitizer function cannot be nil")
	}

	if _, exists := operators["sanitize"][name]; exists {
		return errors.New("sanitizer already exists")
	}

	operators["sanitize"][name] = fn
	return nil
}

// GetBodyNormalizers devuelve una copia de los normalizadores registrados
func GetBodyNormalizers() map[string]fieldProcessorFunc {
	result := make(map[string]fieldProcessorFunc)
	maps.Copy(result, operators["normalize"])
	return result
}

// GetBodySanitizers devuelve una copia de los sanitizadores registrados
func GetBodySanitizers() map[string]fieldProcessorFunc {
	result := make(map[string]fieldProcessorFunc)
	maps.Copy(result, operators["sanitize"])
	return result
}

func validateAny(ctx *EndpointContext, val any) error {
	if val == nil {
		return errors.New("cannot validate nil value")
	}

	// Caso 1: tiene método Validate
	if v, ok := val.(Validable); ok {
		return v.Validate(ctx)
	}

	// Caso 2: validación automática con validator
	if isValidable(val) {
		return ctx.ValidateStruct(val)
	}

	// Caso 3: slice/map de structs validables
	rt := reflect.TypeOf(val)
	switch rt.Kind() {
	case reflect.Slice, reflect.Array:
		v := reflect.ValueOf(val)
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i).Interface()
			if err := validateAny(ctx, elem); err != nil {
				return fmt.Errorf("validation error at index %d: %w", i, err)
			}
		}
		return nil
	case reflect.Map:
		v := reflect.ValueOf(val)
		for _, key := range v.MapKeys() {
			elem := v.MapIndex(key).Interface()
			if err := validateAny(ctx, elem); err != nil {
				return fmt.Errorf("validation error at key %v: %w", key, err)
			}
		}
		return nil
	}

	// Caso 4: No es validable, no se hace nada
	return nil
}

func isValidable(val any) bool {
	if val == nil {
		return false
	}

	rt := reflect.TypeOf(val)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	if rt.Kind() != reflect.Struct {
		return false
	}

	if cached, ok := bodyStructFieldsCache.Load(rt); ok {
		return cached.(cachedBodyStructMetadata).hasValidate
	}

	err := registerStruct(val)
	if err != nil {
		return false
	}
	if cached, ok := bodyStructFieldsCache.Load(rt); ok {
		return cached.(cachedBodyStructMetadata).hasValidate
	}

	return false
}
