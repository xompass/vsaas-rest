package rest

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEchoFileUploadHandler(t *testing.T) {
	// Create temporary directories for testing
	tempDir := t.TempDir()
	uploadDir := filepath.Join(tempDir, "uploads")
	tempUploadDir := filepath.Join(tempDir, "temp")

	// Create test configuration
	config := &FileUploadConfig{
		MaxFileSize: 5 * 1024 * 1024, // 5MB default
		TypeSizeLimits: map[FileExtension]int64{
			FileExtensionJPG: 2 * 1024 * 1024,  // 2MB for JPG
			FileExtensionPDF: 10 * 1024 * 1024, // 10MB for PDF
		},
		FileFields: map[string]*FileFieldConfig{
			"avatar": {
				FieldName:    "avatar",
				Required:     true,
				MaxFileSize:  1 * 1024 * 1024, // 1MB for avatar
				AllowedTypes: []FileExtension{FileExtensionJPG, FileExtensionPNG},
				MaxFiles:     1,
			},
			"documents": {
				FieldName:    "documents",
				Required:     false,
				AllowedTypes: []FileExtension{FileExtensionPDF, FileExtensionDOC},
				MaxFiles:     3,
			},
		},
		UploadPath:         uploadDir,
		TempPath:           tempUploadDir,
		KeepFilesAfterSend: false,
	}

	handler := NewEchoFileUploadHandler(config)

	t.Run("successful file upload", func(t *testing.T) {
		// Create test files content
		avatarContent := strings.Repeat("a", 100) // Small file
		docContent := strings.Repeat("d", 500)    // Small document

		// Create multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add avatar file
		avatarPart, err := writer.CreateFormFile("avatar", "avatar.jpg")
		require.NoError(t, err)
		_, err = avatarPart.Write([]byte(avatarContent))
		require.NoError(t, err)

		// Add document file
		docPart, err := writer.CreateFormFile("documents", "document.pdf")
		require.NoError(t, err)
		_, err = docPart.Write([]byte(docContent))
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		// Create Echo context
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Process upload
		uploadedFiles, formValues, err := handler.ProcessStreamingFileUploads(c)
		require.NoError(t, err)

		// formValues should be empty since we only uploaded files
		assert.Empty(t, formValues)

		// Verify results
		assert.Len(t, uploadedFiles, 2)
		assert.Len(t, uploadedFiles["avatar"], 1)
		assert.Len(t, uploadedFiles["documents"], 1)

		// Check avatar file
		avatarFile := uploadedFiles["avatar"][0]
		assert.Equal(t, "avatar", avatarFile.FieldName)
		assert.Equal(t, "avatar.jpg", avatarFile.OriginalName)
		assert.Equal(t, ".jpg", avatarFile.Extension)
		assert.Equal(t, int64(len(avatarContent)), avatarFile.Size)
		assert.True(t, strings.HasSuffix(avatarFile.Filename, ".jpg"))

		// Check document file
		docFile := uploadedFiles["documents"][0]
		assert.Equal(t, "documents", docFile.FieldName)
		assert.Equal(t, "document.pdf", docFile.OriginalName)
		assert.Equal(t, ".pdf", docFile.Extension)
		assert.Equal(t, int64(len(docContent)), docFile.Size)

		// Verify files exist on disk
		assert.FileExists(t, avatarFile.Path)
		assert.FileExists(t, docFile.Path)

		// Clean up
		handler.CleanupAfterResponse(uploadedFiles)
	})

	t.Run("file size exceeds limit", func(t *testing.T) {
		// Create a large content that exceeds the 1MB limit for avatar
		// We'll create a streaming scenario that writes chunks
		largeContent := strings.Repeat("x", 1.5*1024*1024) // 1.5MB - exceeds the 1MB limit

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		avatarPart, err := writer.CreateFormFile("avatar", "large_avatar.jpg")
		require.NoError(t, err)
		_, err = avatarPart.Write([]byte(largeContent))
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		// Create Echo context
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Process upload - should fail due to size limit
		_, _, err = handler.ProcessStreamingFileUploads(c)
		require.Error(t, err)

		// Should be a size limit error
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusRequestEntityTooLarge, httpErr.Code)
	})

	t.Run("unsupported file type", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Try to upload .txt file as avatar (not allowed)
		avatarPart, err := writer.CreateFormFile("avatar", "avatar.txt")
		require.NoError(t, err)
		_, err = avatarPart.Write([]byte("text content"))
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		// Create Echo context
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Process upload - should fail
		_, _, err = handler.ProcessStreamingFileUploads(c)
		require.Error(t, err)

		// Should be an unsupported media type error
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnsupportedMediaType, httpErr.Code)
	})

	t.Run("required field missing", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Only upload document, but avatar is required
		docPart, err := writer.CreateFormFile("documents", "document.pdf")
		require.NoError(t, err)
		_, err = docPart.Write([]byte("document content"))
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		// Create Echo context
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Process upload - should fail
		_, _, err = handler.ProcessStreamingFileUploads(c)
		require.Error(t, err)

		// Should be a bad request error
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Contains(t, httpErr.Message, "required")
	})

	t.Run("too many files", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add required avatar
		avatarPart, err := writer.CreateFormFile("avatar", "avatar.jpg")
		require.NoError(t, err)
		_, err = avatarPart.Write([]byte("avatar content"))
		require.NoError(t, err)

		// Add too many documents (max is 3)
		for i := 0; i < 4; i++ {
			docPart, err := writer.CreateFormFile("documents", fmt.Sprintf("document%d.pdf", i))
			require.NoError(t, err)
			_, err = docPart.Write([]byte("document content"))
			require.NoError(t, err)
		}

		err = writer.Close()
		require.NoError(t, err)

		// Create Echo context
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Process upload - should fail
		_, _, err = handler.ProcessStreamingFileUploads(c)
		require.Error(t, err)

		// Should be a bad request error
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Contains(t, httpErr.Message, "maximum file limit")
	})

	t.Run("cleanup files after response", func(t *testing.T) {
		// Create small test file
		content := "test content"
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		avatarPart, err := writer.CreateFormFile("avatar", "avatar.jpg")
		require.NoError(t, err)
		_, err = avatarPart.Write([]byte(content))
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		// Create Echo context
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Process upload
		uploadedFiles, _, err := handler.ProcessStreamingFileUploads(c)
		require.NoError(t, err)

		avatarFile := uploadedFiles["avatar"][0]
		filePath := avatarFile.Path

		// File should exist before cleanup
		assert.FileExists(t, filePath)

		// Clean up
		handler.CleanupAfterResponse(uploadedFiles)

		// Wait a bit for cleanup goroutine
		time.Sleep(200 * time.Millisecond)

		// File should be deleted after cleanup
		assert.NoFileExists(t, filePath)
	})
}
