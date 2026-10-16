package http

import (
	"encoding/json"
	nethttp "net/http"
	"time"
)

// externalNewsLimit — сколько новостей отдаём странице.
const externalNewsLimit = 12

// externalNewsView — новость внешнего источника для отдачи в JSON.
type externalNewsView struct {
	Title     string `json:"title"`
	Link      string `json:"link"`
	Source    string `json:"source"`
	Summary   string `json:"summary"`
	Published string `json:"published"`
}

// handleExternalNews отдаёт странице новости внешних RSS-лент в виде JSON.
// Ленты забирает сервер (у него нет проблем с CORS), браузер обращается к нам.
func (s *Server) handleExternalNews(w nethttp.ResponseWriter, r *nethttp.Request) {
	if s.news == nil {
		nethttp.Error(w, "внешние новости недоступны", nethttp.StatusServiceUnavailable)
		return
	}

	items, err := s.news.ExternalNews(r.Context())
	if err != nil {
		nethttp.Error(w, "не удалось загрузить внешние новости", nethttp.StatusBadGateway)
		return
	}

	if len(items) > externalNewsLimit {
		items = items[:externalNewsLimit]
	}

	views := make([]externalNewsView, 0, len(items))
	for _, item := range items {
		published := ""
		if !item.PublishedAt.IsZero() {
			published = item.PublishedAt.Format(time.RFC3339)
		}
		views = append(views, externalNewsView{
			Title:     item.Title,
			Link:      item.Link,
			Source:    item.Source,
			Summary:   item.Summary,
			Published: published,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(views); err != nil {
		return
	}
}
