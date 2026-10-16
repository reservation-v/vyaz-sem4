package http

import (
	"errors"
	"fmt"
	"log"
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
	TypeCode     string
	TypeTitle    string
	Capacity     int
	PricePerHour int
	Deposit      int
}

// bookingClientView — клиент для выпадающего списка и таблицы.
type bookingClientView struct {
	ID       int
	FullName string
	Phone    string
	Email    string
	Company  string
}

// spaceTypeView — тип пространства для фильтра поиска.
type spaceTypeView struct {
	Code  string
	Title string
}

// bookingRowView — строка таблицы броней.
type bookingRowView struct {
	ID            int
	Client        string
	Company       string
	Space         string
	SpaceType     string
	DateLabel     string
	TimeLabel     string
	DurationLabel string
	Price         int
	Deposit       int
	Total         int
	StatusLabel   string
	StatusClass   string
}

// bookingFormData — значения формы брони (ввод или редактирование).
type bookingFormData struct {
	Mode      string // "create" или "edit"
	ID        int
	SpaceID   int
	ClientID  int
	Date      string
	Rate      string
	StartHour int
	Duration  int
}

// clientFormData — значения формы клиента (ввод или редактирование).
type clientFormData struct {
	Mode     string // "create" или "edit"
	ID       int
	FullName string
	Phone    string
	Email    string
	Company  string
}

// bookingPageData — данные страницы «Бронирование».
type bookingPageData struct {
	basePageData
	Spaces           []bookingSpaceView
	SpaceTypes       []spaceTypeView
	ClientSpaceTypes []spaceTypeView // типы, которые бронировал выбранный клиент
	Clients          []bookingClientView
	Rows             []bookingRowView
	StartHours       []int
	Durations        []int
	Form             bookingFormData
	ClientForm       clientFormData
	Filter           bookingFilterView
	Summary          bookingSummaryView
	Flash            string
	FlashKind        string // "ok" или "err"
	// Tab - вкладка собственного меню страницы, видимая при отрисовке.
	Tab string
	// TabHint - вкладка, которую просит открыть текущий запрос: редактирование
	// брони или клиента, активный поиск. Пустое значение - оставить текущую.
	TabHint string
}

type bookingFilterView struct {
	Search    string
	ClientID  int
	SpaceType string
}

// bookingSummaryView — краткое содержание всей БД для постоянной панели.
type bookingSummaryView struct {
	Offices           int
	SpaceTypes        int
	Spaces            int
	Clients           int
	Bookings          int
	Confirmed         int
	Cancelled         int
	Revenue           int
	TopSpaceTypeTitle string
	TopSpaceTypeCount int
}

// Границы почасовой брони — дублируют валидацию application
// (порядок часов для выпадающих списков формы).
const (
	bookingMinStartHour = 8
	bookingMaxStartHour = 21
	bookingMaxHours     = 8
)

