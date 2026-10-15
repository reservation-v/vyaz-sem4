package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"coworking/internal/domain"
)

// BookingRepository — порт хранилища пространств и броней (DIP:
// интерфейс принадлежит application, реализация — infrastructure).
type BookingRepository interface {
	ListSpaces(ctx context.Context) ([]domain.Space, error)
	ListSpaceTypes(ctx context.Context) ([]domain.SpaceType, error)

	// SearchBookings возвращает брони с подтянутыми клиентом и пространством.
	SearchBookings(ctx context.Context, filter BookingFilter) ([]BookingDetails, error)
	BookingByID(ctx context.Context, id int) (BookingDetails, error)
	BookingStatistics(ctx context.Context, filter BookingFilter) (BookingStatistics, error)
	// DatabaseSummary возвращает краткое содержание всей БД для постоянной панели.
	DatabaseSummary(ctx context.Context) (DatabaseSummary, error)

	// SaveBooking сохраняет новую бронь и возвращает её с присвоенным ID.
	SaveBooking(ctx context.Context, booking domain.Booking) (domain.Booking, error)
	// UpdateBooking сохраняет изменённую бронь (редактирование).
	UpdateBooking(ctx context.Context, booking domain.Booking) (domain.Booking, error)
	// CancelBooking отменяет бронь, сохраняя запись для истории и отчётов.
	CancelBooking(ctx context.Context, id int) error
}

// ClientRepository — порт хранилища клиентов.
type ClientRepository interface {
	ListClients(ctx context.Context) ([]domain.Client, error)
	ClientByID(ctx context.Context, id int) (domain.Client, error)
	SaveClient(ctx context.Context, client domain.Client) (domain.Client, error)
	UpdateClient(ctx context.Context, client domain.Client) (domain.Client, error)
	// DeleteClient удаляет клиента вместе с его бронями.
	DeleteClient(ctx context.Context, id int) error
}

// BookingDetails — бронь с подтянутыми для показа данными клиента и пространства.
type BookingDetails struct {
	Booking        domain.Booking
	ClientName     string
	ClientCompany  string
	SpaceName      string
	SpaceTypeCode  domain.SpaceTypeCode
	SpaceTypeTitle string
}

// BookingFilter ограничивает список броней и расчёт отчёта.
//
// Search — поиск по одному значению (совпадение с именем клиента,
// компанией или названием пространства).
// ClientID и SpaceType — поиск по двум синхронизированным значениям из двух
// таблиц: сначала выбирается клиент (clients), затем тип пространства
// (space_types) только из тех, что бронировал этот клиент.
// SpaceID используется внутри для проверки занятости пространства.
type BookingFilter struct {
	Search    string
	ClientID  int
	SpaceType domain.SpaceTypeCode
	SpaceID   int
}

// BookingStatistics содержит агрегаты по отфильтрованным данным БД.
type BookingStatistics struct {
	TotalBookings     int
	ConfirmedBookings int
	CancelledBookings int
	Revenue           int
	BySpaceType       []BookingTypeStatistics
}

// BookingTypeStatistics содержит показатели для одного типа пространства.
type BookingTypeStatistics struct {
	SpaceType domain.SpaceTypeCode
	Title     string
	Count     int
	Revenue   int
}

