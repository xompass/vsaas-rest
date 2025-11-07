package rest

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// Test struct similar to UploadStepFileRequest
type TestUploadRequest struct {
	Description string                 `json:"description,omitempty" validate:"omitempty,max=500"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Count       int                    `json:"count,omitempty"`
	IsEnabled   bool                   `json:"is_enabled,omitempty"`
}

func TestBindFormToStruct_MultipartWithFormData(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		expectDescription string
		expectError       bool
	}{
		{
			name:              "with_description",
			description:       "Test file description",
			expectDescription: "Test file description",
			expectError:       false,
		},
		{
			name:              "empty_description",
			description:       "",
			expectDescription: "",
			expectError:       false,
		},
		{
			name:              "no_description_field",
			description:       "", // Will not be included in form
			expectDescription: "",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create multipart form data
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Add a test file
			fileWriter, err := writer.CreateFormFile("images", "test.jpg")
			assert.NoError(t, err)
			_, err = fileWriter.Write([]byte("fake image content"))
			assert.NoError(t, err)

			// Add description field if not testing the "no_description_field" case
			if tt.name != "no_description_field" {
				err = writer.WriteField("description", tt.description)
				assert.NoError(t, err)
			}

			err = writer.Close()
			assert.NoError(t, err)

			// Create HTTP request
			req := httptest.NewRequest(http.MethodPost, "/test", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			// Create Echo context
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Create endpoint context
			ec := &EndpointContext{
				EchoCtx: c,
			}

			// Simulate file upload processing (like ProcessStreamingFileUploads does)
			formValues := make(map[string][]string)
			if tt.name != "no_description_field" {
				formValues["description"] = []string{tt.description}
			}
			ec.FormValues = formValues
			ec.UploadedFiles = map[string][]*UploadedFile{
				"images": {
					{
						FieldName: "images",
						Filename:  "test.jpg",
						Size:      int64(len("fake image content")),
						MimeType:  "image/jpeg",
						TempPath:  "/tmp/test.jpg",
					},
				},
			}

			// Test binding
			target := &TestUploadRequest{}
			err = bindFormToStruct(ec, target)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectDescription, target.Description)
			}
		})
	}
}

func TestBindFormToStruct_ComplexDataTypes(t *testing.T) {
	// Test complex data types including JSON strings in form fields
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add a test file
	fileWriter, err := writer.CreateFormFile("images", "test.jpg")
	assert.NoError(t, err)
	_, err = fileWriter.Write([]byte("fake image content"))
	assert.NoError(t, err)

	// Add simple fields
	err = writer.WriteField("description", "Test description")
	assert.NoError(t, err)
	err = writer.WriteField("count", "42")
	assert.NoError(t, err)
	err = writer.WriteField("is_enabled", "true")
	assert.NoError(t, err)

	// Add JSON as string (common in multipart forms)
	metadataJSON := `{"key1": "value1", "key2": 123, "nested": {"subkey": "subvalue"}}`
	err = writer.WriteField("metadata", metadataJSON)
	assert.NoError(t, err)

	// Add array as multiple values
	err = writer.WriteField("tags", "tag1")
	assert.NoError(t, err)
	err = writer.WriteField("tags", "tag2")
	assert.NoError(t, err)
	err = writer.WriteField("tags", "tag3")
	assert.NoError(t, err)

	err = writer.Close()
	assert.NoError(t, err)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodPost, "/test", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Create Echo context
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Create endpoint context
	ec := &EndpointContext{
		EchoCtx: c,
	}

	// Simulate file upload processing with captured form values
	formValues := map[string][]string{
		"description": {"Test description"},
		"count":       {"42"},
		"is_enabled":  {"true"},
		"metadata":    {metadataJSON},
		"tags":        {"tag1", "tag2", "tag3"},
	}
	ec.FormValues = formValues
	ec.UploadedFiles = map[string][]*UploadedFile{
		"images": {
			{
				FieldName: "images",
				Filename:  "test.jpg",
				Size:      int64(len("fake image content")),
				MimeType:  "image/jpeg",
				TempPath:  "/tmp/test.jpg",
			},
		},
	}

	// Test binding
	target := &TestUploadRequest{}
	err = bindFormToStruct(ec, target)

	assert.NoError(t, err)
	assert.Equal(t, "Test description", target.Description)
	assert.Equal(t, 42, target.Count)
	assert.Equal(t, true, target.IsEnabled)

	// Verify JSON was parsed correctly
	assert.NotNil(t, target.Metadata)
	assert.Equal(t, "value1", target.Metadata["key1"])
	assert.Equal(t, float64(123), target.Metadata["key2"]) // JSON numbers become float64

	// Verify nested JSON
	nested, ok := target.Metadata["nested"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "subvalue", nested["subkey"])

	// Verify array was parsed correctly
	assert.Equal(t, []string{"tag1", "tag2", "tag3"}, target.Tags)
}

func TestBindFormValuesToStruct_VariousTypes(t *testing.T) {
	type TestStruct struct {
		Name        string  `json:"name"`
		Age         int     `json:"age"`
		Price       float64 `json:"price"`
		IsActive    bool    `json:"is_active"`
		Description string  `json:"description,omitempty"`
	}

	formValues := map[string][]string{
		"name":      {"John Doe"},
		"age":       {"30"},
		"price":     {"99.99"},
		"is_active": {"true"},
		// description is omitted to test omitempty behavior
	}

	// Create mock endpoint context
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	ec := &EndpointContext{
		EchoCtx:    c,
		FormValues: formValues,
	}

	target := &TestStruct{}
	err := bindMultipartFormValues(ec, target)

	assert.NoError(t, err)
	assert.Equal(t, "John Doe", target.Name)
	assert.Equal(t, 30, target.Age)
	assert.Equal(t, 99.99, target.Price)
	assert.Equal(t, true, target.IsActive)
	assert.Equal(t, "", target.Description) // Should remain empty
}

func TestBindFormValuesToStruct_ErrorCases(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	ec := &EndpointContext{
		EchoCtx:    c,
		FormValues: map[string][]string{"test": {"value"}},
	}

	tests := []struct {
		name      string
		target    any
		expectErr bool
	}{
		{
			name:      "nil_target",
			target:    nil,
			expectErr: false, // Should not error, just return nil
		},
		{
			name:      "valid_struct_pointer",
			target:    &TestUploadRequest{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bindMultipartFormValues(ec, tt.target)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
