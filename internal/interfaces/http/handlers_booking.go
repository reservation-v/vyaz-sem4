package http

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"coworking/internal/application"
	"coworking/internal/domain"
)

// bookingSpaceView — пространство для выпадающего списка формы.
type bookingSpaceView struct {
	ID           int
	Name         string
	Type         string
	TypeLabel    string
	Capacity     int
	PricePerHour int
	Deposit      int
}

// bookingRowView — строка таблицы броней.
type bookingRowView struct {
	ID            int
	Client        string
	Space         string
	DateLabel     string
	TimeLabel     string
	DurationLabel string
	Price         int
	Deposit       int
	Total         int
	StatusLabel   string
	StatusClass   string
	CanCancel     bool
}

// bookingFormData — значения формы при повторном показе после ошибки.
type bookingFormData struct {
	SpaceID   int
	Client    string
	Date      string
	Rate      string
	StartHour int
	Duration  int
}

// bookingPageData — данные страницы «Бронирование».
type bookingPageData struct {
	basePageData
	Spaces     []bookingSpaceView
	Rows       []bookingRowView
	StartHours []int
	Durations  []int
	Form       bookingFormData
	Flash      string
	FlashKind  string // "ok" или "err"
}

var spaceTypeLabels = map[domain.SpaceType]string{
	domain.SpaceTypeDesk:  "Деск-место",
	domain.SpaceTypeCabin: "Кабинет",
	domain.SpaceTypeRoom:  "Переговорка",
}

// Границы почасовой брони — дублируют валидацию application
// (порядок часов для выпадающих списков формы).
const (
	bookingMinStartHour = 8
	bookingMaxStartHour = 21
	bookingMaxHours     = 8
)

// handleBookingPage показывает форму новой брони и список броней.
func (s *Server) handleBookingPage(w http.ResponseWriter, r *http.Request) {
	spaces, err := s.bookings.Spaces(r.Context())
	if err != nil {
		http.Error(w, "не удалось загрузить пространства", http.StatusInternalServerError)
		return
	}

	bookings, err := s.bookings.Bookings(r.Context())
	if err != nil {
		http.Error(w, "не удалось загрузить брони", http.StatusInternalServerError)
		return
	}

	form := bookingFormData{Date: defaultDate(r), Rate: string(domain.BookingRateHour), StartHour: 10, Duration: 2}
	if len(spaces) > 0 {
		form.SpaceID = spaces[0].ID
	}

	status := http.StatusOK
	flash, kind := flashFromQuery(r)
	if kind == "err" {
		status = http.StatusUnprocessableEntity
	}

	s.renderPage(w, "booking", status, bookingPageData{
		basePageData: s.basePage("Бронирование", "booking"),
		Spaces:       toBookingSpaceViews(spaces),
		Rows:         toBookingRowViews(spaces, bookings),
		StartHours:   hourRange(bookingMinStartHour, bookingMaxStartHour),
		Durations:    hourRange(1, bookingMaxHours),
		Form:         form,
		Flash:        flash,
		FlashKind:    kind,
	})
}

