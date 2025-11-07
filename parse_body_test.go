package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xompass/vsaas-rest/http_errors"
)

// Test helper to create a complete REST app for testing
func createTestApp() *RestApp {
	validate := validator.New()
	registerTagNameFunc(validate)

	return &RestApp{
		EchoApp:           echo.New(),
		ValidatorInstance: validate,
	}
}

// Test helper to create an endpoint with body params
func createTestEndpoint(bodyParamsFunc func() any) *Endpoint {
	return &Endpoint{
		Name:       "test",
		Method:     MethodPOST,
		Path:       "/test",
		BodyParams: bodyParamsFunc,
		Handler: func(c *EndpointContext) error {
			return c.JSON(map[string]string{"status": "ok"})
		},
	}
}

// Test request body struct
type TestRequestBody struct {
	Name     string `json:"name" normalize:"trim,lowercase" validate:"required,min=2"`
	Email    string `json:"email" sanitize:"html" normalize:"trim" validate:"required,email"`
	Age      int    `json:"age" validate:"gte=0,lte=150"`
	Website  string `json:"website" sanitize:"html"`
	Username string `json:"username" sanitize:"alphanumeric" normalize:"lowercase" validate:"required,min=3"`
}

type ComplexRequestBody struct {
	User    TestRequestBody   `json:"user" sanitize:"dive" normalize:"dive"`
	Tags    []string          `json:"tags" normalize:"dive,trim"`
	Configs map[string]string `json:"configs" sanitize:"dive,html"`
}

type InterfaceRequestBody struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (irb *InterfaceRequestBody) Normalize(ctx *EndpointContext) error {
	irb.Name = "norm_" + irb.Name
	return nil
}

func (irb *InterfaceRequestBody) Sanitize(ctx *EndpointContext) error {
	irb.Name = "clean_" + irb.Name
	return nil
}

