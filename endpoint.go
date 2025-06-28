package rest

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xompass/vsaas-rest/database"
)

// Este archivo define la estructura y el registro de endpoints HTTP en la aplicación.
// Incluye las siguientes funcionalidades principales:
// 1. Definición de la estructura Endpoint, que incluye detalles como nombre, método HTTP, ruta, roles, manejador, parámetros de consulta y cuerpo, y limitación de tasa.
// 2. Registro de endpoints en la aplicación utilizando la función Register.
// 3. Implementación de middlewares para autenticación, análisis de cuerpo y consulta, y limitación de solicitudes.
// 4. Validación de parámetros de consulta y cuerpo utilizando la biblioteca go-playground/validator.
// 5. Manejo de errores personalizados y generación de mensajes de error amigables.

type RateLimit struct {
	Max    int
	Window time.Duration
	Key    string
}

type Validable interface {
	Validate(ctx *EndpointContext) error
}

type EndpointRole interface {
	RoleName() string
}

type EndpointContext struct {
	App           *RestApp
	FiberCtx      *fiber.Ctx
	Endpoint      *Endpoint
	ParsedBody    any
	ParsedQuery   map[string]any
	ParsedPath    map[string]any
	ParsedHeader  map[string]any
	UploadedFiles map[string][]*UploadedFile // Added for file uploads
	IpAddress     string
	Principal     Principal
	Token         AuthToken
}

func (eCtx *EndpointContext) ValidateStruct(v any) error {
	return eCtx.App.ValidatorInstance.Struct(v)
}

