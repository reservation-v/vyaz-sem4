package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"coworking/internal/domain"
)

// NewsRepository — порт хранилища своих новостей и подключённых RSS-лент.
type NewsRepository interface {
	ListNews(ctx context.Context, limit int) ([]domain.NewsItem, error)
	SaveNews(ctx context.Context, item domain.NewsItem) (domain.NewsItem, error)
	DeleteNews(ctx context.Context, id int) error

	ListNewsSources(ctx context.Context) ([]domain.NewsSource, error)
	SaveNewsSource(ctx context.Context, source domain.NewsSource) (domain.NewsSource, error)
	DeleteNewsSource(ctx context.Context, id int) error
}

// ExternalNewsSource — порт внешнего источника новостей (RSS- или Atom-ленты).
type ExternalNewsSource interface {
	// Source возвращает название источника для подписи под новостью.
	Source() string
	// Fetch забирает свежие новости источника.
	Fetch(ctx context.Context) ([]domain.NewsItem, error)
}

// NewsFeedFactory создаёт источник по названию и адресу ленты.
type NewsFeedFactory func(title, url string) ExternalNewsSource

// NewsFeedPublisher публикует свои новости в формате RSS для других приложений.
type NewsFeedPublisher interface {
	Publish(items []domain.NewsItem, baseURL string) ([]byte, error)
}

// Ошибки сценариев новостей (handler превращает их в сообщения на странице).
var (
	ErrNewsNotFound        = errors.New("news not found")
	ErrNewsSourceNotFound  = errors.New("news source not found")
	ErrInvalidNewsTitle    = errors.New("invalid news title")
	ErrInvalidNewsSummary  = errors.New("invalid news summary")
	ErrInvalidNewsDate     = errors.New("invalid news date")
	ErrInvalidNewsSource   = errors.New("invalid news source")
	ErrDuplicateNewsSource = errors.New("news source already exists")
	ErrTooManyNewsSources  = errors.New("too many news sources")
)

const (
	ownNewsLimit      = 30
	totalNewsLimit    = 100
	perSourceLimit    = 12
	maxNewsSources    = 20
	maxParallelFetch  = 8
	maxNewsTitleRunes = 200
	maxNewsSummary    = 2000
	maxSourceTitle    = 100
)

// SourceStatus — состояние одного источника для страницы новостей.
type SourceStatus struct {
	ID    int
	Title string
	URL   string
	Own   bool
	Error string
}

// NewsFeed — объединённая лента: свои новости из БД и новости всех лент.
type NewsFeed struct {
	Items   []domain.NewsItem
	Sources []SourceStatus
}

// NewsInput — данные формы своей новости.
type NewsInput struct {
	Title       string
	Summary     string
	PublishedAt string
}

// NewsSourceInput — данные формы подключения RSS-ленты.
type NewsSourceInput struct {
	Title string
	URL   string
}

type cachedFeed struct {
	items     []domain.NewsItem
	fetchedAt time.Time
}

type fetchResult struct {
	items []domain.NewsItem
	err   error
}

// NewsService — сценарий обмена новостями: собирает свои новости из БД и
// новости всех подключённых RSS-лент в одну ленту. Внешние ленты читает
// сервер (браузеру мешает CORS), результат ненадолго кэшируется по источнику.
type NewsService struct {
	repo    NewsRepository
	feedFor NewsFeedFactory
	ttl     time.Duration

	mu    sync.Mutex
	cache map[string]cachedFeed
}

// NewNewsService создаёт сценарий с хранилищем, фабрикой лент и временем
// жизни кэша одного источника.
func NewNewsService(repo NewsRepository, feedFor NewsFeedFactory, ttl time.Duration) *NewsService {
	return &NewsService{
		repo:    repo,
		feedFor: feedFor,
		ttl:     ttl,
		cache:   make(map[string]cachedFeed),
	}
}

// News возвращает объединённую ленту: свои новости из БД и новости всех
// активных внешних источников, без дублей и по убыванию даты.
func (s *NewsService) News(ctx context.Context) (NewsFeed, error) {
	own, err := s.OwnNews(ctx, ownNewsLimit)
	if err != nil {
		return NewsFeed{}, err
	}

	sources, err := s.repo.ListNewsSources(ctx)
	if err != nil {
		return NewsFeed{}, fmt.Errorf("list news sources: %w", err)
	}

	results := make([]fetchResult, len(sources))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallelFetch)

	for i, source := range sources {
		wg.Add(1)
		go func(i int, source domain.NewsSource) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[i].items, results[i].err = s.fetchSource(ctx, source)
		}(i, source)
	}
	wg.Wait()

	statuses := make([]SourceStatus, 0, len(sources)+1)
	statuses = append(statuses, SourceStatus{Title: "Место", Own: true})

	items := append([]domain.NewsItem(nil), own...)
	for i, source := range sources {
		status := SourceStatus{ID: source.ID, Title: source.Title, URL: source.URL}
		if results[i].err != nil {
			status.Error = "источник недоступен"
		}
		statuses = append(statuses, status)
		items = append(items, results[i].items...)
	}

	items = dedupeNews(items)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
	if len(items) > totalNewsLimit {
		items = items[:totalNewsLimit]
	}

	return NewsFeed{Items: items, Sources: statuses}, nil
}

