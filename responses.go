package rest

import "github.com/xompass/vsaas-rest/http_errors"

type Count struct {
	Count int64 `json:"count"`
} // @name CountResponse

type Exists struct {
	Exists bool `json:"exists"`
} // @name ExistsResponse

// Deprecated: Use http_errors.ErrorResponse instead
func NewErrorResponse(code int, message string, details ...any) http_errors.ErrorResponse {
	return http_errors.NewErrorResponse(code, message, details...)
}
