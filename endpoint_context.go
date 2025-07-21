package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/xompass/vsaas-rest/database"
	"github.com/xompass/vsaas-rest/http_errors"
)

type EndpointContext struct {
	App           *RestApp
	EchoCtx       echo.Context
	Endpoint      *Endpoint
	ParsedBody    any
	ParsedQuery   map[string]any
	ParsedPath    map[string]any
	ParsedHeader  map[string]any
	UploadedFiles map[string][]*UploadedFile
	IpAddress     string
	Principal     Principal
	Token         AuthToken
	context       context.Context
}

func (eCtx *EndpointContext) Context() context.Context {
	return eCtx.context
}

func (eCtx *EndpointContext) ValidateStruct(v any) error {
	if v == nil {
		return nil
	}
	return eCtx.App.ValidatorInstance.Struct(v)
}

func (eCtx *EndpointContext) SanitizeStruct(v any) error {
	if v == nil {
		return nil
	}

	return processStruct(v, "sanitize")
}

func (eCtx *EndpointContext) NormalizeStruct(v any) error {
	if v == nil {
		return nil
	}

	return processStruct(v, "normalize")
}

// GetFilterParam retrieves the filter parameter from either the query or header.
func (eCtx *EndpointContext) GetFilterParam() (*database.FilterBuilder, error) {
	if filter, ok := eCtx.ParsedQuery["filter"]; ok {
		if filter == nil {
			return nil, nil
		}
		if filterBuilder, ok := filter.(*database.FilterBuilder); ok {
			return filterBuilder, nil
		}

		return nil, http_errors.BadRequestError("Invalid filter query parameter")
	}

	if filter, ok := eCtx.ParsedHeader["filter"]; ok {
		if filterBuilder, ok := filter.(*database.FilterBuilder); ok {
			return filterBuilder, nil
		}

		return nil, http_errors.BadRequestError("Invalid filter header parameter")
	}

	return nil, nil
}

/**
 * RespondAndLog sends a response and logs the audit if enabled.
 * @param response The response data to send.
 * @param affectedModelId The ID of the model affected by the operation, used for logging.
 * @param contentType The type of response to send (JSON, XML, Text, HTML, NoContent).
 * @param statuCode Optional status code to override the default 200 OK.
 * @return error if any issue occurs while sending the response or logging the audit.
 */
func (ctx *EndpointContext) RespondAndLog(response any, affectedModelId any, contentType ResponseType, statusCode ...int) error {
	if !ctx.Endpoint.AuditDisabled {
		if ctx.Endpoint.app.auditLogConfig.Enabled && ctx.Endpoint.app.auditLogConfig.Handler != nil {
			err := ctx.Endpoint.app.auditLogConfig.Handler(ctx, response, affectedModelId)
			if err != nil {
				ctx.App.Errorf("Failed to log audit: %v", err)
			}
		}
	}

	status := http.StatusOK
	if len(statusCode) > 0 {
		status = statusCode[0]
	}

	switch contentType {
	case ResponseTypeJSON:
		return ctx.EchoCtx.JSON(status, response)
	case ResponseTypeXML:
		return ctx.EchoCtx.XML(status, response)
	case ResponseTypeText:
		if str, ok := response.(string); ok {
			return ctx.EchoCtx.String(status, str)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "text response must be string")
	case ResponseTypeHTML:
		if str, ok := response.(string); ok {
			return ctx.EchoCtx.HTML(status, str)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "html response must be string")
	case ResponseTypeNoContent:
		return ctx.EchoCtx.NoContent(status)

	default:
		return echo.NewHTTPError(http.StatusNotAcceptable, "unsupported content type")
	}
}

// JSON sends a JSON response
func (ctx *EndpointContext) JSON(response any, statusCode ...int) error {
	status := http.StatusOK
	if len(statusCode) > 0 {
		status = statusCode[0]
	}

	return ctx.EchoCtx.JSON(status, response)
}

// XML sends an XML response
func (ctx *EndpointContext) XML(response any, statusCode ...int) error {
	status := http.StatusOK
	if len(statusCode) > 0 {
		status = statusCode[0]
	}

	return ctx.EchoCtx.XML(status, response)
}

// Text sends a plain text response
func (ctx *EndpointContext) Text(response string, statusCode ...int) error {
	status := http.StatusOK
	if len(statusCode) > 0 {
		status = statusCode[0]
	}

	return ctx.EchoCtx.String(status, response)
}

// HTML sends an HTML response
func (ctx *EndpointContext) HTML(response string, statusCode ...int) error {
	status := http.StatusOK
	if len(statusCode) > 0 {
		status = statusCode[0]
	}

	return ctx.EchoCtx.HTML(status, response)
}

// NoContent sends a 204 No Content response
func (ctx *EndpointContext) NoContent() error {
	return ctx.EchoCtx.NoContent(http.StatusNoContent)
}

// Get retrieves a value from the context by key
func (ctx *EndpointContext) Get(key string) any {
	return ctx.EchoCtx.Get(key)
}

// Set allows setting a value in the context
func (ctx *EndpointContext) Set(key string, value any) {
	ctx.EchoCtx.Set(key, value)
}

// GetUploadedFiles returns uploaded files for a specific field name
func (ctx *EndpointContext) GetUploadedFiles(fieldName string) []*UploadedFile {
	if ctx.UploadedFiles == nil {
		return nil
	}
	return ctx.UploadedFiles[fieldName]
}

// GetFirstUploadedFile returns the first uploaded file for a specific field name
func (ctx *EndpointContext) GetFirstUploadedFile(fieldName string) *UploadedFile {
	files := ctx.GetUploadedFiles(fieldName)
	if len(files) > 0 {
		return files[0]
	}
	return nil
}

// HasUploadedFiles returns true if there are uploaded files for the specified field
func (ctx *EndpointContext) HasUploadedFiles(fieldName string) bool {
	files := ctx.GetUploadedFiles(fieldName)
	return len(files) > 0
}

// GetAllUploadedFiles returns all uploaded files across all fields
func (ctx *EndpointContext) GetAllUploadedFiles() map[string][]*UploadedFile {
	return ctx.UploadedFiles
}
