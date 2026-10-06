package http

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	nethttp "net/http"
	"strings"
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
	Visits    int64 // счётчик просмотров страниц, заполняется сервером
}

// layoutFuncs — функции, доступные всем шаблонам сайта.
func layoutFuncs() template.FuncMap {
	return template.FuncMap{
		"currentYear": func() int { return time.Now().Year() },
		// money форматирует число с разделителем тысяч: 1800 → «1 800».
		// Принимает int и int64 (счётчик посещений хранится как int64).
		"money": func(n any) string {
			s := fmt.Sprint(n)
			var b strings.Builder
			for i, ch := range s {
				if i > 0 && (len(s)-i)%3 == 0 {
					b.WriteByte(' ')
				}
				b.WriteRune(ch)
			}
			return b.String()
		},
		// shortDate переводит ГГГГ-ММ-ДД в ДД.ММ.ГГГГ для таблицы броней.
		"shortDate": func(value string) string {
			t, err := time.Parse("2006-01-02", value)
			if err != nil {
				return value
			}
			return t.Format("02.01.2006")
		},
	}
}

// loadTemplates разбирает layout и все страницы сайта.
func loadTemplates() (map[string]*template.Template, error) {
	pages := []string{"home", "soon", "about", "office", "tariffs", "news", "achievements", "contacts", "rules", "faq", "booking", "residents"}
	result := make(map[string]*template.Template, len(pages))

	for _, page := range pages {
		tmpl, err := template.New("layout.html").Funcs(layoutFuncs()).ParseFS(templateFS, "web/templates/layout.html", "web/templates/"+page+".html")
		if err != nil {
			return nil, fmt.Errorf("http: parse %s: %w", page, err)
		}
		result[page] = tmpl
	}

	return result, nil
}

// renderPage отрисовывает страницу в общем layout.
// Шаблон выполняется в буфер: при ошибке рендера отдаём 500 целиком,
// а не обрезанную страницу с уже записанным кодом 200.
func (s *Server) renderPage(w nethttp.ResponseWriter, page string, status int, data any) {
	tmpl, ok := s.templates[page]
	if !ok {
		nethttp.Error(w, "шаблон не найден", nethttp.StatusInternalServerError)
		return
	}

	var body bytes.Buffer
	if err := tmpl.ExecuteTemplate(&body, "layout", data); err != nil {
		nethttp.Error(w, "не удалось отрисовать страницу", nethttp.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}
