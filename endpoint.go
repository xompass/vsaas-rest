package rest

import (
	"time"

	"github.com/labstack/echo/v4"
)

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
	FileUploadConfig      *FileUploadConfig      // Global file upload settings for this endpoint
	echoFileUploadHandler *EchoFileUploadHandler // Internal file upload handler for Echo
}

func (ep *Endpoint) run(c echo.Context) error {
	if ep.Disabled {
		return NewErrorResponse(404, "Endpoint not found")
	}

	ctx := &EndpointContext{
		EchoCtx:   c,
		Endpoint:  ep,
		App:       ep.app,
		IpAddress: c.RealIP(),
	}

	err := parseBody(ep, ctx)
	if err != nil {
		return err
	}

	// Process file uploads only if the endpoint has file upload configuration
	var uploadedFiles map[string][]*UploadedFile
	if ep.FileUploadConfig != nil && ep.echoFileUploadHandler != nil {
		uploadedFiles, err = ep.echoFileUploadHandler.ProcessStreamingFileUploads(c)
		if err != nil {
			return err
		}
		ctx.UploadedFiles = uploadedFiles

		// Setup cleanup after response if configured
		if !ep.FileUploadConfig.KeepFilesAfterSend {
			defer ep.echoFileUploadHandler.CleanupAfterResponse(uploadedFiles)
		}
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
