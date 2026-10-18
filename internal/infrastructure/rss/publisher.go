package rss

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"coworking/internal/application"
	"coworking/internal/domain"
)

// Проверка на этапе компиляции: Publisher реализует порт application.
var _ application.NewsFeedPublisher = (*Publisher)(nil)

// Publisher собирает собственную RSS-ленту приложения из новостей БД.
type Publisher struct {
	title       string
	description string
	language    string
}

// NewPublisher создаёт публикатор RSS с названием, описанием и языком ленты.
func NewPublisher(title, description, language string) *Publisher {
	return &Publisher{title: title, description: description, language: language}
}

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string       `xml:"title"`
	Link          string       `xml:"link"`
	Description   string       `xml:"description"`
	Language      string       `xml:"language,omitempty"`
	LastBuildDate string       `xml:"lastBuildDate,omitempty"`
	Items         []rssOutItem `xml:"item"`
}

type rssOutItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link,omitempty"`
	GUID        rssGUID `xml:"guid"`
	Description string  `xml:"description,omitempty"`
	PubDate     string  `xml:"pubDate"`
}

type rssGUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr,omitempty"`
}

// Publish отдаёт свои новости в формате RSS 2.0. baseURL — адрес сайта,
// из него строятся ссылки на страницу новостей.
func (p *Publisher) Publish(items []domain.NewsItem, baseURL string) ([]byte, error) {
	channel := rssChannel{
		Title:       p.title,
		Link:        newsPageURL(baseURL),
		Description: p.description,
		Language:    p.language,
	}

	for _, item := range items {
		link := strings.TrimSpace(item.Link)
		if link == "" {
			link = newsItemURL(baseURL, item.ID)
		}

		guid := link
		permaLink := "true"
		if guid == "" {
			guid = fmt.Sprintf("news-%d", item.ID)
			permaLink = "false"
		}

		publishedAt := item.PublishedAt
		if publishedAt.IsZero() {
			publishedAt = time.Now()
		}

		channel.Items = append(channel.Items, rssOutItem{
			Title:       item.Title,
			Link:        link,
			GUID:        rssGUID{Value: guid, IsPermaLink: permaLink},
			Description: item.Summary,
			PubDate:     publishedAt.Format(time.RFC1123Z),
		})
	}

	if len(channel.Items) > 0 {
		channel.LastBuildDate = channel.Items[0].PubDate
	} else {
		channel.LastBuildDate = time.Now().Format(time.RFC1123Z)
	}

	body, err := xml.MarshalIndent(rssDocument{Version: "2.0", Channel: channel}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal rss: %w", err)
	}

	result := make([]byte, 0, len(xml.Header)+len(body))
	result = append(result, xml.Header...)
	result = append(result, body...)
	result = append(result, '\n')
	return result, nil
}

func newsPageURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return ""
	}
	return base + "/news"
}

func newsItemURL(baseURL string, id int) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" || id <= 0 {
		return ""
	}
	return base + "/news#news-" + strconv.Itoa(id)
}
