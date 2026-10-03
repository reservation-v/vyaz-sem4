package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"coworking/internal/application"
	"coworking/internal/domain"
)

// Проверка на этапе компиляции: BookingRepository удовлетворяет порту application.
var _ application.BookingRepository = (*BookingRepository)(nil)

// BookingRepository хранит пространства и брони в памяти.
type BookingRepository struct {
	mu       sync.Mutex
	spaces   []domain.Space
	bookings []domain.Booking
	nextID   int
}

// NewBookingRepository создаёт хранилище с пространствами офиса «Место».
func NewBookingRepository() *BookingRepository {
	spaces := seedSpaces()
	bookings := seedBookings(spaces)
	for i := range bookings {
		bookings[i].ID = i + 1
	}

	return &BookingRepository{
		spaces:   spaces,
		bookings: bookings,
		nextID:   len(bookings) + 1,
	}
}

// ListSpaces возвращает копию каталога пространств.
func (r *BookingRepository) ListSpaces(_ context.Context) ([]domain.Space, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]domain.Space(nil), r.spaces...), nil
}

// ListBookings возвращает копию списка броней.
func (r *BookingRepository) ListBookings(_ context.Context) ([]domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]domain.Booking(nil), r.bookings...), nil
}

// SaveBooking присваивает брони ID и сохраняет её.
func (r *BookingRepository) SaveBooking(_ context.Context, booking domain.Booking) (domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	booking.ID = r.nextID
	r.nextID++
	booking.CreatedAt = time.Now()
	r.bookings = append(r.bookings, booking)

	return booking, nil
}

// BookingByID возвращает бронь по идентификатору.
func (r *BookingRepository) BookingByID(_ context.Context, id int) (domain.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, booking := range r.bookings {
		if booking.ID == id {
			return booking, nil
		}
	}

	return domain.Booking{}, application.ErrBookingNotFound
}

// DeleteBooking удаляет бронь по идентификатору.
func (r *BookingRepository) DeleteBooking(_ context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, booking := range r.bookings {
		if booking.ID == id {
			r.bookings = append(r.bookings[:i], r.bookings[i+1:]...)
			return nil
		}
	}

	return application.ErrBookingNotFound
}

// seedSpaces — пространства офиса «Место» по решениям проекта:
// 24 деск-места (4 зоны по 6), 6 кабинетов, 3 переговорки.
func seedSpaces() []domain.Space {
	spaces := make([]domain.Space, 0, 33)

	// Деск-места: 200 ₽/час, зоны A–D по 6 столов.
	zones := []string{"A", "B", "C", "D"}
	for _, zone := range zones {
		for n := 1; n <= 6; n++ {
			spaces = append(spaces, domain.Space{
				ID:           len(spaces) + 1,
				Name:         fmt.Sprintf("Деск %s%d", zone, n),
				Type:         domain.SpaceTypeDesk,
				Capacity:     1,
				PricePerHour: 200,
			})
		}
	}

	// Кабинеты: 2–3 места — 350 ₽/час, 4–6 мест — 450 ₽/час.
	for n := 1; n <= 3; n++ {
		spaces = append(spaces, domain.Space{
			ID:           len(spaces) + 1,
			Name:         fmt.Sprintf("Кабинет 2–3 №%d", n),
			Type:         domain.SpaceTypeCabin,
			Capacity:     3,
			PricePerHour: 350,
		})
	}
	for n := 1; n <= 3; n++ {
		spaces = append(spaces, domain.Space{
			ID:           len(spaces) + 1,
			Name:         fmt.Sprintf("Кабинет 4–6 №%d", n),
			Type:         domain.SpaceTypeCabin,
			Capacity:     6,
			PricePerHour: 450,
		})
	}

	// Переговорки: 700/1 000/1 800 ₽/час + депозит 2 000/3 000/5 000 ₽.
	spaces = append(spaces,
		domain.Space{ID: len(spaces) + 1, Name: "Переговорка «Встреча»", Type: domain.SpaceTypeRoom, Capacity: 4, PricePerHour: 700, Deposit: 2000},
		domain.Space{ID: len(spaces) + 1, Name: "Переговорка «Проект»", Type: domain.SpaceTypeRoom, Capacity: 6, PricePerHour: 1000, Deposit: 3000},
		domain.Space{ID: len(spaces) + 1, Name: "Переговорка «Амфитеатр»", Type: domain.SpaceTypeRoom, Capacity: 12, PricePerHour: 1800, Deposit: 5000},
	)

	return spaces
}

// seedBookings — пара демонстрационных броней на сегодня и завтра,
// чтобы таблица броней не пустовала при первом запуске.
func seedBookings(spaces []domain.Space) []domain.Booking {
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	// Деск A2 — сегодня с 10:00 на 2 часа (индекс 1: A1, A2, ...).
	desk := spaces[1]
	room := spaces[len(spaces)-2] // «Проект»

	return []domain.Booking{
		{
			SpaceID:    desk.ID,
			ClientName: "Мария Соколова",
			Date:       today,
			Rate:       domain.BookingRateHour,
			StartHour:  10,
			Duration:   2,
			Price:      desk.PricePerHour * 2,
			Deposit:    desk.Deposit,
			Status:     domain.BookingStatusConfirmed,
			CreatedAt:  time.Now().Add(-time.Hour),
		},
		{
			SpaceID:    room.ID,
			ClientName: "Студия «Волна»",
			Date:       tomorrow,
			Rate:       domain.BookingRateHour,
			StartHour:  12,
			Duration:   2,
			Price:      room.PricePerHour * 2,
			Deposit:    room.Deposit,
			Status:     domain.BookingStatusConfirmed,
			CreatedAt:  time.Now().Add(-30 * time.Minute),
		},
	}
}