// DatabaseSummary — краткое содержание всей БД (агрегация для постоянной панели).
type DatabaseSummary struct {
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

// BookingInput — данные формы брони для ввода и редактирования.
type BookingInput struct {
	SpaceID   int
	ClientID  int
	Date      string
	Rate      domain.BookingRate
	StartHour int
	Duration  int
}

// ClientInput — данные формы клиента для ввода и редактирования.
type ClientInput struct {
	FullName string
	Phone    string
	Email    string
	Company  string
}

// Ошибки сценариев (handler превращает их в сообщения на странице).
var (
	ErrInvalidClientName    = errors.New("invalid client name")
	ErrInvalidClientContact = errors.New("invalid client contact")
	ErrInvalidDate          = errors.New("invalid date")
	ErrInvalidHours         = errors.New("invalid hours")
	ErrSpaceNotFound        = errors.New("space not found")
	ErrClientNotFound       = errors.New("client not found")
	ErrOverlap              = errors.New("space already booked")
	ErrBookingNotFound      = errors.New("booking not found")
	ErrRateNotAvailable     = errors.New("rate not available for this space")
	ErrInvalidFilter        = errors.New("invalid booking filter")
)

const (
	maxClientNameRunes = 80
	minStartHour       = 8
	// maxStartHour — последний час начала почасовой брони.
	maxStartHour = 21
	// maxEndHour — конец рабочего дня: ни одна бронь не может выйти за 22:00.
	maxEndHour      = 22
	maxBookingHours = 8
)

// periodPrices — фиксированные цены периодов для деск-мест (₽).
var periodPrices = map[domain.BookingRate]int{
	domain.BookingRateDay:   900,
	domain.BookingRateWeek:  3500,
	domain.BookingRateMonth: 12000,
}

// BookingService — use case: ввод, показ, редактирование и отмена броней,
// а также ввод клиентов, поиск и агрегация по данным БД.
type BookingService struct {
	bookings BookingRepository
	clients  ClientRepository
	now      func() time.Time // переопределяется в тестах
}

// NewBookingService создаёт сценарий бронирования с переданными хранилищами.
func NewBookingService(bookings BookingRepository, clients ClientRepository) *BookingService {
	return &BookingService{bookings: bookings, clients: clients, now: time.Now}
}

// Spaces возвращает пространства для брони: деск-места первыми, кабинеты в конце.
func (s *BookingService) Spaces(ctx context.Context) ([]domain.Space, error) {
	spaces, err := s.bookings.ListSpaces(ctx)
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(spaces, func(a, b domain.Space) int {
		return spaceTypeOrder(a.Type.Code) - spaceTypeOrder(b.Type.Code)
	})
	return spaces, nil
}

// BookingByID возвращает одну бронь с данными клиента и пространства (для формы редактирования).
func (s *BookingService) BookingByID(ctx context.Context, id int) (BookingDetails, error) {
	return s.bookings.BookingByID(ctx, id)
}

// SpaceTypes возвращает классификатор типов пространств.
func (s *BookingService) SpaceTypes(ctx context.Context) ([]domain.SpaceType, error) {
	return s.bookings.ListSpaceTypes(ctx)
}

// ClientSpaceTypes возвращает типы пространств, которые бронировал клиент.
// Это второй, зависимый (синхронизированный) список поиска по двум значениям:
// его значения определяются выбранным клиентом через JOIN
// clients → bookings → spaces → space_types.
// Для clientID == 0 возвращает пустой список.
func (s *BookingService) ClientSpaceTypes(ctx context.Context, clientID int) ([]domain.SpaceType, error) {
	if clientID == 0 {
		return nil, nil
	}
	if _, err := s.clients.ClientByID(ctx, clientID); err != nil {
		return nil, err
	}

	details, err := s.bookings.SearchBookings(ctx, BookingFilter{ClientID: clientID})
	if err != nil {
		return nil, err
	}

	seen := make(map[domain.SpaceTypeCode]struct{}, len(details))
	types := make([]domain.SpaceType, 0, len(details))
	for _, detail := range details {
		if _, ok := seen[detail.SpaceTypeCode]; ok {
			continue
		}
		seen[detail.SpaceTypeCode] = struct{}{}
		types = append(types, domain.SpaceType{Code: detail.SpaceTypeCode, Title: detail.SpaceTypeTitle})
	}

	slices.SortStableFunc(types, func(a, b domain.SpaceType) int {
		return spaceTypeOrder(a.Code) - spaceTypeOrder(b.Code)
	})
	return types, nil
}

// spaceTypeOrder задаёт порядок типов в списках: деск-места, переговорки, кабинеты.
func spaceTypeOrder(spaceType domain.SpaceTypeCode) int {
	switch spaceType {
	case domain.SpaceTypeDesk:
		return 0
	case domain.SpaceTypeRoom:
		return 1
	case domain.SpaceTypeCabin:
		return 2
	default:
		return 3
	}
}

// Bookings возвращает брони, отсортированные по дате и времени начала.
func (s *BookingService) Bookings(ctx context.Context) ([]BookingDetails, error) {
	return s.BookingsFiltered(ctx, BookingFilter{})
}

// BookingsFiltered возвращает подходящие под фильтр брони, отсортированные по времени.
func (s *BookingService) BookingsFiltered(ctx context.Context, filter BookingFilter) ([]BookingDetails, error) {
	if err := validateBookingFilter(filter); err != nil {
		return nil, err
	}

	details, err := s.bookings.SearchBookings(ctx, filter)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(details, func(a, b BookingDetails) int {
		if a.Booking.Date != b.Booking.Date {
			return strings.Compare(a.Booking.Date, b.Booking.Date)
		}
		if a.Booking.StartHour != b.Booking.StartHour {
			return a.Booking.StartHour - b.Booking.StartHour
		}
		return a.Booking.CreatedAt.Compare(b.Booking.CreatedAt)
	})

	return details, nil
}

// Statistics рассчитывает отчёт по данным бронирования, применяя те же фильтры, что и к списку.
func (s *BookingService) Statistics(ctx context.Context, filter BookingFilter) (BookingStatistics, error) {
	if err := validateBookingFilter(filter); err != nil {
		return BookingStatistics{}, err
	}
	return s.bookings.BookingStatistics(ctx, filter)
}

// Summary возвращает краткое содержание всей БД для постоянной панели агрегации.
func (s *BookingService) Summary(ctx context.Context) (DatabaseSummary, error) {
	return s.bookings.DatabaseSummary(ctx)
}

// Clients возвращает список клиентов для выпадающего списка и таблицы.
func (s *BookingService) Clients(ctx context.Context) ([]domain.Client, error) {
	return s.clients.ListClients(ctx)
}

// CreateClient выполняет ввод нового клиента после контроля значений.
func (s *BookingService) CreateClient(ctx context.Context, input ClientInput) (domain.Client, error) {
	client, err := validateClientInput(input)
	if err != nil {
		return domain.Client{}, err
	}
	return s.clients.SaveClient(ctx, client)
}

// UpdateClient редактирует существующего клиента после контроля значений.
func (s *BookingService) UpdateClient(ctx context.Context, id int, input ClientInput) (domain.Client, error) {
	if _, err := s.clients.ClientByID(ctx, id); err != nil {
		return domain.Client{}, err
	}
	client, err := validateClientInput(input)
	if err != nil {
		return domain.Client{}, err
	}
	client.ID = id
	return s.clients.UpdateClient(ctx, client)
}

// DeleteClient удаляет клиента вместе с его бронями. Брони уходят вслед
// (ON DELETE CASCADE в схеме БД): предварительная отмена не нужна, запись
// клиента удаляется без условий.
func (s *BookingService) DeleteClient(ctx context.Context, id int) error {
	return s.clients.DeleteClient(ctx, id)
}

func validateBookingFilter(filter BookingFilter) error {
	if filter.SpaceType != "" && filter.SpaceType != domain.SpaceTypeDesk &&
		filter.SpaceType != domain.SpaceTypeCabin && filter.SpaceType != domain.SpaceTypeRoom {
		return fmt.Errorf("%w: неизвестный тип пространства", ErrInvalidFilter)
	}
	return nil
}

// CreateBooking создаёт бронь после проверок пространства, клиента, периода,
// тарифа и занятости пространства (ввод).
func (s *BookingService) CreateBooking(ctx context.Context, input BookingInput) (domain.Booking, error) {
	booking, err := s.prepareBooking(ctx, input)
	if err != nil {
		return domain.Booking{}, err
	}
	booking.Status = domain.BookingStatusConfirmed

	if err := s.ensureNoOverlap(ctx, booking, 0); err != nil {
		return domain.Booking{}, err
	}
	return s.bookings.SaveBooking(ctx, booking)
}

// UpdateBooking редактирует бронь: меняет пространство, клиента, дату, тариф и время.
func (s *BookingService) UpdateBooking(ctx context.Context, id int, input BookingInput) (domain.Booking, error) {
	existing, err := s.bookings.BookingByID(ctx, id)
	if err != nil {
		return domain.Booking{}, err
	}

	booking, err := s.prepareBooking(ctx, input)
	if err != nil {
		return domain.Booking{}, err
	}
	booking.ID = id
	booking.Status = existing.Booking.Status // статус меняется отдельно (отмена)

	if err := s.ensureNoOverlap(ctx, booking, id); err != nil {
		return domain.Booking{}, err
	}
	return s.bookings.UpdateBooking(ctx, booking)
}

// CancelBooking отменяет бронь, сохраняя запись в истории.
func (s *BookingService) CancelBooking(ctx context.Context, id int) error {
	return s.bookings.CancelBooking(ctx, id)
}

// prepareBooking проверяет входные данные и рассчитывает стоимость и депозит.
func (s *BookingService) prepareBooking(ctx context.Context, input BookingInput) (domain.Booking, error) {
	bookDate, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("%w: дата в формате ГГГГ-ММ-ДД", ErrInvalidDate)
	}

	today := s.now().Format("2006-01-02")
	if bookDate.Format("2006-01-02") < today {
		return domain.Booking{}, fmt.Errorf("%w: дата не может быть в прошлом", ErrInvalidDate)
	}

	space, err := s.spaceByID(ctx, input.SpaceID)
	if err != nil {
		return domain.Booking{}, err
	}
	if !space.IsActive {
		return domain.Booking{}, fmt.Errorf("%w: пространство %q больше не доступно", ErrSpaceNotFound, space.Name)
	}

	if _, err := s.clients.ClientByID(ctx, input.ClientID); err != nil {
		return domain.Booking{}, err
	}

	if input.Rate != domain.BookingRateHour && space.Type.Code != domain.SpaceTypeDesk {
		return domain.Booking{}, fmt.Errorf("%w: тариф %q для пространства %q", ErrRateNotAvailable, input.Rate, space.Name)
	}

	booking := domain.Booking{
		SpaceID:  input.SpaceID,
		ClientID: input.ClientID,
		Date:     input.Date,
		Rate:     input.Rate,
		Deposit:  space.Deposit,
	}

	switch input.Rate {
	case domain.BookingRateHour:
		if input.StartHour < minStartHour || input.StartHour > maxStartHour ||
			input.Duration < 1 || input.Duration > maxBookingHours ||
			input.StartHour+input.Duration > maxEndHour {
			return domain.Booking{}, fmt.Errorf(
				"%w: начало с %d:00 до %d:00, длительность от 1 до %d часов, конец брони не позже %d:00",
				ErrInvalidHours, minStartHour, maxStartHour, maxBookingHours, maxEndHour,
			)
		}
		booking.StartHour = input.StartHour
		booking.Duration = input.Duration
		booking.Price = space.PricePerHour * input.Duration
	case domain.BookingRateDay:
		booking.StartHour = minStartHour
		booking.Duration = 1
		booking.Price = periodPrices[input.Rate]
	case domain.BookingRateWeek:
		booking.StartHour = minStartHour
		booking.Duration = 7
		booking.Price = periodPrices[input.Rate]
	case domain.BookingRateMonth:
		booking.StartHour = minStartHour
		booking.Duration = 1
		booking.Price = periodPrices[input.Rate]
	default:
		return domain.Booking{}, fmt.Errorf("%w: неизвестный тариф %q", ErrInvalidHours, input.Rate)
	}

	return booking, nil
}

