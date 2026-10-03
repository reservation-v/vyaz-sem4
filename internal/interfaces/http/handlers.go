package http

import (
	"fmt"
	nethttp "net/http"

	"coworking/internal/domain"
)

// serviceView — представление сервиса для шаблона главной страницы.
type serviceView struct {
	Num         string
	Name        string
	Description string
	Group       string
	StatusLabel string
	StatusClass string
	URL         string
	Slug        string
}

// homePageData — данные главной страницы (хаб сервисов).
type homePageData struct {
	basePageData
	Services []serviceView
}

// soonPageData — данные страницы «раздел в разработке».
type soonPageData struct {
	basePageData
	Section string
}

// handleHome отдаёт главную страницу со списком сервисов.
func (s *Server) handleHome(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.URL.Path != "/" {
		nethttp.NotFound(w, r)
		return
	}

	services, err := s.catalog.ListServices(r.Context())
	if err != nil {
		nethttp.Error(w, "не удалось загрузить сервисы", nethttp.StatusInternalServerError)
		return
	}

	s.renderPage(w, "home", nethttp.StatusOK, homePageData{
		basePageData: s.basePage("Главная", "home"),
		Services:     toServiceViews(services),
	})
}

// handleSoon отдаёт заглушку для разделов, которые ещё не готовы.
func (s *Server) handleSoon(w nethttp.ResponseWriter, r *nethttp.Request) {
	section := r.URL.Query().Get("section")
	if section == "" {
		section = "Раздел"
	}

	s.renderPage(w, "soon", nethttp.StatusOK, soonPageData{
		basePageData: s.basePage(section, ""),
		Section:      section,
	})
}

// staticPage отдаёт статическую страницу раздела сайта.
func (s *Server) staticPage(page, title, activeNav string) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		s.renderPage(w, page, nethttp.StatusOK, s.basePage(title, activeNav))
	}
}

// basePage собирает общие данные страницы и учитывает её просмотр.
func (s *Server) basePage(title, activeNav string) basePageData {
	return basePageData{
		Title:     title,
		SiteTitle: s.siteTitle,
		ActiveNav: activeNav,
		Visits:    s.visits.Add(1),
	}
}

// toServiceViews преобразует доменные сервисы в представления для шаблона.
func toServiceViews(services []domain.Service) []serviceView {
	views := make([]serviceView, 0, len(services))

	for i, svc := range services {
		view := serviceView{
			Num:         fmt.Sprintf("%02d", i+1),
			Name:        svc.Name,
			Description: svc.Description,
			Group:       svc.Group,
			URL:         svc.URL,
			Slug:        svc.Slug,
		}

		switch svc.Status {
		case domain.ServiceStatusReady:
			view.StatusLabel = "Готово"
			view.StatusClass = "tag-ready"
		default:
			view.StatusLabel = "В разработке"
			view.StatusClass = "tag-soon"
		}

		views = append(views, view)
	}

	return views
}
