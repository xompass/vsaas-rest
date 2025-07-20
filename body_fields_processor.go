package rest

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/text/unicode/norm"
)

// bodyStructFieldsCache caches field processing information by struct type and tag key
// to avoid expensive reflection operations on repeated calls
var bodyStructFieldsCache sync.Map

// fieldProcessorFunc defines the signature for field processing functions
type fieldProcessorFunc func(reflect.Value)

type tagProcessors struct {
	funcs []fieldProcessorFunc
	dive  bool
}

// cachedStructField contains pre-computed information about struct fields
// that need processing, including their positions and associated functions
type cachedStructField struct {
	index     []int // Field index path for nested access
	normalize *tagProcessors
	sanitize  *tagProcessors
}

type cachedBodyStructMetadata struct {
	fields      []cachedStructField
	hasValidate bool
}

var operators = map[string]map[string]fieldProcessorFunc{
	"normalize": {
		"trim":      trimNormalizer,
		"lowercase": lowercaseNormalizer,
		"uppercase": uppercaseNormalizer,
		"unaccent":  unaccentNormalizer,
		"unicode":   unicodeNormalizer,
	},
	"sanitize": {
		"html":         htmlSanitizer,
		"alphanumeric": alphanumericSanitizer,
		"numeric":      numericSanitizer,
	},
}

var htmlPolicy = bluemonday.UGCPolicy()

func parseTag(tag string) []string {
	parts := strings.Split(tag, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// buildStructFields builds a list of cachedStructField for the given struct type
func buildStructFields(t reflect.Type) (cachedBodyStructMetadata, error) {
	var fields []cachedStructField
	hasValidate := false

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // unexported
			continue
		}

		// Marcar si hay al menos un tag validate
		if tag := sf.Tag.Get("validate"); tag != "" {
			hasValidate = true
		}

		sanitizeTag := ""
		normalizeTag := ""

		if tag := sf.Tag.Get("sanitize"); tag != "" {
			sanitizeTag = tag
		}

		if tag := sf.Tag.Get("normalize"); tag != "" {
			normalizeTag = tag
		}

		if sanitizeTag == "" && normalizeTag == "" {
			continue
		}

		fs := cachedStructField{
			index: []int{i},
		}

		diveable := isDiveable(sf.Type)
		if normalizeTag != "" {
			tags := parseTag(normalizeTag)

			hasDive := slices.Contains(tags, "dive")
			if hasDive && !diveable {
				return cachedBodyStructMetadata{}, fmt.Errorf("field %s is marked with 'dive' but is not diveable", sf.Name)
			}

			fs.normalize = &tagProcessors{
				dive: hasDive,
			}
			for _, tag := range tags {
				if fn, ok := operators["normalize"][tag]; ok {
					fs.normalize.funcs = append(fs.normalize.funcs, fn)
				}
			}
		}

		if sanitizeTag != "" {
			tags := parseTag(sanitizeTag)
			hasDive := slices.Contains(tags, "dive")
			if hasDive && !diveable {
				return cachedBodyStructMetadata{}, fmt.Errorf("field %s is marked with 'dive' but is not diveable", sf.Name)
			}

			fs.sanitize = &tagProcessors{
				dive: hasDive,
			}

			for _, tag := range tags {
				if fn, ok := operators["sanitize"][tag]; ok {
					fs.sanitize.funcs = append(fs.sanitize.funcs, fn)
				}
			}
		}

		fields = append(fields, fs)
	}

	return cachedBodyStructMetadata{
		fields:      fields,
		hasValidate: hasValidate,
	}, nil
}

func registerStruct(v any) error {
	if v == nil {
		return nil
	}

	rt := reflect.TypeOf(v)

	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	if rt.Kind() != reflect.Struct {
		return fmt.Errorf("expected a struct, got: %s", rt.Kind())
	}

	cacheKey := rt
	if _, ok := bodyStructFieldsCache.Load(cacheKey); ok {
		return nil
	}

	meta, err := buildStructFields(rt)
	if err != nil {
		return err
	}

	bodyStructFieldsCache.Store(cacheKey, meta)
	return nil
}

