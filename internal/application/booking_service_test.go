package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"coworking/internal/domain"
)

// stubBookingRepo — тестовое хранилище броней в памяти.
type stubBookingRepo struct {
	spaces   []domain.Space
	bookings []domain.Booking
	nextID   int
}

func (r *stubBookingRepo) ListSpaces(context.Context) ([]domain.Space, error) {
	return append([]domain.Space(nil), r.spaces...), nil
}

func (r *stubBookingRepo) ListBookings(context.Context) ([]domain.Booking, error) {
	return append([]domain.Booking(nil), r.bookings...), nil
}

func (r *stubBookingRepo) SaveBooking(_ context.Context, booking domain.Booking) (domain.Booking, error) {
	booking.ID = r.nextID
	r.nextID++
	booking.CreatedAt = time.Now()
	r.bookings = append(r.bookings, booking)
	return booking, nil
}

func (r *stubBookingRepo) BookingByID(_ context.Context, id int) (domain.Booking, error) {
	for _, booking := range r.bookings {
		if booking.ID == id {
			return booking, nil
		}
	}
	return domain.Booking{}, ErrBookingNotFound
}

func (r *stubBookingRepo) DeleteBooking(_ context.Context, id int) error {
	for i, booking := range r.bookings {
		if booking.ID == id {
			r.bookings = append(r.bookings[:i], r.bookings[i+1:]...)
			return nil
		}
	}
	return ErrBookingNotFound
}

// fixedNow — фиксированные «сейчас» для детерминированных тестов.
var fixedNow = time.Date(2026, time.October, 10, 9, 0, 0, 0, time.UTC)

func newTestBookingService() *BookingService {
	svc := NewBookingService(&stubBookingRepo{
		spaces: []domain.Space{
			{ID: 1, Name: "Деск A1", Type: domain.SpaceTypeDesk, Capacity: 1, PricePerHour: 200},
			{ID: 2, Name: "Переговорка «Встреча»", Type: domain.SpaceTypeRoom, Capacity: 4, PricePerHour: 700, Deposit: 2000},
			{ID: 3, Name: "Кабинет 4–6 №1", Type: domain.SpaceTypeCabin, Capacity: 6, PricePerHour: 450},
		},
		nextID: 1,
	})
	svc.now = func() time.Time { return fixedNow }
	return svc
}

// createHour — почасовая бронь: сигнатура короче для табличных тестов.
func createHour(t *testing.T, svc *BookingService, spaceID int, client, date string, startHour, duration int) (domain.Booking, error) {
	t.Helper()
	return svc.CreateBooking(context.Background(), spaceID, client, date, domain.BookingRateHour, startHour, duration)
}

func TestCreateBookingWithDesk(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 1, "  Иван Петров  ", "2026-10-12", 10, 3)
	if err != nil {
		t.Fatalf("CreateBooking() error = %v", err)
	}

	if booking.Price != 600 { // 200 ₽/ч × 3
		t.Errorf("Price = %d, want 600", booking.Price)
	}
	if booking.Deposit != 0 {
		t.Errorf("Deposit = %d, want 0", booking.Deposit)
	}
	if booking.Status != domain.BookingStatusConfirmed {
		t.Errorf("Status = %q, want confirmed", booking.Status)
	}
	if booking.ClientName != "Иван Петров" {
		t.Errorf("ClientName = %q, want trimmed name", booking.ClientName)
	}
	if booking.ID == 0 {
		t.Error("ID = 0, want assigned id")
	}
}

func TestCreateBookingWithRoomDeposit(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 2, "Студия «Волна»", "2026-10-12", 14, 2)
	if err != nil {
		t.Fatalf("CreateBooking() error = %v", err)
	}

	if booking.Price != 1400 { // 700 ₽/ч × 2
		t.Errorf("Price = %d, want 1400", booking.Price)
	}
	if booking.Deposit != 2000 {
		t.Errorf("Deposit = %d, want 2000", booking.Deposit)
	}
}

func TestCreateBookingPeriodRates(t *testing.T) {
	tests := []struct {
		rate         domain.BookingRate
		wantPrice    int
		wantDuration int
	}{
		{domain.BookingRateDay, 900, 1},
		{domain.BookingRateWeek, 3500, 7},
		{domain.BookingRateMonth, 12000, 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.rate), func(t *testing.T) {
			svc := newTestBookingService()

			booking, err := svc.CreateBooking(context.Background(), 1, "Иван", "2026-10-12", tt.rate, 0, 0)
			if err != nil {
				t.Fatalf("CreateBooking() error = %v", err)
			}
			if booking.Price != tt.wantPrice {
				t.Errorf("Price = %d, want %d", booking.Price, tt.wantPrice)
			}
			if booking.Duration != tt.wantDuration {
				t.Errorf("Duration = %d, want %d", booking.Duration, tt.wantDuration)
			}
		})
	}
}

