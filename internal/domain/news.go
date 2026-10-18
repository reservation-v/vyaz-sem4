package domain

import "time"

// NewsItem — новость в общей ленте сайта. Своя новость приходит из базы
// данных, внешняя — из RSS- или Atom-ленты другого приложения.
type NewsItem struct {
	ID          int       // идентификатор своей новости, 0 для внешней
	Title       string    // заголовок
	Link        string    // ссылка на материал (у своих новостей может быть пустой)
	Source      string    // название источника: «Место», «vc.ru · коворкинг» и т.д.
	Summary     string    // краткий текст без HTML-тегов
	PublishedAt time.Time // дата публикации
	Own         bool      // true, если новость хранится в нашей базе данных
}

// NewsSource — подключённая внешняя RSS-лента (таблица news_sources).
type NewsSource struct {
	ID       int
	Title    string
	URL      string
	IsActive bool
}
