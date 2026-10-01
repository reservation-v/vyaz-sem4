package http

import (
	"embed"
	"fmt"
	"html/template"
	nethttp "net/http"
	"time"
)

//go:embed web/templates/*.html
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

// basePageData — общие пооля всех страниц.
type basePageData struct {
	Title     string
	SiteTitle string
	ActiveNav string
}

// loadTemplates разбирает layout и все страницы сайта.
func loadTemplates() (map[string]*template.Template, error) {
	pages := []string{"home", "soon"}
	result := make(map[string]*template.Template, len(pages))

	for _, page := range pages {
		tmpl, err := template.New("layout.html").Funcs(template.FuncMap{
			"currentYear": func() int { return time.Now().Year() },
		}).ParseFS(templateFS, "web/templates/layout.html", "web/templates/"+page+".html")
		if err != nil {
			return nil, fmt.Errorf("http: parse %s: %w", page, err)
		}
		result[page] = tmpl
	}

	return result, nil
}

// renderPage отрисовывает страницу в общем layout.
func (s *Server) renderPage(w nethttp.ResponseWriter, page string, status int, data any) {
	tmpl, ok := s.templates[page]
	if !ok {
		nethttp.Error(w, "шаблон не найден", nethttp.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		nethttp.Error(w, "не удалось отрисовать страницу", nethttp.StatusInternalServerError)
	}
}
