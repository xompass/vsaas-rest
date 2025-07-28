package rest

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/karagenc/fj4echo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/xompass/vsaas-rest/http_errors"
)

// EchoAppConfig contiene todas las configuraciones para el Echo app
type EchoAppConfig struct {
	CORS     *CORSConfig
	Security *SecurityConfig
	// Se pueden agregar más configuraciones aquí en el futuro
	// Rate limiting, compression, etc.
}

// CORSConfig configuración para CORS middleware
type CORSConfig struct {
	Enabled          bool
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	AllowOriginFunc  func(origin string) (bool, error)
	// Se pueden agregar más opciones de CORS según sea necesario
}

// SecurityConfig configuración para Security middleware
type SecurityConfig struct {
	Enabled bool
	// Se pueden agregar más opciones de seguridad según sea necesario
}

// DefaultEchoAppConfig retorna una configuración por defecto
func DefaultEchoAppConfig() EchoAppConfig {
	return EchoAppConfig{
		CORS: &CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowHeaders: []string{"*"},
		},
		Security: &SecurityConfig{
			Enabled: true,
		},
	}
}

func NewEchoApp(config ...EchoAppConfig) *echo.Echo {
	// Usar configuración por defecto si no se proporciona ninguna
	var appConfig EchoAppConfig
	if len(config) > 0 {
		appConfig = config[0]
	} else {
		appConfig = DefaultEchoAppConfig()
	}

	app := echo.New()
	app.Use(middleware.Recover())

	// Configurar CORS
	if appConfig.CORS != nil && appConfig.CORS.Enabled {
		corsConfig := middleware.CORSConfig{}

		if appConfig.CORS.AllowCredentials {
			corsConfig.AllowCredentials = true
		}

		if len(appConfig.CORS.AllowOrigins) > 0 {
			corsConfig.AllowOrigins = appConfig.CORS.AllowOrigins
		}

		if len(appConfig.CORS.AllowMethods) > 0 {
			corsConfig.AllowMethods = appConfig.CORS.AllowMethods
		}

		if len(appConfig.CORS.AllowHeaders) > 0 {
			corsConfig.AllowHeaders = appConfig.CORS.AllowHeaders
		}

		if appConfig.CORS.AllowOriginFunc != nil {
			corsConfig.AllowOriginFunc = appConfig.CORS.AllowOriginFunc
		}

		app.Use(middleware.CORSWithConfig(corsConfig))
	}

	// Configurar Security
	if appConfig.Security != nil && appConfig.Security.Enabled {
		app.Use(middleware.Secure())
	}

	app.JSONSerializer = fj4echo.New()

	isProduction := os.Getenv("APP_ENV") == "production"

	app.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		if err == nil {
			return
		}

		// Log the full error internally
		if e, ok := err.(*errors.Error); ok {
			log.Printf("Unhandled error: %s\n%s", e.Error(), e.ErrorStack())
		} else {
			log.Printf("Unhandled error: %s", err.Error())
		}

		code := http.StatusInternalServerError
		responseError := http_errors.ErrorResponse{
			Code:    code,
			Message: "Internal Server Error",
		}

		switch e := err.(type) {
		case *echo.HTTPError:
			code = e.Code

			if e.Message != nil {
				if str, ok := e.Message.(string); ok {
					responseError = http_errors.NewErrorResponse(code, str)
				} else if msg, ok := e.Message.(error); ok {
					responseError = http_errors.NewErrorResponse(code, msg.Error())
				} else {
					log.Printf("Unexpected HTTPError: %v", e.Message)
				}
			}
		case http_errors.ErrorResponse:
			responseError = e
			code = e.Code
		default:
			if !isProduction {
				if goErr, ok := e.(*errors.Error); ok {
					stack := strings.Split(strings.ReplaceAll(goErr.ErrorStack(), "\t", "    "), "\n")
					responseError = http_errors.NewErrorResponse(http.StatusInternalServerError, goErr.Error(), stack)
				} else {
					responseError = http_errors.NewErrorResponse(http.StatusInternalServerError, e.Error())
				}
			}
		}

		c.JSON(code, responseError)
	}

	return app
}