func TestPeriodRateRejectedForRoomsAndCabins(t *testing.T) {
	for _, spaceID := range []int{2, 3} {
		svc := newTestBookingService()
		_, err := svc.CreateBooking(context.Background(), spaceID, "Иван", "2026-10-12", domain.BookingRateDay, 0, 0)
		if !errors.Is(err, ErrRateNotAvailable) {
			t.Errorf("space %d: error = %v, want ErrRateNotAvailable", spaceID, err)
		}
	}
}

func TestCreateBookingOverlap(t *testing.T) {
	svc := newTestBookingService()

	if _, err := createHour(t, svc, 1, "Иван", "2026-10-12", 10, 2); err != nil {
		t.Fatalf("first booking: %v", err)
	}

	// Полное пересечение.
	if _, err := createHour(t, svc, 1, "Пётр", "2026-10-12", 11, 1); !errors.Is(err, ErrOverlap) {
		t.Errorf("overlap error = %v, want ErrOverlap", err)
	}

	// Бронь, начинающаяся в момент окончания предыдущей (10 + 2 = 12), — свободна.
	if _, err := createHour(t, svc, 1, "Пётр", "2026-10-12", 12, 1); err != nil {
		t.Errorf("adjacent booking error = %v, want nil", err)
	}
}

func TestSameTimeDifferentSpace(t *testing.T) {
	svc := newTestBookingService()

	if _, err := createHour(t, svc, 1, "Иван", "2026-10-12", 10, 2); err != nil {
		t.Fatalf("first booking: %v", err)
	}

	if _, err := createHour(t, svc, 3, "Пётр", "2026-10-12", 10, 2); err != nil {
		t.Errorf("different space error = %v, want nil", err)
	}
}

func TestCancelledBookingDeleted(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 1, "Иван", "2026-10-12", 10, 2)
	if err != nil {
		t.Fatalf("first booking: %v", err)
	}

	if err := svc.CancelBooking(context.Background(), booking.ID); err != nil {
		t.Fatalf("CancelBooking() error = %v", err)
	}

	// Отменённая бронь удалена, место снова свободно.
	if _, err := svc.repo.BookingByID(context.Background(), booking.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("BookingByID() error = %v, want ErrBookingNotFound", err)
	}
	if _, err := createHour(t, svc, 1, "Пётр", "2026-10-12", 10, 2); err != nil {
		t.Errorf("booking after cancel error = %v, want nil", err)
	}
}

func TestPeriodBookingsOverlap(t *testing.T) {
	t.Run("day blocks hourly same date", func(t *testing.T) {
		svc := newTestBookingService()
		if _, err := svc.CreateBooking(context.Background(), 1, "Иван", "2026-10-12", domain.BookingRateDay, 0, 0); err != nil {
			t.Fatalf("day booking: %v", err)
		}
		if _, err := createHour(t, svc, 1, "Пётр", "2026-10-12", 10, 1); !errors.Is(err, ErrOverlap) {
			t.Errorf("hour on same date error = %v, want ErrOverlap", err)
		}
		if _, err := createHour(t, svc, 1, "Пётр", "2026-10-13", 10, 1); err != nil {
			t.Errorf("hour next day error = %v, want nil", err)
		}
	})

	t.Run("week blocks any day inside", func(t *testing.T) {
		svc := newTestBookingService()
		if _, err := svc.CreateBooking(context.Background(), 1, "Иван", "2026-10-12", domain.BookingRateWeek, 0, 0); err != nil {
			t.Fatalf("week booking: %v", err)
		}
		if _, err := createHour(t, svc, 1, "Пётр", "2026-10-16", 10, 1); !errors.Is(err, ErrOverlap) {
			t.Errorf("hour inside week error = %v, want ErrOverlap", err)
		}
		if _, err := createHour(t, svc, 1, "Пётр", "2026-10-19", 10, 1); err != nil {
			t.Errorf("hour after week error = %v, want nil", err)
		}
	})

	t.Run("two week bookings overlap", func(t *testing.T) {
		svc := newTestBookingService()
		if _, err := svc.CreateBooking(context.Background(), 1, "Иван", "2026-10-12", domain.BookingRateWeek, 0, 0); err != nil {
			t.Fatalf("first week booking: %v", err)
		}
		if _, err := svc.CreateBooking(context.Background(), 1, "Пётр", "2026-10-14", domain.BookingRateWeek, 0, 0); !errors.Is(err, ErrOverlap) {
			t.Errorf("overlapping week error = %v, want ErrOverlap", err)
		}
		if _, err := svc.CreateBooking(context.Background(), 1, "Пётр", "2026-10-19", domain.BookingRateWeek, 0, 0); err != nil {
			t.Errorf("adjacent week error = %v, want nil", err)
		}
	})
}

