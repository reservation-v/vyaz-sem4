package application

import (
	"context"
	"sort"
	"sync"
	"time"

	"coworking/internal/domain"
)

// ExternalNewsSource — порт внешнего источника новостей (RSS-лента другого приложения).
type ExternalNewsSource interface {
	// Source возвращает название источника для подписи под новостью.
	Source() string
	// Fetch забирает свежие новости источника.
	Fetch(ctx context.Context) ([]domain.ExternalNews, error)
}

// NewsService — use case: собрать внешние новости из нескольких источников.
// Результат ненадолго кэшируется, чтобы не дёргать чужие сайты на каждый показ
// страницы. Так приложение обменивается данными с чужой RSS-лентой через сервер,
// а не из браузера (браузеру мешает CORS).
type NewsService struct {
	sources []ExternalNewsSource
	ttl     time.Duration

	mu       sync.Mutex
	cached   []domain.ExternalNews
	cachedAt time.Time
}

// perSourceLimit — сколько свежих новостей берём с одного источника. Ограничение
// нужно, чтобы очень говорливая лента не вытеснила тематические новости другой.
const perSourceLimit = 12

// NewNewsService создаёт сценарий с источниками и временем жизни кэша.
func NewNewsService(ttl time.Duration, sources ...ExternalNewsSource) *NewsService {
	return &NewsService{sources: sources, ttl: ttl}
}

// ExternalNews возвращает новости всех источников: без дублей и по убыванию даты.
func (s *NewsService) ExternalNews(ctx context.Context) ([]domain.ExternalNews, error) {
	s.mu.Lock()
	cached := s.cached
	cachedAt := s.cachedAt
	s.mu.Unlock()

	if len(cached) > 0 && time.Since(cachedAt) < s.ttl {
		return append([]domain.ExternalNews(nil), cached...), nil
	}

	var items []domain.ExternalNews
	var firstErr error

	for _, source := range s.sources {
		got, err := source.Fetch(ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(got) > perSourceLimit {
			got = got[:perSourceLimit]
		}
		items = append(items, got...)
	}

	if len(items) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, nil
	}

	items = dedupeNews(items)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})

	s.mu.Lock()
	s.cached = items
	s.cachedAt = time.Now()
	s.mu.Unlock()

	return append([]domain.ExternalNews(nil), items...), nil
}

// dedupeNews убирает повторы по ссылке, сохраняя порядок.
func dedupeNews(items []domain.ExternalNews) []domain.ExternalNews {
	seen := make(map[string]struct{}, len(items))
	result := items[:0]

	for _, item := range items {
		if item.Link != "" {
			if _, ok := seen[item.Link]; ok {
				continue
			}
			seen[item.Link] = struct{}{}
		}
		result = append(result, item)
	}

	return result
}