// spaceByID находит пространство по идентификатору.
func (s *BookingService) spaceByID(ctx context.Context, id int) (domain.Space, error) {
	spaces, err := s.bookings.ListSpaces(ctx)
	if err != nil {
		return domain.Space{}, err
	}
	for _, space := range spaces {
		if space.ID == id {
			return space, nil
		}
	}
	return domain.Space{}, fmt.Errorf("%w: id %d", ErrSpaceNotFound, id)
}

// ensureNoOverlap проверяет, что пространство не занято другой действующей бронью.
// excludeID исключает из проверки саму редактируемую бронь.
func (s *BookingService) ensureNoOverlap(ctx context.Context, booking domain.Booking, excludeID int) error {
	details, err := s.bookings.SearchBookings(ctx, BookingFilter{SpaceID: booking.SpaceID})
	if err != nil {
		return err
	}

	for _, detail := range details {
		existing := detail.Booking
		if existing.ID == excludeID || existing.Status != domain.BookingStatusConfirmed {
			continue
		}
		if bookingsOverlap(existing, booking) {
			return fmt.Errorf("%w: пространство занято в выбранное время", ErrOverlap)
		}
	}
	return nil
}

// validateClientInput контролирует значения атрибутов клиента.
func validateClientInput(input ClientInput) (domain.Client, error) {
	fullName := strings.TrimSpace(input.FullName)
	if fullName == "" || utf8.RuneCountInString(fullName) > maxClientNameRunes {
		return domain.Client{}, fmt.Errorf("%w: имя от 1 до %d символов", ErrInvalidClientName, maxClientNameRunes)
	}

	email := strings.TrimSpace(input.Email)
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return domain.Client{}, fmt.Errorf("%w: укажите корректный e-mail", ErrInvalidClientContact)
		}
	}

	return domain.Client{
		FullName: fullName,
		Phone:    strings.TrimSpace(input.Phone),
		Email:    email,
		Company:  strings.TrimSpace(input.Company),
	}, nil
}

