package rest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupTestStaticDir(t *testing.T) string {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test files
	indexContent := `<!DOCTYPE html><html><body>SPA Index</body></html>`
	jsContent := `console.log('test');`
	cssContent := `body { margin: 0; }`

	err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte(jsContent), 0644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "style.css"), []byte(cssContent), 0644)
	assert.NoError(t, err)

	// Create subdirectory with file
	assetsDir := filepath.Join(tmpDir, "assets")
	err = os.Mkdir(assetsDir, 0755)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(assetsDir, "logo.png"), []byte("fake-png-data"), 0644)
	assert.NoError(t, err)

	return tmpDir
}

func TestServeStatic_BasicFileServing(t *testing.T) {
	tmpDir := setupTestStaticDir(t)

	app := NewRestApp(RestAppOptions{
		Name: "Test",
		Port: 8080,
	})

	err := app.ServeStatic(StaticConfig{
		Prefix:    "/",
		Directory: tmpDir,
	})
	assert.NoError(t, err)

	// Test serving index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.EchoApp.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "SPA Index")
}

func TestServeStatic_SPAMode(t *testing.T) {
	tmpDir := setupTestStaticDir(t)

	app := NewRestApp(RestAppOptions{
		Name: "Test",
		Port: 8080,
	})

	err := app.ServeStatic(StaticConfig{
		Prefix:    "/",
		Directory: tmpDir,
		EnableSPA: true,
	})
	assert.NoError(t, err)

	// Test that non-existent route returns index.html
	req := httptest.NewRequest(http.MethodGet, "/app/dashboard", nil)
	rec := httptest.NewRecorder()
	app.EchoApp.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "SPA Index")
}

func TestServeStatic_HeadersConfiguration(t *testing.T) {
	tmpDir := setupTestStaticDir(t)

	app := NewRestApp(RestAppOptions{
		Name: "Test",
		Port: 8080,
	})

	err := app.ServeStatic(StaticConfig{
		Prefix:    "/",
		Directory: tmpDir,
		EnableSPA: true,
		Headers: map[string]string{
			"X-Custom-Header": "test-value",
		},
		IndexHeaders: map[string]string{
			"Cache-Control": "no-cache",
		},
		AssetHeaders: map[string]string{
			"Cache-Control": "max-age=3600",
		},
	})
	assert.NoError(t, err)

	// Test index headers
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	rec := httptest.NewRecorder()
	app.EchoApp.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test-value", rec.Header().Get("X-Custom-Header"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))

	// Test asset headers (JS file)
	req = httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec = httptest.NewRecorder()
	app.EchoApp.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test-value", rec.Header().Get("X-Custom-Header"))
	assert.Equal(t, "max-age=3600", rec.Header().Get("Cache-Control"))
}

func TestServeStatic_CustomHeaderMatcher(t *testing.T) {
	tmpDir := setupTestStaticDir(t)

	app := NewRestApp(RestAppOptions{
		Name: "Test",
		Port: 8080,
	})

	err := app.ServeStatic(StaticConfig{
		Prefix:    "/",
		Directory: tmpDir,
		Headers: map[string]string{
			"X-Base": "base-value",
		},
		HeaderMatcher: func(requestPath, filePath string) map[string]string {
			if requestPath == "/app.js" {
				return map[string]string{
					"X-Custom": "js-file",
				}
			}
			return nil
		},
	})
	assert.NoError(t, err)

	// Test custom matcher for JS file
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	app.EchoApp.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "base-value", rec.Header().Get("X-Base"))
	assert.Equal(t, "js-file", rec.Header().Get("X-Custom"))
}

func TestSecureStaticHeaders(t *testing.T) {
	headers := SecureStaticHeaders()

	assert.Equal(t, "SAMEORIGIN", headers["X-Frame-Options"])
	assert.Equal(t, "nosniff", headers["X-Content-Type-Options"])
	assert.Equal(t, "strict-origin-when-cross-origin", headers["Referrer-Policy"])
}

func TestSPAIndexHeaders(t *testing.T) {
	headers := SPAIndexHeaders()

	assert.Equal(t, "no-cache, no-store, must-revalidate", headers["Cache-Control"])
	assert.Equal(t, "no-cache", headers["Pragma"])
	assert.Equal(t, "0", headers["Expires"])
	// Should also include secure headers
	assert.Equal(t, "SAMEORIGIN", headers["X-Frame-Options"])
}

func TestCachedAssetHeaders(t *testing.T) {
	headers := CachedAssetHeaders()

	assert.Equal(t, "public, max-age=31536000, immutable", headers["Cache-Control"])
	// Should also include secure headers
	assert.Equal(t, "SAMEORIGIN", headers["X-Frame-Options"])
}

func TestIsAssetFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/app.js", true},
		{"/style.css", true},
		{"/logo.png", true},
		{"/index.html", false},
		{"/data.json", true},
		{"/font.woff2", true},
		{"/video.mp4", true},
		{"/unknown.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isAssetFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeHeaders(t *testing.T) {
	base := map[string]string{
		"X-Base":   "base",
		"X-Common": "from-base",
	}

	override := map[string]string{
		"X-Override": "override",
		"X-Common":   "from-override",
	}

	result := mergeHeaders(base, override)

	assert.Equal(t, "base", result["X-Base"])
	assert.Equal(t, "override", result["X-Override"])
	assert.Equal(t, "from-override", result["X-Common"]) // Override wins
}

func TestServeStatic_InvalidDirectory(t *testing.T) {
	app := NewRestApp(RestAppOptions{
		Name: "Test",
		Port: 8080,
	})

	err := app.ServeStatic(StaticConfig{
		Prefix:    "/",
		Directory: "/nonexistent/directory",
	})

	assert.Error(t, err)
	assert.IsType(t, &echo.HTTPError{}, err)
}

func TestServeStatic_EmptyDirectory(t *testing.T) {
	app := NewRestApp(RestAppOptions{
		Name: "Test",
		Port: 8080,
	})

	err := app.ServeStatic(StaticConfig{
		Prefix: "/",
	})

	assert.Error(t, err)
}
