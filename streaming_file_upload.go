package rest

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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

// StreamingFileUploadHandler handles file uploads with Fiber's streaming body functionality
type StreamingFileUploadHandler struct {
	config       *FileUploadConfig
	fieldConfigs map[string]*FileFieldConfig
}

// NewStreamingFileUploadHandler creates a new streaming file upload handler
func NewStreamingFileUploadHandler(config *FileUploadConfig) *StreamingFileUploadHandler {
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

	return &StreamingFileUploadHandler{
		config: config,
	}
}

// ProcessStreamingFileUploads processes multipart form data using Fiber's streaming body
func (h *StreamingFileUploadHandler) ProcessStreamingFileUploads(c *fiber.Ctx) (map[string][]*UploadedFile, error) {
	// Get content type and boundary
	contentType := c.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Content-Type must be multipart/form-data")
	}

	// Parse media type to get boundary
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Failed to parse Content-Type")
	}

	boundary, ok := params["boundary"]
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Missing boundary in Content-Type")
	}

	// Get the request body stream
	bodyStream := c.Context().RequestBodyStream()

	// Create multipart reader with the stream
	reader := multipart.NewReader(bodyStream, boundary)

	uploadedFiles := make(map[string][]*UploadedFile)

	// Process each part of the multipart form
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break // No more parts
		}
		if err != nil {
			h.cleanupFiles(uploadedFiles)
			return nil, fiber.NewError(fiber.StatusBadRequest, "Failed to read multipart data")
		}

		// Skip non-file parts
		if part.FileName() == "" {
			part.Close()
			continue
		}

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
			return nil, err
		}

		// Add to results
		if uploadedFiles[fieldName] == nil {
			uploadedFiles[fieldName] = make([]*UploadedFile, 0)
		}
		uploadedFiles[fieldName] = append(uploadedFiles[fieldName], uploadedFile)

		part.Close()
	}

	// Validate field requirements
	for fieldName, fieldConfig := range h.fieldConfigs {
		files := uploadedFiles[fieldName]

		// Check if required field is missing
		if fieldConfig.Required && len(files) == 0 {
			h.cleanupFiles(uploadedFiles)
			return nil, fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Field '%s' is required", fieldName))
		}

		// Check max files limit
		if fieldConfig.MaxFiles > 0 && len(files) > fieldConfig.MaxFiles {
			h.cleanupFiles(uploadedFiles)
			return nil, fiber.NewError(fiber.StatusBadRequest,
				fmt.Sprintf("Field '%s' exceeds maximum file limit of %d", fieldName, fieldConfig.MaxFiles))
		}
	}

	return uploadedFiles, nil
}

// processStreamingFile processes a single file part with streaming validation
func (h *StreamingFileUploadHandler) processStreamingFile(fieldName string, part *multipart.Part) (*UploadedFile, error) {
	filename := part.FileName()

	// Get file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "File must have an extension")
	}

	fieldConfig := h.fieldConfigs[fieldName]

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
	}

	// Create the destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to create destination file")
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
				return nil, fiber.NewError(fiber.StatusRequestEntityTooLarge,
					fmt.Sprintf("File size exceeds limit of %d bytes for field '%s'", maxSize, fieldName))
			}

			// Write to destination
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				os.Remove(filePath)
				return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to write file")
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(filePath)
			return nil, fiber.NewError(fiber.StatusBadRequest, "Failed to read uploaded file")
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
func (h *StreamingFileUploadHandler) validateFileExtension(ext string, fieldConfig *FileFieldConfig) error {
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

	return fiber.NewError(fiber.StatusUnsupportedMediaType,
		fmt.Sprintf("File type '%s' is not allowed", ext))
}

// getMaxFileSize determines the maximum file size for a given extension and field
func (h *StreamingFileUploadHandler) getMaxFileSize(ext FileExtension, fieldConfig *FileFieldConfig) int64 {
	// Check field-specific type size limits first
	if fieldConfig != nil && fieldConfig.TypeSizeLimits != nil {
		if limit, exists := fieldConfig.TypeSizeLimits[ext]; exists {
			return limit
		}
	}

	// Check global type size limits
	if h.config.TypeSizeLimits != nil {
		if limit, exists := h.config.TypeSizeLimits[ext]; exists {
			return limit
		}
	}

	// Check field-specific max file size
	if fieldConfig != nil && fieldConfig.MaxFileSize > 0 {
		return fieldConfig.MaxFileSize
	}

	// Use global max file size
	return h.config.MaxFileSize
}

// cleanupFiles removes uploaded files (used for error cleanup)
func (h *StreamingFileUploadHandler) cleanupFiles(uploadedFiles map[string][]*UploadedFile) {
	for _, files := range uploadedFiles {
		for _, file := range files {
			if file.Path != "" {
				os.Remove(file.Path)
			}
		}
	}
}

// CleanupAfterResponse removes temporary files after sending response
func (h *StreamingFileUploadHandler) CleanupAfterResponse(uploadedFiles map[string][]*UploadedFile) {
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
