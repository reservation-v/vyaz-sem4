package http

import (
	nethttp "net/http"

	"coworking/docs"
)

// handleDocs отдаёт встроенную HTML-документацию проекта.
func (s *Server) handleDocs(w nethttp.ResponseWriter, r *nethttp.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(nethttp.StatusOK)
	_, _ = w.Write(docs.HTML)
}
