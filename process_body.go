package rest

import (
	"fmt"
	"reflect"
)

func processBody(ctx *EndpointContext) error {
	body := ctx.ParsedBody
	if body == nil {
		return nil
	}

	if n, ok := body.(Normalizeable); ok {
		if err := n.Normalize(ctx); err != nil {
			return err
		}
	} else {
		if err := ctx.NormalizeStruct(body); err != nil {
			return err
		}
	}

	if s, ok := body.(Sanitizeable); ok {
		if err := s.Sanitize(ctx); err != nil {
			return err
		}
	} else {
		if err := ctx.SanitizeStruct(body); err != nil {
			return err
		}
	}

	// Validation
	if v, ok := body.(Validable); ok {
		if err := v.Validate(ctx); err != nil {
			return err
		}
	} else {
		if isValidable(body) {
			return ctx.ValidateStruct(body)
		}

		rt := reflect.TypeOf(body)
		switch rt.Kind() {
		case reflect.Slice, reflect.Array:
			v := reflect.ValueOf(body)
			for i := 0; i < v.Len(); i++ {
				elem := v.Index(i).Interface()
				if err := validateAny(ctx, elem); err != nil {
					return fmt.Errorf("validation error at index %d: %w", i, err)
				}
			}
		case reflect.Map:
			v := reflect.ValueOf(body)
			for _, key := range v.MapKeys() {
				elem := v.MapIndex(key).Interface()
				if err := validateAny(ctx, elem); err != nil {
					return fmt.Errorf("validation error at key %v: %w", key, err)
				}
			}
		}
	}

	return nil
}
