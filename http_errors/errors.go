package http_errors

type ErrorResponse struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
	ErrorCode  string `json:"errorCode"`
	Details    any    `json:"details,omitempty"` // Optional field for additional error details
} // @name ErrorResponse

func (e ErrorResponse) Error() string {
	return e.Message
}

func NewErrorResponse(statusCode int, errorCode string, message string, details ...any) ErrorResponse {
	if len(details) > 0 {
		return ErrorResponse{
			Message:    message,
			StatusCode: statusCode,
			ErrorCode:  errorCode,
			Details:    details[0], // Take the first detail if provided
		}
	}

	return ErrorResponse{
		Message:    message,
		StatusCode: statusCode,
		ErrorCode:  errorCode,
	}
}

func BadRequestError(message string, details ...any) ErrorResponse {
	return NewErrorResponse(400, "INVALID_REQUEST", message, details...)
}

func BadRequestErrorWithCode(errorCode string, message string, details ...any) ErrorResponse {
	return NewErrorResponse(400, errorCode, message, details...)
}

func UnauthorizedError(message string, details ...any) ErrorResponse {
	return NewErrorResponse(401, "UNAUTHORIZED", message, details...)
}

func UnauthorizedErrorWithCode(errorCode string, message string, details ...any) ErrorResponse {
	return NewErrorResponse(401, errorCode, message, details...)
}

func ForbiddenError(message string, details ...any) ErrorResponse {
	return NewErrorResponse(403, "FORBIDDEN", message, details...)
}

func ForbiddenErrorWithCode(errorCode string, message string, details ...any) ErrorResponse {
	return NewErrorResponse(403, errorCode, message, details...)
}

func NotFoundError(message string, details ...any) ErrorResponse {
	return NewErrorResponse(404, "NOT_FOUND", message, details...)
}

func NotFoundErrorWithCode(errorCode string, message string, details ...any) ErrorResponse {
	return NewErrorResponse(404, errorCode, message, details...)
}

func ConflictError(message string, details ...any) ErrorResponse {
	return NewErrorResponse(409, "CONFLICT", message, details...)
}

func ConflictErrorWithCode(errorCode string, message string, details ...any) ErrorResponse {
	return NewErrorResponse(409, errorCode, message, details...)
}

func UnprocessableEntityError(message string, details ...any) ErrorResponse {
	return NewErrorResponse(422, "UNPROCESSABLE_ENTITY", message, details...)
}

func UnprocessableEntityErrorWithCode(errorCode string, message string, details ...any) ErrorResponse {
	return NewErrorResponse(422, errorCode, message, details...)
}

func TooManyRequestsError(message string, details ...any) ErrorResponse {
	return NewErrorResponse(429, "TOO_MANY_REQUESTS", message, details...)
}

func TooManyRequestsErrorWithCode(errorCode string, message string, details ...any) ErrorResponse {
	return NewErrorResponse(429, errorCode, message, details...)
}

func InternalServerError(message string, details ...any) ErrorResponse {
	return NewErrorResponse(500, "INTERNAL_SERVER_ERROR", message, details...)
}

func InternalServerErrorWithCode(errorCode string, message string, details ...any) ErrorResponse {
	return NewErrorResponse(500, errorCode, message, details...)
}
