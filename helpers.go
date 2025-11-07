package rest

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/xompass/vsaas-rest/database"
	"github.com/xompass/vsaas-rest/http_errors"
	"github.com/xompass/vsaas-rest/lbq"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func sanitizeStruct(ctx *EndpointContext, v any) error {
	if v == nil {
		return nil
	}

	if sanitizable, ok := v.(Sanitizeable); ok {
		return sanitizable.Sanitize(ctx)
	}

	return processStruct(v, "sanitize")
}

func normalizeStruct(ctx *EndpointContext, v any) error {
	if v == nil {
		return nil
	}

	if normalizable, ok := v.(Normalizeable); ok {
		return normalizable.Normalize(ctx)
	}

	return processStruct(v, "normalize")
}

func parseBody(e *Endpoint, ec *EndpointContext) error {
	if e.Method != MethodPOST && e.Method != MethodPUT && e.Method != MethodPATCH {
		return nil
	}

	if e.BodyParams == nil {
		return nil
	}

	form := e.BodyParams()

	if form == nil {
		return http_errors.BadRequestError("Request body cannot be nil")
	}

	// Protect against multipart file uploads when FileUploadConfig is not configured
	if err := validateMultipartWithoutFileConfig(e, ec); err != nil {
		return err
	}

	if err := bindFormToStruct(ec, form); err != nil {
		log.Println("cannot bind to struct", err)
		return http_errors.BadRequestError("Failed to bind request body", fmt.Sprintf("Failed to bind request body: %s", err.Error()))
	}

	if err := sanitizeStruct(ec, form); err != nil {
		var errResponse *http_errors.ErrorResponse
		if errors.As(err, &errResponse) {
			return errResponse
		}

		return http_errors.BadRequestError("Failed to sanitize request body", getFriendlyValidationErrors(err))
	}

	if err := normalizeStruct(ec, form); err != nil {
		var errResponse *http_errors.ErrorResponse
		if errors.As(err, &errResponse) {
			return errResponse
		}
		return http_errors.BadRequestError("Failed to normalize request body", getFriendlyValidationErrors(err))
	}

	if err := validateAny(ec, form); err != nil {
		var errResponse *http_errors.ErrorResponse
		if errors.As(err, &errResponse) {
			return errResponse
		}
		return http_errors.BadRequestError("Failed to validate request body", getFriendlyValidationErrors(err))
	}

	ec.ParsedBody = form
	return nil
}

type ParamErrors []http_errors.ErrorResponse

