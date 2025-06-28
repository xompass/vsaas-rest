package rest

import (
	"bytes"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTxtUpload(t *testing.T) {
	endpoint := Endpoint{
		Name:   "FileUploadTest",
		Path:   "/upload",
		Method: MethodPOST,
		Handler: func(c *EndpointContext) error {
			log.Println(c.UploadedFiles)
			return c.JSON(map[string]string{"status": "success"})
		},
		FileUploadConfig: &FileUploadConfig{
			FileFields: map[string]*FileFieldConfig{
				"file": {
					FieldName:    "file",
					Required:     true,
					AllowedTypes: []FileExtension{FileExtensionTXT},
				},
			},
		},
	}

	app := NewRestApp(RestAppOptions{})

	app.RegisterEndpoint(&endpoint)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("meta", "test metadata")
	fileWriter, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}

	_, err = fileWriter.Write([]byte("contenido del archivo"))
	if err != nil {
		t.Fatal(err)
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Error testing request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", resp.StatusCode)
	}

	defer resp.Body.Close()
}
