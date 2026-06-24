package serve

import (
	"embed"
	"io/fs"
	"mime"
)

// staticAssets embeds the built SPA bundle. The web client compiles into
// `web/dist/` via `npm --prefix web run build`. A placeholder `.keep` keeps
// the directory present in git so `go build` always succeeds even when the
// SPA has not been built.
//
//go:embed all:web_dist
var staticAssets embed.FS

// init registers MIME types Go's stdlib does not know about so the embedded
// FileServer serves them with the correct Content-Type. .webmanifest in
// particular is unregistered on most platforms; without this a PWA install
// prompt is silently suppressed because the manifest is served as text/plain.
func init() {
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// EmbeddedSPA returns an fs.FS rooted at the SPA's index.html, or nil when
// nothing has been built yet (in which case the server falls back to the
// placeholder HTML so users know what to run).
func EmbeddedSPA() fs.FS {
	sub, err := fs.Sub(staticAssets, "web_dist")
	if err != nil {
		return nil
	}
	// Sniff for index.html; treat its absence as "not built".
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}
