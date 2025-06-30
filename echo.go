package rest

import (
	"github.com/karagenc/fj4echo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func NewEchoApp() *echo.Echo {
	app := echo.New()
	app.Use(middleware.Recover())
	app.Use(middleware.CORS())
	app.Use(middleware.Secure())

	app.JSONSerializer = fj4echo.New()

	/* app.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		code := http.StatusInternalServerError
		responseError := &ErrorResponse{
			Code:    code,
			Message: "Internal Server Error",
		}

		switch e := err.(type) {
		case *echo.HTTPError:
			code = e.Code
			responseError = NewErrorResponse(code, e.Error())
		case *ErrorResponse:
			responseError = e
			code = e.Code
		case *errors.Error:
			responseError = NewErrorResponse(http.StatusInternalServerError, e.Error(), e.ErrorStack())
			code = http.StatusInternalServerError
		default:
			if err.Error() != "" {
				responseError.Message = err.Error()
			}

		}

		c.JSON(code, responseError)
	} */

	return app
}
