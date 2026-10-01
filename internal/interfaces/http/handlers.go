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
	Featured    bool
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
		basePageData: basePageData{
			Title:     "Главная",
			SiteTitle: s.siteTitle,
			ActiveNav: "home",
		},
		Services: toServiceViews(services),
	})
}

// handleSoon отдаёт заглушку для разделов, которые ещё не готовы.
func (s *Server) handleSoon(w nethttp.ResponseWriter, r *nethttp.Request) {
	section := r.URL.Query().Get("section")
	if section == "" {
		section = "Раздел"
	}

	s.renderPage(w, "soon", nethttp.StatusOK, soonPageData{
		basePageData: basePageData{
			Title:     section,
			SiteTitle: s.siteTitle,
		},
		Section: section,
	})
}

// toServiceViews преобразует доменные сервисы в представления для шаблона.
// Первый готовый сервис становится крупной карточкой.
func toServiceViews(services []domain.Service) []serviceView {
	views := make([]serviceView, 0, len(services))
	featuredMarked := false

	for i, svc := range services {
		view := serviceView{
			Num:         fmt.Sprintf("%02d", i+1),
			Name:        svc.Name,
			Description: svc.Description,
			Group:       svc.Group,
			URL:         svc.URL,
			Featured:    svc.Status == domain.ServiceStatusReady && !featuredMarked,
		}
		if view.Featured {
			featuredMarked = true
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
