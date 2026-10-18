// Package rss — адаптер внешнего источника новостей: читает RSS- или Atom-ленту
// другого приложения и превращает её записи в доменные новости.
package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"coworking/internal/application"
	"coworking/internal/domain"
)

// Проверка на этапе компиляции: Feed удовлетворяет порту application.
var _ application.ExternalNewsSource = (*Feed)(nil)

const (
	maxBodyBytes = 2 << 20 // 2 МиБ - защита от слишком больших ответов
	userAgent    = "MestoNewsBot/1.0"
	summaryLimit = 240 // символов в кратком тексте
)

// Feed — один внешний источник новостей (RSS- или Atom-лента).
type Feed struct {
	title  string
	url    string
	client *http.Client
}

// NewFeed создаёт источник с заданным названием и адресом ленты.
// Адрес проверяется при загрузке; клиент не пускает запросы во внутреннюю сеть.
func NewFeed(title, url string) *Feed {
	return NewFeedWithClient(title, url, newHTTPClient())
}

// NewFeedWithClient создаёт источник с готовым HTTP-клиентом.
// Нужен для тестов и для особых случаев конфигурации.
func NewFeedWithClient(title, url string, client *http.Client) *Feed {
	if client == nil {
		client = newHTTPClient()
	}
	return &Feed{title: title, url: url, client: client}
}

// Source возвращает название источника.
func (f *Feed) Source() string {
	return f.title
}

// Fetch забирает ленту по сети и разбирает её.
func (f *Feed) Fetch(ctx context.Context) ([]domain.NewsItem, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, fmt.Errorf("rss %s: %w", f.title, err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, */*;q=0.8")

	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("rss %s: %w", f.title, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rss %s: статус %d", f.title, response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("rss %s: чтение: %w", f.title, err)
	}

	items, err := parse(body, f.title)
	if err != nil {
		return nil, fmt.Errorf("rss %s: разбор: %w", f.title, err)
	}

	return items, nil
}

// document описывает сразу RSS (channel/item) и Atom (feed/entry):
// лишние поля для конкретного формата просто остаются пустыми.
type document struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Links   []atomLink `xml:"link"`
	Summary string     `xml:"summary"`
	Updated string     `xml:"updated"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// parse превращает XML-ленту в список доменных новостей.
func parse(body []byte, source string) ([]domain.NewsItem, error) {
	var doc document
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}

	items := make([]domain.NewsItem, 0, len(doc.Channel.Items)+len(doc.Entries))

	for _, node := range doc.Channel.Items {
		items = append(items, domain.NewsItem{
			Title:       cleanText(node.Title),
			Link:        strings.TrimSpace(node.Link),
			Source:      source,
			Summary:     makeSummary(node.Description),
			PublishedAt: parseTime(node.PubDate),
		})
	}

	for _, node := range doc.Entries {
		items = append(items, domain.NewsItem{
			Title:       cleanText(node.Title),
			Link:        atomLinkHref(node.Links),
			Source:      source,
			Summary:     makeSummary(node.Summary),
			PublishedAt: parseTime(node.Updated),
		})
	}

	return items, nil
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

// cleanText убирает HTML-теги, раскрывает сущности и сжимает пробелы.
func cleanText(raw string) string {
	text := tagPattern.ReplaceAllString(raw, " ")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

// makeSummary готовит краткий текст новости и ограничивает его длину.
func makeSummary(raw string) string {
	text := cleanText(raw)
	runes := []rune(text)
	if len(runes) <= summaryLimit {
		return text
	}
	return strings.TrimSpace(string(runes[:summaryLimit])) + "…"
}

// atomLinkHref выбирает основную ссылку записи Atom.
func atomLinkHref(links []atomLink) string {
	for _, link := range links {
		if link.Href != "" && (link.Rel == "" || link.Rel == "alternate") {
			return link.Href
		}
	}
	if len(links) > 0 {
		return links[0].Href
	}
	return ""
}

var timeLayouts = []string{
	time.RFC1123Z, // Mon, 02 Jan 2006 15:04:05 -0700
	time.RFC1123,  // Mon, 02 Jan 2006 15:04:05 MST
	time.RFC3339,  // 2006-01-02T15:04:05Z07:00
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2006-01-02 15:04:05",
}

// parseTime разбирает дату в разных форматах; при неудаче - нулевое время.
func parseTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range timeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
