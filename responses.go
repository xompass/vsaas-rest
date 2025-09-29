package rest

type Count struct {
	Count int64 `json:"count"`
} // @name CountResponse

type Exists struct {
	Exists bool `json:"exists"`
} // @name ExistsResponse