// OwnNews возвращает только свои новости из БД. Нужен для публикации
// собственной RSS-ленты, которую забирают другие приложения.
func (s *NewsService) OwnNews(ctx context.Context, limit int) ([]domain.NewsItem, error) {
	items, err := s.repo.ListNews(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list own news: %w", err)
	}

	for i := range items {
		items[i].Own = true
		if items[i].Source == "" {
			items[i].Source = "Место"
		}
	}

	return items, nil
}

// CreateNews добавляет свою новость в базу данных.
func (s *NewsService) CreateNews(ctx context.Context, input NewsInput) (domain.NewsItem, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" || utf8.RuneCountInString(title) > maxNewsTitleRunes {
		return domain.NewsItem{}, ErrInvalidNewsTitle
	}

	summary := strings.TrimSpace(input.Summary)
	if utf8.RuneCountInString(summary) > maxNewsSummary {
		return domain.NewsItem{}, ErrInvalidNewsSummary
	}

	publishedAt := time.Now()
	if raw := strings.TrimSpace(input.PublishedAt); raw != "" {
		parsed, err := parseNewsDate(raw)
		if err != nil {
			return domain.NewsItem{}, ErrInvalidNewsDate
		}
		publishedAt = parsed
	}

	return s.repo.SaveNews(ctx, domain.NewsItem{
		Title:       title,
		Summary:     summary,
		PublishedAt: publishedAt,
		Own:         true,
	})
}

// DeleteNews удаляет свою новость из базы данных.
func (s *NewsService) DeleteNews(ctx context.Context, id int) error {
	return s.repo.DeleteNews(ctx, id)
}

// AddSource подключает внешнюю RSS-ленту по произвольному адресу.
func (s *NewsService) AddSource(ctx context.Context, input NewsSourceInput) (domain.NewsSource, error) {
	rawURL := strings.TrimSpace(input.URL)
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return domain.NewsSource{}, ErrInvalidNewsSource
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = parsed.Host
	}
	if utf8.RuneCountInString(title) > maxSourceTitle {
		return domain.NewsSource{}, ErrInvalidNewsSource
	}

	existing, err := s.repo.ListNewsSources(ctx)
	if err != nil {
		return domain.NewsSource{}, fmt.Errorf("list news sources: %w", err)
	}
	if len(existing) >= maxNewsSources {
		return domain.NewsSource{}, ErrTooManyNewsSources
	}

	return s.repo.SaveNewsSource(ctx, domain.NewsSource{
		Title:    title,
		URL:      parsed.String(),
		IsActive: true,
	})
}

// DeleteSource отключает внешнюю RSS-ленту.
func (s *NewsService) DeleteSource(ctx context.Context, id int) error {
	return s.repo.DeleteNewsSource(ctx, id)
}

// fetchSource читает одну ленту с кэшем. Если сеть недоступна, но есть
// прошлая удачная загрузка, возвращаем её и сообщаем об ошибке статусом.
func (s *NewsService) fetchSource(ctx context.Context, source domain.NewsSource) ([]domain.NewsItem, error) {
	now := time.Now()

	s.mu.Lock()
	cached, hasCache := s.cache[source.URL]
	s.mu.Unlock()

	if hasCache && now.Sub(cached.fetchedAt) < s.ttl {
		return cloneNews(cached.items), nil
	}

	if s.feedFor == nil {
		return nil, errors.New("news feed factory is not configured")
	}

	items, err := s.feedFor(source.Title, source.URL).Fetch(ctx)
	if err != nil {
		if hasCache {
			return cloneNews(cached.items), err
		}
		return nil, err
	}

	if len(items) > perSourceLimit {
		items = items[:perSourceLimit]
	}
	for i := range items {
		items[i].Own = false
		items[i].Source = source.Title
	}

	s.mu.Lock()
	s.cache[source.URL] = cachedFeed{items: cloneNews(items), fetchedAt: now}
	s.mu.Unlock()

	return items, nil
}

// parseNewsDate понимает дату из <input type="date">, значение
// datetime-local и полный RFC 3339.
func parseNewsDate(raw string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed, nil
	}

	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", "2006-01-02T15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, ErrInvalidNewsDate
}

// dedupeNews убирает повторы по ссылке, сохраняя порядок. У своих новостей
// ссылки может не быть, поэтому они не склеиваются между собой.
func dedupeNews(items []domain.NewsItem) []domain.NewsItem {
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

func cloneNews(items []domain.NewsItem) []domain.NewsItem {
	return append([]domain.NewsItem(nil), items...)
}
