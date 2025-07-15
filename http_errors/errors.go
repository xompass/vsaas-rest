package http_errors

type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
	Details any    `json:"details,omitempty"` // Optional field for additional error details
} // @name ErrorResponse

func (e *ErrorResponse) Error() string {
	return e.Message
}

func NewErrorResponse(code int, message string, details ...any) *ErrorResponse {
	if len(details) > 0 {
		return &ErrorResponse{
			Message: message,
			Code:    code,
			Details: details[0], // Take the first detail if provided
		}
	}

	return &ErrorResponse{
		Message: message,
		Code:    code,
	}
}

func BadRequestError(message string, details ...any) *ErrorResponse {
	return NewErrorResponse(400, message, details...)
}

func UnauthorizedError(message string, details ...any) *ErrorResponse {
	return NewErrorResponse(401, message, details...)
}

func ForbiddenError(message string, details ...any) *ErrorResponse {
	return NewErrorResponse(403, message, details...)
}

func NotFoundError(message string, details ...any) *ErrorResponse {
	return NewErrorResponse(404, message, details...)
}

func ConflictError(message string, details ...any) *ErrorResponse {
	return NewErrorResponse(409, message, details...)
}

func TooManyRequestsError(message string, details ...any) *ErrorResponse {
	return NewErrorResponse(429, message, details...)
}

func InternalServerError(message string, details ...any) *ErrorResponse {
	return NewErrorResponse(500, message, details...)
}