// handleBookingPage показывает формы ввода/редактирования, поиск, отчёт и сводку.
func (s *Server) handleBookingPage(w http.ResponseWriter, r *http.Request) {
	form, err := s.bookingFormFromRequest(r)
	if err != nil && !errors.Is(err, application.ErrBookingNotFound) {
		http.Error(w, "не удалось загрузить бронь для редактирования", http.StatusInternalServerError)
		return
	}
	clientForm, err := s.clientFormFromRequest(r)
	if err != nil && !errors.Is(err, application.ErrClientNotFound) {
		http.Error(w, "не удалось загрузить клиента для редактирования", http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	flash, kind := flashFromQuery(r)
	if kind == "err" {
		status = http.StatusUnprocessableEntity
	}

	data, err := s.buildBookingPage(r, form, clientForm, flash, kind)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderPage(w, "booking", status, data)
}

// handleBookingCreate обрабатывает отправку формы новой брони (ввод).
func (s *Server) handleBookingCreate(w http.ResponseWriter, r *http.Request) {
	if err := parseBody(r); err != nil {
		http.Error(w, "неверные данные формы", http.StatusBadRequest)
		return
	}

	input := application.BookingInput{
		SpaceID:   formInt(r, "space_id"),
		ClientID:  formInt(r, "client_id"),
		Date:      r.FormValue("date"),
		Rate:      domain.BookingRate(r.FormValue("rate")),
		StartHour: formInt(r, "start_hour"),
		Duration:  formInt(r, "duration"),
	}

	booking, err := s.bookings.CreateBooking(r.Context(), input)
	if err != nil {
		log.Printf("create booking failed: %v", err)
		s.renderBookingError(w, r, bookingFormData{
			Mode: "create", SpaceID: input.SpaceID, ClientID: input.ClientID, Date: input.Date,
			Rate: string(input.Rate), StartHour: input.StartHour, Duration: input.Duration,
		}, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/booking?notice=created&id=%d", booking.ID), http.StatusSeeOther)
}

// handleBookingUpdate обрабатывает отправку формы редактирования брони.
func (s *Server) handleBookingUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := parseBody(r); err != nil {
		http.Error(w, "неверные данные формы", http.StatusBadRequest)
		return
	}

	input := application.BookingInput{
		SpaceID:   formInt(r, "space_id"),
		ClientID:  formInt(r, "client_id"),
		Date:      r.FormValue("date"),
		Rate:      domain.BookingRate(r.FormValue("rate")),
		StartHour: formInt(r, "start_hour"),
		Duration:  formInt(r, "duration"),
	}

	if _, err := s.bookings.UpdateBooking(r.Context(), id, input); err != nil {
		if errors.Is(err, application.ErrBookingNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("update booking %d failed: %v", id, err)
		s.renderBookingError(w, r, bookingFormData{
			Mode: "edit", ID: id, SpaceID: input.SpaceID, ClientID: input.ClientID, Date: input.Date,
			Rate: string(input.Rate), StartHour: input.StartHour, Duration: input.Duration,
		}, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/booking?notice=updated&id=%d", id), http.StatusSeeOther)
}

// handleBookingDelete удаляет бронь по её id. Отмена брони выполняется
// удалением: запись исчезает из таблицы и из счётчиков.
func (s *Server) handleBookingDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := s.bookings.DeleteBooking(r.Context(), id); err != nil {
		if errors.Is(err, application.ErrBookingNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/booking?notice=delete-error", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/booking?notice=deleted", http.StatusSeeOther)
}

// handleClientCreate обрабатывает ввод нового клиента.
func (s *Server) handleClientCreate(w http.ResponseWriter, r *http.Request) {
	if err := parseBody(r); err != nil {
		http.Error(w, "неверные данные формы", http.StatusBadRequest)
		return
	}

	input := clientInputFromForm(r)
	if _, err := s.bookings.CreateClient(r.Context(), input); err != nil {
		log.Printf("create client failed: %v", err)
		s.renderClientError(w, r, clientFormData{
			Mode: "create", FullName: input.FullName, Phone: input.Phone, Email: input.Email, Company: input.Company,
		}, err)
		return
	}

	http.Redirect(w, r, "/booking?notice=client-created", http.StatusSeeOther)
}

// handleClientUpdate обрабатывает редактирование клиента.
func (s *Server) handleClientUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := parseBody(r); err != nil {
		http.Error(w, "неверные данные формы", http.StatusBadRequest)
		return
	}

	input := clientInputFromForm(r)
	if _, err := s.bookings.UpdateClient(r.Context(), id, input); err != nil {
		if errors.Is(err, application.ErrClientNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("update client %d failed: %v", id, err)
		s.renderClientError(w, r, clientFormData{
			Mode: "edit", ID: id, FullName: input.FullName, Phone: input.Phone, Email: input.Email, Company: input.Company,
		}, err)
		return
	}

	http.Redirect(w, r, "/booking?notice=client-updated", http.StatusSeeOther)
}

// handleClientDelete удаляет клиента вместе с его бронями.
func (s *Server) handleClientDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := s.bookings.DeleteClient(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, application.ErrClientNotFound):
			http.NotFound(w, r)
		default:
			log.Printf("delete client %d failed: %v", id, err)
			http.Redirect(w, r, "/booking?notice=client-delete-error", http.StatusSeeOther)
		}
		return
	}

	http.Redirect(w, r, "/booking?notice=client-deleted", http.StatusSeeOther)
}

// renderBookingError повторно показывает страницу с ошибкой и введёнными данными брони.
func (s *Server) renderBookingError(w http.ResponseWriter, r *http.Request, form bookingFormData, cause error) {
	data, err := s.buildBookingPage(r, form, clientFormData{Mode: "create"}, bookingErrorMessage(cause), "err")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderPage(w, "booking", http.StatusUnprocessableEntity, data)
}

// renderClientError повторно показывает страницу с ошибкой и данными формы клиента.
func (s *Server) renderClientError(w http.ResponseWriter, r *http.Request, clientForm clientFormData, cause error) {
	data, err := s.buildBookingPage(r, createBookingForm(), clientForm, clientErrorMessage(cause), "err")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderPage(w, "booking", http.StatusUnprocessableEntity, data)
}

// buildBookingPage собирает все данные страницы: пространства, клиентов, брони, отчёт и сводку.
func (s *Server) buildBookingPage(r *http.Request, form bookingFormData, clientForm clientFormData, flash, kind string) (bookingPageData, error) {
	filter := bookingFilterFromRequest(r)

	spaces, err := s.bookings.Spaces(r.Context())
	if err != nil {
		log.Printf("booking page: load spaces: %v", err)
		return bookingPageData{}, fmt.Errorf("не удалось загрузить пространства")
	}
	spaceTypes, err := s.bookings.SpaceTypes(r.Context())
	if err != nil {
		log.Printf("booking page: load space types: %v", err)
		return bookingPageData{}, fmt.Errorf("не удалось загрузить типы пространств")
	}
	clients, err := s.bookings.Clients(r.Context())
	if err != nil {
		log.Printf("booking page: load clients: %v", err)
		return bookingPageData{}, fmt.Errorf("не удалось загрузить клиентов")
	}

	// Поиск по двум синхронизированным значениям: второй список зависит от клиента.
	var clientSpaceTypes []domain.SpaceType
	if filter.ClientID != 0 {
		clientSpaceTypes, err = s.bookings.ClientSpaceTypes(r.Context(), filter.ClientID)
		switch {
		case errors.Is(err, application.ErrClientNotFound):
			// Неизвестный клиент — игнорируем это условие поиска.
			filter.ClientID = 0
			clientSpaceTypes = nil
		case err != nil:
			log.Printf("booking page: load client space types: %v", err)
			return bookingPageData{}, fmt.Errorf("не удалось загрузить типы пространств клиента")
		default:
			// Тип, которого нет у выбранного клиента, отбрасываем.
			if filter.SpaceType != "" && !spaceTypeInList(clientSpaceTypes, filter.SpaceType) {
				filter.SpaceType = ""
			}
		}
	}

	details, err := s.bookings.BookingsFiltered(r.Context(), filter)
	if err != nil {
		if errors.Is(err, application.ErrInvalidFilter) {
			return bookingPageData{}, fmt.Errorf("проверьте параметры поиска")
		}
		log.Printf("booking page: load bookings: %v", err)
		return bookingPageData{}, fmt.Errorf("не удалось загрузить брони")
	}

	summary, err := s.bookings.Summary(r.Context())
	if err != nil {
		log.Printf("booking page: load summary: %v", err)
		return bookingPageData{}, fmt.Errorf("не удалось рассчитать сводку")
	}

	if form.Mode != "edit" {
		if form.Date == "" {
			form.Date = defaultDate(r)
		}
		if form.Rate == "" {
			form.Rate = string(domain.BookingRateHour)
		}
		if form.StartHour == 0 {
			form.StartHour = 10
		}
		if form.Duration == 0 {
			form.Duration = 2
		}
		if form.SpaceID == 0 && len(spaces) > 0 {
			form.SpaceID = spaces[0].ID
		}
		if form.ClientID == 0 && len(clients) > 0 {
			form.ClientID = clients[0].ID
		}
	}

	tabHint := bookingTabHint(form, clientForm, filter)
	tab := tabHint
	if tab == "" {
		tab = "clients"
	}

	return bookingPageData{
		basePageData:     s.basePage("Бронирование", "booking"),
		Spaces:           toBookingSpaceViews(spaces),
		SpaceTypes:       toSpaceTypeViews(spaceTypes),
		ClientSpaceTypes: toSpaceTypeViews(clientSpaceTypes),
		Clients:          toBookingClientViews(clients),
		Rows:             toBookingRowViews(details),
		StartHours:       hourRange(bookingMinStartHour, bookingMaxStartHour),
		Durations:        hourRange(1, bookingMaxHours),
		Form:             form,
		ClientForm:       clientForm,
		Filter:           toBookingFilterView(filter),
		Summary:          toBookingSummaryView(summary),
		Flash:            flash,
		FlashKind:        kind,
		Tab:              tab,
		TabHint:          tabHint,
	}, nil
}

// bookingFormFromRequest определяет форму брони: редактирование по ?edit=<id> или ввод.
func (s *Server) bookingFormFromRequest(r *http.Request) (bookingFormData, error) {
	idText := r.URL.Query().Get("edit")
	if idText == "" {
		return createBookingForm(), nil
	}
	id, err := strconv.Atoi(idText)
	if err != nil {
		return createBookingForm(), nil
	}

	details, err := s.bookings.BookingByID(r.Context(), id)
	if err != nil {
		return createBookingForm(), err
	}
	return bookingFormData{
		Mode:      "edit",
		ID:        details.Booking.ID,
		SpaceID:   details.Booking.SpaceID,
		ClientID:  details.Booking.ClientID,
		Date:      details.Booking.Date,
		Rate:      string(details.Booking.Rate),
		StartHour: details.Booking.StartHour,
		Duration:  details.Booking.Duration,
	}, nil
}

// clientFormFromRequest определяет форму клиента: редактирование по ?edit_client=<id> или ввод.
func (s *Server) clientFormFromRequest(r *http.Request) (clientFormData, error) {
	idText := r.URL.Query().Get("edit_client")
	if idText == "" {
		return clientFormData{Mode: "create"}, nil
	}
	id, err := strconv.Atoi(idText)
	if err != nil {
		return clientFormData{Mode: "create"}, nil
	}

	clients, err := s.bookings.Clients(r.Context())
	if err != nil {
		return clientFormData{Mode: "create"}, err
	}
	for _, client := range clients {
		if client.ID == id {
			return clientFormData{
				Mode:     "edit",
				ID:       client.ID,
				FullName: client.FullName,
				Phone:    client.Phone,
				Email:    client.Email,
				Company:  client.Company,
			}, nil
		}
	}
	return clientFormData{Mode: "create"}, application.ErrClientNotFound
}

// createBookingForm — форма новой брони со значениями по умолчанию.
func createBookingForm() bookingFormData {
	return bookingFormData{Mode: "create", Rate: string(domain.BookingRateHour), StartHour: 10, Duration: 2}
}

// clientInputFromForm читает поля формы клиента.
func clientInputFromForm(r *http.Request) application.ClientInput {
	return application.ClientInput{
		FullName: r.FormValue("full_name"),
		Phone:    r.FormValue("phone"),
		Email:    r.FormValue("email"),
		Company:  r.FormValue("company"),
	}
}

// parseBody читает форму любого типа (urlencoded и multipart от JS-формы).
func parseBody(r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return err
		}
	}
	return nil
}

// bookingErrorMessage превращает ошибку бронирования в понятное сообщение.
func bookingErrorMessage(err error) string {
	switch {
	case errors.Is(err, application.ErrInvalidDate):
		return "Дата не может быть в прошлом. Формат — ГГГГ-ММ-ДД."
	case errors.Is(err, application.ErrInvalidHours):
		return "Коворкинг работает с 8:00 до 22:00: начало брони с 8:00 до 21:00, длительность от 1 до 8 часов, конец — не позже 22:00."
	case errors.Is(err, application.ErrSpaceNotFound):
		return "Выбранного пространства нет в списке. Обновите страницу."
	case errors.Is(err, application.ErrClientNotFound):
		return "Выбранного клиента нет в списке. Обновите страницу или добавьте клиента."
	case errors.Is(err, application.ErrRateNotAvailable):
		return "Подневная, понедельная и помесячная бронь доступна только для деск-мест."
	case errors.Is(err, application.ErrOverlap):
		return "Пространство уже занято на это время. Выберите другой час или пространство."
	default:
		return "Не удалось сохранить бронь. Попробуйте ещё раз."
	}
}

// clientErrorMessage превращает ошибку ввода/редактирования клиента в сообщение.
func clientErrorMessage(err error) string {
	switch {
	case errors.Is(err, application.ErrInvalidClientName):
		return "Укажите имя клиента: от 1 до 80 символов."
	case errors.Is(err, application.ErrInvalidClientContact):
		return "Проверьте e-mail клиента: например, client@example.com."
	default:
		return "Не удалось сохранить клиента. Попробуйте ещё раз."
	}
}

// flashFromQuery читает сообщение после редиректа (PRG-паттерн).
func flashFromQuery(r *http.Request) (string, string) {
	switch r.URL.Query().Get("notice") {
	case "created":
		return "Бронь создана. Оплата — на ресепшене при входе.", "ok"
	case "updated":
		return "Бронь изменена.", "ok"
	case "deleted":
		return "Бронь удалена из базы данных. Депозит за переговорку вернётся в течение дня.", "ok"
	case "client-created":
		return "Клиент добавлен.", "ok"
	case "client-updated":
		return "Данные клиента изменены.", "ok"
	case "client-deleted":
		return "Клиент удалён вместе с его бронями.", "ok"
	case "client-delete-error":
		return "Не удалось удалить клиента. Обновите страницу и попробуйте снова.", "err"
	case "delete-error":
		return "Не удалось удалить бронь. Обновите страницу и попробуйте снова.", "err"
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

// bookingTabHint выбирает вкладку собственного меню страницы под текущий запрос:
// редактирование брони или клиента, либо активный поиск. Пустая строка означает
// «не навязывать вкладку» - интерфейс оставит открытой текущую.
func bookingTabHint(form bookingFormData, clientForm clientFormData, filter application.BookingFilter) string {
	switch {
	case form.Mode == "edit":
		return "booking"
	case clientForm.Mode == "edit":
		return "clients"
	case filter.Search != "" || filter.ClientID != 0 || filter.SpaceType != "":
		return "search"
	default:
		return ""
	}
}

func bookingFilterFromRequest(r *http.Request) application.BookingFilter {
	query := r.URL.Query()
	return application.BookingFilter{
		Search:    strings.TrimSpace(query.Get("search")),
		ClientID:  queryInt(query.Get("client")),
		SpaceType: domain.SpaceTypeCode(query.Get("space_type")),
	}
}

func toBookingFilterView(filter application.BookingFilter) bookingFilterView {
	return bookingFilterView{
		Search:    filter.Search,
		ClientID:  filter.ClientID,
		SpaceType: string(filter.SpaceType),
	}
}

// spaceTypeInList сообщает, входит ли тип в список типов клиента.
func spaceTypeInList(types []domain.SpaceType, code domain.SpaceTypeCode) bool {
	if code == "" {
		return true
	}
	for _, item := range types {
		if item.Code == code {
			return true
		}
	}
	return false
}

func toBookingSummaryView(summary application.DatabaseSummary) bookingSummaryView {
	return bookingSummaryView{
		Offices:           summary.Offices,
		SpaceTypes:        summary.SpaceTypes,
		Spaces:            summary.Spaces,
		Clients:           summary.Clients,
		Bookings:          summary.Bookings,
		Confirmed:         summary.Confirmed,
		Cancelled:         summary.Cancelled,
		Revenue:           summary.Revenue,
		TopSpaceTypeTitle: summary.TopSpaceTypeTitle,
		TopSpaceTypeCount: summary.TopSpaceTypeCount,
	}
}

func toBookingSpaceViews(spaces []domain.Space) []bookingSpaceView {
	views := make([]bookingSpaceView, 0, len(spaces))
	for _, space := range spaces {
		views = append(views, bookingSpaceView{
			ID:           space.ID,
			Name:         space.Name,
			TypeCode:     string(space.Type.Code),
			TypeTitle:    space.Type.Title,
			Capacity:     space.Capacity,
			PricePerHour: space.PricePerHour,
			Deposit:      space.Deposit,
		})
	}
	return views
}

func toSpaceTypeViews(types []domain.SpaceType) []spaceTypeView {
	views := make([]spaceTypeView, 0, len(types))
	for _, item := range types {
		views = append(views, spaceTypeView{Code: string(item.Code), Title: item.Title})
	}
	return views
}

func toBookingClientViews(clients []domain.Client) []bookingClientView {
	views := make([]bookingClientView, 0, len(clients))
	for _, client := range clients {
		views = append(views, bookingClientView{
			ID:       client.ID,
			FullName: client.FullName,
			Phone:    client.Phone,
			Email:    client.Email,
			Company:  client.Company,
		})
	}
	return views
}

func toBookingRowViews(details []application.BookingDetails) []bookingRowView {
	views := make([]bookingRowView, 0, len(details))
	for _, detail := range details {
		booking := detail.Booking
		row := bookingRowView{
			ID:            booking.ID,
			Client:        detail.ClientName,
			Company:       detail.ClientCompany,
			Space:         detail.SpaceName,
			SpaceType:     detail.SpaceTypeTitle,
			DateLabel:     booking.Date,
			TimeLabel:     bookingTimeLabel(booking),
			DurationLabel: bookingDurationLabel(booking),
			Price:         booking.Price,
			Deposit:       booking.Deposit,
			Total:         booking.Price + booking.Deposit,
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

// queryInt читает целое значение параметра строки запроса (0 при ошибке).
func queryInt(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
