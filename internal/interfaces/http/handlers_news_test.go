package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"coworking/internal/application"
	"coworking/internal/domain"
	"coworking/internal/infrastructure/memory"
	"coworking/internal/infrastructure/rss"
)

type fakeNewsRepo struct {
	items        []domain.NewsItem
	sources      []domain.NewsSource
	nextNewsID   int
	nextSourceID int
}

func newFakeNewsRepo() *fakeNewsRepo {
	return &fakeNewsRepo{nextNewsID: 1, nextSourceID: 1}
}

func (r *fakeNewsRepo) ListNews(context.Context, int) ([]domain.NewsItem, error) {
	return append([]domain.NewsItem(nil), r.items...), nil
}

func (r *fakeNewsRepo) SaveNews(_ context.Context, item domain.NewsItem) (domain.NewsItem, error) {
	item.ID = r.nextNewsID
	r.nextNewsID++
	r.items = append([]domain.NewsItem{item}, r.items...)
	return item, nil
}

func (r *fakeNewsRepo) DeleteNews(_ context.Context, id int) error {
	for i, item := range r.items {
		if item.ID == id {
			r.items = append(r.items[:i], r.items[i+1:]...)
			return nil
		}
	}
	return application.ErrNewsNotFound
}

func (r *fakeNewsRepo) ListNewsSources(context.Context) ([]domain.NewsSource, error) {
	return append([]domain.NewsSource(nil), r.sources...), nil
}

func (r *fakeNewsRepo) SaveNewsSource(_ context.Context, source domain.NewsSource) (domain.NewsSource, error) {
	for _, existing := range r.sources {
		if existing.URL == source.URL {
			return domain.NewsSource{}, application.ErrDuplicateNewsSource
		}
	}
	source.ID = r.nextSourceID
	r.nextSourceID++
	r.sources = append(r.sources, source)
	return source, nil
}

func (r *fakeNewsRepo) DeleteNewsSource(_ context.Context, id int) error {
	for i, source := range r.sources {
		if source.ID == id {
			r.sources = append(r.sources[:i], r.sources[i+1:]...)
			return nil
		}
	}
	return application.ErrNewsSourceNotFound
}

type fakeNewsFeed struct {
	name  string
	items []domain.NewsItem
	err   error
}

func (f *fakeNewsFeed) Source() string { return f.name }

func (f *fakeNewsFeed) Fetch(context.Context) ([]domain.NewsItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.NewsItem(nil), f.items...), nil
}

func newTestNewsService(repo *fakeNewsRepo, feeds map[string]*fakeNewsFeed) *application.NewsService {
	factory := func(title, feedURL string) application.ExternalNewsSource {
		if feed, ok := feeds[feedURL]; ok {
			return feed
		}
		return &fakeNewsFeed{name: title}
	}
	return application.NewNewsService(repo, factory, time.Minute)
}

func newTestServer(t *testing.T, opts ...Option) *Server {
	t.Helper()

	catalog := application.NewServiceCatalog(memory.NewServiceRepository())
	srv, err := NewServer(catalog, opts...)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func TestNewsPageRendersMergedFeed(t *testing.T) {
	repo := newFakeNewsRepo()
	repo.items = []domain.NewsItem{
		{ID: 1, Title: "Своя новость", Summary: "Текст своей новости", PublishedAt: time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)},
	}
	repo.sources = []domain.NewsSource{
		{ID: 10, Title: "Тестовая лента", URL: "https://test.example/rss", IsActive: true},
	}

	feed := &fakeNewsFeed{name: "Тестовая лента", items: []domain.NewsItem{
		{Title: "Внешняя новость", Link: "https://example.com/1", Summary: "Текст из ленты", PublishedAt: time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)},
	}}

	srv := newTestServer(t, WithNewsService(newTestNewsService(repo, map[string]*fakeNewsFeed{
		"https://test.example/rss": feed,
	})))

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/news", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d, ожидался 200", rec.Code)
	}

	body := rec.Body.String()
	markers := []string{
		`id="news-list"`,
		`Своя новость`,
		`Внешняя новость`,
		`news-source-own`,
		`news-source-external`,
		`Тестовая лента`,
		`data-tab="feed"`,
		`data-tab="add-news"`,
		`data-tab="add-source"`,
		`id="panel-add-news"`,
		`id="panel-add-source"`,
		`Рекомендуемые ленты`,
		`vc.ru · коворкинг`,
		`Inc. Russia`,
		`Добавить свою новость`,
		`Подключить RSS-ленту`,
		`id="rss-open"`,
		`id="rss-modal"`,
		`id="rss-download"`,
		`download="mesto-news.xml"`,
		`/news/rss.xml`,
		`<h1 class="page-title">Новости</h1>`,
	}
	for _, marker := range markers {
		if !strings.Contains(body, marker) {
			t.Errorf("в HTML страницы нет %q", marker)
		}
	}

	sourcesPos := strings.Index(body, "Подключённые источники")
	suggestionsPos := strings.Index(body, "Рекомендуемые ленты")
	if sourcesPos < 0 || suggestionsPos < 0 || sourcesPos > suggestionsPos {
		t.Error("сначала должен идти список подключённых источников, затем рекомендации")
	}
}

func TestNewsPageRendersWithoutService(t *testing.T) {
	srv := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/news", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d, ожидался 200", rec.Code)
	}

	body := rec.Body.String()
	for _, marker := range []string{`id="news-list"`, `Добавить свою новость`, `Подключить RSS-ленту`, `Рекомендуемые ленты`} {
		if !strings.Contains(body, marker) {
			t.Errorf("в HTML страницы нет %q", marker)
		}
	}
}