// bookingDays возвращает календарный охват брони — полуинтервал [start, end).
func bookingDays(booking domain.Booking) (start, end time.Time) {
	day, err := time.Parse("2006-01-02", booking.Date)
	if err != nil {
		return
	}
	start = day

	switch booking.Rate {
	case domain.BookingRateWeek:
		end = day.AddDate(0, 0, 7)
	case domain.BookingRateMonth:
		nextMonth := time.Date(day.Year(), day.Month()+1, 1, 0, 0, 0, 0, day.Location())
		lastDay := nextMonth.AddDate(0, 1, -1).Day()
		if day.Day() > lastDay {
			end = time.Date(nextMonth.Year(), nextMonth.Month(), lastDay, 0, 0, 0, 0, day.Location())
		} else {
			end = time.Date(nextMonth.Year(), nextMonth.Month(), day.Day(), 0, 0, 0, 0, day.Location())
		}
	default: // hour и day занимают свою календарную дату
		end = day.AddDate(0, 0, 1)
	}
	return
}

// bookingsOverlap проверяет пересечение двух броней одного пространства.
// Периодные брони занимают сутки целиком; две почасовые в один день сверяются по часам.
func bookingsOverlap(a, b domain.Booking) bool {
	aStart, aEnd := bookingDays(a)
	bStart, bEnd := bookingDays(b)

	if !(aStart.Before(bEnd) && bStart.Before(aEnd)) {
		return false
	}

	if a.Rate == domain.BookingRateHour && b.Rate == domain.BookingRateHour {
		return a.Date == b.Date &&
			a.StartHour < b.StartHour+b.Duration &&
			b.StartHour < a.StartHour+a.Duration
	}
	return true
}