func (pe ParamErrors) Error() string {
	var messages []string
	for _, err := range pe {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

func parseAllParams(e *Endpoint, ec *EndpointContext) error {
	ec.ParsedQuery = make(map[string]any)
	ec.ParsedPath = make(map[string]any)
	ec.ParsedHeader = make(map[string]any)

	var paramErrors ParamErrors

	for _, param := range e.Accepts {
		val, err := parseParam(ec, param)
		if err != nil {
			errResponse, ok := err.(http_errors.ErrorResponse)
			if !ok {
				errResponse = http_errors.BadRequestError("Invalid parameter", fmt.Sprintf("Parameter %s: %s", param.name, err.Error()))
			}

			paramErrors = append(paramErrors, errResponse)
			continue
		}

		switch param.in {
		case InQuery:
			ec.ParsedQuery[param.name] = val
		case InPath:
			ec.ParsedPath[param.name] = val
		case InHeader:
			ec.ParsedHeader[param.name] = val
		}
	}

	if len(paramErrors) > 0 {
		return paramErrors
	}

	return nil
}

func parseParam(ctx *EndpointContext, param Param) (any, error) {
	if ctx == nil || ctx.EchoCtx == nil {
		return nil, http_errors.BadRequestError("Invalid context", "Endpoint context is required to get path parameters")
	}

	var raw string

	switch param.in {
	case InQuery:
		raw = ctx.EchoCtx.QueryParam(param.name)
	case InPath:
		raw = ctx.EchoCtx.Param(param.name)
	case InHeader:
		headers := ctx.EchoCtx.Request().Header[param.name]
		if len(headers) > 0 {
			raw = headers[0]
		} else {
			raw = ""
		}
	}

	if param.required {
		if param.in == InQuery {
			if _, exists := ctx.EchoCtx.QueryParams()[param.name]; !exists {
				return nil, http_errors.BadRequestError("Missing parameter", fmt.Sprintf("Parameter %s is required", param.name))
			}
		} else if raw == "" {
			return nil, http_errors.BadRequestError("Missing parameter", fmt.Sprintf("Parameter %s is required", param.name))
		}
	}

	if param.Parser != nil {
		val, err := param.Parser(raw)
		if err != nil {
			return nil, http_errors.BadRequestError("Invalid parameter", fmt.Sprintf("Parameter %s is invalid: %s", param.name, err.Error()))
		}

		return val, nil
	}

	if raw == "" && param.in != InQuery {
		return nil, nil
	}

	if raw == "" && param.in == InQuery && param.paramType != string(PathParamTypeBool) {
		return nil, nil
	}

	switch param.paramType {
	case string(PathParamTypeString):
		return raw, nil
	case string(PathParamTypeInt):
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, http_errors.BadRequestError("Invalid parameter", "Parameter "+param.name+" must be an integer")
		}

		return value, nil
	case string(PathParamTypeBool):
		if param.in == InQuery {
			// check for params like ?param, this must be equivalent to ?param=true
			if _, exists := ctx.EchoCtx.QueryParams()[param.name]; exists && raw == "" {
				return true, nil
			}
			if raw == "" {
				return false, nil
			}
		}

		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, http_errors.BadRequestError("Invalid parameter", "Parameter "+param.name+" must be a boolean")
		}
		return value, nil
	case string(PathParamTypeFloat):
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, http_errors.BadRequestError("Invalid parameter", "Parameter "+param.name+" must be a float")
		}

		return value, nil
	case string(PathParamTypeDate):
		value, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return nil, http_errors.BadRequestError("Invalid parameter", "Parameter "+param.name+" must be a date in the format YYYY-MM-DD")
		}
		return value, nil
	case string(PathParamTypeDateTime):
		value, err := time.Parse("2006-01-02T15:04:05Z07:00", raw)
		if err != nil {
			return nil, http_errors.BadRequestError("Invalid parameter", "Parameter "+param.name+" must be a datetime in the format YYYY-MM-DDTHH:MM:SSZ")
		}
		return value, nil
	case string(PathParamTypeObjectID):
		if oid, err := bson.ObjectIDFromHex(raw); err != nil {
			return nil, http_errors.BadRequestError("Invalid parameter", "Parameter "+param.name+" must be a valid ObjectID")
		} else {
			return oid, nil
		}
	case string(QueryParamTypeFilter):
		filter, err := lbq.ParseFilter(raw)
		if err != nil {
			log.Println("Error parsing filter:", err)
			return nil, http_errors.BadRequestError("Invalid filter", "Parameter "+param.name+" must be a valid filter: "+err.Error())
		}

		filterBuilder := database.NewFilter()

		if filter != nil {
			filterBuilder = filterBuilder.FromLBFilter(filter)
		}

		return filterBuilder, nil

	case string(QueryParamTypeWhere):
		where, err := lbq.ParseWhere(raw)
		if err != nil {
			return nil, http_errors.BadRequestError("Invalid where clause", "Parameter "+param.name+" must be a valid where clause: "+err.Error())
		}

		whereBuilder := database.NewWhere().Raw(where)
		return whereBuilder, nil
	default:
		return nil, http_errors.BadRequestError("Invalid parameter type", "Parameter "+param.name+" has an invalid type")
	}
}

