package rest

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// FileUploadConfig represents the global file upload configuration
type FileUploadConfig struct {
	MaxFileSize        int64                       // Default max file size in bytes
	FileFields         map[string]*FileFieldConfig // Configuration for specific file fields
	TypeSizeLimits     map[FileExtension]int64     // Size limits per file type
	UploadPath         string                      // Base upload directory
	TempPath           string                      // Temporary files directory
	KeepFilesAfterSend bool                        // Whether to keep files after response
}

// FileFieldConfig represents configuration for a specific file field
type FileFieldConfig struct {
	FieldName      string                  // Form field name
	Required       bool                    // Whether the field is required
	MaxFileSize    int64                   // Max file size for this field (0 = use global)
	AllowedTypes   []FileExtension         // Allowed extensions for this field (nil = use global)
	MaxFiles       int                     // Maximum number of files for this field (0 = unlimited)
	TypeSizeLimits map[FileExtension]int64 // Size limits per file type for this field
}

// UploadedFile represents an uploaded file
type UploadedFile struct {
	FieldName    string `json:"field_name"`
	OriginalName string `json:"original_name"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	Extension    string `json:"extension"`
	MimeType     string `json:"mime_type"`
	Path         string `json:"path"`
	TempPath     string `json:"temp_path"`
}

// StreamingFileUploadHandler handles file uploads with Echo's multipart capabilities
type EchoFileUploadHandler struct {
	config *FileUploadConfig
}

// NewEchoFileUploadHandler creates a new Echo file upload handler
func NewEchoFileUploadHandler(config *FileUploadConfig) *EchoFileUploadHandler {
	if config.TempPath == "" {
		config.TempPath = os.TempDir()
	}
	if config.UploadPath == "" {
		config.UploadPath = "./uploads"
	}
	if !config.KeepFilesAfterSend {
		// Ensure temp directory exists
		os.MkdirAll(config.TempPath, 0755)
	}
	os.MkdirAll(config.UploadPath, 0755)

	return &EchoFileUploadHandler{
		config: config,
	}
}

// ProcessStreamingFileUploads processes multipart form data using Echo's multipart parsing with size limits
func (h *EchoFileUploadHandler) ProcessStreamingFileUploads(c echo.Context) (map[string][]*UploadedFile, map[string][]string, error) {
	// Get content type and verify it's multipart
	contentType := c.Request().Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return nil, nil, echo.NewHTTPError(http.StatusBadRequest, "Content-Type must be multipart/form-data")
	}

	// Parse the multipart form with custom reader to handle streaming
	reader, err := c.Request().MultipartReader()
	if err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusBadRequest, "Failed to create multipart reader: "+err.Error())
	}

	uploadedFiles := make(map[string][]*UploadedFile)
	formValues := make(map[string][]string)

	// Process each part of the multipart form
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break // No more parts
		}
		if err != nil {
			h.cleanupFiles(uploadedFiles)
			return nil, nil, echo.NewHTTPError(http.StatusBadRequest, "Failed to read multipart data: "+err.Error())
		}

		// Check if this is a file or a regular form field
		if part.FileName() == "" {
			// This is a regular form field, not a file
			fieldName := part.FormName()
			if fieldName != "" {
				// Read the value
				valueBytes, readErr := io.ReadAll(part)
				if readErr != nil {
					part.Close()
					h.cleanupFiles(uploadedFiles)
					return nil, nil, echo.NewHTTPError(http.StatusBadRequest, "Failed to read form field: "+readErr.Error())
				}

				// Store the form value
				if formValues[fieldName] == nil {
					formValues[fieldName] = []string{}
				}
				formValues[fieldName] = append(formValues[fieldName], string(valueBytes))
			}
			part.Close()
			continue
		}

		// This is a file field
		fieldName := part.FormName()
		if fieldName == "" {
			part.Close()
			continue
		}

		// Process the file part with streaming
		uploadedFile, err := h.processStreamingFile(fieldName, part)
		if err != nil {
			part.Close()
			h.cleanupFiles(uploadedFiles)
			return nil, nil, err
		}

		// Add to results
		if uploadedFiles[fieldName] == nil {
			uploadedFiles[fieldName] = make([]*UploadedFile, 0)
		}
		uploadedFiles[fieldName] = append(uploadedFiles[fieldName], uploadedFile)

		part.Close()
	}

	// Validate field requirements
	if err := h.validateFieldRequirements(uploadedFiles); err != nil {
		h.cleanupFiles(uploadedFiles)
		return nil, nil, err
	}

	return uploadedFiles, formValues, nil
}

// validateFieldRequirements validates that all required fields are present and limits are respected
func (h *EchoFileUploadHandler) validateFieldRequirements(uploadedFiles map[string][]*UploadedFile) error {
	for fieldName, fieldConfig := range h.config.FileFields {
		files := uploadedFiles[fieldName]

		// Check if required field is missing
		if fieldConfig.Required && len(files) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Field '%s' is required", fieldName))
		}

		maxFiles := fieldConfig.MaxFiles
		if maxFiles == 0 {
			maxFiles = 1
		}

		// Check max files limit
		if maxFiles > 0 && len(files) > maxFiles {
			return echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("Field '%s' exceeds maximum file limit of %d", fieldName, maxFiles))
		}
	}
	return nil
}

// processStreamingFile processes a single file part with streaming validation
func (h *EchoFileUploadHandler) processStreamingFile(fieldName string, part *multipart.Part) (*UploadedFile, error) {
	filename := part.FileName()

	// Get file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "File must have an extension")
	}

	fieldConfig := h.config.FileFields[fieldName]

	// Validate file extension
	if err := h.validateFileExtension(ext, fieldConfig); err != nil {
		return nil, err
	}

	// Determine max file size
	maxSize := h.getMaxFileSize(FileExtension(ext), fieldConfig)

	// Generate unique filename
	uniqueFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	// Determine file path
	var filePath string
	if !h.config.KeepFilesAfterSend {
		filePath = filepath.Join(h.config.TempPath, uniqueFilename)
	} else {
		filePath = filepath.Join(h.config.UploadPath, uniqueFilename)
		log.Println("Saving file to:", filePath)
	}

	// Create the destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create destination file: "+err.Error())
	}
	defer dst.Close()

	// Stream the file with size validation using buffered reading
	var totalSize int64
	buffer := make([]byte, 32*1024) // 32KB buffer for optimal performance

	for {
		n, err := part.Read(buffer)
		if n > 0 {
			totalSize += int64(n)

			// Check if size exceeds limit BEFORE writing
			if maxSize > 0 && totalSize > maxSize {
				dst.Close()
				os.Remove(filePath)
				return nil, echo.NewHTTPError(http.StatusRequestEntityTooLarge,
					fmt.Sprintf("File size exceeds limit of %d bytes for field '%s' (file type: %s)", maxSize, fieldName, ext))
			}

			// Write to destination
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				os.Remove(filePath)
				return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to write file: "+writeErr.Error())
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(filePath)
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Failed to read uploaded file: "+err.Error())
		}
	}

	// Get MIME type from headers or detect it
	mimeType := part.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Create uploaded file info
	uploadedFile := &UploadedFile{
		FieldName:    fieldName,
		OriginalName: filename,
		Filename:     uniqueFilename,
		Size:         totalSize,
		Extension:    ext,
		MimeType:     mimeType,
		Path:         filePath,
	}

	if !h.config.KeepFilesAfterSend {
		uploadedFile.TempPath = filePath
	}

	return uploadedFile, nil
}

// validateFileExtension validates if the file extension is allowed
func (h *EchoFileUploadHandler) validateFileExtension(ext string, fieldConfig *FileFieldConfig) error {
	allowedTypes := fieldConfig.AllowedTypes

	// If no restrictions, allow all
	if len(allowedTypes) == 0 {
		return nil
	}

	for _, allowedExt := range allowedTypes {
		if string(allowedExt) == ext {
			return nil
		}
	}

	return echo.NewHTTPError(http.StatusUnsupportedMediaType,
		fmt.Sprintf("File type '%s' is not allowed for field. Allowed types: %v", ext, allowedTypes))
}

// getMaxFileSize determines the maximum file size for a given extension and field
func (h *EchoFileUploadHandler) getMaxFileSize(ext FileExtension, fieldConfig *FileFieldConfig) int64 {
	// Priority order (highest to lowest):
	// 1. Field-specific type size limits
	// 2. Field-specific max file size
	// 3. Global type size limits
	// 4. Global max file size

	// Check field-specific type size limits first (highest priority)
	if fieldConfig != nil && fieldConfig.TypeSizeLimits != nil {
		if limit, exists := fieldConfig.TypeSizeLimits[ext]; exists {
			return limit
		}
	}

	// Check field-specific max file size (second priority)
	if fieldConfig != nil && fieldConfig.MaxFileSize > 0 {
		return fieldConfig.MaxFileSize
	}

	// Check global type size limits (third priority)
	if h.config.TypeSizeLimits != nil {
		if limit, exists := h.config.TypeSizeLimits[ext]; exists {
			return limit
		}
	}

	// Use global max file size (lowest priority)
	return h.config.MaxFileSize
}

// cleanupFiles removes uploaded files (used for error cleanup)
func (h *EchoFileUploadHandler) cleanupFiles(uploadedFiles map[string][]*UploadedFile) {
	for _, files := range uploadedFiles {
		for _, file := range files {
			if file.Path != "" {
				os.Remove(file.Path)
			}
		}
	}
}

// CleanupAfterResponse removes temporary files after sending response
func (h *EchoFileUploadHandler) CleanupAfterResponse(uploadedFiles map[string][]*UploadedFile) {
	if h.config.KeepFilesAfterSend {
		return
	}

	// Use goroutine to cleanup after a small delay
	go func() {
		time.Sleep(100 * time.Millisecond) // Small delay to ensure response is sent
		for _, files := range uploadedFiles {
			for _, file := range files {
				if file.TempPath != "" {
					os.Remove(file.TempPath)
				}
			}
		}
	}()
}
