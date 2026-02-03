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

// TestRealWorldUploadScenario tests the exact scenario from UploadStepFile
func TestRealWorldUploadScenario(t *testing.T) {
	// Test struct that mirrors UploadStepFileRequest
	type UploadStepFileRequest struct {
		Description string         `json:"description,omitempty" validate:"omitempty,max=500"`
		Metadata    map[string]any `json:"metadata,omitempty"`
		Settings    []string       `json:"settings,omitempty"`
	}

	// Create multipart form data similar to real usage
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add test files
	fileWriter, err := writer.CreateFormFile("images", "evidence1.jpg")
	assert.NoError(t, err)
	_, err = fileWriter.Write([]byte("fake image 1 content"))
	assert.NoError(t, err)

	fileWriter2, err := writer.CreateFormFile("videos", "recording.mp4")
	assert.NoError(t, err)
	_, err = fileWriter2.Write([]byte("fake video content"))
	assert.NoError(t, err)

	// Add form fields
	err = writer.WriteField("description", "Incident evidence from patrol")
	assert.NoError(t, err)

	// Add complex JSON metadata (common in real usage)
	metadataJSON := `{
		"incident_id": "INC-2023-001",
		"location": {
			"latitude": 40.7128,
			"longitude": -74.0060,
			"address": "123 Main St, New York"
		},
		"timestamp": "2023-11-05T14:30:00Z",
		"officer": {
			"id": "OFF-001",
			"name": "John Doe"
		},
		"severity": 3,
		"tags": ["evidence", "patrol", "urgent"]
	}`
	err = writer.WriteField("metadata", metadataJSON)
	assert.NoError(t, err)

	// Add array settings
	err = writer.WriteField("settings", "auto_backup")
	assert.NoError(t, err)
	err = writer.WriteField("settings", "notify_supervisor")
	assert.NoError(t, err)
	err = writer.WriteField("settings", "gps_tag")
	assert.NoError(t, err)

	err = writer.Close()
	assert.NoError(t, err)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodPost, "/projects/123/active-procedures/456/steps/789/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Create Echo context
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Create endpoint context
	ec := &EndpointContext{
		EchoCtx: c,
	}

	// Simulate file upload processing - this would be done by ProcessStreamingFileUploads
	formValues := map[string][]string{
		"description": {"Incident evidence from patrol"},
		"metadata":    {metadataJSON},
		"settings":    {"auto_backup", "notify_supervisor", "gps_tag"},
	}
	ec.FormValues = formValues
	ec.UploadedFiles = map[string][]*UploadedFile{
		"images": {
			{
				FieldName: "images",
				Filename:  "evidence1.jpg",
				Size:      int64(len("fake image 1 content")),
				MimeType:  "image/jpeg",
				TempPath:  "/tmp/evidence1.jpg",
			},
		},
		"videos": {
			{
				FieldName: "videos",
				Filename:  "recording.mp4",
				Size:      int64(len("fake video content")),
				MimeType:  "video/mp4",
				TempPath:  "/tmp/recording.mp4",
			},
		},
	}

	// Test binding
	target := &UploadStepFileRequest{}
	err = bindFormToStruct(ec, target)

	// Verify no errors
	assert.NoError(t, err)

	// Verify simple fields
	assert.Equal(t, "Incident evidence from patrol", target.Description)

	// Verify JSON was properly parsed
	assert.NotNil(t, target.Metadata)
	assert.Equal(t, "INC-2023-001", target.Metadata["incident_id"])
	assert.Equal(t, float64(3), target.Metadata["severity"]) // JSON numbers become float64

	// Verify nested JSON objects
	location, ok := target.Metadata["location"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, float64(40.7128), location["latitude"])
	assert.Equal(t, float64(-74.0060), location["longitude"])
	assert.Equal(t, "123 Main St, New York", location["address"])

	officer, ok := target.Metadata["officer"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "OFF-001", officer["id"])
	assert.Equal(t, "John Doe", officer["name"])

	// Verify JSON arrays
	tags, ok := target.Metadata["tags"].([]any)
	assert.True(t, ok)
	assert.Len(t, tags, 3)
	assert.Contains(t, tags, "evidence")
	assert.Contains(t, tags, "patrol")
	assert.Contains(t, tags, "urgent")

	// Verify form arrays
	assert.Equal(t, []string{"auto_backup", "notify_supervisor", "gps_tag"}, target.Settings)
}

// TestErrorCasesInRealWorld tests edge cases that might occur in production
func TestErrorCasesInRealWorld(t *testing.T) {
	type TestRequest struct {
		InvalidJSON map[string]any `json:"invalid_json,omitempty"`
		ValidField  string         `json:"valid_field,omitempty"`
	}

	tests := []struct {
		name        string
		formValues  map[string][]string
		expectError bool
		description string
	}{
		{
			name: "invalid_json_should_error",
			formValues: map[string][]string{
				"invalid_json": {`{"unclosed": "json`}, // Invalid JSON
				"valid_field":  {"should still work"},
			},
			expectError: true,
			description: "Invalid JSON in form field should cause error",
		},
		{
			name: "empty_json_should_not_error",
			formValues: map[string][]string{
				"invalid_json": {""}, // Empty string should not error
				"valid_field":  {"works fine"},
			},
			expectError: false,
			description: "Empty JSON field should not cause error",
		},
		{
			name: "no_json_field_should_not_error",
			formValues: map[string][]string{
				"valid_field": {"works fine"},
				// invalid_json field is omitted
			},
			expectError: false,
			description: "Missing JSON field should not cause error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create proper multipart request
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Add a dummy file to make it a real multipart request
			fileWriter, err := writer.CreateFormFile("files", "test.txt")
			assert.NoError(t, err)
			_, err = fileWriter.Write([]byte("test content"))
			assert.NoError(t, err)

			// Add the form fields
			for key, values := range tt.formValues {
				for _, value := range values {
					err = writer.WriteField(key, value)
					assert.NoError(t, err)
				}
			}

			err = writer.Close()
			assert.NoError(t, err)

			// Create HTTP request with proper content type
			req := httptest.NewRequest(http.MethodPost, "/test", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			// Create Echo context
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			ec := &EndpointContext{
				EchoCtx:    c,
				FormValues: tt.formValues,
				UploadedFiles: map[string][]*UploadedFile{
					"files": {{
						FieldName: "files",
						Filename:  "test.txt",
						Size:      100,
						MimeType:  "text/plain",
						TempPath:  "/tmp/test.txt",
					}},
				},
			}

			target := &TestRequest{}
			err = bindFormToStruct(ec, target)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				// Valid field should always be set correctly
				if val, exists := tt.formValues["valid_field"]; exists && len(val) > 0 {
					assert.Equal(t, val[0], target.ValidField)
				}
			}
		})
	}
}
