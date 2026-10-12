package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"coworking/internal/domain"
)

// BookingRepository — порт хранилища броней (DIP:
// интерфейс принадлежит application, реализация — infrastructure).
type BookingRepository interface {
	ListSpaces(ctx context.Context) ([]domain.Space, error)
	ListBookings(ctx context.Context) ([]domain.Booking, error)
	SearchBookings(ctx context.Context, filter BookingFilter) ([]domain.Booking, error)
	BookingStatistics(ctx context.Context, filter BookingFilter) (BookingStatistics, error)
	// SaveBooking сохраняет новую бронь и возвращает её с присвоенным ID.
	SaveBooking(ctx context.Context, booking domain.Booking) (domain.Booking, error)
	BookingByID(ctx context.Context, id int) (domain.Booking, error)
	// DeleteBooking отменяет бронь, сохраняя запись для истории и отчётов.
	DeleteBooking(ctx context.Context, id int) error
}

// BookingFilter ограничивает список броней и расчёт отчёта.
type BookingFilter struct {
	Search    string
	SpaceType domain.SpaceType
	Status    domain.BookingStatus
	DateFrom  string
	DateTo    string
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
	SpaceType domain.SpaceType
	Count     int
	Revenue   int
}

// Ошибки сценария бронирования (handler превращает их в сообщения на странице).
var (
	ErrInvalidClientName = errors.New("invalid client name")
	ErrInvalidDate       = errors.New("invalid date")
	ErrInvalidHours      = errors.New("invalid hours")
	ErrSpaceNotFound     = errors.New("space not found")
	ErrOverlap           = errors.New("space already booked")
	ErrBookingNotFound   = errors.New("booking not found")
	ErrRateNotAvailable  = errors.New("rate not available for this space")
	ErrInvalidFilter     = errors.New("invalid booking filter")
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

// BookingService — use case: создание, показ и отмена броней.
type BookingService struct {
	repo BookingRepository
	now  func() time.Time // переопределяется в тестах
}

// NewBookingService создаёт сценарий бронирования с переданным хранилищем.
func NewBookingService(repo BookingRepository) *BookingService {
	return &BookingService{repo: repo, now: time.Now}
}

// Spaces возвращает пространства для брони: деск-места первыми, кабинеты в конце.
func (s *BookingService) Spaces(ctx context.Context) ([]domain.Space, error) {
	spaces, err := s.repo.ListSpaces(ctx)
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(spaces, func(a, b domain.Space) int {
		return spaceTypeOrder(a.Type) - spaceTypeOrder(b.Type)
	})
	return spaces, nil
}

// spaceTypeOrder задаёт порядок типов в списках: деск-места, переговорки, кабинеты.
func spaceTypeOrder(spaceType domain.SpaceType) int {
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
func (s *BookingService) Bookings(ctx context.Context) ([]domain.Booking, error) {
	return s.BookingsFiltered(ctx, BookingFilter{})
}

// BookingsFiltered возвращает подходящие под фильтр брони по дате и времени.
func (s *BookingService) BookingsFiltered(ctx context.Context, filter BookingFilter) ([]domain.Booking, error) {
	if err := validateBookingFilter(filter); err != nil {
		return nil, err
	}

	bookings, err := s.repo.SearchBookings(ctx, filter)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(bookings, func(a, b domain.Booking) int {
		if a.Date != b.Date {
			return strings.Compare(a.Date, b.Date)
		}
		if a.StartHour != b.StartHour {
			return a.StartHour - b.StartHour
		}
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	return bookings, nil
}

// Statistics рассчитывает отчёт по данным бронирования, применяя те же фильтры, что и к списку.
func (s *BookingService) Statistics(ctx context.Context, filter BookingFilter) (BookingStatistics, error) {
	if err := validateBookingFilter(filter); err != nil {
		return BookingStatistics{}, err
	}
	return s.repo.BookingStatistics(ctx, filter)
}

func validateBookingFilter(filter BookingFilter) error {
	if filter.SpaceType != "" && filter.SpaceType != domain.SpaceTypeDesk &&
		filter.SpaceType != domain.SpaceTypeCabin && filter.SpaceType != domain.SpaceTypeRoom {
		return fmt.Errorf("%w: неизвестный тип пространства", ErrInvalidFilter)
	}
	if filter.Status != "" && filter.Status != domain.BookingStatusConfirmed && filter.Status != domain.BookingStatusCancelled {
		return fmt.Errorf("%w: неизвестный статус брони", ErrInvalidFilter)
	}

	var from, to time.Time
	var err error
	if filter.DateFrom != "" {
		from, err = time.Parse("2006-01-02", filter.DateFrom)
		if err != nil {
			return fmt.Errorf("%w: дата начала должна быть в формате ГГГГ-ММ-ДД", ErrInvalidFilter)
		}
	}
	if filter.DateTo != "" {
		to, err = time.Parse("2006-01-02", filter.DateTo)
		if err != nil {
			return fmt.Errorf("%w: дата окончания должна быть в формате ГГГГ-ММ-ДД", ErrInvalidFilter)
		}
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return fmt.Errorf("%w: дата начала позже даты окончания", ErrInvalidFilter)
	}
	return nil
}

// CreateBooking создаёт бронь после проверок имени, периода, тарифа и занятости пространства.
func (s *BookingService) CreateBooking(
	ctx context.Context,
	spaceID int,
	clientName, date string,
	rate domain.BookingRate,
	startHour, duration int,
) (domain.Booking, error) {
	clientName = strings.TrimSpace(clientName)
	if clientName == "" || utf8.RuneCountInString(clientName) > maxClientNameRunes {
		return domain.Booking{}, fmt.Errorf("%w: имя клиента от 1 до %d символов", ErrInvalidClientName, maxClientNameRunes)
	}

	bookDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("%w: дата в формате ГГГГ-ММ-ДД", ErrInvalidDate)
	}

	today := s.now().Format("2006-01-02")
	if bookDate.Format("2006-01-02") < today {
		return domain.Booking{}, fmt.Errorf("%w: дата не может быть в прошлом", ErrInvalidDate)
	}

	spaces, err := s.repo.ListSpaces(ctx)
	if err != nil {
		return domain.Booking{}, err
	}

	var space domain.Space
	for _, candidate := range spaces {
		if candidate.ID == spaceID {
			space = candidate
			break
		}
	}
	if space.ID == 0 {
		return domain.Booking{}, fmt.Errorf("%w: id %d", ErrSpaceNotFound, spaceID)
	}

	// Периодные тарифы доступны только деск-местам.
	if rate != domain.BookingRateHour && space.Type != domain.SpaceTypeDesk {
		return domain.Booking{}, fmt.Errorf("%w: тариф %q для пространства %q", ErrRateNotAvailable, rate, space.Name)
	}

	booking := domain.Booking{
		SpaceID:    spaceID,
		ClientName: clientName,
		Date:       date,
		Rate:       rate,
		Status:     domain.BookingStatusConfirmed,
	}

	switch rate {
	case domain.BookingRateHour:
		if startHour < minStartHour || startHour > maxStartHour ||
			duration < 1 || duration > maxBookingHours ||
			startHour+duration > maxEndHour {
			return domain.Booking{}, fmt.Errorf(
				"%w: начало с %d:00 до %d:00, длительность от 1 до %d часов, конец брони не позже %d:00",
				ErrInvalidHours, minStartHour, maxStartHour, maxBookingHours, maxEndHour,
			)
		}
		booking.StartHour = startHour
		booking.Duration = duration
		booking.Price = space.PricePerHour * duration

	case domain.BookingRateDay:
		booking.Duration = 1
		booking.Price = periodPrices[rate]
	case domain.BookingRateWeek:
		booking.Duration = 7
		booking.Price = periodPrices[rate]
	case domain.BookingRateMonth:
		booking.Duration = 1
		booking.Price = periodPrices[rate]
	default:
		return domain.Booking{}, fmt.Errorf("%w: неизвестный тариф %q", ErrInvalidHours, rate)
	}

	booking.Deposit = space.Deposit

	bookings, err := s.repo.ListBookings(ctx)
	if err != nil {
		return domain.Booking{}, err
	}

	for _, existing := range bookings {
		if existing.SpaceID != spaceID || existing.Status != domain.BookingStatusConfirmed {
			continue
		}
		if bookingsOverlap(existing, booking) {
			return domain.Booking{}, fmt.Errorf("%w: %s уже занят в эти даты", ErrOverlap, space.Name)
		}
	}

	return s.repo.SaveBooking(ctx, booking)
}

// CancelBooking отменяет бронь, сохраняя запись в истории.
func (s *BookingService) CancelBooking(ctx context.Context, id int) error {
	return s.repo.DeleteBooking(ctx, id)
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
