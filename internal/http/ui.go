package httpserver

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	assets "github.com/takayoshiotake/shiroyagi"
)

var uiTemplates = template.Must(template.ParseFS(
	assets.Files,
	"web/templates/*.html",
	"web/templates/partials/*.html",
))

var staticAssets = map[string]struct {
	path        string
	contentType string
}{
	"css/app.css":    {path: "static/css/app.css", contentType: "text/css; charset=utf-8"},
	"js/htmx.min.js": {path: "static/js/htmx.min.js", contentType: "text/javascript; charset=utf-8"},
}

func renderUI(w http.ResponseWriter, status int, name string, data any) error {
	var output bytes.Buffer
	if err := uiTemplates.ExecuteTemplate(&output, name, data); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := output.WriteTo(w)
	return err
}

func handleStaticAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := staticAssets[r.PathValue("path")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	content, err := fs.ReadFile(assets.Files, asset.path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", asset.contentType)
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, r.PathValue("path"), time.Time{}, bytes.NewReader(content))
}