func (eCtx *EndpointContext) GetFilterParam() (*database.FilterBuilder, error) {
	if filter, ok := eCtx.ParsedQuery["filter"]; ok {
		if filter == nil {
			return nil, nil // No filter provided, return nil
		}
		if filterBuilder, ok := filter.(*database.FilterBuilder); ok {
			return filterBuilder, nil
		}
		return nil, NewErrorResponse(400, "Invalid filter parameter", "Filter must be a valid FilterBuilder")
	}

	if filter, ok := eCtx.ParsedHeader["filter"]; ok {
		if filterBuilder, ok := filter.(*database.FilterBuilder); ok {
			return filterBuilder, nil
		}
		return nil, NewErrorResponse(400, "Invalid filter header", "Filter must be a valid FilterBuilder")
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
func (ctx *EndpointContext) RespondAndLog(response any, affectedModelId any, contentType ResponseType, statuCode ...uint8) error {
	if !ctx.Endpoint.AuditDisabled {
		if ctx.Endpoint.app.auditLogConfig.Enabled && ctx.Endpoint.app.auditLogConfig.Handler != nil {
			err := ctx.Endpoint.app.auditLogConfig.Handler(ctx, response, affectedModelId)
			if err != nil {
				ctx.App.Errorf("Failed to log audit:", err)
			}
		}
	}

	status := fiber.StatusOK
	if len(statuCode) > 0 {
		status = int(statuCode[0])
	}

	switch contentType {
	case ResponseTypeJSON:
		return ctx.FiberCtx.Status(status).JSON(response)
	case ResponseTypeXML:
		return ctx.FiberCtx.Status(status).XML(response)
	case ResponseTypeText:
		if str, ok := response.(string); ok {
			return ctx.FiberCtx.Status(status).SendString(str)
		}
		return fiber.NewError(fiber.StatusInternalServerError, "text response must be string")
	case ResponseTypeHTML:
		if str, ok := response.(string); ok {
			return ctx.FiberCtx.Status(status).Type("html").SendString(str)
		}
		return fiber.NewError(fiber.StatusInternalServerError, "html response must be string")
	case ResponseTypeNoContent:
		return ctx.FiberCtx.SendStatus(fiber.StatusNoContent)

	default:
		return fiber.NewError(fiber.StatusNotAcceptable, "unsupported content type")
	}
}

func (ctx *EndpointContext) JSON(response any, statusCode ...uint8) error {
	status := fiber.StatusOK
	if len(statusCode) > 0 {
		status = int(statusCode[0])
	}

	return ctx.FiberCtx.Status(status).JSON(response)
}

func (ctx *EndpointContext) XML(response any, statusCode ...uint8) error {
	status := fiber.StatusOK
	if len(statusCode) > 0 {
		status = int(statusCode[0])
	}

	return ctx.FiberCtx.Status(status).XML(response)
}

func (ctx *EndpointContext) Text(response string, statusCode ...uint8) error {
	status := fiber.StatusOK
	if len(statusCode) > 0 {
		status = int(statusCode[0])
	}

	return ctx.FiberCtx.Status(status).SendString(response)
}

func (ctx *EndpointContext) HTML(response string, statusCode ...uint8) error {
	status := fiber.StatusOK
	if len(statusCode) > 0 {
		status = int(statusCode[0])
	}

	return ctx.FiberCtx.Status(status).Type("html").SendString(response)
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

type Param struct {
	in        ParamLocation
	name      string
	paramType string
	required  bool
	Parser    func(string) (any, error)
}

func NewQueryParam(name string, paramType QueryParamType, required ...bool) Param {
	requiredValue := false
	if len(required) > 0 {
		requiredValue = required[0]
	}
	return Param{
		in:        InQuery,
		name:      name,
		paramType: string(paramType),
		required:  requiredValue,
	}
}

func NewPathParam(name string, paramType PathParamType, required ...bool) Param {
	requiredValue := false
	if len(required) > 0 {
		requiredValue = required[0]
	}
	return Param{
		in:        InPath,
		name:      name,
		paramType: string(paramType),
		required:  requiredValue,
	}
}

func NewHeaderParam(name string, paramType HeaderParamType, required ...bool) Param {
	requiredValue := false
	if len(required) > 0 {
		requiredValue = required[0]
	}
	return Param{
		in:        InHeader,
		name:      name,
		paramType: string(paramType),
		required:  requiredValue,
	}
}

type Endpoint struct {
	Name            string
	Method          EndpointMethod
	Path            string
	Handler         func(c *EndpointContext) error
	Disabled        bool             // If true, the endpoint is disabled and will not be registered or accessible.
	BodyParams      func() Validable // Function that returns a Validable struct for body validation.
	Scope           string
	RateLimiter     func(*EndpointContext) RateLimit // Function to get rate limit configuration for the endpoint.
	Public          bool                             // If true, the endpoint is publicly accessible without authentication.
	Roles           []EndpointRole                   // List of roles that can access this endpoint.
	AllowedIncludes map[EndpointRole][]string
	ActionType      string // e.g., "create", "read", "update", "delete". Used for logging.
	Model           string // The related model or resource, e.g., "User", "Order", etc. Used for logging
	app             *RestApp
	Accepts         []Param
	AuditDisabled   bool           // Disable audit logging for this endpoint
	MetaData        map[string]any // Additional metadata for the endpoint

	// File upload configuration
	FileUploadConfig  *FileUploadConfig           // Global file upload settings for this endpoint
	fileUploadHandler *StreamingFileUploadHandler // Internal file upload handler (legacy)
}

func (ep *Endpoint) run(c *fiber.Ctx) error {
	if ep.Disabled {
		return NewErrorResponse(fiber.StatusNotFound, "Endpoint not found")
	}

	ctx := &EndpointContext{
		FiberCtx:  c,
		Endpoint:  ep,
		App:       ep.app,
		IpAddress: c.IP() + "",
	}

	err := parseBody(ep, ctx)
	if err != nil {
		return err
	}

	uploadedFiles, err := ep.fileUploadHandler.ProcessStreamingFileUploads(c)
	if err != nil {
		return err
	}
	ctx.UploadedFiles = uploadedFiles

	// Setup cleanup after response if configured
	if ep.FileUploadConfig != nil && !ep.FileUploadConfig.KeepFilesAfterSend {
		defer ep.fileUploadHandler.CleanupAfterResponse(uploadedFiles)
	}

	err = parseAllParams(ep, ctx)
	if err != nil {
		return err
	}

	_, err = ctx.GetFilterParam()
	if err != nil {
		return err
	}

	err = ep.app.Authorize(ctx)
	if err != nil {
		return err
	}

	// TODO: validate includes
	/* if !helpers.ValidateInclude(ctx.Filter, ep.AllowedIncludes[ctx.Role]) {
		return ErrorResponse{
			Code: 401,
		}
	} */

	// TODO: Implement rate limiting

	err = checkRateLimit(ctx)
	if err != nil {
		return err
	}

	if err := ep.Handler(ctx); err != nil {
		return err
	}

	return nil
}
