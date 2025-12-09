package rest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// HeaderMatchFunc is a function that returns headers based on the request path and file path
type HeaderMatchFunc func(requestPath string, filePath string) map[string]string

// StaticConfig holds the configuration for serving static files
type StaticConfig struct {
	Prefix          string            // URL prefix (e.g., "/", "/assets")
	Directory       string            // Physical directory to serve
	EnableSPA       bool              // Enable SPA mode (fallback to index.html)
	IndexFile       string            // Index file name (default: "index.html")
	EnableBrowse    bool              // Allow directory browsing
	ExcludePrefixes []string          // Path prefixes to exclude from SPA fallback (e.g., "/api", "/swagger")
	Headers         map[string]string // Base headers for all files
	IndexHeaders    map[string]string // Headers specific to index file
	AssetHeaders    map[string]string // Headers for assets (.js, .css, images, etc.)
	HeaderMatcher   HeaderMatchFunc   // Custom function for header matching (takes priority)
}

// SecureStaticHeaders returns secure default headers for static files
func SecureStaticHeaders() map[string]string {
	return map[string]string{
		"X-Frame-Options":        "SAMEORIGIN",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
}

// CachedAssetHeaders returns headers for cacheable assets (with hash in filename)
func CachedAssetHeaders() map[string]string {
	headers := SecureStaticHeaders()
	headers["Cache-Control"] = "public, max-age=31536000, immutable"
	return headers
}

// SPAIndexHeaders returns headers for index.html in SPA mode (no caching)
func SPAIndexHeaders() map[string]string {
	headers := SecureStaticHeaders()
	headers["Cache-Control"] = "no-cache, no-store, must-revalidate"
	headers["Pragma"] = "no-cache"
	headers["Expires"] = "0"
	return headers
}

// isAssetFile determines if a file is a cacheable asset
func isAssetFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	assetExtensions := []string{
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp",
		".woff", ".woff2", ".ttf", ".eot", ".otf", ".ico", ".webm", ".mp4",
		".pdf", ".zip", ".json", ".xml",
	}

	for _, assetExt := range assetExtensions {
		if ext == assetExt {
			return true
		}
	}
	return false
}

// mergeHeaders merges multiple header maps, with later maps overwriting earlier ones
func mergeHeaders(headerMaps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, headers := range headerMaps {
		for key, value := range headers {
			result[key] = value
		}
	}
	return result
}

// getHeadersForFile determines which headers to apply based on configuration and file type
func (config *StaticConfig) getHeadersForFile(requestPath string, filePath string) map[string]string {
	// Start with base headers
	headers := make(map[string]string)
	if config.Headers != nil {
		headers = mergeHeaders(headers, config.Headers)
	}

	// If custom matcher is provided, use it (takes priority)
	if config.HeaderMatcher != nil {
		customHeaders := config.HeaderMatcher(requestPath, filePath)
		if customHeaders != nil {
			return mergeHeaders(headers, customHeaders)
		}
		// If matcher returns nil, continue with default logic
	}

	// Determine file type and apply specific headers
	indexFile := config.IndexFile
	if indexFile == "" {
		indexFile = "index.html"
	}

	fileName := filepath.Base(requestPath)

	// Check if it's the index file
	if fileName == indexFile || strings.HasSuffix(requestPath, "/"+indexFile) {
		if config.IndexHeaders != nil {
			return mergeHeaders(headers, config.IndexHeaders)
		}
		// If no IndexHeaders specified but SPA is enabled, use no-cache
		if config.EnableSPA {
			return mergeHeaders(headers, SPAIndexHeaders())
		}
	}

	// Check if it's an asset file
	if isAssetFile(requestPath) {
		if config.AssetHeaders != nil {
			return mergeHeaders(headers, config.AssetHeaders)
		}
	}

	return headers
}

