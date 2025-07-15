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

func NewEchoApp() *echo.Echo {
	app := echo.New()
	app.Use(middleware.Recover())
	app.Use(middleware.CORS())
	app.Use(middleware.Secure())

	app.JSONSerializer = fj4echo.New()

	isProduction := os.Getenv("APP_ENV") == "production"

	app.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		// Log the full error internally
		if e, ok := err.(*errors.Error); ok {
			log.Printf("Unhandled error: %s\n%s", e.Error(), e.ErrorStack())
		} else {
			log.Printf("Unhandled error: %s", err.Error())
		}

		code := http.StatusInternalServerError
		responseError := &http_errors.ErrorResponse{
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
		case *http_errors.ErrorResponse:
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
