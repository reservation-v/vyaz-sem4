package http

import "coworking/internal/application"

// newsSuggestionView — готовая RSS-лента, которую можно подключить в один клик.
type newsSuggestionView struct {
	Title       string
	URL         string
	Description string
	Connected   bool
}

// defaultNewsSuggestions — проверенные ленты по теме коворкинга и близким
// областям. Они нужны как подсказка для быстрого теста страницы.
func defaultNewsSuggestions() []newsSuggestionView {
	return []newsSuggestionView{
		{
			Title:       "vc.ru · коворкинг",
			URL:         "https://vc.ru/rss/tag/coworking",
			Description: "Публикации о гибких офисах, аренде рабочих мест и рынке коворкингов.",
		},
		{
			Title:       "Inc. Russia",
			URL:         "https://incrussia.ru/feed/",
			Description: "Деловые новости о предпринимателях, стартапах и развитии бизнеса.",
		},
		{
			Title:       "Ведомости",
			URL:         "https://www.vedomosti.ru/rss/news",
			Description: "Деловая пресса: компании, рынки, коммерческая недвижимость и экономика.",
		},
		{
			Title:       "Коммерсантъ",
			URL:         "https://www.kommersant.ru/RSS/news.xml",
			Description: "Оперативные деловые новости, экономика и комментарии экспертов.",
		},
		{
			Title:       "BFM.ru",
			URL:         "https://www.bfm.ru/news.rss",
			Description: "Деловые и финансовые новости, экономика и городская повестка.",
		},
		{
			Title:       "Хабр",
			URL:         "https://habr.com/ru/rss/news/",
			Description: "Новости IT и технологий: команды, офисы, сервисы и рабочие инструменты.",
		},
		{
			Title:       "Cossa",
			URL:         "https://www.cossa.ru/rss/",
			Description: "Маркетинг, digital и медиа: идеи для продвижения коворкинга.",
		},
		{
			Title:       "РБК",
			URL:         "https://rssexport.rbc.ru/rbcnews/news/30/full.rss",
			Description: "Ключевые деловые новости России и мира, экономика и рынки.",
		},
		{
			Title:       "РИА Новости",
			URL:         "https://ria.ru/export/rss2/index.xml",
			Description: "Главные новости России и мира от государственного агентства.",
		},
	}
}

// suggestionsWithConnected помечает ленты, которые уже подключены.
func suggestionsWithConnected(sources []application.SourceStatus) []newsSuggestionView {
	suggestions := defaultNewsSuggestions()

	connected := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if source.URL != "" {
			connected[source.URL] = struct{}{}
		}
	}

	for i := range suggestions {
		_, suggestions[i].Connected = connected[suggestions[i].URL]
	}
	return suggestions
}
