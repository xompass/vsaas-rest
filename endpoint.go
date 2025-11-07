package rest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xompass/vsaas-rest/http_errors"
)

// ContentType defines the types of content an endpoint can accept
type ContentType string

const (
	ContentTypeJSON      ContentType = "application/json"
	ContentTypeMultipart ContentType = "multipart/form-data"
	ContentTypeFormData  ContentType = "application/x-www-form-urlencoded"
	ContentTypeAny       ContentType = "*/*" // Accept any content type
)

type RateLimit struct {
	Max    int
	Window time.Duration
	Key    string
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

func NewPathParam(name string, paramType PathParamType) Param {
	return Param{
		in:        InPath,
		name:      name,
		paramType: string(paramType),
		required:  true, // Path parameters are always required
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
	Disabled        bool       // If true, the endpoint is disabled and will not be registered or accessible.
	BodyParams      func() any // Function that returns a struct for body binding.
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
	Timeout         uint16         // Maximum timeout for the endpoint in seconds
	MetaData        map[string]any // Additional metadata for the endpoint

	// Content type configuration
	AcceptedContentTypes []ContentType // Explicitly define what content types this endpoint accepts

	// File upload configuration
	FileUploadConfig      *FileUploadConfig      // Global file upload settings for this endpoint
	echoFileUploadHandler *EchoFileUploadHandler // Internal file upload handler for Echo
}

func (ep *Endpoint) run(c echo.Context) error {
	if ep.Disabled {
		return http_errors.NotFoundError("Endpoint not found")
	}

	stdContext := c.Request().Context()
	if ep.Timeout > 0 {
		var cancel context.CancelFunc
		stdContext, cancel = context.WithTimeout(stdContext, time.Duration(ep.Timeout)*time.Second)
		defer cancel()
	}

	ctx := &EndpointContext{
		EchoCtx:   c,
		context:   stdContext,
		Endpoint:  ep,
		App:       ep.app,
		IpAddress: c.RealIP(),
	}

	// Validate Content-Type if endpoint has body parameters or file upload configuration
	if err := ep.validateContentType(c); err != nil {
		return err
	}

	// Process file uploads FIRST if the endpoint has file upload configuration
	// This prevents conflicts with body parsing when both BodyParams and FileUploadConfig are present
	var uploadedFiles map[string][]*UploadedFile
	if ep.FileUploadConfig != nil && ep.echoFileUploadHandler != nil {
		var err error
		var formValues map[string][]string
		uploadedFiles, formValues, err = ep.echoFileUploadHandler.ProcessStreamingFileUploads(c)
		if err != nil {
			return err
		}
		ctx.UploadedFiles = uploadedFiles
		ctx.FormValues = formValues

		// Setup cleanup after response if configured
		if !ep.FileUploadConfig.KeepFilesAfterSend {
			defer ep.echoFileUploadHandler.CleanupAfterResponse(uploadedFiles)
		}
	}

	err := parseBody(ep, ctx)
	if err != nil {
		return err
	}

	err = processBody(ctx)
	if err != nil {
		return err
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

// validateContentType validates that the request's Content-Type is acceptable for this endpoint
func (ep *Endpoint) validateContentType(c echo.Context) error {
	// Only validate for methods that typically have a body
	if ep.Method != MethodPOST && ep.Method != MethodPUT && ep.Method != MethodPATCH {
		return nil
	}

	// Skip validation if endpoint doesn't expect any body content
	if ep.BodyParams == nil && ep.FileUploadConfig == nil {
		return nil
	}

	requestContentType := c.Request().Header.Get("Content-Type")
	if requestContentType == "" {
		// Allow empty content type for endpoints that have optional body params
		if ep.BodyParams != nil && ep.FileUploadConfig == nil {
			return nil // JSON endpoints can work without explicit Content-Type
		}
		return http_errors.BadRequestError("Content-Type header is required")
	}

	acceptedTypes := ep.getAcceptedContentTypes()

	// Check if the request content type matches any accepted type
	for _, acceptedType := range acceptedTypes {
		if acceptedType == ContentTypeAny {
			return nil // Accept any content type
		}

		// Check for exact match or prefix match (e.g., "multipart/form-data; boundary=...")
		if requestContentType == string(acceptedType) ||
			(acceptedType == ContentTypeMultipart && strings.HasPrefix(requestContentType, "multipart/")) ||
			(acceptedType == ContentTypeJSON && strings.HasPrefix(requestContentType, "application/json")) {
			return nil
		}
	}

	// Build error message with accepted types
	var acceptedStrings []string
	for _, acceptedType := range acceptedTypes {
		acceptedStrings = append(acceptedStrings, string(acceptedType))
	}

	return http_errors.BadRequestErrorWithCode("UNSUPPORTED_CONTENT_TYPE",
		fmt.Sprintf("Content-Type '%s' is not supported. Accepted types: %s",
			requestContentType, strings.Join(acceptedStrings, ", ")))
}

// getAcceptedContentTypes returns the content types this endpoint accepts
// If not explicitly set, it determines them based on endpoint configuration
func (ep *Endpoint) getAcceptedContentTypes() []ContentType {
	// If explicitly set, use those
	if len(ep.AcceptedContentTypes) > 0 {
		return ep.AcceptedContentTypes
	}

	// Auto-determine based on endpoint configuration
	var types []ContentType

	// If has file upload config, accept multipart (prioritize file uploads)
	if ep.FileUploadConfig != nil {
		types = append(types, ContentTypeMultipart)
	} else if ep.BodyParams != nil {
		// If has body params but no file config, accept JSON and form data
		types = append(types, ContentTypeJSON)
		// Also accept form data for endpoints with body params
		types = append(types, ContentTypeFormData)
	}

	// Default to JSON if nothing else is configured but endpoint has body handling
	if len(types) == 0 && (ep.BodyParams != nil || ep.FileUploadConfig != nil) {
		types = append(types, ContentTypeJSON)
	}

	return types
}