// createStaticMiddleware creates a middleware that applies headers to static files
func (config *StaticConfig) createStaticMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestPath := c.Request().URL.Path

			// Remove prefix from path to get file path
			filePath := strings.TrimPrefix(requestPath, config.Prefix)
			if filePath == "" || filePath == "/" {
				indexFile := config.IndexFile
				if indexFile == "" {
					indexFile = "index.html"
				}
				filePath = indexFile
			}

			// Get headers for this file
			headers := config.getHeadersForFile(requestPath, filePath)

			// Apply headers to response
			for key, value := range headers {
				c.Response().Header().Set(key, value)
			}

			return next(c)
		}
	}
}

// ServeStatic configures the application to serve static files
func (receiver *RestApp) ServeStatic(config StaticConfig) error {
	// Validate configuration
	if config.Directory == "" {
		return echo.NewHTTPError(http.StatusInternalServerError, "Static directory is required")
	}

	// Check if directory exists
	if _, err := os.Stat(config.Directory); os.IsNotExist(err) {
		receiver.Warnf("Static directory does not exist: %s", config.Directory)
		return echo.NewHTTPError(http.StatusInternalServerError, "Static directory does not exist: "+config.Directory)
	}

	// Set default values
	if config.Prefix == "" {
		config.Prefix = "/"
	}
	if config.IndexFile == "" {
		config.IndexFile = "index.html"
	}

	receiver.Infof("Serving static files from %s at %s (SPA: %v)", config.Directory, config.Prefix, config.EnableSPA)

	// If SPA mode is enabled, we handle everything in the catch-all route
	// to avoid conflicts with other routes (API, Swagger, etc.)
	if config.EnableSPA {
		receiver.setupSPAFallback(config)
		return nil
	}

	// For non-SPA mode, use Echo's static middleware
	staticConfig := middleware.StaticConfig{
		Root:   config.Directory,
		Index:  config.IndexFile,
		Browse: config.EnableBrowse,
		HTML5:  false,
	}

	// Apply custom middleware for headers
	headerMiddleware := config.createStaticMiddleware()

	// Register static file handler with middleware
	receiver.EchoApp.Use(headerMiddleware)
	receiver.EchoApp.Use(middleware.StaticWithConfig(staticConfig))

	return nil
}

// setupSPAFallback sets up a fallback handler for SPA mode
func (receiver *RestApp) setupSPAFallback(config StaticConfig) {
	indexPath := filepath.Join(config.Directory, config.IndexFile)

	// Use Echo's HTTPErrorHandler to serve SPA on 404
	// This way it only triggers when no other route matches
	originalHandler := receiver.EchoApp.HTTPErrorHandler
	
	receiver.EchoApp.HTTPErrorHandler = func(err error, c echo.Context) {
		// Only handle 404 errors for SPA fallback
		if he, ok := err.(*echo.HTTPError); ok && he.Code == http.StatusNotFound {
			requestPath := c.Request().URL.Path
			
			// Skip excluded prefixes (e.g., /api, /swagger)
			skipSPA := false
			for _, prefix := range config.ExcludePrefixes {
				if strings.HasPrefix(requestPath, prefix) {
					skipSPA = true
					break
				}
			}
			
			if !skipSPA {
				// Check if the requested path is a file that exists
				filePath := filepath.Join(config.Directory, strings.TrimPrefix(requestPath, config.Prefix))

				// If file exists, serve it with appropriate headers
				if fileInfo, err := os.Stat(filePath); err == nil && !fileInfo.IsDir() {
					// Get headers for this file
					headers := config.getHeadersForFile(requestPath, filePath)
					for key, value := range headers {
						c.Response().Header().Set(key, value)
					}
					if err := c.File(filePath); err == nil {
						return // Successfully served file
					}
				}

				// Otherwise, serve index.html for SPA routing
				// Apply index headers
				headers := config.getHeadersForFile(config.IndexFile, indexPath)
				for key, value := range headers {
					c.Response().Header().Set(key, value)
				}

				if err := c.File(indexPath); err == nil {
					return // Successfully served index.html
				}
			}
		}
		
		// For all other errors or if SPA fallback failed, use original handler
		originalHandler(err, c)
	}
}