func TestCreateBookingValidation(t *testing.T) {
	tests := []struct {
		name      string
		spaceID   int
		client    string
		date      string
		startHour int
		duration  int
		wantErr   error
	}{
		{name: "empty client", spaceID: 1, client: "   ", date: "2026-10-12", startHour: 10, duration: 1, wantErr: ErrInvalidClientName},
		{name: "long client", spaceID: 1, client: string(make([]rune, 81)), date: "2026-10-12", startHour: 10, duration: 1, wantErr: ErrInvalidClientName},
		{name: "bad date format", spaceID: 1, client: "Иван", date: "12.10.2026", startHour: 10, duration: 1, wantErr: ErrInvalidDate},
		{name: "past date", spaceID: 1, client: "Иван", date: "2026-10-09", startHour: 10, duration: 1, wantErr: ErrInvalidDate},
		{name: "today ok", spaceID: 1, client: "Иван", date: "2026-10-10", startHour: 10, duration: 1, wantErr: nil},
		{name: "too early", spaceID: 1, client: "Иван", date: "2026-10-12", startHour: 7, duration: 1, wantErr: ErrInvalidHours},
		{name: "too late start", spaceID: 1, client: "Иван", date: "2026-10-12", startHour: 22, duration: 1, wantErr: ErrInvalidHours},
		{name: "ends after close", spaceID: 1, client: "Иван", date: "2026-10-12", startHour: 21, duration: 2, wantErr: ErrInvalidHours},
		{name: "closing hour ok", spaceID: 1, client: "Иван", date: "2026-10-12", startHour: 21, duration: 1, wantErr: nil},
		{name: "duration zero", spaceID: 1, client: "Иван", date: "2026-10-12", startHour: 10, duration: 0, wantErr: ErrInvalidHours},
		{name: "duration too long", spaceID: 1, client: "Иван", date: "2026-10-12", startHour: 10, duration: 9, wantErr: ErrInvalidHours},
		{name: "unknown rate", spaceID: 1, client: "Иван", date: "2026-10-12", startHour: 10, duration: 1, wantErr: ErrInvalidHours},
		{name: "unknown space", spaceID: 999, client: "Иван", date: "2026-10-12", startHour: 10, duration: 1, wantErr: ErrSpaceNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestBookingService()

			rate := domain.BookingRateHour
			if tt.name == "unknown rate" {
				rate = "year"
			}

			_, err := svc.CreateBooking(context.Background(), tt.spaceID, tt.client, tt.date, rate, tt.startHour, tt.duration)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateBooking() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCancelBooking(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 1, "Иван", "2026-10-12", 10, 1)
	if err != nil {
		t.Fatalf("CreateBooking() error = %v", err)
	}

	if err := svc.CancelBooking(context.Background(), booking.ID); err != nil {
		t.Fatalf("CancelBooking() error = %v", err)
	}

	// Повторная отмена и несуществующая бронь — одна ошибка.
	if err := svc.CancelBooking(context.Background(), booking.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("second cancel error = %v, want ErrBookingNotFound", err)
	}
	if err := svc.CancelBooking(context.Background(), 404); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("unknown booking error = %v, want ErrBookingNotFound", err)
	}
}

func TestBookingsSortedByDateAndHour(t *testing.T) {
	svc := newTestBookingService()

	for _, params := range []struct {
		date string
		hour int
	}{
		{"2026-10-12", 10},
		{"2026-10-11", 15},
		{"2026-10-12", 9},
	} {
		if _, err := createHour(t, svc, 1, "Иван", params.date, params.hour, 1); err != nil {
			t.Fatalf("CreateBooking() error = %v", err)
		}
	}

	bookings, err := svc.Bookings(context.Background())
	if err != nil {
		t.Fatalf("Bookings() error = %v", err)
	}

	want := []string{"2026-10-11:15", "2026-10-12:09", "2026-10-12:10"}
	for i, w := range want {
		got := bookings[i].Date + ":" + fmtHour(bookings[i].StartHour)
		if got != w {
			t.Errorf("booking %d = %s, want %s", i, got, w)
		}
	}
}

func fmtHour(hour int) string {
	return time.Date(0, 1, 1, hour, 0, 0, 0, time.UTC).Format("15")
}