func TestParseBody_Success(t *testing.T) {
	app := createTestApp()

	tests := []struct {
		name           string
		method         EndpointMethod
		bodyParamsFunc func() any
		requestBody    map[string]any
		expectedResult func(t *testing.T, parsedBody any)
	}{
		{
			name:   "simple normalization and sanitization",
			method: MethodPOST,
			bodyParamsFunc: func() any {
				return &TestRequestBody{}
			},
			requestBody: map[string]any{
				"name":     "  JOHN DOE  ",
				"email":    "  <script>alert('xss')</script>john@example.com  ",
				"age":      25,
				"website":  "<b>Bold</b> text with <script>bad</script>",
				"username": "User123!@#Name",
			},
			expectedResult: func(t *testing.T, parsedBody any) {
				body, ok := parsedBody.(*TestRequestBody)
				require.True(t, ok)
				assert.Equal(t, "john doe", body.Name)
				assert.Equal(t, "john@example.com", body.Email)
				assert.Equal(t, 25, body.Age)
				assert.Contains(t, body.Website, "Bold") // bluemonday should keep safe HTML
				assert.NotContains(t, body.Website, "script")
				assert.Equal(t, "user123name", body.Username) // alphanumeric + lowercase
			},
		},
		{
			name:   "complex nested structure",
			method: MethodPUT,
			bodyParamsFunc: func() any {
				return &ComplexRequestBody{}
			},
			requestBody: map[string]any{
				"user": map[string]any{
					"name":     "  JANE DOE  ",
					"email":    "  jane@example.com  ",
					"age":      30,
					"username": "Jane123!@#",
				},
				"tags": []string{"  golang  ", "  rest-api  "},
				"configs": map[string]string{
					"theme": "<script>alert('bad')</script>dark",
				},
			},
			expectedResult: func(t *testing.T, parsedBody any) {
				body, ok := parsedBody.(*ComplexRequestBody)
				require.True(t, ok)
				assert.Equal(t, "jane doe", body.User.Name)
				assert.Equal(t, "jane@example.com", body.User.Email)
				assert.Equal(t, "jane123", body.User.Username)
				assert.Equal(t, []string{"golang", "rest-api"}, body.Tags)
				assert.Equal(t, "dark", body.Configs["theme"]) // script should be removed
			},
		},
		{
			name:   "interface implementation",
			method: MethodPOST,
			bodyParamsFunc: func() any {
				return &InterfaceRequestBody{}
			},
			requestBody: map[string]any{
				"name": "test",
				"type": "user",
			},
			expectedResult: func(t *testing.T, parsedBody any) {
				body, ok := parsedBody.(*InterfaceRequestBody)
				require.True(t, ok)
				// Should have both sanitize and normalize from interface
				assert.Equal(t, "norm_clean_test", body.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body JSON
			bodyJSON, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			// Create HTTP request
			req := httptest.NewRequest(string(tt.method), "/test", bytes.NewReader(bodyJSON))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Create Echo context
			e := echo.New()
			c := e.NewContext(req, rec)

			// Create endpoint and context
			endpoint := createTestEndpoint(tt.bodyParamsFunc)
			endpoint.Method = tt.method
			endpoint.app = app

			ctx := &EndpointContext{
				App:     app,
				EchoCtx: c,
			}

			// Test parseBody
			err = parseBody(endpoint, ctx)
			require.NoError(t, err)

			// Verify results
			require.NotNil(t, ctx.ParsedBody)
			tt.expectedResult(t, ctx.ParsedBody)
		})
	}
}

func TestParseBody_ValidationErrors(t *testing.T) {
	app := createTestApp()

	tests := []struct {
		name        string
		requestBody map[string]any
		expectError bool
		errorCheck  func(t *testing.T, err error)
	}{
		{
			name: "required field missing",
			requestBody: map[string]any{
				"email":    "john@example.com",
				"age":      25,
				"username": "john123",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				responseError, ok := err.(http_errors.ErrorResponse)
				if !ok {
					t.Fatalf("Expected http_errors.ErrorResponse, got %T", err)
				}

				details, ok := responseError.Details.(map[string]string)
				if !ok {
					t.Fatalf("Expected details to be map[string]string, got %T", responseError.Details)
				}

				assert.NotNil(t, details["name"])
				assert.Contains(t, details["name"], "required")
			},
		},
		{
			name: "invalid email",
			requestBody: map[string]any{
				"name":     "John Doe",
				"email":    "invalid-email",
				"age":      25,
				"username": "john123",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				responseError, ok := err.(http_errors.ErrorResponse)
				if !ok {
					t.Fatalf("Expected http_errors.ErrorResponse, got %T", err)
				}
				details, ok := responseError.Details.(map[string]string)
				if !ok {
					t.Fatalf("Expected details to be map[string]string, got %T", responseError.Details)
				}
				assert.NotNil(t, details["email"])
				assert.Contains(t, details["email"], "valid email")
			},
		},
		{
			name: "age out of range",
			requestBody: map[string]any{
				"name":     "John Doe",
				"email":    "john@example.com",
				"age":      200, // Too old
				"username": "john123",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				responseError, ok := err.(http_errors.ErrorResponse)
				if !ok {
					t.Fatalf("Expected http_errors.ErrorResponse, got %T", err)
				}
				details, ok := responseError.Details.(map[string]string)
				if !ok {
					t.Fatalf("Expected details to be map[string]string, got %T", responseError.Details)
				}
				assert.NotNil(t, details["age"])
				assert.Contains(t, details["age"], "must be less than or equal")
			},
		},
		{
			name: "username too short after sanitization",
			requestBody: map[string]any{
				"name":     "John Doe",
				"email":    "john@example.com",
				"age":      25,
				"username": "j!@", // Will become "j" after alphanumeric sanitization
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				responseError, ok := err.(http_errors.ErrorResponse)
				if !ok {
					t.Fatalf("Expected http_errors.ErrorResponse, got %T", err)
				}
				details, ok := responseError.Details.(map[string]string)
				if !ok {
					t.Fatalf("Expected details to be map[string]string, got %T", responseError.Details)
				}
				assert.NotNil(t, details["username"])
				assert.Contains(t, details["username"], "greater than")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body JSON
			bodyJSON, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			// Create HTTP request
			req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyJSON))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Create Echo context
			e := echo.New()
			c := e.NewContext(req, rec)

			// Create endpoint and context
			endpoint := createTestEndpoint(func() any {
				return &TestRequestBody{}
			})
			endpoint.app = app

			ctx := &EndpointContext{
				App:     app,
				EchoCtx: c,
			}

			// Test parseBody
			err = parseBody(endpoint, ctx)

			if tt.expectError {
				require.Error(t, err)
				tt.errorCheck(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseBody_EdgeCases(t *testing.T) {
	app := createTestApp()

	t.Run("GET request - should skip body parsing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		endpoint := &Endpoint{
			Method: MethodGET,
			BodyParams: func() any {
				return &TestRequestBody{}
			},
		}

		ctx := &EndpointContext{
			App:     app,
			EchoCtx: c,
		}

		err := parseBody(endpoint, ctx)
		assert.NoError(t, err)
		assert.Nil(t, ctx.ParsedBody)
	})

	t.Run("no BodyParams function", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		endpoint := &Endpoint{
			Method:     MethodPOST,
			BodyParams: nil,
		}

		ctx := &EndpointContext{
			App:     app,
			EchoCtx: c,
		}

		err := parseBody(endpoint, ctx)
		assert.NoError(t, err)
		assert.Nil(t, ctx.ParsedBody)
	})

	t.Run("BodyParams returns nil", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		endpoint := createTestEndpoint(func() any {
			return nil
		})
		endpoint.app = app

		ctx := &EndpointContext{
			App:     app,
			EchoCtx: c,
		}

		err := parseBody(endpoint, ctx)
		assert.Error(t, err)
		respError, ok := err.(http_errors.ErrorResponse)
		if !ok {
			t.Fatalf("Expected http_errors.ErrorResponse, got %T", err)
		}

		// BadRequestError with only message doesn't set Details field, so check Message instead
		assert.Equal(t, "Request body cannot be nil", respError.Message)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		endpoint := createTestEndpoint(func() any {
			return &TestRequestBody{}
		})
		endpoint.app = app

		ctx := &EndpointContext{
			App:     app,
			EchoCtx: c,
		}

		err := parseBody(endpoint, ctx)
		assert.Error(t, err)

		respError, ok := err.(http_errors.ErrorResponse)
		if !ok {
			t.Fatalf("Expected http_errors.ErrorResponse, got %T", err)
		}
		details, ok := respError.Details.(string)
		if !ok {
			t.Fatalf("Expected details to be string, got %T", respError.Details)
		}

		assert.Contains(t, details, "Failed to bind request body")
	})
}

func TestParseBody_IntegrationWithEndpoint(t *testing.T) {
	app := createTestApp()

	// Test complete endpoint flow
	t.Run("complete endpoint execution", func(t *testing.T) {
		// Create endpoint
		endpoint := &Endpoint{
			Name:   "test-endpoint",
			Method: MethodPOST,
			Path:   "/test",
			BodyParams: func() any {
				return &TestRequestBody{}
			},
			Handler: func(c *EndpointContext) error {
				body, ok := c.ParsedBody.(*TestRequestBody)
				require.True(t, ok)

				// Verify processing occurred
				assert.Equal(t, "john doe", body.Name)      // normalized
				assert.NotContains(t, body.Email, "script") // sanitized

				return c.JSON(map[string]string{
					"processed_name":  body.Name,
					"processed_email": body.Email,
				})
			},
			app: app,
		}

		// Create request
		requestBody := map[string]any{
			"name":     "  JOHN DOE  ",
			"email":    "<script>alert('xss')</script>john@example.com",
			"age":      25,
			"username": "john123",
		}

		bodyJSON, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		e := echo.New()
		c := e.NewContext(req, rec)

		// Execute endpoint
		err = endpoint.run(c)
		require.NoError(t, err)

		// Verify response
		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]string
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "john doe", response["processed_name"])
		assert.Equal(t, "john@example.com", response["processed_email"])
	})
}
