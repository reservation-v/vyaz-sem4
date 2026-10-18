package rss

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"coworking/internal/domain"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Пример</title>
    <item>
      <title>Новость &amp; деталь</title>
      <link>https://example.com/1</link>
      <description><![CDATA[<p>Первый <b>текст</b> новости</p>]]></description>
      <pubDate>Fri, 16 Oct 2026 08:29:06 +0300</pubDate>
    </item>
    <item>
      <title>Вторая новость</title>
      <link>https://example.com/2</link>
      <description>Второй текст без тегов</description>
      <pubDate>Thu, 15 Oct 2026 10:00:00 +0300</pubDate>
    </item>
  </channel>
</rss>`

const sampleAtom = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Атом</title>
  <entry>
    <title>Запись Atom</title>
    <link rel="alternate" href="https://example.org/a"/>
    <summary>Текст записи</summary>
    <updated>2026-10-16T09:00:00Z</updated>
  </entry>
</feed>`

func TestParseRSS(t *testing.T) {
	items, err := parse([]byte(sampleRSS), "пример")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("получено %d записей, ожидалось 2", len(items))
	}

	first := items[0]
	if first.Title != "Новость & деталь" {
		t.Errorf("заголовок = %q", first.Title)
	}
	if first.Link != "https://example.com/1" {
		t.Errorf("ссылка = %q", first.Link)
	}
	if first.Source != "пример" {
		t.Errorf("источник = %q", first.Source)
	}
	if !strings.Contains(first.Summary, "Первый текст новости") {
		t.Errorf("краткий текст = %q", first.Summary)
	}
	if first.PublishedAt.IsZero() {
		t.Errorf("дата не разобрана")
	}
}

func TestParseAtom(t *testing.T) {
	items, err := parse([]byte(sampleAtom), "атом")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("получено %d записей, ожидалось 1", len(items))
	}
	if items[0].Link != "https://example.org/a" {
		t.Errorf("ссылка = %q", items[0].Link)
	}
}

func TestFetchReadsFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer server.Close()

	// В тесте лента живёт на 127.0.0.1, поэтому берём клиент тестового сервера:
	// боевой клиент намеренно блокирует loopback и частные адреса (SSRF).
	feed := NewFeedWithClient("пример", server.URL, server.Client())
	items, err := feed.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("получено %d записей, ожидалось 2", len(items))
	}
}

func TestFetchRejectsBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	feed := NewFeedWithClient("пример", server.URL, server.Client())
	if _, err := feed.Fetch(context.Background()); err == nil {
		t.Fatal("ожидалась ошибка при статусе 500")
	}
}

func TestMakeSummaryTrimsLongText(t *testing.T) {
	long := strings.Repeat("слово ", 100)
	summary := makeSummary(long)
	if len([]rune(summary)) > summaryLimit+1 {
		t.Errorf("краткий текст слишком длинный: %d символов", len([]rune(summary)))
	}
	if !strings.HasSuffix(summary, "…") {
		t.Errorf("ожидалось многоточие в конце: %q", summary)
	}
}

func TestPublisherBuildsRSS(t *testing.T) {
	publisher := NewPublisher("Место", "Новости коворкинга", "ru")
	body, err := publisher.Publish([]domain.NewsItem{
		{
			ID:          7,
			Title:       "Новость & событие",
			Summary:     "Краткий текст",
			PublishedAt: time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC),
			Own:         true,
		},
	}, "https://mesto.example")
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	text := string(body)
	for _, marker := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<rss version="2.0">`,
		`<title>Новость &amp; событие</title>`,
		`<link>https://mesto.example/news#news-7</link>`,
		`<guid isPermaLink="true">https://mesto.example/news#news-7</guid>`,
		`<pubDate>Sun, 18 Oct 2026 10:00:00 +0000</pubDate>`,
	} {
		if !strings.Contains(text, marker) {
			t.Errorf("в RSS нет %q\n%s", marker, text)
		}
	}
}

func TestPublisherUsesStableGUIDWithoutLink(t *testing.T) {
	publisher := NewPublisher("Место", "Новости", "ru")
	body, err := publisher.Publish([]domain.NewsItem{
		{ID: 3, Title: "Без ссылки", PublishedAt: time.Now()},
	}, "")
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if !strings.Contains(string(body), `<guid isPermaLink="false">news-3</guid>`) {
		t.Errorf("ожидался стабильный guid без ссылки:\n%s", body)
	}
}

func TestCheckFeedURL(t *testing.T) {
	valid, _ := url.Parse("https://example.com/rss")
	if err := checkFeedURL(valid); err != nil {
		t.Errorf("валидный адрес отклонён: %v", err)
	}

	for _, raw := range []string{"ftp://example.com/rss", "file:///etc/passwd", "https://"} {
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if err := checkFeedURL(parsed); err == nil {
			t.Errorf("адрес %q должен быть отклонён", raw)
		}
	}
}

func TestForbiddenIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":   true,
		"10.0.0.5":    true,
		"172.16.0.5":  true,
		"192.168.1.5": true,
		"169.254.1.1": true,
		"100.64.0.1":  true,
		"::1":         true,
		"fe80::1":     true,
		"8.8.8.8":     false,
		"1.1.1.1":     false,
	}

	for raw, want := range cases {
		ip := net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("не разобран IP %q", raw)
		}
		if got := forbiddenIP(ip); got != want {
			t.Errorf("forbiddenIP(%s) = %v, ожидалось %v", raw, got, want)
		}
	}
}
