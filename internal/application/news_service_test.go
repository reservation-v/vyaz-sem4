package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"coworking/internal/domain"
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
	return ErrNewsNotFound
}

func (r *fakeNewsRepo) ListNewsSources(context.Context) ([]domain.NewsSource, error) {
	return append([]domain.NewsSource(nil), r.sources...), nil
}

func (r *fakeNewsRepo) SaveNewsSource(_ context.Context, source domain.NewsSource) (domain.NewsSource, error) {
	for _, existing := range r.sources {
		if existing.URL == source.URL {
			return domain.NewsSource{}, ErrDuplicateNewsSource
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
	return ErrNewsSourceNotFound
}

type fakeSource struct {
	name  string
	items []domain.NewsItem
	err   error
	calls int
}

func (f *fakeSource) Source() string { return f.name }

func (f *fakeSource) Fetch(context.Context) ([]domain.NewsItem, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.NewsItem(nil), f.items...), nil
}

func feedItem(title, link string, at time.Time) domain.NewsItem {
	return domain.NewsItem{Title: title, Link: link, PublishedAt: at}
}

func testFeedFactory(feeds map[string]*fakeSource) NewsFeedFactory {
	return func(title, url string) ExternalNewsSource {
		if feed, ok := feeds[url]; ok {
			return feed
		}
		return &fakeSource{name: title}
	}
}

func TestNewsServiceMergesOwnAndExternalNews(t *testing.T) {
	repo := newFakeNewsRepo()
	repo.items = []domain.NewsItem{
		{ID: 1, Title: "Своя новость", Summary: "Текст", PublishedAt: time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)},
	}
	repo.sources = []domain.NewsSource{
		{ID: 1, Title: "vc.ru", URL: "https://vc.ru/rss", IsActive: true},
	}

	external := &fakeSource{name: "vc.ru", items: []domain.NewsItem{
		feedItem("Внешняя новость", "https://vc.ru/1", time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)),
	}}

	service := NewNewsService(repo, testFeedFactory(map[string]*fakeSource{"https://vc.ru/rss": external}), time.Minute)
	feed, err := service.News(context.Background())
	if err != nil {
		t.Fatalf("News: %v", err)
	}

	if len(feed.Items) != 2 {
		t.Fatalf("новостей = %d, ожидалось 2", len(feed.Items))
	}
	if feed.Items[0].Title != "Внешняя новость" {
		t.Errorf("первой должна быть свежая внешняя новость, получено %q", feed.Items[0].Title)
	}
	if !feed.Items[1].Own || feed.Items[1].Source != "Место" {
		t.Errorf("своя новость не подписана источником «Место»: %+v", feed.Items[1])
	}
	if len(feed.Sources) != 2 || !feed.Sources[0].Own {
		t.Errorf("ожидался список источников: «Место» и внешний, получено %+v", feed.Sources)
	}
}

func TestNewsServiceDedupesAndSorts(t *testing.T) {
	repo := newFakeNewsRepo()
	older := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

	repo.sources = []domain.NewsSource{
		{ID: 1, Title: "a", URL: "https://a.example/rss", IsActive: true},
		{ID: 2, Title: "b", URL: "https://b.example/rss", IsActive: true},
	}

	first := &fakeSource{name: "a", items: []domain.NewsItem{
		feedItem("Старая", "https://example.com/1", older),
		feedItem("Новая", "https://example.com/2", newer),
	}}
	second := &fakeSource{name: "b", items: []domain.NewsItem{
		feedItem("Дубль", "https://example.com/2", newer),
	}}

	service := NewNewsService(repo, testFeedFactory(map[string]*fakeSource{
		"https://a.example/rss": first,
		"https://b.example/rss": second,
	}), time.Minute)

	feed, err := service.News(context.Background())
	if err != nil {
		t.Fatalf("News: %v", err)
	}

	if len(feed.Items) != 2 {
		t.Fatalf("новостей = %d, ожидалось 2 (дубль должен исчезнуть)", len(feed.Items))
	}
	if feed.Items[0].Title != "Новая" {
		t.Errorf("первой должна быть свежая новость, получено %q", feed.Items[0].Title)
	}
}