func TestNewsPageMarksConnectedSuggestion(t *testing.T) {
	repo := newFakeNewsRepo()
	repo.sources = []domain.NewsSource{
		{ID: 1, Title: "vc.ru · коворкинг", URL: "https://vc.ru/rss/tag/coworking", IsActive: true},
	}

	srv := newTestServer(t, WithNewsService(newTestNewsService(repo, nil)))

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/news", nil))

	body := rec.Body.String()
	if !strings.Contains(body, "Уже подключена") {
		t.Error("подключённая лента не помечена в списке подсказок")
	}
}

func TestNewsAPIReturnsMergedFeed(t *testing.T) {
	repo := newFakeNewsRepo()
	repo.items = []domain.NewsItem{
		{ID: 1, Title: "Своя новость", PublishedAt: time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)},
	}
	repo.sources = []domain.NewsSource{
		{ID: 10, Title: "Тестовая лента", URL: "https://test.example/rss", IsActive: true},
	}

	feed := &fakeNewsFeed{name: "Тестовая лента", items: []domain.NewsItem{
		{Title: "Внешняя новость", Link: "https://example.com/1", PublishedAt: time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)},
	}}

	srv := newTestServer(t, WithNewsService(newTestNewsService(repo, map[string]*fakeNewsFeed{
		"https://test.example/rss": feed,
	})))

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/news", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d, ожидался 200", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Errorf("Content-Type = %q, ожидался JSON", contentType)
	}

	var payload newsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("разбор JSON: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("новостей = %d, ожидалось 2", len(payload.Items))
	}
	if payload.Items[0].Title != "Внешняя новость" {
		t.Errorf("первой должна быть свежая внешняя новость, получено %q", payload.Items[0].Title)
	}
	if payload.Items[1].Source != "Место" || !payload.Items[1].Own {
		t.Errorf("своя новость должна быть подписана источником «Место»: %+v", payload.Items[1])
	}
	if len(payload.Sources) != 2 {
		t.Fatalf("источников = %d, ожидалось 2", len(payload.Sources))
	}
}

func TestNewsAPIReturns503WithoutService(t *testing.T) {
	srv := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/news", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("код ответа = %d, ожидался 503", rec.Code)
	}
}

func TestNewsRSSPublishesOwnNews(t *testing.T) {
	repo := newFakeNewsRepo()
	repo.items = []domain.NewsItem{
		{ID: 1, Title: "Своя новость", Summary: "Текст", PublishedAt: time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)},
	}

	srv := newTestServer(t,
		WithNewsService(newTestNewsService(repo, nil)),
		WithNewsPublisher(rss.NewPublisher("Место", "Новости коворкинга", "ru")),
	)

	rec := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://mesto.example/news/rss.xml", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	srv.Router().ServeHTTP(rec, request)

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d, ожидался 200", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/rss+xml") {
		t.Errorf("Content-Type = %q, ожидался RSS", contentType)
	}

	body := rec.Body.String()
	for _, marker := range []string{
		`<rss version="2.0">`,
		`<title>Своя новость</title>`,
		`https://mesto.example/news#news-1`,
		`<guid isPermaLink="true">https://mesto.example/news#news-1</guid>`,
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("в RSS нет %q\n%s", marker, body)
		}
	}
}

func TestCreateNewsAddsToRepo(t *testing.T) {
	repo := newFakeNewsRepo()
	srv := newTestServer(t, WithNewsService(newTestNewsService(repo, nil)))

	form := url.Values{
		"title":        {"Новая новость"},
		"summary":      {"Текст новости"},
		"published_at": {"2026-10-18"},
	}
	request := httptest.NewRequest(http.MethodPost, "/news", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, request)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("код ответа = %d, ожидался 303", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/news?notice=news-created" {
		t.Errorf("Location = %q", location)
	}
	if len(repo.items) != 1 || repo.items[0].Title != "Новая новость" {
		t.Errorf("новость не сохранена: %+v", repo.items)
	}
}

func TestCreateNewsRejectsEmptyTitle(t *testing.T) {
	repo := newFakeNewsRepo()
	srv := newTestServer(t, WithNewsService(newTestNewsService(repo, nil)))

	form := url.Values{"summary": {"Без заголовка"}}
	request := httptest.NewRequest(http.MethodPost, "/news", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, request)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("код ответа = %d, ожидался 422", rec.Code)
	}
	if len(repo.items) != 0 {
		t.Errorf("новость не должна сохраняться: %+v", repo.items)
	}
	if !strings.Contains(rec.Body.String(), "Укажите заголовок новости") {
		t.Error("на странице нет сообщения об ошибке")
	}
}

func TestAddNewsSourceAddsToRepo(t *testing.T) {
	repo := newFakeNewsRepo()
	srv := newTestServer(t, WithNewsService(newTestNewsService(repo, nil)))

	form := url.Values{
		"title": {"Хабр"},
		"url":   {"https://habr.com/ru/rss/news/"},
	}
	request := httptest.NewRequest(http.MethodPost, "/news/sources", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, request)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("код ответа = %d, ожидался 303", rec.Code)
	}
	if len(repo.sources) != 1 || repo.sources[0].Title != "Хабр" {
		t.Errorf("источник не сохранён: %+v", repo.sources)
	}
}

func TestAddNewsSourceRejectsDuplicate(t *testing.T) {
	repo := newFakeNewsRepo()
	repo.sources = []domain.NewsSource{{ID: 1, Title: "Хабр", URL: "https://habr.com/ru/rss/news/", IsActive: true}}
	srv := newTestServer(t, WithNewsService(newTestNewsService(repo, nil)))

	form := url.Values{"url": {"https://habr.com/ru/rss/news/"}}
	request := httptest.NewRequest(http.MethodPost, "/news/sources", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, request)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("код ответа = %d, ожидался 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "уже подключена") {
		t.Error("на странице нет сообщения о дубле")
	}
}
