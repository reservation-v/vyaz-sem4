package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	"coworking/internal/application"
	"coworking/internal/domain"
)

// newsItemView — новость в шаблоне и в JSON-ответе для обновления страницы.
type newsItemView struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Source      string `json:"source"`
	SourceClass string `json:"-"`
	Summary     string `json:"summary"`
	DateLabel   string `json:"-"`
	Published   string `json:"published"`
	Own         bool   `json:"own"`
}

// newsSourceView — подключённый источник в списке на странице.
type newsSourceView struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Own   bool   `json:"own"`
	Error string `json:"error,omitempty"`
}

// newsFormData — значения формы своей новости.
type newsFormData struct {
	Title       string
	Summary     string
	PublishedAt string
}

// newsSourceFormData — значения формы подключения RSS-ленты.
type newsSourceFormData struct {
	Title string
	URL   string
}

// newsPageData — данные страницы «Новости».
type newsPageData struct {
	basePageData
	Items       []newsItemView
	Sources     []newsSourceView
	Suggestions []newsSuggestionView
	Form        newsFormData
	SourceForm  newsSourceFormData
	Flash       string
	FlashKind   string // "ok" или "err"
	RSSURL      string
	Tab         string // активная вкладка: feed, add-news или add-source
}

// newsListResponse — ответ /api/news для обновления ленты без перезагрузки.
type newsListResponse struct {
	Items   []newsItemView   `json:"items"`
	Sources []newsSourceView `json:"sources"`
}

// handleNewsPage показывает одну общую ленту: свои новости из БД и новости
// всех подключённых RSS-лент, с указанием источника у каждой новости.
func (s *Server) handleNewsPage(w nethttp.ResponseWriter, r *nethttp.Request) {
	flash, kind := newsFlash(r)
	status := nethttp.StatusOK
	if kind == "err" {
		status = nethttp.StatusUnprocessableEntity
	}

	s.renderNewsPage(w, r, defaultNewsForm(), newsSourceFormData{}, flash, kind, "feed", status)
}

// handleNewsCreate добавляет свою новость в базу данных.
func (s *Server) handleNewsCreate(w nethttp.ResponseWriter, r *nethttp.Request) {
	if s.news == nil {
		nethttp.Error(w, "новости недоступны", nethttp.StatusServiceUnavailable)
		return
	}
	if err := parseBody(r); err != nil {
		nethttp.Error(w, "неверные данные формы", nethttp.StatusBadRequest)
		return
	}

	input := application.NewsInput{
		Title:       r.FormValue("title"),
		Summary:     r.FormValue("summary"),
		PublishedAt: r.FormValue("published_at"),
	}

	if _, err := s.news.CreateNews(r.Context(), input); err != nil {
		log.Printf("create news failed: %v", err)
		form := newsFormData{Title: input.Title, Summary: input.Summary, PublishedAt: input.PublishedAt}
		if form.PublishedAt == "" {
			form.PublishedAt = defaultNewsForm().PublishedAt
		}
		s.renderNewsPage(w, r, form, newsSourceFormData{}, newsErrorMessage(err), "err", "add-news", nethttp.StatusUnprocessableEntity)
		return
	}

	nethttp.Redirect(w, r, "/news?notice=news-created", nethttp.StatusSeeOther)
}

// handleNewsDelete удаляет свою новость.
func (s *Server) handleNewsDelete(w nethttp.ResponseWriter, r *nethttp.Request) {
	if s.news == nil {
		nethttp.Error(w, "новости недоступны", nethttp.StatusServiceUnavailable)
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		nethttp.NotFound(w, r)
		return
	}

	if err := s.news.DeleteNews(r.Context(), id); err != nil {
		if errors.Is(err, application.ErrNewsNotFound) {
			nethttp.NotFound(w, r)
			return
		}
		log.Printf("delete news %d failed: %v", id, err)
		nethttp.Redirect(w, r, "/news?notice=news-delete-error", nethttp.StatusSeeOther)
		return
	}

	nethttp.Redirect(w, r, "/news?notice=news-deleted", nethttp.StatusSeeOther)
}

