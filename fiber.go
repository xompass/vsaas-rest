package rest

import (
	"os"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/go-errors/errors"

	"github.com/gofiber/fiber/v2"
)

// Este archivo configura y crea una nueva aplicación Fiber con varias características y middlewares.
// Incluye las siguientes funcionalidades principales:
// 1. Configuración de la aplicación Fiber, incluyendo el nombre de la aplicación y el encabezado de proxy.
// 2. Configuración de codificadores y decodificadores JSON utilizando la biblioteca Sonic.
// 3. Manejo de errores personalizados y generación de mensajes de error amigables.
// 4. Implementación de middlewares para favicon, recuperación de pánicos y compresión de respuestas.

func NewFiberApp(cfg ...*fiber.Config) *fiber.App {
	defaultConfig := getDefaultConfig()
	if len(cfg) > 0 && cfg[0] != nil {
		aux := cfg[0]
		defaultConfig = mergeConfigs(defaultConfig, *aux)
	}

	app := fiber.New(defaultConfig)

	/* // favicon
	app.Use(favicon.New())

	// Recover from panics
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))

	// Compress responses
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault,
	})) */

	return app
}

func mergeConfigs(defaultConfig, customConfig fiber.Config) fiber.Config {
	if customConfig.ProxyHeader == "" {
		customConfig.ProxyHeader = defaultConfig.ProxyHeader
	}

	if customConfig.TrustedProxies == nil {
		customConfig.TrustedProxies = defaultConfig.TrustedProxies
	}

	if customConfig.TrustedProxies != nil && !customConfig.EnableTrustedProxyCheck {
		customConfig.EnableTrustedProxyCheck = true
	}

	if customConfig.AppName == "" {
		customConfig.AppName = defaultConfig.AppName
	}

	if customConfig.JSONEncoder == nil {
		customConfig.JSONEncoder = defaultConfig.JSONEncoder
	}

	if customConfig.JSONDecoder == nil {
		customConfig.JSONDecoder = defaultConfig.JSONDecoder
	}

	if customConfig.ErrorHandler == nil {
		customConfig.ErrorHandler = defaultConfig.ErrorHandler
	}

	return customConfig
}

func getDefaultConfig() fiber.Config {
	defaultConfig := fiber.Config{
		AppName: "",
		JSONEncoder: func(v interface{}) ([]byte, error) {
			return sonic.Marshal(v)
		},
		JSONDecoder: func(data []byte, v interface{}) error {
			return sonic.Unmarshal(data, v)
		},
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			var er *ErrorResponse
			if errors.As(err, &er) {
				return ctx.Status(er.Code).JSON(er)
			}

			var fe *fiber.Error
			if errors.As(err, &fe) {
				return ctx.Status(fe.Code).JSON(NewErrorResponse(fe.Code, fe.Message))
			}

			var e *errors.Error
			if errors.As(err, &e) {
				return ctx.Status(fiber.StatusInternalServerError).JSON(NewErrorResponse(fiber.StatusInternalServerError, e.Error(), e.ErrorStack()))
			}

			code := fiber.StatusInternalServerError
			ce := NewErrorResponse(code, err.Error())
			return ctx.Status(code).JSON(ce)
		},
	}

	if value, exists := os.LookupEnv("REST_PROXY_HEADER"); exists {
		defaultConfig.ProxyHeader = value
	}

	if value, exists := os.LookupEnv("REST_TRUSTED_PROXIES"); exists {
		defaultConfig.TrustedProxies = strings.Split(value, ",")
		defaultConfig.EnableTrustedProxyCheck = true

		if defaultConfig.ProxyHeader == "" {
			defaultConfig.ProxyHeader = fiber.HeaderXForwardedFor
		}
	}

	return defaultConfig
}
