package rest

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/xompass/vsaas-rest/database"
)

type AuditLogConfig struct {
	Enabled bool
	Handler func(ctx *EndpointContext, response any, affectedModelId any) error
}

type RestAppOptions struct {
	Name              string
	Port              uint16
	Datasource        *database.Datasource
	LogLevel          LogLevel
	EnableRateLimiter bool
	Authorizer        Authorizer
	AuditLogConfig    *AuditLogConfig
}

type RestApp struct {
	EchoApp           *echo.Echo
	Datasource        *database.Datasource
	redisClient       *redis.Client
	options           RestAppOptions
	ValidatorInstance *validator.Validate
	environment       string
	authorizer        Authorizer
	auditLogConfig    AuditLogConfig
	logger            *slog.Logger
}

func (receiver *RestApp) GetEnvironment() string {
	if receiver.environment == "" {
		env, ok := os.LookupEnv("APP_ENV")
		if !ok {
			env = "development"
		}
		receiver.environment = strings.ToLower(env)
	}

	return receiver.environment
}

func (receiver *RestApp) Debugf(format string, args ...any) {
	receiver.log(LogLevelDebug, format, args...)
}

func (receiver *RestApp) Infof(format string, args ...any) {
	receiver.log(LogLevelInfo, format, args...)
}

func (receiver *RestApp) Warnf(format string, args ...any) {
	receiver.log(LogLevelWarn, format, args...)
}

func (receiver *RestApp) Errorf(format string, args ...any) {
	receiver.log(LogLevelError, format, args...)
}

func (receiver *RestApp) log(level LogLevel, format string, args ...any) {
	if receiver == nil || receiver.logger == nil || receiver.options.LogLevel > level {
		return
	}

	var slogLevel slog.Level
	switch level {
	case LogLevelDebug:
		slogLevel = slog.LevelDebug
	case LogLevelInfo:
		slogLevel = slog.LevelInfo
	case LogLevelWarn:
		slogLevel = slog.LevelWarn
	case LogLevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	message := fmt.Sprintf(format, args...)
	receiver.logger.Log(context.Background(), slogLevel, message)
}

func (receiver *RestApp) Authorize(ctx *EndpointContext) error {
	if receiver.authorizer == nil {
		receiver.Warnf("No authorizer configured for the application")
		return nil
	}
	principal, token, err := receiver.authorizer(ctx)
	if err != nil {
		receiver.Errorf("Authorization error: %v", err)
		return err
	}
	if principal == nil {
		return nil
	}

	ctx.Principal = principal
	ctx.Token = token
	return nil
}

func NewRestApp(appOptions RestAppOptions) *RestApp {
	e := NewEchoApp()

	validate := validator.New()
	registerTagNameFunc(validate)

	app := &RestApp{
		EchoApp:           e,
		Datasource:        appOptions.Datasource,
		options:           appOptions,
		ValidatorInstance: validate,
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.Level(appOptions.LogLevel),
		})),
	}

	if appOptions.Authorizer != nil {
		app.authorizer = appOptions.Authorizer
	}

	if appOptions.EnableRateLimiter {
		app.redisClient = newRedisClient()
	}

	if appOptions.AuditLogConfig != nil {
		app.auditLogConfig = *appOptions.AuditLogConfig
	}

	return app
}

func (receiver *RestApp) Destroy() error {
	if receiver == nil {
		return nil
	}
	if receiver.Datasource != nil {
		receiver.Datasource.Destroy()
	}

	if receiver.redisClient != nil {
		receiver.redisClient.Close()
	}

	return nil
}

func (receiver *RestApp) Test(req *http.Request, timeout ...int) (*http.Response, error) {
	return nil, nil
}

func (receiver *RestApp) Start() error {
	return receiver.EchoApp.Start(fmt.Sprint(":", receiver.options.Port))
}

func (receiver *RestApp) Group(path string, m ...MiddlewareFunc) *RouterGroup {
	g := receiver.EchoApp.Group(path)
	for _, handler := range m {
		g.Use(convertMiddleware(handler))
	}
	return &RouterGroup{echoGroup: g}
}

func (receiver *RestApp) RegisterEndpoint(ep *Endpoint, r *RouterGroup) {
	if ep == nil {
		return
	}

	var router *echo.Group = r.echoGroup

	if ep.FileUploadConfig != nil {
		ep.echoFileUploadHandler = NewEchoFileUploadHandler(ep.FileUploadConfig)
	}

	var executor func(path string, handler echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	switch ep.Method {
	case MethodGET:
		executor = router.GET
	case MethodHEAD:
		executor = router.HEAD
	case MethodPOST:
		executor = router.POST
	case MethodPUT:
		executor = router.PUT
	case MethodPATCH:
		executor = router.PATCH
	case MethodDELETE:
		executor = router.DELETE
	}

	if executor != nil {
		ep.app = receiver

		if ep.Method == MethodPOST || ep.Method == MethodPUT || ep.Method == MethodPATCH {
			if ep.BodyParams != nil {
				registerStruct(ep.BodyParams())
			}
		}

		executor(ep.Path, ep.run)
	} else {
		log.Fatalf("Unsupported HTTP method %s for endpoint %s", ep.Method, ep.Name)
		return
	}
}

func (receiver *RestApp) RegisterEndpoints(endpoints []*Endpoint, r *RouterGroup) {
	for _, ep := range endpoints {
		if ep == nil {
			continue
		}
		receiver.RegisterEndpoint(ep, r)
	}
}

// Group creates a subgroup with the specified path and middleware
func (rg *RouterGroup) Group(path string, m ...MiddlewareFunc) *RouterGroup {
	g := rg.echoGroup.Group(path)
	for _, handler := range m {
		g.Use(convertMiddleware(handler))
	}
	return &RouterGroup{echoGroup: g}
}

// Use adds middleware to the router group
func (rg *RouterGroup) Use(m ...MiddlewareFunc) {
	for _, middleware := range m {
		rg.echoGroup.Use(convertMiddleware(middleware))
	}
}

// registerTagNameFunc sets the tag name function for the validator
// to use the "json" tag for field names in validation errors.
func registerTagNameFunc(validate *validator.Validate) {
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		parts := strings.SplitN(fld.Tag.Get("json"), ",", 2)
		if len(parts) == 0 {
			return fld.Name
		}
		name := parts[0]
		if name == "-" {
			return ""
		}
		return name
	})
}
