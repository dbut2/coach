package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets
var assetsFS embed.FS

// Assets is the embedded static asset tree (compiled CSS, self-hosted JS and
// fonts), rooted so that paths look like "css/app.css", "js/htmx.min.js", etc.
func Assets() fs.FS {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}

// AssetsHandler serves the embedded assets, intended to be mounted at /assets.
func AssetsHandler() http.Handler {
	return http.FileServer(http.FS(Assets()))
}