// handleNewsSourceCreate подключает произвольную RSS-ленту.
func (s *Server) handleNewsSourceCreate(w nethttp.ResponseWriter, r *nethttp.Request) {
	if s.news == nil {
		nethttp.Error(w, "новости недоступны", nethttp.StatusServiceUnavailable)
		return
	}
	if err := parseBody(r); err != nil {
		nethttp.Error(w, "неверные данные формы", nethttp.StatusBadRequest)
		return
	}

	input := application.NewsSourceInput{
		Title: r.FormValue("title"),
		URL:   r.FormValue("url"),
	}

	if _, err := s.news.AddSource(r.Context(), input); err != nil {
		log.Printf("add news source failed: %v", err)
		s.renderNewsPage(w, r, defaultNewsForm(),
			newsSourceFormData{Title: input.Title, URL: input.URL},
			newsErrorMessage(err), "err", "add-source", nethttp.StatusUnprocessableEntity)
		return
	}

	nethttp.Redirect(w, r, "/news?notice=source-added", nethttp.StatusSeeOther)
}

// handleNewsSourceDelete отключает RSS-ленту.
func (s *Server) handleNewsSourceDelete(w nethttp.ResponseWriter, r *nethttp.Request) {
	if s.news == nil {
		nethttp.Error(w, "новости недоступны", nethttp.StatusServiceUnavailable)
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		nethttp.NotFound(w, r)
		return
	}

	if err := s.news.DeleteSource(r.Context(), id); err != nil {
		if errors.Is(err, application.ErrNewsSourceNotFound) {
			nethttp.NotFound(w, r)
			return
		}
		log.Printf("delete news source %d failed: %v", id, err)
		nethttp.Redirect(w, r, "/news?notice=source-delete-error", nethttp.StatusSeeOther)
		return
	}

	nethttp.Redirect(w, r, "/news?notice=source-deleted", nethttp.StatusSeeOther)
}

