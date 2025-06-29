package rest

import (
	"bytes"
	"encoding/base64"
	"io"
	"log"
	"math"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestTxtUpload(t *testing.T) {
	endpoint := Endpoint{
		Name:   "FileUploadTest",
		Path:   "/upload",
		Method: MethodPOST,
		Handler: func(c *EndpointContext) error {
			for field, files := range c.GetAllUploadedFiles() {
				log.Printf("Field: %s, Files: %d", field, len(files))
				for _, file := range files {
					log.Printf("File: %s, Size: %d bytes", file.OriginalName, file.Size)
				}
			}
			return c.JSON(map[string]string{"status": "success"})
		},
		FileUploadConfig: &FileUploadConfig{
			KeepFilesAfterSend: true,
			MaxFileSize:        1024, // 1KB limit for testing
			FileFields: map[string]*FileFieldConfig{
				"file": {
					FieldName:    "file",
					Required:     true,
					AllowedTypes: []FileExtension{FileExtensionTXT},
				},
			},
		},
	}

	app := NewRestApp(RestAppOptions{
		FiberConfig: &fiber.Config{
			BodyLimit: 1024 * 2,
		},
	})
	app.RegisterEndpoint(&endpoint)
	// app.FiberApp.Server().StreamRequestBody = true
	app.FiberApp.Post("/upload2", func(c *fiber.Ctx) error {
		log.Println(app.FiberApp.Server().StreamRequestBody, app.FiberApp.Server().MaxRequestBodySize)

		reader := c.Context().RequestBodyStream()
		// Read 1MiB at a time
		buffer := make([]byte, 0, 1024*1024)
		for {
			length, err := io.ReadFull(reader, buffer[:cap(buffer)])
			// Cap the buffer based on the actual length read
			buffer = buffer[:length]
			if err != nil {
				// When the error is EOF, there are no longer any bytes to read
				// meaning the request is completed
				if err == io.EOF {
					break
				}

				// If the error is an unexpected EOF, the requested size to read
				// was larger than what was available. This is not an issue for
				// as long as the length returned above is used, or the buffer
				// is capped as it is above. Any error that is not an unexpected
				// EOF is an actual error, which we handle accordingly
				if err != io.ErrUnexpectedEOF {
					return err
				}
			}

			// You may now use the buffer to handle the chunk of length bytes
			log.Printf("Read %d bytes: %x ...", length, buffer[0])
		}
		return nil

		/* // Get first file from form field "file":
		file, err := c.FormFile("file")
		if err != nil {
			return err
		}
		// Save file to root directory:
		return c.SaveFile(file, fmt.Sprintf("./%s", file.Filename)) */
	})

	// Test uploading a file
	uploadFile(t, app, "file", "test.txt", []byte(randomBase64String(300000000)))
}

/*
func TestTxtUploadMaxLength(t *testing.T) {
	endpoint := Endpoint{
		Name:   "FileUploadTestMaxLength",
		Path:   "/upload",
		Method: MethodPOST,
		Handler: func(c *EndpointContext) error {
			// In this test, we expect the file to be rejected due to exceeding the max length
			return c.JSON(map[string]string{"status": "success"})
		},
		FileUploadConfig: &FileUploadConfig{
			KeepFilesAfterSend: true,
			MaxFileSize:        1024, // 1KB limit for testing
			FileFields: map[string]*FileFieldConfig{
				"file": {
					FieldName:    "file",
					Required:     true,
					AllowedTypes: []FileExtension{FileExtensionTXT},
				},
			},
		},
	}

	app := NewRestApp(RestAppOptions{
		FiberConfig: &fiber.Config{
			BodyLimit: 512,
		},
	})
	app.RegisterEndpoint(&endpoint)

	// Test uploading a file
	resp := uploadFile(t, app, "file", "test.txt", []byte(randomBase64String(2000)))
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("Expected status code 413, got %d", resp.StatusCode)
	}

} */

func uploadFile(t *testing.T, app *RestApp, fieldName, fileName string, fileContent []byte) *http.Response {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("meta", "test metadata")
	fileWriter, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fileWriter.Write(fileContent)
	if err != nil {
		t.Fatal(err)
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload2", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Error testing request: %v", err)
	}

	defer resp.Body.Close()

	return resp
}

func randomBase64String(l int) string {
	buff := make([]byte, int(math.Ceil(float64(l)/float64(1.33333333333))))
	rand.Read(buff)
	str := base64.RawURLEncoding.EncodeToString(buff)
	return str[:l] // strip 1 extra character we get from odd length results
}