// processStruct processes a struct by applying registered field processors
// based on the specified tag key (e.g., "normalize", "sanitize")
// It handles nested structures and slices/maps of structs.
// It caches the field processing information to optimize repeated calls.
// The struct must be passed as a pointer to allow modifications.
// If the struct is nil or not a pointer to a struct, it does nothing.
func processStruct(v any, operator ...string) error {
	if v == nil {
		return nil
	}

	if len(operator) > 0 {
		if _, ok := operators[operator[0]]; !ok {
			// Invalid operator, return without processing
			return errors.New("invalid operator: " + operator[0])
		}
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("expected a non-nil pointer to a struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errors.New("expected a struct, got: " + rv.Kind().String())
	}

	rt := rv.Type()
	cacheKey := rt

	var meta cachedBodyStructMetadata
	if cached, ok := bodyStructFieldsCache.Load(cacheKey); ok {
		meta = cached.(cachedBodyStructMetadata)
	} else {
		_meta, err := buildStructFields(rt)
		if err != nil {
			return err
		}

		meta = _meta

		bodyStructFieldsCache.Store(cacheKey, meta)
	}
	fields := meta.fields

	for _, fs := range fields {
		fv := rv.FieldByIndex(fs.index)
		if !fv.IsValid() || !fv.CanSet() {
			continue
		}

		var funcs []fieldProcessorFunc
		requiresDiveNormalization := fs.normalize != nil && fs.normalize.dive
		requiresDiveSanitization := fs.sanitize != nil && fs.sanitize.dive
		if len(operator) > 0 {
			switch operator[0] {
			case "sanitize":
				if fs.sanitize != nil {
					funcs = fs.sanitize.funcs
				}
			case "normalize":
				if fs.normalize != nil {
					funcs = fs.normalize.funcs
				}
			}
		} else {
			if fs.normalize != nil {
				funcs = slices.Concat(funcs, fs.normalize.funcs)
			}
			if fs.sanitize != nil {
				funcs = slices.Concat(funcs, fs.sanitize.funcs)
			}
		}

		if requiresDiveNormalization || requiresDiveSanitization {
			fieldName := rt.FieldByIndex(fs.index).Name
			switch fv.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < fv.Len(); i++ {
					elem := fv.Index(i)
					if elem.IsValid() {
						err := applyProcessors(elem, funcs, operator...)
						if err != nil {
							return fmt.Errorf("error processing field '%s' at index %d: %w", fieldName, i, err)
						}
					}
				}
			case reflect.Map:
				for _, key := range fv.MapKeys() {
					val := fv.MapIndex(key)
					if !val.IsValid() {
						continue
					}

					switch val.Kind() {
					case reflect.Ptr:
						if !val.IsNil() {
							// Process pointer elements in place if possible
							err := applyProcessors(val, funcs, operator...)
							if err != nil {
								return fmt.Errorf("error processing field '%s' for key '%v': %w", fieldName, key, err)
							}
						}
					case reflect.Struct:
						// Only create copy for structs since they're not addressable from maps
						valCopy := reflect.New(val.Type()).Elem()
						valCopy.Set(val)
						err := processStruct(valCopy.Addr().Interface(), operator...)
						if err != nil {
							return fmt.Errorf("error processing field '%s' for key '%v': %w", fieldName, key, err)
						}
						fv.SetMapIndex(key, valCopy)
					default:
						// For primitive types, only create copy if we have processors to apply
						if len(funcs) > 0 {
							valCopy := reflect.New(val.Type()).Elem()
							valCopy.Set(val)
							err := applyProcessors(valCopy, funcs, operator...)
							if err != nil {
								return fmt.Errorf("error processing field '%s' for key '%v': %w", fieldName, key, err)
							}
							fv.SetMapIndex(key, valCopy)
						}
					}
				}
			case reflect.Struct, reflect.Ptr:
				err := applyProcessors(fv, nil, operator...)
				if err != nil {
					return fmt.Errorf("error processing nested struct field '%s': %w", fieldName, err)
				}
			}
		} else {
			err := applyProcessors(fv, funcs, operator...)
			if err != nil {
				return fmt.Errorf("error applying processors to field '%s': %w", rt.FieldByIndex(fs.index).Name, err)
			}
		}
	}

	return nil
}

func applyProcessors(v reflect.Value, funcs []fieldProcessorFunc, operator ...string) error {
	if !v.IsValid() {
		return nil
	}

	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		if v.CanAddr() {
			return processStruct(v.Addr().Interface(), operator...)
		} else {
			// If the struct is not addressable, we cannot process it in place
			// This should only happen in special cases handled by the caller
			return nil
		}
	}

	// Only apply functions if we have any
	if len(funcs) > 0 {
		for _, fn := range funcs {
			if fn != nil {
				fn(v)
			}
		}
	}
	return nil
}

// processStringValue applies a transformation function to string values
func processStringValue(v reflect.Value, transform func(string) string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(transform(v.String()))
	case reflect.Ptr:
		if !v.IsNil() && v.Elem().Kind() == reflect.String {
			v.Elem().SetString(transform(v.Elem().String()))
		}
	}
}

// htmlSanitizer applies HTML sanitization using bluemonday
func htmlSanitizer(v reflect.Value) {
	processStringValue(v, htmlPolicy.Sanitize)
}

// alphanumericSanitizer removes all non-alphanumeric characters from a string
func alphanumericSanitizer(v reflect.Value) {
	processStringValue(v, func(s string) string {
		var b strings.Builder
		b.Grow(len(s))
		for _, r := range s {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			}
		}
		return b.String()
	})
}

// numericSanitizer removes all non-digit characters from a string
func numericSanitizer(v reflect.Value) {
	processStringValue(v, func(s string) string {
		var b strings.Builder
		b.Grow(len(s))
		for _, r := range s {
			if unicode.IsDigit(r) {
				b.WriteRune(r)
			}
		}
		return b.String()
	})
}

// trimNormalizer removes leading and trailing whitespace from strings
func trimNormalizer(v reflect.Value) {
	processStringValue(v, strings.TrimSpace)
}

// lowercaseNormalizer converts strings to lowercase
func lowercaseNormalizer(v reflect.Value) {
	processStringValue(v, strings.ToLower)
}

// uppercaseNormalizer converts strings to uppercase
func uppercaseNormalizer(v reflect.Value) {
	processStringValue(v, strings.ToUpper)
}

// unaccentNormalizer removes diacritics from strings
func unaccentNormalizer(v reflect.Value) {
	processStringValue(v, removeDiacritics)
}

// unicodeNormalizer normalizes Unicode strings to NFC form.
func unicodeNormalizer(v reflect.Value) {
	processStringValue(v, norm.NFC.String)
}

func removeDiacritics(s string) string {
	t := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(s)) // Pre-allocate capacity to avoid reallocations
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String()) // Normalize back to composed form
}

func isStruct(v reflect.Type) bool {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v.Kind() == reflect.Struct
}

func isSliceOrArray(v reflect.Type) bool {
	return v.Kind() == reflect.Slice || v.Kind() == reflect.Array
}

func isMap(v reflect.Type) bool {
	return v.Kind() == reflect.Map
}

func isDiveable(v reflect.Type) bool {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return isStruct(v) || isSliceOrArray(v) || isMap(v)
}
