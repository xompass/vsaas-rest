package rest

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/redis/go-redis/v9"
	"github.com/xompass/vsaas-rest/database"
)

type LogLevel uint8

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

var LogLevelLabels = map[LogLevel]string{
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO",
	LogLevelWarn:  "WARN",
	LogLevelError: "ERROR",
}

type AuditLogConfig struct {
	Enabled bool
	Handler func(ctx *EndpointContext, response any, affectedModelId any) error
}

type RestAppOptions struct {
	Name              string
	Port              uint16
	Datasource        *database.Datasource
	FiberConfig       *fiber.Config
	LogLevel          LogLevel
	EnableRateLimiter bool
	Authorizer        Authorizer
	AuditLogConfig    *AuditLogConfig
}

type RestApp struct {
	FiberApp          *fiber.App
	Datasource        *database.Datasource
	redisClient       *redis.Client
	options           RestAppOptions
	ValidatorInstance *validator.Validate
	environment       string
	authorizer        Authorizer
	auditLogConfig    AuditLogConfig
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
	if receiver == nil || receiver.options.LogLevel > level {
		return
	}

	label, exists := LogLevelLabels[level]
	if !exists {
		label = "UNKNOWN"
	}

	args = append([]any{label}, args...)

	log.Printf("[%s] "+format, args...)
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
	f := NewFiberApp(appOptions.FiberConfig)

	validate := validator.New()

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

	app := &RestApp{
		FiberApp:          f,
		Datasource:        appOptions.Datasource,
		options:           appOptions,
		ValidatorInstance: validate,
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

	// cors
	f.Use(cors.New())

	f.Use(etag.New())

	f.Use(helmet.New())

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

func (receiver *RestApp) Start() error {
	return receiver.FiberApp.Listen(fmt.Sprint(":", receiver.options.Port))
}

func (receiver *RestApp) Group(path string, m ...fiber.Handler) fiber.Router {
	return receiver.FiberApp.Group(path, m...)
}

func (receiver *RestApp) RegisterEndpoint(ep *Endpoint, r ...fiber.Router) {
	if ep == nil {
		return
	}

	var router fiber.Router

	if len(r) > 0 {
		if len(r) > 1 {
			panic("Only one router can be passed to RegisterEndpoint")
		}
		router = r[0]
	}

	if router == nil {
		router = receiver.FiberApp
	}

	if ep.FileUploadConfig != nil {
		ep.fileUploadHandler = NewStreamingFileUploadHandler(ep.FileUploadConfig)
	}

	var executor func(path string, handlers ...fiber.Handler) fiber.Router
	switch ep.Method {
	case MethodGET:
		executor = router.Get
	case MethodHEAD:
		executor = router.Head
	case MethodPOST:
		executor = router.Post
	case MethodPUT:
		executor = router.Put
	case MethodPATCH:
		executor = router.Patch
	case MethodDELETE:
		executor = router.Delete
	}

	if executor != nil {
		ep.app = receiver
		name := ""
		if ep.Name != "" {
			name = ep.Name
		}

		executor(ep.Path, ep.run).Name(name)
	}
}

func (receiver *RestApp) RegisterEndpoints(endpoints []*Endpoint, r fiber.Router) {
	for _, ep := range endpoints {
		if ep == nil {
			continue
		}
		receiver.RegisterEndpoint(ep, r)
	}
}
