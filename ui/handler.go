package ui

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
)

type Option func(*config)

type config struct {
	PreviewAPIBase  string `json:"preview_api_base"`
	TryItOutEnabled bool   `json:"try_it_out_enabled"`
	TryItOutBase    string `json:"try_it_out_base"`
}

func defaultConfig() config {
	return config{
		PreviewAPIBase: "/_shapeshifter/api",
		TryItOutBase:   "/",
	}
}

func WithPreviewAPIBase(path string) Option {
	return func(c *config) {
		if strings.TrimSpace(path) == "" {
			return
		}
		c.PreviewAPIBase = cleanBase(path)
	}
}

func WithTryItOut(enabled bool) Option {
	return func(c *config) {
		c.TryItOutEnabled = enabled
	}
}

func WithTryItOutBase(path string) Option {
	return func(c *config) {
		if strings.TrimSpace(path) == "" {
			return
		}
		c.TryItOutBase = cleanBase(path)
		if c.TryItOutBase != "/" {
			c.TryItOutBase += "/"
		}
	}
}

func Handler(opts ...Option) http.Handler {
	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return http.NotFoundHandler()
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "config.json" {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cfg)
			return
		}
		if path == "" {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-store")
		}
		files.ServeHTTP(w, r)
	})
}

func cleanBase(path string) string {
	path = "/" + strings.Trim(strings.TrimSpace(path), "/")
	if path == "/" {
		return path
	}
	return strings.TrimRight(path, "/")
}
