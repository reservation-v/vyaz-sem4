package domain

import "time"

// ExternalNews — новость, полученная из внешней RSS-ленты (другого приложения).
type ExternalNews struct {
	Title       string    // заголовок
	Link        string    // ссылка на материал
	Source      string    // название источника (например, «vc.ru · коворкинг»)
	Summary     string    // краткий текст без HTML-тегов
	PublishedAt time.Time // дата публикации
}
