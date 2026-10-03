// Package http — адаптер входящего слоя (delivery): HTTP-сервер сайта.
// Зависит только от application: получает готовые use cases и рендерит их.
package http

import (
	"fmt"
	"html/template"
	"io/fs"
	nethttp "net/http"
	"sync/atomic"

	"coworking/internal/application"
)

// Server — HTTP-сервер сайта коворкинга.
type Server struct {
	catalog       *application.ServiceCatalog
	bookings      *application.BookingService // nil — страница /booking остаётся статической
	siteTitle     string
	templates     map[string]*template.Template
	staticHandler nethttp.Handler
	visits        atomic.Int64 // просмотры страниц (в памяти, сбрасываются при рестарте)
}

// Option настраивает Server при создании (functional options).
type Option func(*Server)

// WithSiteTitle задаёт название сайта в шапке и подвале.
func WithSiteTitle(title string) Option {
	return func(s *Server) {
		s.siteTitle = title
	}
}

// WithBookingService включает приложение бронирования на /booking.
func WithBookingService(bookings *application.BookingService) Option {
	return func(s *Server) {
		s.bookings = bookings
	}
}

// NewServer создаёт HTTP-сервер с переданным use case каталога.
func NewServer(catalog *application.ServiceCatalog, opts ...Option) (*Server, error) {
	if catalog == nil {
		return nil, fmt.Errorf("http: service catalog is nil")
	}

	s := &Server{
		catalog:   catalog,
		siteTitle: "Место",
	}

	for _, opt := range opts {
		opt(s)
	}

	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	s.templates = templates

	staticRoot, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		return nil, fmt.Errorf("http: static subtree: %w", err)
	}
	s.staticHandler = nethttp.FileServer(nethttp.FS(staticRoot))

	return s, nil
}

// Router собирает все маршруты сайта.
func (s *Server) Router() nethttp.Handler {
	mux := nethttp.NewServeMux()

	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /about", s.staticPage("about", "О коворкинге", "about"))
	mux.HandleFunc("GET /office", s.staticPage("office", "Офис", "office"))
	mux.HandleFunc("GET /tariffs", s.staticPage("tariffs", "Тарифы", "tariffs"))
	mux.HandleFunc("GET /news", s.staticPage("news", "Новости", "news"))
	mux.HandleFunc("GET /achievements", s.staticPage("achievements", "Достижения", "achievements"))
	mux.HandleFunc("GET /contacts", s.staticPage("contacts", "Контакты", "contacts"))
	mux.HandleFunc("GET /rules", s.staticPage("rules", "Правила", "rules"))
	mux.HandleFunc("GET /faq", s.staticPage("faq", "Вопросы&Ответы", "faq"))
	if s.bookings != nil {
		mux.HandleFunc("GET /booking", s.handleBookingPage)
		mux.HandleFunc("POST /booking", s.handleBookingCreate)
		mux.HandleFunc("POST /booking/{id}/cancel", s.handleBookingCancel)
	} else {
		mux.HandleFunc("GET /booking", s.staticPage("booking", "Бронирование", "booking"))
	}
	mux.HandleFunc("GET /soon", s.handleSoon)
	mux.Handle("GET /static/", nethttp.StripPrefix("/static/", s.staticHandler))

	return mux
}
