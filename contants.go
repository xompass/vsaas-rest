package rest

type ResponseType string

const (
	ResponseTypeJSON      ResponseType = "json"
	ResponseTypeXML       ResponseType = "xml"
	ResponseTypeText      ResponseType = "text"
	ResponseTypeHTML      ResponseType = "html"
	ResponseTypeNoContent ResponseType = "no_content"
	ResponseTypeFile      ResponseType = "file"
)

type EndpointMethod string

const (
	MethodHEAD   EndpointMethod = "Head"
	MethodGET    EndpointMethod = "Get"
	MethodPOST   EndpointMethod = "Post"
	MethodPUT    EndpointMethod = "Put"
	MethodPATCH  EndpointMethod = "Patch"
	MethodDELETE EndpointMethod = "Delete"
)

type ParamLocation string

const (
	InQuery  ParamLocation = "query"
	InPath   ParamLocation = "path"
	InHeader ParamLocation = "header"
)

type PathParamType string

const (
	PathParamTypeString   PathParamType = "string"
	PathParamTypeInt      PathParamType = "int"
	PathParamTypeFloat    PathParamType = "float"
	PathParamTypeBool     PathParamType = "bool"
	PathParamTypeDate     PathParamType = "date"
	PathParamTypeDateTime PathParamType = "datetime"
	PathParamTypeObjectID PathParamType = "objectid"
)

type QueryParamType string

const (
	QueryParamTypeString   QueryParamType = "string"
	QueryParamTypeInt      QueryParamType = "int"
	QueryParamTypeFloat    QueryParamType = "float"
	QueryParamTypeBool     QueryParamType = "bool"
	QueryParamTypeDate     QueryParamType = "date"
	QueryParamTypeDateTime QueryParamType = "datetime"
	QueryParamTypeObjectID QueryParamType = "objectid"
	QueryParamTypeFilter   QueryParamType = "filter"
	QueryParamTypeWhere    QueryParamType = "where"
)

type HeaderParamType string

const (
	HeaderParamTypeString   HeaderParamType = "string"
	HeaderParamTypeInt      HeaderParamType = "int"
	HeaderParamTypeFloat    HeaderParamType = "float"
	HeaderParamTypeBool     HeaderParamType = "bool"
	HeaderParamTypeDate     HeaderParamType = "date"
	HeaderParamTypeDateTime HeaderParamType = "datetime"
	HeaderParamTypeObjectID HeaderParamType = "objectid"
	HeaderParamTypeFilter   HeaderParamType = "filter"
	HeaderParamTypeWhere    HeaderParamType = "where"
)

type ActionType string

const (
	ActionTypeRead           ActionType = "read"
	ActionTypeCreate         ActionType = "create"
	ActionTypeUpdate         ActionType = "update"
	ActionTypeDelete         ActionType = "delete"
	ActionTypeLogin          ActionType = "login"
	ActionTypeLogout         ActionType = "logout"
	ActionTypeResetPassword  ActionType = "reset_password"
	ActionTypeChangePassword ActionType = "change_password"
)