// handleNewsList отдаёт объединённую ленту в JSON. Страница использует его
// для обновления без перезагрузки, другие приложения могут забрать RSS
// по адресу /news/rss.xml.
func (s *Server) handleNewsList(w nethttp.ResponseWriter, r *nethttp.Request) {
	if s.news == nil {
		nethttp.Error(w, "новости недоступны", nethttp.StatusServiceUnavailable)
		return
	}

	feed, err := s.news.News(r.Context())
	if err != nil {
		log.Printf("load news for api failed: %v", err)
		nethttp.Error(w, "не удалось загрузить новости", nethttp.StatusInternalServerError)
		return
	}

	response := newsListResponse{
		Items:   toNewsItemViews(feed.Items),
		Sources: toNewsSourceViews(feed.Sources),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}

// handleNewsRSS публикует собственные новости приложения в формате RSS 2.0.
// Именно эту ленту могут забрать другие приложения: обмен идёт в обе стороны.
func (s *Server) handleNewsRSS(w nethttp.ResponseWriter, r *nethttp.Request) {
	if s.news == nil || s.newsPublisher == nil {
		nethttp.Error(w, "RSS-лента недоступна", nethttp.StatusServiceUnavailable)
		return
	}

	items, err := s.news.OwnNews(r.Context(), 50)
	if err != nil {
		log.Printf("load own news for rss failed: %v", err)
		nethttp.Error(w, "не удалось собрать RSS-ленту", nethttp.StatusInternalServerError)
		return
	}

	body, err := s.newsPublisher.Publish(items, requestBaseURL(r))
	if err != nil {
		log.Printf("publish rss failed: %v", err)
		nethttp.Error(w, "не удалось собрать RSS-ленту", nethttp.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(nethttp.StatusOK)
	_, _ = w.Write(body)
}

// renderNewsPage собирает общую ленту и отрисовывает страницу. Используется
// и для обычного показа, и для повторного показа формы с ошибкой.
func (s *Server) renderNewsPage(w nethttp.ResponseWriter, r *nethttp.Request, form newsFormData, sourceForm newsSourceFormData, flash, kind, tab string, status int) {
	data := newsPageData{
		basePageData: s.basePage("Новости", "news"),
		Form:         form,
		SourceForm:   sourceForm,
		Flash:        flash,
		FlashKind:    kind,
		RSSURL:       "/news/rss.xml",
		Tab:          tab,
		Suggestions:  defaultNewsSuggestions(),
	}

	if s.news != nil {
		feed, err := s.news.News(r.Context())
		if err != nil {
			log.Printf("load news page failed: %v", err)
			if data.Flash == "" {
				data.Flash = "Не удалось загрузить новости. Попробуйте обновить страницу."
				data.FlashKind = "err"
			}
			if status == nethttp.StatusOK {
				status = nethttp.StatusInternalServerError
			}
		} else {
			data.Items = toNewsItemViews(feed.Items)
			data.Sources = toNewsSourceViews(feed.Sources)
			data.Suggestions = suggestionsWithConnected(feed.Sources)
		}
	}

	s.renderPage(w, "news", status, data)
}

func toNewsItemViews(items []domain.NewsItem) []newsItemView {
	views := make([]newsItemView, 0, len(items))

	for _, item := range items {
		published := ""
		if !item.PublishedAt.IsZero() {
			published = item.PublishedAt.Format(time.RFC3339)
		}

		sourceClass := "external"
		if item.Own {
			sourceClass = "own"
		}

		views = append(views, newsItemView{
			ID:          item.ID,
			Title:       item.Title,
			Link:        item.Link,
			Source:      item.Source,
			SourceClass: sourceClass,
			Summary:     item.Summary,
			DateLabel:   newsDateLabel(item.PublishedAt),
			Published:   published,
			Own:         item.Own,
		})
	}

	return views
}

func toNewsSourceViews(sources []application.SourceStatus) []newsSourceView {
	views := make([]newsSourceView, 0, len(sources))
	for _, source := range sources {
		views = append(views, newsSourceView{
			ID:    source.ID,
			Title: source.Title,
			URL:   source.URL,
			Own:   source.Own,
			Error: source.Error,
		})
	}
	return views
}

var russianMonths = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// newsDateLabel переводит дату в человеческий вид: «1 октября 2026».
func newsDateLabel(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	value = value.In(time.Local)
	return fmt.Sprintf("%d %s %d", value.Day(), russianMonths[value.Month()-1], value.Year())
}

// defaultNewsForm подставляет сегодняшнюю дату в форму новой новости.
func defaultNewsForm() newsFormData {
	return newsFormData{PublishedAt: time.Now().Format("2006-01-02")}
}

// newsFlash читает сообщение после редиректа (PRG-паттерн).
func newsFlash(r *nethttp.Request) (string, string) {
	switch r.URL.Query().Get("notice") {
	case "news-created":
		return "Новость добавлена в базу данных и появилась в общей ленте.", "ok"
	case "news-deleted":
		return "Новость удалена из базы данных.", "ok"
	case "news-delete-error":
		return "Не удалось удалить новость. Обновите страницу и попробуйте снова.", "err"
	case "source-added":
		return "RSS-лента подключена. Её новости появятся в общей ленте.", "ok"
	case "source-deleted":
		return "RSS-лента отключена.", "ok"
	case "source-delete-error":
		return "Не удалось отключить ленту. Обновите страницу и попробуйте снова.", "err"
	default:
		return "", ""
	}
}

// newsErrorMessage превращает ошибку сценария в понятный текст для страницы.
func newsErrorMessage(err error) string {
	switch {
	case errors.Is(err, application.ErrInvalidNewsTitle):
		return "Укажите заголовок новости длиной до 200 символов."
	case errors.Is(err, application.ErrInvalidNewsSummary):
		return "Текст новости слишком длинный: до 2000 символов."
	case errors.Is(err, application.ErrInvalidNewsDate):
		return "Укажите корректную дату публикации."
	case errors.Is(err, application.ErrInvalidNewsSource):
		return "Укажите корректный адрес ленты: http:// или https://."
	case errors.Is(err, application.ErrDuplicateNewsSource):
		return "Эта RSS-лента уже подключена."
	case errors.Is(err, application.ErrTooManyNewsSources):
		return "Подключено слишком много лент. Отключите одну из существующих."
	default:
		return "Не удалось сохранить данные. Попробуйте ещё раз."
	}
}

// requestBaseURL строит абсолютный адрес сайта для ссылок в RSS.
func requestBaseURL(r *nethttp.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}

	host := r.Host
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}
