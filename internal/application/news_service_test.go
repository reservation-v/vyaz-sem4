package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"coworking/internal/domain"
)

type fakeSource struct {
	name  string
	items []domain.ExternalNews
	err   error
	calls int
}

func (f *fakeSource) Source() string { return f.name }

func (f *fakeSource) Fetch(context.Context) ([]domain.ExternalNews, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.ExternalNews(nil), f.items...), nil
}

func newsItem(title, link string, at time.Time) domain.ExternalNews {
	return domain.ExternalNews{Title: title, Link: link, Source: "тест", PublishedAt: at}
}

func TestNewsServiceDedupesAndSorts(t *testing.T) {
	older := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

	first := &fakeSource{name: "a", items: []domain.ExternalNews{
		newsItem("Старая", "https://example.com/1", older),
		newsItem("Новая", "https://example.com/2", newer),
	}}
	second := &fakeSource{name: "b", items: []domain.ExternalNews{
		newsItem("Дубль", "https://example.com/2", newer),
	}}

	service := NewNewsService(time.Minute, first, second)
	items, err := service.ExternalNews(context.Background())
	if err != nil {
		t.Fatalf("ExternalNews: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("получено %d новостей, ожидалось 2 (дубль должен исчезнуть)", len(items))
	}
	if items[0].Title != "Новая" {
		t.Errorf("первой должна быть свежая новость, получено %q", items[0].Title)
	}
}

func TestNewsServiceUsesOtherSourceOnError(t *testing.T) {
	broken := &fakeSource{name: "broken", err: errors.New("сеть недоступна")}
	ok := &fakeSource{name: "ok", items: []domain.ExternalNews{
		newsItem("Живая новость", "https://example.com/1", time.Now()),
	}}

	service := NewNewsService(time.Minute, broken, ok)
	items, err := service.ExternalNews(context.Background())
	if err != nil {
		t.Fatalf("ExternalNews: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("получено %d новостей, ожидалась 1", len(items))
	}
}

func TestNewsServiceFailsWhenAllSourcesFail(t *testing.T) {
	broken := &fakeSource{name: "broken", err: errors.New("сеть недоступна")}
	service := NewNewsService(time.Minute, broken)

	if _, err := service.ExternalNews(context.Background()); err == nil {
		t.Fatal("ожидалась ошибка, когда все источники недоступны")
	}
}

func TestNewsServiceCachesResult(t *testing.T) {
	source := &fakeSource{name: "a", items: []domain.ExternalNews{
		newsItem("Новость", "https://example.com/1", time.Now()),
	}}

	service := NewNewsService(time.Minute, source)
	for i := 0; i < 3; i++ {
		if _, err := service.ExternalNews(context.Background()); err != nil {
			t.Fatalf("ExternalNews: %v", err)
		}
	}

	if source.calls != 1 {
		t.Errorf("источник опрошен %d раз, ожидался 1 (кэш)", source.calls)
	}
}
