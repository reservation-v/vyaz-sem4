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
	repo := newFakeRepo()
	bookings := application.NewBookingService(repo, repo)

	srv, err := NewServer(catalog, WithSiteTitle("Место"), WithBookingService(bookings))
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return srv
}

func doGet(t *testing.T, srv *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	return rec
}

func doPostForm(t *testing.T, srv *Server, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	return rec
}

// bookingsTableHTML вырезает разметку таблицы броней, чтобы проверять строки результата,
// а не совпадения в выпадающих списках формы.
func bookingsTableHTML(body string) string {
	idx := strings.Index(body, "booking-table")
	if idx < 0 {
		return ""
	}
	rest := body[idx:]
	end := strings.Index(rest, "</table>")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func TestBookingPageRendersFormAndSeeds(t *testing.T) {
	srv := newTestBookingServer(t)

	rec := doGet(t, srv, "/booking")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /booking code = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"Новая бронь",
		"Новый клиент",
		"Брони и встречи",
		"Забронировать",
		"Деск A1",
		"Переговорка «Амфитеатр»",
		"Мария Соколова",
		"База данных «Место»",
		"Поиск по базе данных",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("GET /booking body missing %q", want)
		}
	}
}

func TestBookingSearchByOneValueAndDatabaseStatistics(t *testing.T) {
	srv := newTestBookingServer(t)

	rec := doGet(t, srv, "/booking?search=Мария&space_type=desk&status=confirmed")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET filtered /booking code = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	table := bookingsTableHTML(body)
	if !strings.Contains(table, "Мария Соколова") {
		t.Error("matching booking is missing from filtered table")
	}
	if strings.Contains(table, "Студия «Волна»") {
		t.Error("non-matching booking is present in filtered table")
	}
	if !strings.Contains(body, `name="search" value="Мария"`) {
		t.Error("search filter value was not kept in the form")
	}
}

func TestBookingSearchByTwoValues(t *testing.T) {
	srv := newTestBookingServer(t)

	// Два значения из двух таблиц: клиент (clients) + тип пространства (space_types).
	rec := doGet(t, srv, "/booking?client_name="+url.QueryEscape("Студия «Волна»")+"&space_type=room")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET two-value /booking code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	table := bookingsTableHTML(body)
	if !strings.Contains(table, "Переговорка «Проект»") {
		t.Error("двухзначный поиск не нашёл бронь Студии «Волна» в переговорке")
	}
	// Бронь Марии на деске не должна попасть в результат.
	if strings.Contains(table, "Деск A2") || strings.Contains(table, "Мария") {
		t.Error("результат двухзначного поиска содержит постороннюю бронь")
	}
}

func TestBookingCreateRedirectsAndShows(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id":   {"1"},
		"client_id":  {"1"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"15"},
		"duration":   {"2"},
	}

	rec := doPostForm(t, srv, "/booking", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /booking code = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "notice=created") {
		t.Errorf("Location = %q, want redirect with notice=created", loc)
	}

	// Новая бронь видна в таблице (строка Деск A1, 15:00–17:00).
	body := doGet(t, srv, "/booking").Body.String()
	if !strings.Contains(body, "Мария Соколова") || !strings.Contains(body, "15:00–17:00") {
		t.Error("created booking not visible on the page")
	}
}

func TestBookingOverlapRerendersWithError(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id":   {"1"},
		"client_id":  {"1"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"15"},
		"duration":   {"2"},
	}

	if rec := doPostForm(t, srv, "/booking", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("first POST code = %d, want 303", rec.Code)
	}

	rec := doPostForm(t, srv, "/booking", form)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("overlapping POST code = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "уже занято") {
		t.Error("overlap error message not shown")
	}
}

func TestBookingEditFormRenders(t *testing.T) {
	srv := newTestBookingServer(t)

	rec := doGet(t, srv, "/booking?edit=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /booking?edit=1 code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Редактирование брони №1") {
		t.Error("edit form heading not shown")
	}
	if !strings.Contains(body, "Сохранить изменения") {
		t.Error("edit form submit button not shown")
	}
	if !strings.Contains(body, `action="/booking/1"`) {
		t.Error("edit form action does not point to the booking")
	}
}