func TestNewsServiceKeepsWorkingSourceOnError(t *testing.T) {
	repo := newFakeNewsRepo()
	repo.sources = []domain.NewsSource{
		{ID: 1, Title: "broken", URL: "https://broken.example/rss", IsActive: true},
		{ID: 2, Title: "ok", URL: "https://ok.example/rss", IsActive: true},
	}

	broken := &fakeSource{name: "broken", err: errors.New("сеть недоступна")}
	ok := &fakeSource{name: "ok", items: []domain.NewsItem{
		feedItem("Живая новость", "https://example.com/1", time.Now()),
	}}

	service := NewNewsService(repo, testFeedFactory(map[string]*fakeSource{
		"https://broken.example/rss": broken,
		"https://ok.example/rss":     ok,
	}), time.Minute)

	feed, err := service.News(context.Background())
	if err != nil {
		t.Fatalf("News: %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("новостей = %d, ожидалась 1", len(feed.Items))
	}
	if feed.Sources[1].Error == "" {
		t.Error("у недоступного источника не выставлена ошибка")
	}
}

func TestNewsServiceCachesEachSource(t *testing.T) {
	repo := newFakeNewsRepo()
	repo.sources = []domain.NewsSource{
		{ID: 1, Title: "a", URL: "https://a.example/rss", IsActive: true},
	}

	source := &fakeSource{name: "a", items: []domain.NewsItem{
		feedItem("Новость", "https://example.com/1", time.Now()),
	}}

	service := NewNewsService(repo, testFeedFactory(map[string]*fakeSource{"https://a.example/rss": source}), time.Minute)
	for i := 0; i < 3; i++ {
		if _, err := service.News(context.Background()); err != nil {
			t.Fatalf("News: %v", err)
		}
	}

	if source.calls != 1 {
		t.Errorf("источник опрошен %d раз, ожидался 1 (кэш)", source.calls)
	}
}

func TestCreateNewsValidation(t *testing.T) {
	repo := newFakeNewsRepo()
	service := NewNewsService(repo, nil, time.Minute)

	if _, err := service.CreateNews(context.Background(), NewsInput{Title: "   "}); !errors.Is(err, ErrInvalidNewsTitle) {
		t.Errorf("пустой заголовок: ошибка = %v, ожидалась ErrInvalidNewsTitle", err)
	}
	if _, err := service.CreateNews(context.Background(), NewsInput{Title: "Новость", PublishedAt: "вчера"}); !errors.Is(err, ErrInvalidNewsDate) {
		t.Errorf("плохая дата: ошибка = %v, ожидалась ErrInvalidNewsDate", err)
	}

	item, err := service.CreateNews(context.Background(), NewsInput{Title: "Новость", Summary: "Текст", PublishedAt: "2026-10-18"})
	if err != nil {
		t.Fatalf("CreateNews: %v", err)
	}
	if item.ID == 0 || !item.Own {
		t.Errorf("новость сохранена неверно: %+v", item)
	}
	if len(repo.items) != 1 {
		t.Errorf("в хранилище %d новостей, ожидалась 1", len(repo.items))
	}
}

func TestAddSourceValidationAndDuplicate(t *testing.T) {
	repo := newFakeNewsRepo()
	service := NewNewsService(repo, nil, time.Minute)

	if _, err := service.AddSource(context.Background(), NewsSourceInput{URL: "ftp://example.com/rss"}); !errors.Is(err, ErrInvalidNewsSource) {
		t.Errorf("ftp: ошибка = %v, ожидалась ErrInvalidNewsSource", err)
	}
	if _, err := service.AddSource(context.Background(), NewsSourceInput{URL: "not a url"}); !errors.Is(err, ErrInvalidNewsSource) {
		t.Errorf("мусор: ошибка = %v, ожидалась ErrInvalidNewsSource", err)
	}

	source, err := service.AddSource(context.Background(), NewsSourceInput{Title: "Пример", URL: "https://example.com/rss"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if source.ID == 0 || source.Title != "Пример" {
		t.Errorf("источник сохранён неверно: %+v", source)
	}

	if _, err := service.AddSource(context.Background(), NewsSourceInput{Title: "Дубль", URL: "https://example.com/rss"}); !errors.Is(err, ErrDuplicateNewsSource) {
		t.Errorf("дубль: ошибка = %v, ожидалась ErrDuplicateNewsSource", err)
	}
}
