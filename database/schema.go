package database

import (
	"log"
	"reflect"
	"strings"
	"time"
)

type FieldTags struct {
	Name      string
	OmitEmpty bool
	MinSize   bool
	Truncate  bool
	Inline    bool
	Skip      bool
	Required  bool
}

type FieldsOptions string

const (
	FieldsAlways FieldsOptions = "always" // Always include the field
	FieldsNever  FieldsOptions = "never"  // Never include the field
)

type FilterTags struct {
	Fields FieldsOptions
}

type Field struct {
	FieldName         string
	BsonName          string
	JsonName          string
	DataType          string
	IsPointer         bool
	FieldType         reflect.Type
	IndirectFieldType reflect.Type
	StructField       reflect.StructField
	Tag               reflect.StructTag
	FilterTags        FilterTags
}

type Schema struct {
	Model                IModel
	Name                 string
	CollectionName       string
	JSONFields           map[string]*Field
	Fields               map[string]*Field
	RequiredFilterFields map[string]*Field
	BannedFields         map[string]*Field
	// Relations            []Relation
	ReflectValue reflect.Value
}

type RelationType string

const (
	RelationTypeHasOne    RelationType = "hasOne"
	RelationTypeHasMany   RelationType = "hasMany"
	RelationTypeBelongsTo RelationType = "belongsTo"
)

type TargetModel struct {
	Name           string
	CollectionName string
}

type Relation struct {
	FieldName    string
	JsonName     string
	RelationType RelationType
	TargetModel  IModel
}

func NewSchema(model IModel) *Schema {
	val := reflect.ValueOf(model)
	schema := Schema{
		Model:                model,
		Name:                 model.GetModelName(),
		CollectionName:       model.GetTableName(),
		JSONFields:           map[string]*Field{},
		Fields:               map[string]*Field{},
		RequiredFilterFields: map[string]*Field{},
		BannedFields:         map[string]*Field{},
		ReflectValue:         val,
	}

	schema.InitFields(&val, "", "")
	/* err := schema.InitRelations(&val)
	if err != nil {
		return nil
	} */
	return &schema
}

func (s *Schema) InitFields(val *reflect.Value, jsonParentField string, bsonParentField string) {
	for i := range val.Type().NumField() {
		field := val.Type().Field(i)
		if err := s.InitField(val, field, jsonParentField, bsonParentField); err != nil {
			log.Println(err)
		}
	}
}

func (s *Schema) AddField(field *Field, topLevelField bool) {
	if topLevelField {
		s.Fields[field.FieldName] = field

		if field.FilterTags.Fields == FieldsNever {
			s.BannedFields[field.FieldName] = field
		} else if field.FilterTags.Fields == FieldsAlways {
			s.RequiredFilterFields[field.FieldName] = field
		}

		/*if field.BsonName != "" {
			s.FieldsByBSONName[field.BsonName] = field
		}*/
	}

	s.JSONFields[field.JsonName] = field
}

