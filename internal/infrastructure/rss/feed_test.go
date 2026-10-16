package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	feed := NewFeed("пример", server.URL)
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

	feed := NewFeed("пример", server.URL)
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
