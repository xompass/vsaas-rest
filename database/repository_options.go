package database

type RepositoryOptions struct {
	Created        bool
	Modified       bool
	Deleted        bool
	RequiredFields []string
}

type UpdateOptions struct {
	Insert bool
	Update bool
}

type MongoUpdate struct {
	CurrentDate any `bson:"$currentDate,omitempty"`
	Inc         any `bson:"$inc,omitempty"`
	Min         any `bson:"$min,omitempty"`
	Max         any `bson:"$max,omitempty"`
	Mul         any `bson:"$mul,omitempty"`
	Rename      any `bson:"$rename,omitempty"`
	Set         any `bson:"$set,omitempty"`
	SetOnInsert any `bson:"$setOnInsert,omitempty"`
	Unset       any `bson:"$unset,omitempty"`
	AddToSet    any `bson:"$addToSet,omitempty"`
	Pop         any `bson:"$pop,omitempty"`
	Pull        any `bson:"$pull,omitempty"`
	PullAll     any `bson:"$pullAll,omitempty"`
	Push        any `bson:"$push,omitempty"`
}

type FieldDetails struct {
	BsonName  string
	JsonName  string
	FieldType string
}
