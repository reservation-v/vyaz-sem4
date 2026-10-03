package http

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"coworking/internal/application"
	"coworking/internal/infrastructure/memory"
)

// newTestBookingServer собирает сервер с подключённым приложением бронирования.
func newTestBookingServer(t *testing.T) *Server {
	t.Helper()

	catalog := application.NewServiceCatalog(memory.NewServiceRepository())
	bookings := application.NewBookingService(memory.NewBookingRepository())

	srv, err := NewServer(catalog, WithSiteTitle("Место"), WithBookingService(bookings))
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return srv
}

func TestBookingPageRendersFormAndSeeds(t *testing.T) {
	srv := newTestBookingServer(t)

	req := httptest.NewRequest(http.MethodGet, "/booking", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /booking code = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"Новая бронь",
		"Брони и встречи",
		"Забронировать",
		"Деск A1",
		"Переговорка «Амфитеатр»",
		"Мария Соколова",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("GET /booking body missing %q", want)
		}
	}
}

func TestBookingCreateRedirectsAndShows(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id":   {"1"},
		"client":     {"Тестовый клиент"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"10"},
		"duration":   {"2"},
	}

	req := httptest.NewRequest(http.MethodPost, "/booking", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /booking code = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "notice=created") {
		t.Errorf("Location = %q, want redirect with notice=created", loc)
	}

	// Новая бронь видна в таблице.
	req = httptest.NewRequest(http.MethodGet, "/booking", nil)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Тестовый клиент") {
		t.Error("created booking not visible on the page")
	}
}

func TestBookingOverlapRerendersWithError(t *testing.T) {
	srv := newTestBookingServer(t)

	post := func() *httptest.ResponseRecorder {
		form := url.Values{
			"space_id":   {"1"},
			"client":     {"Клиент"},
			"date":       {time.Now().Format("2006-01-02")},
			"rate":       {"hour"},
			"start_hour": {"10"},
			"duration":   {"2"},
		}
		req := httptest.NewRequest(http.MethodPost, "/booking", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		return rec
	}

	if rec := post(); rec.Code != http.StatusSeeOther {
		t.Fatalf("first POST code = %d, want 303", rec.Code)
	}

	rec := post()
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("overlapping POST code = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "уже занято") {
		t.Error("overlap error message not shown")
	}
}

func TestBookingCancelFlow(t *testing.T) {
	srv := newTestBookingServer(t)

	// Сиды занимают ID 1–2, новая бронь получит ID 3.
	form := url.Values{
		"space_id":   {"2"},
		"client":     {"Отменяемый клиент"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"15"},
		"duration":   {"1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/booking", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST code = %d, want 303", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/booking/3/cancel", nil)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /booking/3/cancel code = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "notice=cancelled") {
		t.Errorf("Location = %q, want notice=cancelled", loc)
	}

	req = httptest.NewRequest(http.MethodGet, "/booking", nil)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "Отменяемый клиент") {
		t.Error("cancelled booking still visible in the table")
	}
	if strings.Contains(body, "/booking/3/cancel") {
		t.Error("cancelled booking still has a cancel button")
	}
}

func TestBookingPeriodCreate(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id": {"1"},
		"client":   {"Постоянный клиент"},
		"date":     {time.Now().Format("2006-01-02")},
		"rate":     {"month"},
	}

	req := httptest.NewRequest(http.MethodPost, "/booking", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /booking (month) code = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}

	// Месячная бронь видна в таблице с меткой периода.
	req = httptest.NewRequest(http.MethodGet, "/booking", nil)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Постоянный клиент") {
		t.Error("month booking not visible on the page")
	}
	if !strings.Contains(body, "Календарный месяц") {
		t.Error("month booking without period label")
	}
}

func TestBookingPeriodRejectedForRoom(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id": {"25"}, // «Кабинет 2–3 №1» — периодные тарифы только для деск-мест
		"client":   {"Клиент"},
		"date":     {time.Now().Format("2006-01-02")},
		"rate":     {"day"},
	}

	req := httptest.NewRequest(http.MethodPost, "/booking", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /booking (day, room) code = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "только для деск-мест") {
		t.Error("period-rate error message not shown")
	}
}

func TestBookingCreateMultipartForm(t *testing.T) {
	srv := newTestBookingServer(t)

	// Браузерный fetch с FormData шлёт multipart/form-data; раньше
	// r.ParseForm() такие запросы не читал и бронь не создавалась.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for key, value := range map[string]string{
		"space_id":   "1",
		"client":     "Мультипарт-клиент",
		"date":       time.Now().Format("2006-01-02"),
		"rate":       "hour",
		"start_hour": "10",
		"duration":   "2",
	} {
		if err := mw.WriteField(key, value); err != nil {
			t.Fatalf("WriteField(%s) error = %v", key, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/booking", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /booking (multipart) code = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/booking", nil)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Мультипарт-клиент") {
		t.Error("multipart booking not visible on the page")
	}
}

func TestBookingEndsAfterClosingRejected(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id":   {"1"},
		"client":     {"Клиент"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"21"},
		"duration":   {"2"},
	}

	req := httptest.NewRequest(http.MethodPost, "/booking", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /booking (21:00 + 2h) code = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "не позже 22:00") {
		t.Error("closing-hour error message not shown")
	}
}
