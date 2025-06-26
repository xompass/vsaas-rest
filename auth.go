package rest

type Principal interface {
	GetPrincipalID() string
	GetPrincipalRole() string
	HasPrincipalPermission(permission string) bool
}

type Authorizer func(*EndpointContext) (Principal, AuthToken, error)

type AuthToken interface {
	IsValid() bool
	GetUserId() string
	GetUserType() string
	GetToken() string
	GetExpiresAt() int64
}
