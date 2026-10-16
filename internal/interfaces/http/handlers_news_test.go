package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"coworking/internal/application"
	"coworking/internal/infrastructure/memory"
	"coworking/internal/infrastructure/rss"
)

const testNewsFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Тестовая лента</title>
    <item>
      <title>Первая новость</title>
      <link>https://example.com/1</link>
      <description><![CDATA[<p>Краткий текст</p>]]></description>
      <pubDate>Fri, 16 Oct 2026 08:29:06 +0300</pubDate>
    </item>
    <item>
      <title>Вторая новость</title>
      <link>https://example.com/2</link>
      <description>Ещё текст</description>
      <pubDate>Thu, 15 Oct 2026 08:00:00 +0300</pubDate>
    </item>
  </channel>
</rss>`

func newTestServer(t *testing.T, opts ...Option) *Server {
	t.Helper()

	catalog := application.NewServiceCatalog(memory.NewServiceRepository())
	srv, err := NewServer(catalog, opts...)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func TestExternalNewsEndpoint(t *testing.T) {
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(testNewsFeed))
	}))
	defer feedServer.Close()

	news := application.NewNewsService(time.Minute, rss.NewFeed("тест", feedServer.URL))
	srv := newTestServer(t, WithNewsService(news))

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/news/external", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d, ожидался 200", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Errorf("Content-Type = %q, ожидался JSON", contentType)
	}

	var items []externalNewsView
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("разбор JSON: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("новостей = %d, ожидалось 2", len(items))
	}
	if items[0].Source != "тест" {
		t.Errorf("источник = %q, ожидался «тест»", items[0].Source)
	}
	if items[0].Title != "Первая новость" {
		t.Errorf("заголовок = %q", items[0].Title)
	}
	if items[0].Published == "" {
		t.Error("дата публикации пустая")
	}
}

func TestExternalNewsEndpointWithoutService(t *testing.T) {
	srv := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/news/external", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("код ответа = %d, ожидался 503", rec.Code)
	}
}

func TestNewsPageRenders(t *testing.T) {
	srv := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/news", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d, ожидался 200", rec.Code)
	}

	body := rec.Body.String()
	markers := []string{
		`id="news-app"`,
		`data-tab="external"`,
		`data-tab="own"`,
		`id="external-news"`,
		`id="external-refresh"`,
		`<h1 class="page-title">Новости</h1>`,
	}
	for _, marker := range markers {
		if !strings.Contains(body, marker) {
			t.Errorf("в HTML страницы нет %q", marker)
		}
	}
}
