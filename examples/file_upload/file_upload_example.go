package main

import (
	"log"

	rest "github.com/xompass/vsaas-rest"
)

// ExampleFileUploadHandler demonstrates how to use the new Echo file upload system
func ExampleFileUploadHandler(ctx *rest.EndpointContext) error {
	// Check if files were uploaded
	if !ctx.HasUploadedFiles("avatar") {
		return ctx.JSON(map[string]string{
			"error": "No avatar file uploaded",
		}, 400)
	}

	// Get the first uploaded file for the "avatar" field
	avatarFile := ctx.GetFirstUploadedFile("avatar")
	if avatarFile == nil {
		return ctx.JSON(map[string]string{
			"error": "Avatar file not found",
		}, 400)
	}

	// Get all uploaded files for "documents" field
	documentFiles := ctx.GetUploadedFiles("documents")

	// Create response with file information
	response := map[string]interface{}{
		"message": "Files uploaded successfully",
		"avatar": map[string]interface{}{
			"original_name": avatarFile.OriginalName,
			"filename":      avatarFile.Filename,
			"size":          avatarFile.Size,
			"extension":     avatarFile.Extension,
			"mime_type":     avatarFile.MimeType,
		},
		"documents": make([]map[string]interface{}, len(documentFiles)),
	}

	// Add document files information
	for i, doc := range documentFiles {
		response["documents"].([]map[string]interface{})[i] = map[string]interface{}{
			"original_name": doc.OriginalName,
			"filename":      doc.Filename,
			"size":          doc.Size,
			"extension":     doc.Extension,
			"mime_type":     doc.MimeType,
		}
	}

	return ctx.JSON(response)
}

func main() {
	// Create a new REST application
	app := rest.NewRestApp(rest.RestAppOptions{
		Name: "File Upload Example",
		Port: 8080,
	})

	// Create file upload configuration
	fileUploadConfig := &rest.FileUploadConfig{
		MaxFileSize: 10 * 1024 * 1024, // 10MB default limit
		TypeSizeLimits: map[rest.FileExtension]int64{
			// Image size limits
			rest.FileExtensionJPEG: 5 * 1024 * 1024, // 5MB for JPEG
			rest.FileExtensionJPG:  5 * 1024 * 1024, // 5MB for JPG
			rest.FileExtensionPNG:  5 * 1024 * 1024, // 5MB for PNG
			rest.FileExtensionGIF:  2 * 1024 * 1024, // 2MB for GIF

			// Document size limits
			rest.FileExtensionPDF: 20 * 1024 * 1024, // 20MB for PDF
			rest.FileExtensionDOC: 15 * 1024 * 1024, // 15MB for DOC
		},
		FileFields: map[string]*rest.FileFieldConfig{
			"avatar": {
				FieldName:   "avatar",
				Required:    true,
				MaxFileSize: 3 * 1024 * 1024, // 3MB limit for avatar specifically
				AllowedTypes: []rest.FileExtension{
					rest.FileExtensionJPEG,
					rest.FileExtensionJPG,
					rest.FileExtensionPNG,
				},
				MaxFiles: 1, // Only one avatar file allowed
			},
			"documents": {
				FieldName:   "documents",
				Required:    false,
				MaxFileSize: 10 * 1024 * 1024, // 10MB limit per document
				AllowedTypes: []rest.FileExtension{
					rest.FileExtensionPDF,
					rest.FileExtensionDOC,
					rest.FileExtensionDOCX,
				},
				MaxFiles: 5, // Up to 5 documents allowed
				// Field-specific type size limits that override global ones
				TypeSizeLimits: map[rest.FileExtension]int64{
					rest.FileExtensionPDF: 25 * 1024 * 1024, // 25MB for PDFs in documents field
				},
			},
		},
		UploadPath:         "./uploads",
		TempPath:           "./temp",
		KeepFilesAfterSend: false, // Files will be cleaned up after response
	}

	// Create an endpoint with file upload support
	endpoint := &rest.Endpoint{
		Name:             "Upload Files",
		Method:           rest.MethodPOST,
		Path:             "/upload",
		Handler:          ExampleFileUploadHandler,
		FileUploadConfig: fileUploadConfig,
		Public:           true, // Make it public for demo purposes
	}

	// Create a route group
	api := app.Group("/api/v1")

	// Register the endpoint
	app.RegisterEndpoint(endpoint, api)

	// Start the server
	log.Println("Starting server on :8080")
	log.Println("Example upload endpoints:")
	log.Println("POST /api/v1/upload - Upload files with fields 'avatar' and 'documents'")
	log.Println("")
	log.Println("Example curl command:")
	log.Println(`curl -X POST http://localhost:8080/api/v1/upload \`)
	log.Println(`  -F "avatar=@/path/to/your/image.jpg" \`)
	log.Println(`  -F "documents=@/path/to/your/document.pdf" \`)
	log.Println(`  -F "documents=@/path/to/another/document.pdf"`)

	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