func (s *Schema) InitRelations(val *reflect.Value) error {
	for i := range val.Type().NumField() {
		fieldStruct := val.Type().Field(i)

		fieldType := fieldStruct.Type
		indirectFieldType := fieldType

		if fieldType.Kind() == reflect.Ptr {
			indirectFieldType = fieldType.Elem()
		}

		fieldValue := reflect.Zero(indirectFieldType)
		val := fieldValue.Interface()
		fieldType = fieldValue.Type()
		fieldKind := fieldValue.Kind()
		modelInterface := reflect.TypeOf((*IModel)(nil)).Elem()
		switch fieldKind { //nolint:exhaustive
		case reflect.Struct:
			if isModel := fieldType.Implements(modelInterface); !isModel {
				continue
			}
			model := val.(IModel)
			_, err := s.parseRelation(model, fieldStruct)

			if err != nil {
				return err
			}
		case reflect.Slice, reflect.Array:
			fieldValue = reflect.New(fieldType.Elem()).Elem()
			val = fieldValue.Interface()
			fieldType = fieldValue.Type()
			// fieldKind = fieldValue.Kind()
			if isModel := fieldType.Implements(modelInterface); !isModel {
				continue
			}
			model := val.(IModel)
			_, err := s.parseRelation(model, fieldStruct)

			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Schema) InitField(model *reflect.Value, fieldStruct reflect.StructField, jsonParentField string, bsonParentField string) error {
	bsonTags, _ := parseFieldTags(fieldStruct, "bson")
	jsonTags, _ := parseFieldTags(fieldStruct, "json")
	filterTags, _ := parseFilterTags(fieldStruct)

	fieldType := fieldStruct.Type

	jsonPrefix := ""
	bsonPrefix := ""
	topLevelField := true
	if jsonParentField != "" {
		jsonPrefix = jsonParentField + "."
		topLevelField = false
	}

	if bsonParentField != "" {
		bsonPrefix = bsonParentField + "."
		topLevelField = false
	}

	field := Field{
		FieldName:         fieldStruct.Name,
		BsonName:          bsonPrefix + bsonTags.Name,
		JsonName:          jsonPrefix + jsonTags.Name,
		FieldType:         fieldType,
		IndirectFieldType: fieldType,
		FilterTags:        filterTags,
	}

	isPointer := false
	if fieldType.Kind() == reflect.Ptr {
		field.IndirectFieldType = fieldType.Elem()
		isPointer = true
	}

	fieldValue := reflect.Zero(field.IndirectFieldType)
	val := fieldValue.Interface()
	fieldType = fieldValue.Type()
	fieldKind := fieldValue.Kind()

	modelInterface := reflect.TypeOf((*IModel)(nil)).Elem()
	switch fieldKind { //nolint:exhaustive
	case reflect.Struct:
		switch val.(type) {
		case time.Time, MongoDate:
			field.DataType = "Date"
			field.IsPointer = isPointer
			s.AddField(&field, topLevelField)
		default:
			if bsonTags.Inline {
				s.InitFields(&fieldValue, jsonParentField, bsonParentField)
			} else if !fieldType.Implements(modelInterface) && !isRelation(model, fieldStruct) {
				field.DataType = fieldType.Name()
				field.IsPointer = isPointer
				s.AddField(&field, topLevelField)
				s.InitFields(&fieldValue, field.JsonName, field.BsonName)
			}
		}
	case reflect.Slice, reflect.Array:
		fieldValue = reflect.New(fieldType.Elem()).Elem()
		val = fieldValue.Interface()
		fieldType = fieldValue.Type()
		fieldKind = fieldValue.Kind()

		if field.IndirectFieldType.Name() == "ObjectID" {
			field.DataType = "ObjectID"
			s.AddField(&field, topLevelField)
		} else if !fieldType.Implements(modelInterface) && !isRelation(model, fieldStruct) {
			field.DataType = fieldType.Name()
			s.AddField(&field, topLevelField)

			// If the slice element is a struct (not a model or relation), traverse its fields
			if fieldKind == reflect.Struct {
				// Skip time.Time and MongoDate types
				switch val.(type) {
				case time.Time, MongoDate:
					// Don't traverse these types
				default:
					// Recursively initialize fields of the slice element
					s.InitFields(&fieldValue, field.JsonName, field.BsonName)
				}
			}
		}
	default:
		field.DataType = field.IndirectFieldType.Name()
		s.AddField(&field, topLevelField)
	}

	return nil
}

func isRelation(model *reflect.Value, fieldStruct reflect.StructField) bool {
	if !fieldStruct.IsExported() || fieldStruct.Tag.Get("bson") != "-" {
		return false
	}

	relationalModelType := reflect.TypeOf((*IRelationalModel)(nil)).Elem()
	var iface any
	if model.Type().Implements(relationalModelType) {
		iface = model.Interface()
	} else if model.CanAddr() && model.Addr().Type().Implements(relationalModelType) {
		iface = model.Addr().Interface()
	} else {
		return false
	}

	relModel := iface.(IRelationalModel)

	if relModel == nil || relModel.Relations() == nil {
		return false
	}

	_, isRelation := relModel.Relations()[fieldStruct.Name]

	return isRelation
}

func (s *Schema) parseRelation(model IModel, fieldStruct reflect.StructField) (bool, error) {
	if !fieldStruct.IsExported() || fieldStruct.Tag.Get("bson") != "-" {
		return false, nil
	}

	lbTag := strings.TrimSpace(fieldStruct.Tag.Get("lb_rel"))
	if lbTag == "embedded" {
		return false, nil
	}

	jsonTags, _ := parseFieldTags(fieldStruct, "json")

	relation := Relation{
		FieldName:    fieldStruct.Name,
		JsonName:     jsonTags.Name,
		RelationType: RelationTypeHasOne,
		TargetModel:  model,
	}

	if _, ok := s.Fields[model.GetModelName()+"Id"]; ok {
		relation.RelationType = RelationTypeBelongsTo
	}

	if fieldStruct.Type.Kind() == reflect.Slice || fieldStruct.Type.Kind() == reflect.Array {
		relation.RelationType = RelationTypeHasMany
	}

	// s.Relations = append(s.Relations, relation)
	return true, nil
}

func parseFieldTags(fieldStruct reflect.StructField, tagName string) (FieldTags, error) {
	key := strings.ToLower(fieldStruct.Name)
	tag, ok := fieldStruct.Tag.Lookup(tagName)

	if !ok && !strings.Contains(string(fieldStruct.Tag), ":") && len(fieldStruct.Tag) > 0 {
		tag = string(fieldStruct.Tag)
	}
	return parseXSONTags(key, tag)
}

func parseFilterTags(fieldStruct reflect.StructField) (FilterTags, error) {
	tag, ok := fieldStruct.Tag.Lookup("filter")
	if !ok {
		return FilterTags{}, nil
	}

	st := FilterTags{}
	for _, str := range strings.Split(tag, ",") {
		if str == "-" {
			continue
		}

		temp := strings.Split(str, "=")
		fieldProp := temp[0]
		fieldValue := temp[1]

		if fieldProp == "fields" {
			st.Fields = FieldsOptions(fieldValue)
		}
	}

	return st, nil
}

func parseXSONTags(key string, tag string) (FieldTags, error) {
	var st FieldTags
	if tag == "-" {
		st.Skip = true
		return st, nil
	}

	for idx, str := range strings.Split(tag, ",") {
		if idx == 0 && str != "" {
			key = str
		}
		switch str {
		case "omitempty":
			st.OmitEmpty = true
		case "minsize":
			st.MinSize = true
		case "truncate":
			st.Truncate = true
		case "inline":
			st.Inline = true
		case "required":
			st.Required = true
		}
	}

	st.Name = key

	return st, nil
}