func getFriendlyValidationErrors(err error) map[string]string {
	friendlyErrors := map[string]string{}
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, e := range ve {
			message := getErrorMessage(e.Tag(), e.Kind().String(), e.Param())
			if message == "" {
				message = "This field is invalid"
			}
			friendlyErrors[e.Field()] = message
		}
	} else {
		message := err.Error()
		// log.Println("Error parsing validation error:", message)
		if strings.Contains(message, "field:") {
			parts := strings.SplitSeq(message, ";")

			for part := range parts {
				field := ""
				tag := ""
				kind := ""
				errorMessage := "This field is invalid"
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}

				if strings.HasPrefix(part, "field:") {
					subparts := strings.Split(strings.Replace(part, "field:", "", 1), ",")
					if len(subparts) > 0 {
						field = subparts[0]
					}

					if len(subparts) > 1 {
						tag = subparts[1]
					}

					if len(subparts) > 2 {
						kind = subparts[2]
					}
				} else if strings.HasPrefix(part, "message:") {
					errorMessage = strings.Split(part, ":")[1]
				}

				if field != "" {
					if tag != "" {
						message = getErrorMessage(tag, kind, "")
						if message != "" {
							errorMessage = message
						}
					}
					friendlyErrors[field] = errorMessage
				} else {
					friendlyErrors["error"] = message
				}
			}

		} else {
			friendlyErrors["error"] = message
		}
	}

	return friendlyErrors
}

func getErrorMessage(tag string, kind string, param string) string {
	switch tag {
	case "required":
		return "This field is required"
	case "max":
		if kind == "String" || kind == "Slice" || kind == "Array" {
			return "This field must have a maximum length of " + param
		}
		return "This field must be less than " + param
	case "min":
		if kind == "String" || kind == "Slice" || kind == "Array" {
			return "This field must have a minimum length of " + param
		}
		return "This field must be greater than " + param
	case "eq":
		return "This field must be equal to " + param
	case "lt":
		return "This field must be less than " + param
	case "lte":
		return "This field must be less than or equal to " + param
	case "gt":
		return "This field must be greater than " + param
	case "gte":
		return "This field must be greater than or equal to " + param
	case "ne", "ne_ignore_case":
		return "This field must not be equal to " + param
	case "email":
		return "This field must be a valid email"
	case "len":
		return "This field must have a length of " + param
	case "oneof":
		return "This field must be one of: " + param
	case "unique":
		return "This field must be unique"
	default:
		return ""
	}
}

// validateMultipartWithoutFileConfig protects against file uploads when FileUploadConfig is not configured
// This prevents large files from being loaded into memory during body parsing
func validateMultipartWithoutFileConfig(e *Endpoint, ec *EndpointContext) error {
	contentType := ec.EchoCtx.Request().Header.Get("Content-Type")

	// Only check multipart requests
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return nil
	}

	// If FileUploadConfig is configured, files are handled separately - no validation needed
	if e.FileUploadConfig != nil {
		return nil
	}

	// For multipart requests without FileUploadConfig, we need to peek into the request
	// to see if it contains files without consuming the entire body

	// Parse the multipart form with a small memory limit to detect files
	// This will fail if there are large files, protecting us from memory exhaustion
	request := ec.EchoCtx.Request()

	// Set a reasonable limit for form parsing when no file upload is configured
	// This allows for small form data but prevents large file uploads
	const maxMemoryForFormData = 1 << 20 // 1MB limit for form data without file config

	err := request.ParseMultipartForm(maxMemoryForFormData)
	if err != nil {
		// If parsing fails due to size limits, it's likely a file upload attempt
		if strings.Contains(err.Error(), "multipart: message too large") ||
			strings.Contains(err.Error(), "request body too large") {
			return http_errors.BadRequestErrorWithCode("FILE_UPLOAD_NOT_CONFIGURED",
				"This endpoint does not support file uploads. Use JSON body instead, or contact support if file uploads are needed.")
		}
		return http_errors.BadRequestError("Invalid multipart form data", err.Error())
	}

	// Check if the multipart form contains any file fields
	if request.MultipartForm != nil && request.MultipartForm.File != nil {
		for fieldName, files := range request.MultipartForm.File {
			if len(files) > 0 {
				return http_errors.BadRequestErrorWithCode("FILE_UPLOAD_NOT_CONFIGURED",
					fmt.Sprintf("File upload detected in field '%s', but this endpoint does not support file uploads. Use JSON body instead.", fieldName))
			}
		}
	}

	return nil
}
