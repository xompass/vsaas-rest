package rest

type Count struct {
	Count int64 `json:"count"`
} // @name CountResponse

type Exists struct {
	Exists bool `json:"exists"`
} // @name ExistsResponse

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
