package realtime

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const defaultUIDistDir = "web/realtime-ui/dist"

//go:embed ui_fallback/index.html
var fallbackUI embed.FS

func newUIHandler() http.Handler {
	distDir := os.Getenv("ARRIVAL_REALTIME_UI_DIST_DIR")
	if distDir == "" {
		distDir = defaultUIDistDir
	}

	if hasIndex(distDir) {
		return newSPAHandler(http.FS(os.DirFS(distDir)))
	}

	sub, err := fs.Sub(fallbackUI, "ui_fallback")
	if err != nil {
		return http.NotFoundHandler()
	}
	return newSPAHandler(http.FS(sub))
}

func hasIndex(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

type spaHandler struct {
	files http.Handler
	fs    http.FileSystem
}

func newSPAHandler(fileSystem http.FileSystem) http.Handler {
	return &spaHandler{
		files: http.FileServer(fileSystem),
		fs:    fileSystem,
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	cleanPath := path.Clean(r.URL.Path)
	if cleanPath == "/" || cleanPath == "/index.html" {
		h.serveIndex(w, r)
		return
	}

	if strings.HasPrefix(cleanPath, "/assets/") || strings.Contains(path.Base(cleanPath), ".") {
		h.files.ServeHTTP(w, r)
		return
	}

	h.serveIndex(w, r)
}

func (h *spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	indexFile, err := h.fs.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer indexFile.Close()

	body, err := io.ReadAll(indexFile)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