func TestBookingUpdateFlow(t *testing.T) {
	srv := newTestBookingServer(t)

	// Новая бронь получит ID 3 (сиды заняли 1–2).
	create := url.Values{
		"space_id":   {"1"},
		"client_id":  {"1"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"15"},
		"duration":   {"1"},
	}
	if rec := doPostForm(t, srv, "/booking", create); rec.Code != http.StatusSeeOther {
		t.Fatalf("create POST code = %d, want 303", rec.Code)
	}

	update := url.Values{
		"space_id":   {"3"},
		"client_id":  {"2"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"16"},
		"duration":   {"2"},
	}
	rec := doPostForm(t, srv, "/booking/3", update)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update POST code = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "notice=updated") {
		t.Errorf("Location = %q, want notice=updated", loc)
	}

	// Открываем форму редактирования: клиент должен быть обновлён.
	body := doGet(t, srv, "/booking?edit=3").Body.String()
	if !strings.Contains(body, "Студия «Волна»") {
		t.Error("updated booking does not show new client")
	}
}

func TestClientCreateFlow(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"full_name": {"Новый Резидент"},
		"company":   {"ООО «Новичок»"},
		"phone":     {"+7 000 000-00-00"},
		"email":     {"resident@example.com"},
	}
	rec := doPostForm(t, srv, "/booking/clients", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /booking/clients code = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "notice=client-created") {
		t.Errorf("Location = %q, want notice=client-created", loc)
	}

	body := doGet(t, srv, "/booking").Body.String()
	if !strings.Contains(body, "Новый Резидент") {
		t.Error("created client not visible on the page")
	}
}

func TestClientCreateValidationError(t *testing.T) {
	srv := newTestBookingServer(t)

	rec := doPostForm(t, srv, "/booking/clients", url.Values{"full_name": {"   "}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid client POST code = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Укажите имя клиента") {
		t.Error("client validation message not shown")
	}
}

func TestBookingCancelFlow(t *testing.T) {
	srv := newTestBookingServer(t)

	// Сиды занимают ID 1–2, новая бронь получит ID 3.
	form := url.Values{
		"space_id":   {"2"},
		"client_id":  {"1"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"18"},
		"duration":   {"1"},
	}
	if rec := doPostForm(t, srv, "/booking", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("POST code = %d, want 303", rec.Code)
	}

	rec := doPostForm(t, srv, "/booking/3/cancel", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /booking/3/cancel code = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "notice=cancelled") {
		t.Errorf("Location = %q, want notice=cancelled", loc)
	}

	body := doGet(t, srv, "/booking").Body.String()
	if !strings.Contains(body, "Отменена") {
		t.Error("cancelled booking status missing from table")
	}
	if strings.Contains(body, "/booking/3/cancel") {
		t.Error("cancelled booking still has a cancel button")
	}
}

func TestBookingPeriodCreate(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id":  {"1"},
		"client_id": {"1"},
		"date":      {time.Now().Format("2006-01-02")},
		"rate":      {"month"},
	}

	if rec := doPostForm(t, srv, "/booking", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /booking (month) code = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}

	body := doGet(t, srv, "/booking").Body.String()
	if !strings.Contains(body, "Календарный месяц") {
		t.Error("month booking without period label")
	}
}

func TestBookingPeriodRejectedForRoom(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id":  {"5"}, // «Кабинет 2–3 №1» — периодные тарифы только для деск-мест
		"client_id": {"1"},
		"date":      {time.Now().Format("2006-01-02")},
		"rate":      {"day"},
	}

	rec := doPostForm(t, srv, "/booking", form)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /booking (day, cabin) code = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "только для деск-мест") {
		t.Error("period-rate error message not shown")
	}
}

func TestBookingCreateMultipartForm(t *testing.T) {
	srv := newTestBookingServer(t)

	// Браузерный fetch с FormData шлёт multipart/form-data.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for key, value := range map[string]string{
		"space_id":   "1",
		"client_id":  "2",
		"date":       time.Now().Format("2006-01-02"),
		"rate":       "hour",
		"start_hour": "15",
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

	page := doGet(t, srv, "/booking").Body.String()
	if !strings.Contains(page, "15:00–17:00") {
		t.Error("multipart booking not visible on the page")
	}
}

func TestBookingEndsAfterClosingRejected(t *testing.T) {
	srv := newTestBookingServer(t)

	form := url.Values{
		"space_id":   {"1"},
		"client_id":  {"1"},
		"date":       {time.Now().Format("2006-01-02")},
		"rate":       {"hour"},
		"start_hour": {"21"},
		"duration":   {"2"},
	}

	rec := doPostForm(t, srv, "/booking", form)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /booking (21:00 + 2h) code = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "не позже 22:00") {
		t.Error("closing-hour error message not shown")
	}
}

func TestBookingUnknownEditFallsBackToCreate(t *testing.T) {
	srv := newTestBookingServer(t)

	rec := doGet(t, srv, "/booking?edit=404")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /booking?edit=404 code = %d, want 200 (fallback)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Новая бронь") {
		t.Error("missing booking should fall back to the create form")
	}
}