// handleBookingCreate обрабатывает отправку формы новой брони.
func (s *Server) handleBookingCreate(w http.ResponseWriter, r *http.Request) {
	// JS-форма (fetch + FormData) шлёт multipart/form-data, обычная отправка —
	// urlencoded. ParseForm не читает multipart, ParseMultipartForm в новых Go
	// отвергает urlencoded — парсим оба типа явно.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "неверные данные формы", http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "неверные данные формы", http.StatusBadRequest)
			return
		}
	}

	spaceID := formInt(r, "space_id")
	startHour := formInt(r, "start_hour")
	duration := formInt(r, "duration")
	rate := domain.BookingRate(r.FormValue("rate"))

	booking, err := s.bookings.CreateBooking(r.Context(), spaceID, r.FormValue("client"), r.FormValue("date"), rate, startHour, duration)
	if err != nil {
		s.renderBookingError(w, r, spaceID, string(rate), startHour, duration, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/booking?notice=created&id=%d", booking.ID), http.StatusSeeOther)
}

// handleBookingCancel отменяет бронь по её id (запись удаляется).
func (s *Server) handleBookingCancel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := s.bookings.CancelBooking(r.Context(), id); err != nil {
		if errors.Is(err, application.ErrBookingNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/booking?notice=cancel-error", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/booking?notice=cancelled", http.StatusSeeOther)
}

// renderBookingError повторно показывает страницу с ошибкой и введёнными данными.
func (s *Server) renderBookingError(w http.ResponseWriter, r *http.Request, spaceID int, rate string, startHour, duration int, err error) {
	spaces, errSpaces := s.bookings.Spaces(r.Context())
	bookings, errBookings := s.bookings.Bookings(r.Context())
	if errSpaces != nil || errBookings != nil {
		http.Error(w, "не удалось загрузить данные", http.StatusInternalServerError)
		return
	}

	form := bookingFormData{
		SpaceID:   spaceID,
		Client:    r.FormValue("client"),
		Date:      r.FormValue("date"),
		Rate:      rate,
		StartHour: startHour,
		Duration:  duration,
	}

	s.renderPage(w, "booking", http.StatusUnprocessableEntity, bookingPageData{
		basePageData: s.basePage("Бронирование", "booking"),
		Spaces:       toBookingSpaceViews(spaces),
		Rows:         toBookingRowViews(spaces, bookings),
		StartHours:   hourRange(bookingMinStartHour, bookingMaxStartHour),
		Durations:    hourRange(1, bookingMaxHours),
		Form:         form,
		Flash:        bookingErrorMessage(err),
		FlashKind:    "err",
	})
}

// bookingErrorMessage превращает ошибку сценария в понятное сообщение.
func bookingErrorMessage(err error) string {
	switch {
	case errors.Is(err, application.ErrInvalidClientName):
		return "Укажите имя клиента: от 1 до 80 символов."
	case errors.Is(err, application.ErrInvalidDate):
		return "Дата не может быть в прошлом. Формат — ГГГГ-ММ-ДД."
	case errors.Is(err, application.ErrInvalidHours):
		return "Коворкинг работает с 8:00 до 22:00: начало брони с 8:00 до 21:00, длительность от 1 до 8 часов, конец — не позже 22:00."
	case errors.Is(err, application.ErrSpaceNotFound):
		return "Выбранного пространства нет в списке. Обновите страницу."
	case errors.Is(err, application.ErrRateNotAvailable):
		return "Подневная, понедельная и помесячная бронь доступна только для деск-мест."
	case errors.Is(err, application.ErrOverlap):
		return "Пространство уже занято на это время. Выберите другой час или пространство."
	default:
		return "Не удалось создать бронь. Попробуйте ещё раз."
	}
}

// flashFromQuery читает сообщение после редиректа (PRG-паттерн).
func flashFromQuery(r *http.Request) (string, string) {
	switch r.URL.Query().Get("notice") {
	case "created":
		return "Бронь создана. Оплата — на ресепшене при входе.", "ok"
	case "cancelled":
		return "Бронь отменена и удалена из списка. Депозит за переговорку вернётся в течение дня.", "ok"
	case "cancel-error":
		return "Не удалось отменить бронь. Обновите страницу и попробуйте снова.", "err"
	default:
		return "", ""
	}
}

// defaultDate — сегодняшняя дата в формате, который понимает <input type="date">.
func defaultDate(r *http.Request) string {
	if date := r.FormValue("date"); date != "" {
		return date
	}
	return time.Now().Format("2006-01-02")
}

func toBookingSpaceViews(spaces []domain.Space) []bookingSpaceView {
	views := make([]bookingSpaceView, 0, len(spaces))
	for _, space := range spaces {
		views = append(views, bookingSpaceView{
			ID:           space.ID,
			Name:         space.Name,
			Type:         string(space.Type),
			TypeLabel:    spaceTypeLabels[space.Type],
			Capacity:     space.Capacity,
			PricePerHour: space.PricePerHour,
			Deposit:      space.Deposit,
		})
	}
	return views
}

func toBookingRowViews(spaces []domain.Space, bookings []domain.Booking) []bookingRowView {
	spaceNames := make(map[int]string, len(spaces))
	for _, space := range spaces {
		spaceNames[space.ID] = space.Name
	}

	views := make([]bookingRowView, 0, len(bookings))
	for _, booking := range bookings {
		row := bookingRowView{
			ID:            booking.ID,
			Client:        booking.ClientName,
			Space:         spaceNames[booking.SpaceID],
			DateLabel:     booking.Date,
			TimeLabel:     bookingTimeLabel(booking),
			DurationLabel: bookingDurationLabel(booking),
			Price:         booking.Price,
			Deposit:       booking.Deposit,
			Total:         booking.Price + booking.Deposit,
			CanCancel:     true,
		}

		switch booking.Status {
		case domain.BookingStatusConfirmed:
			row.StatusLabel = "Подтверждена"
			row.StatusClass = "status-confirmed"
		default:
			row.StatusLabel = "Отменена"
			row.StatusClass = "status-cancelled"
		}

		views = append(views, row)
	}
	return views
}

// bookingTimeLabel — время брони: часы для почасовой, период для остальных.
func bookingTimeLabel(booking domain.Booking) string {
	switch booking.Rate {
	case domain.BookingRateDay:
		return "Весь день"
	case domain.BookingRateWeek:
		return "Неделя с даты"
	case domain.BookingRateMonth:
		return "Календарный месяц"
	default:
		return fmt.Sprintf("%02d:00–%02d:00", booking.StartHour, booking.StartHour+booking.Duration)
	}
}

// bookingDurationLabel — длительность брони в читаемом виде.
func bookingDurationLabel(booking domain.Booking) string {
	switch booking.Rate {
	case domain.BookingRateDay:
		return "1 день"
	case domain.BookingRateWeek:
		return "7 дней"
	case domain.BookingRateMonth:
		return "1 месяц"
	default:
		return fmt.Sprintf("%d ч", booking.Duration)
	}
}

// hourRange возвращает [start, end] — часы для выпадающих списков.
func hourRange(start, end int) []int {
	hours := make([]int, 0, end-start+1)
	for h := start; h <= end; h++ {
		hours = append(hours, h)
	}
	return hours
}

// formInt читает целое поле формы (0 при ошибке).
func formInt(r *http.Request, name string) int {
	value, err := strconv.Atoi(r.FormValue(name))
	if err != nil {
		return 0
	}
	return value
}
