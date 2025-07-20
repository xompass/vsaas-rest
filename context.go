package rest

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Context represents a generic HTTP request/response context
type Context interface {
	Request() *http.Request
	Response() http.ResponseWriter
	Param(name string) string
	Query(name string) string
	Body() ([]byte, error)
	JSON(code int, i any) error
	String(code int, s string) error
	Bind(i any) error
}

// HandlerFunc represents a generic HTTP handler function
type HandlerFunc func(Context) error

// MiddlewareFunc represents a generic middleware function
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// RouterGroup wraps framework-specific router groups to provide a generic interface
type RouterGroup struct {
	echoGroup *echo.Group
}

// EchoContext wraps echo.Context to implement our generic Context interface
type EchoContext struct {
	echo.Context
}

func (ec *EchoContext) Request() *http.Request {
	return ec.Context.Request()
}

func (ec *EchoContext) Response() http.ResponseWriter {
	return ec.Context.Response().Writer
}

func (ec *EchoContext) Param(name string) string {
	return ec.Context.Param(name)
}

func (ec *EchoContext) Query(name string) string {
	return ec.Context.QueryParam(name)
}

func (ec *EchoContext) Body() ([]byte, error) {
	req := ec.Context.Request()
	return io.ReadAll(req.Body)
}

func (ec *EchoContext) JSON(code int, i any) error {
	return ec.Context.JSON(code, i)
}

func (ec *EchoContext) String(code int, s string) error {
	return ec.Context.String(code, s)
}

func (ec *EchoContext) Bind(i any) error {
	return ec.Context.Bind(i)
}

// convertMiddleware converts our generic middleware to Echo middleware
func convertMiddleware(mw MiddlewareFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			wrappedNext := func(ctx Context) error {
				return next(c)
			}
			wrappedHandler := mw(wrappedNext)
			return wrappedHandler(&EchoContext{c})
		}
	}
}
