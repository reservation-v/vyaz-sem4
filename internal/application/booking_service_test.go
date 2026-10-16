package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"coworking/internal/domain"
)

// stubRepo — тестовое хранилище пространств, клиентов и броней в памяти.
type stubRepo struct {
	spaces        []domain.Space
	spaceTypes    []domain.SpaceType
	clients       []domain.Client
	bookings      []domain.Booking
	nextBookingID int
	nextClientID  int
}

func newStubRepo() *stubRepo {
	desk := domain.SpaceType{ID: 1, Code: domain.SpaceTypeDesk, Title: "Деск-место"}
	room := domain.SpaceType{ID: 2, Code: domain.SpaceTypeRoom, Title: "Переговорка"}
	cabin := domain.SpaceType{ID: 3, Code: domain.SpaceTypeCabin, Title: "Кабинет"}

	return &stubRepo{
		spaceTypes: []domain.SpaceType{desk, room, cabin},
		spaces: []domain.Space{
			{ID: 1, Name: "Деск A1", Type: desk, Capacity: 1, PricePerHour: 200, IsActive: true},
			{ID: 2, Name: "Переговорка «Встреча»", Type: room, Capacity: 4, PricePerHour: 700, Deposit: 2000, IsActive: true},
			{ID: 3, Name: "Кабинет 4–6 №1", Type: cabin, Capacity: 6, PricePerHour: 450, IsActive: true},
		},
		clients: []domain.Client{
			{ID: 1, FullName: "Иван Петров"},
			{ID: 2, FullName: "Студия «Волна»", Company: "Студия «Волна»"},
			{ID: 3, FullName: "Мария Соколова"},
			{ID: 4, FullName: "Пётр Сидоров"},
		},
		nextBookingID: 1,
		nextClientID:  5,
	}
}

func (r *stubRepo) ListSpaces(context.Context) ([]domain.Space, error) {
	return append([]domain.Space(nil), r.spaces...), nil
}

func (r *stubRepo) ListSpaceTypes(context.Context) ([]domain.SpaceType, error) {
	return append([]domain.SpaceType(nil), r.spaceTypes...), nil
}

func (r *stubRepo) spaceByID(id int) (domain.Space, bool) {
	for _, space := range r.spaces {
		if space.ID == id {
			return space, true
		}
	}
	return domain.Space{}, false
}

func (r *stubRepo) clientByID(id int) (domain.Client, bool) {
	for _, client := range r.clients {
		if client.ID == id {
			return client, true
		}
	}
	return domain.Client{}, false
}

func (r *stubRepo) details(booking domain.Booking) BookingDetails {
	space, _ := r.spaceByID(booking.SpaceID)
	client, _ := r.clientByID(booking.ClientID)
	return BookingDetails{
		Booking:        booking,
		ClientName:     client.FullName,
		ClientCompany:  client.Company,
		SpaceName:      space.Name,
		SpaceTypeCode:  space.Type.Code,
		SpaceTypeTitle: space.Type.Title,
	}
}

func (r *stubRepo) SearchBookings(_ context.Context, filter BookingFilter) ([]BookingDetails, error) {
	out := make([]BookingDetails, 0, len(r.bookings))
	for _, booking := range r.bookings {
		detail := r.details(booking)
		space, _ := r.spaceByID(booking.SpaceID)

		if filter.SpaceID != 0 && booking.SpaceID != filter.SpaceID {
			continue
		}
		if filter.SpaceType != "" && space.Type.Code != filter.SpaceType {
			continue
		}
		if filter.ClientID != 0 && booking.ClientID != filter.ClientID {
			continue
		}
		if filter.Search != "" {
			haystack := strings.ToLower(detail.ClientName + " " + detail.ClientCompany + " " + detail.SpaceName)
			if !strings.Contains(haystack, strings.ToLower(filter.Search)) {
				continue
			}
		}
		out = append(out, detail)
	}
	return out, nil
}

func (r *stubRepo) BookingByID(_ context.Context, id int) (BookingDetails, error) {
	for _, booking := range r.bookings {
		if booking.ID == id {
			return r.details(booking), nil
		}
	}
	return BookingDetails{}, ErrBookingNotFound
}

func (r *stubRepo) BookingStatistics(ctx context.Context, filter BookingFilter) (BookingStatistics, error) {
	details, err := r.SearchBookings(ctx, filter)
	if err != nil {
		return BookingStatistics{}, err
	}

	stats := BookingStatistics{TotalBookings: len(details)}
	byType := make(map[domain.SpaceTypeCode]*BookingTypeStatistics)
	for _, detail := range details {
		entry := byType[detail.SpaceTypeCode]
		if entry == nil {
			entry = &BookingTypeStatistics{SpaceType: detail.SpaceTypeCode, Title: detail.SpaceTypeTitle}
			byType[detail.SpaceTypeCode] = entry
		}
		entry.Count++
		if detail.Booking.Status == domain.BookingStatusConfirmed {
			stats.ConfirmedBookings++
			stats.Revenue += detail.Booking.Price
			entry.Revenue += detail.Booking.Price
		} else {
			stats.CancelledBookings++
		}
	}
	for _, entry := range byType {
		stats.BySpaceType = append(stats.BySpaceType, *entry)
	}
	return stats, nil
}

func (r *stubRepo) DatabaseSummary(context.Context) (DatabaseSummary, error) {
	summary := DatabaseSummary{
		SpaceTypes: len(r.spaceTypes),
		Spaces:     len(r.spaces),
		Clients:    len(r.clients),
		Bookings:   len(r.bookings),
	}
	topByType := make(map[string]int)
	for _, booking := range r.bookings {
		if booking.Status != domain.BookingStatusConfirmed {
			summary.Cancelled++
			continue
		}
		summary.Confirmed++
		summary.Revenue += booking.Price
		space, _ := r.spaceByID(booking.SpaceID)
		topByType[space.Type.Title]++
	}
	for title, count := range topByType {
		if count > summary.TopSpaceTypeCount {
			summary.TopSpaceTypeTitle = title
			summary.TopSpaceTypeCount = count
		}
	}
	return summary, nil
}

func (r *stubRepo) SaveBooking(_ context.Context, booking domain.Booking) (domain.Booking, error) {
	booking.ID = r.nextBookingID
	r.nextBookingID++
	booking.CreatedAt = time.Now()
	r.bookings = append(r.bookings, booking)
	return booking, nil
}

func (r *stubRepo) UpdateBooking(_ context.Context, booking domain.Booking) (domain.Booking, error) {
	for i := range r.bookings {
		if r.bookings[i].ID == booking.ID {
			booking.CreatedAt = r.bookings[i].CreatedAt
			r.bookings[i] = booking
			return booking, nil
		}
	}
	return domain.Booking{}, ErrBookingNotFound
}

func (r *stubRepo) DeleteBooking(_ context.Context, id int) error {
	for i := range r.bookings {
		if r.bookings[i].ID == id {
			r.bookings = append(r.bookings[:i], r.bookings[i+1:]...)
			return nil
		}
	}
	return ErrBookingNotFound
}

func (r *stubRepo) ListClients(context.Context) ([]domain.Client, error) {
	return append([]domain.Client(nil), r.clients...), nil
}

func (r *stubRepo) ClientByID(_ context.Context, id int) (domain.Client, error) {
	if client, ok := r.clientByID(id); ok {
		return client, nil
	}
	return domain.Client{}, ErrClientNotFound
}

func (r *stubRepo) SaveClient(_ context.Context, client domain.Client) (domain.Client, error) {
	client.ID = r.nextClientID
	r.nextClientID++
	client.CreatedAt = time.Now()
	r.clients = append(r.clients, client)
	return client, nil
}

func (r *stubRepo) UpdateClient(_ context.Context, client domain.Client) (domain.Client, error) {
	for i := range r.clients {
		if r.clients[i].ID == client.ID {
			client.CreatedAt = r.clients[i].CreatedAt
			r.clients[i] = client
			return client, nil
		}
	}
	return domain.Client{}, ErrClientNotFound
}

func (r *stubRepo) DeleteClient(_ context.Context, id int) error {
	found := false
	for i := range r.clients {
		if r.clients[i].ID == id {
			found = true
			r.clients = append(r.clients[:i], r.clients[i+1:]...)
			break
		}
	}
	if !found {
		return ErrClientNotFound
	}

	// Брони клиента уходят вместе с ним (ON DELETE CASCADE в схеме БД).
	kept := r.bookings[:0]
	for _, booking := range r.bookings {
		if booking.ClientID == id {
			continue
		}
		kept = append(kept, booking)
	}
	r.bookings = kept
	return nil
}

// fixedNow — фиксированные «сейчас» для детерминированных тестов.
var fixedNow = time.Date(2026, time.October, 10, 9, 0, 0, 0, time.UTC)

func newTestBookingService() *BookingService {
	repo := newStubRepo()
	svc := NewBookingService(repo, repo)
	svc.now = func() time.Time { return fixedNow }
	return svc
}

func TestSpacesDesksFirstCabinsLast(t *testing.T) {
	repo := newStubRepo()
	// Ставим каталог в обратном порядке: кабинет, переговорка, деск.
	repo.spaces = []domain.Space{repo.spaces[2], repo.spaces[1], repo.spaces[0]}
	svc := NewBookingService(repo, repo)

	spaces, err := svc.Spaces(context.Background())
	if err != nil {
		t.Fatalf("Spaces() error = %v", err)
	}

	want := []domain.SpaceTypeCode{domain.SpaceTypeDesk, domain.SpaceTypeRoom, domain.SpaceTypeCabin}
	if len(spaces) != len(want) {
		t.Fatalf("len(spaces) = %d, want %d", len(spaces), len(want))
	}
	for i, wantCode := range want {
		if spaces[i].Type.Code != wantCode {
			t.Errorf("spaces[%d].Type.Code = %q, want %q", i, spaces[i].Type.Code, wantCode)
		}
	}
}

// createHour — почасовая бронь для указанного клиента: короткая сигнатура для тестов.
func createHour(t *testing.T, svc *BookingService, spaceID, clientID int, date string, startHour, duration int) (domain.Booking, error) {
	t.Helper()
	return svc.CreateBooking(context.Background(), BookingInput{
		SpaceID: spaceID, ClientID: clientID, Date: date,
		Rate: domain.BookingRateHour, StartHour: startHour, Duration: duration,
	})
}

func TestCreateBookingWithDesk(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 3)
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
	if booking.ClientID != 1 {
		t.Errorf("ClientID = %d, want 1", booking.ClientID)
	}
	if booking.ID == 0 {
		t.Error("ID = 0, want assigned id")
	}
}

func TestCreateBookingWithRoomDeposit(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 2, 2, "2026-10-12", 14, 2)
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

			booking, err := svc.CreateBooking(context.Background(), BookingInput{
				SpaceID: 1, ClientID: 1, Date: "2026-10-12", Rate: tt.rate,
			})
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
		_, err := svc.CreateBooking(context.Background(), BookingInput{
			SpaceID: spaceID, ClientID: 1, Date: "2026-10-12", Rate: domain.BookingRateDay,
		})
		if !errors.Is(err, ErrRateNotAvailable) {
			t.Errorf("space %d: error = %v, want ErrRateNotAvailable", spaceID, err)
		}
	}
}

func TestCreateBookingOverlap(t *testing.T) {
	svc := newTestBookingService()

	if _, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 2); err != nil {
		t.Fatalf("first booking: %v", err)
	}

	// Полное пересечение.
	if _, err := createHour(t, svc, 1, 4, "2026-10-12", 11, 1); !errors.Is(err, ErrOverlap) {
		t.Errorf("overlap error = %v, want ErrOverlap", err)
	}

	// Бронь, начинающаяся в момент окончания предыдущей (10 + 2 = 12), — свободна.
	if _, err := createHour(t, svc, 1, 4, "2026-10-12", 12, 1); err != nil {
		t.Errorf("adjacent booking error = %v, want nil", err)
	}
}

func TestSameTimeDifferentSpace(t *testing.T) {
	svc := newTestBookingService()

	if _, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 2); err != nil {
		t.Fatalf("first booking: %v", err)
	}

	if _, err := createHour(t, svc, 3, 4, "2026-10-12", 10, 2); err != nil {
		t.Errorf("different space error = %v, want nil", err)
	}
}

func TestDeletedBookingIsRemovedAndFreesSpace(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 2)
	if err != nil {
		t.Fatalf("first booking: %v", err)
	}

	if err := svc.DeleteBooking(context.Background(), booking.ID); err != nil {
		t.Fatalf("DeleteBooking() error = %v", err)
	}

	// Отмена выполняется удалением: записи в базе больше нет.
	if _, err := svc.BookingByID(context.Background(), booking.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("BookingByID() error = %v, want ErrBookingNotFound", err)
	}
	if _, err := createHour(t, svc, 1, 4, "2026-10-12", 10, 2); err != nil {
		t.Errorf("booking after delete error = %v, want nil", err)
	}
}

func TestPeriodBookingsOverlap(t *testing.T) {
	t.Run("day blocks hourly same date", func(t *testing.T) {
		svc := newTestBookingService()
		if _, err := svc.CreateBooking(context.Background(), BookingInput{SpaceID: 1, ClientID: 1, Date: "2026-10-12", Rate: domain.BookingRateDay}); err != nil {
			t.Fatalf("day booking: %v", err)
		}
		if _, err := createHour(t, svc, 1, 4, "2026-10-12", 10, 1); !errors.Is(err, ErrOverlap) {
			t.Errorf("hour on same date error = %v, want ErrOverlap", err)
		}
		if _, err := createHour(t, svc, 1, 4, "2026-10-13", 10, 1); err != nil {
			t.Errorf("hour next day error = %v, want nil", err)
		}
	})

	t.Run("week blocks any day inside", func(t *testing.T) {
		svc := newTestBookingService()
		if _, err := svc.CreateBooking(context.Background(), BookingInput{SpaceID: 1, ClientID: 1, Date: "2026-10-12", Rate: domain.BookingRateWeek}); err != nil {
			t.Fatalf("week booking: %v", err)
		}
		if _, err := createHour(t, svc, 1, 4, "2026-10-16", 10, 1); !errors.Is(err, ErrOverlap) {
			t.Errorf("hour inside week error = %v, want ErrOverlap", err)
		}
		if _, err := createHour(t, svc, 1, 4, "2026-10-19", 10, 1); err != nil {
			t.Errorf("hour after week error = %v, want nil", err)
		}
	})

	t.Run("two week bookings overlap", func(t *testing.T) {
		svc := newTestBookingService()
		if _, err := svc.CreateBooking(context.Background(), BookingInput{SpaceID: 1, ClientID: 1, Date: "2026-10-12", Rate: domain.BookingRateWeek}); err != nil {
			t.Fatalf("first week booking: %v", err)
		}
		if _, err := svc.CreateBooking(context.Background(), BookingInput{SpaceID: 1, ClientID: 4, Date: "2026-10-14", Rate: domain.BookingRateWeek}); !errors.Is(err, ErrOverlap) {
			t.Errorf("overlapping week error = %v, want ErrOverlap", err)
		}
		if _, err := svc.CreateBooking(context.Background(), BookingInput{SpaceID: 1, ClientID: 4, Date: "2026-10-19", Rate: domain.BookingRateWeek}); err != nil {
			t.Errorf("adjacent week error = %v, want nil", err)
		}
	})
}

func TestCreateBookingValidation(t *testing.T) {
	tests := []struct {
		name      string
		spaceID   int
		clientID  int
		date      string
		startHour int
		duration  int
		wantErr   error
	}{
		{name: "bad date format", spaceID: 1, clientID: 1, date: "12.10.2026", startHour: 10, duration: 1, wantErr: ErrInvalidDate},
		{name: "past date", spaceID: 1, clientID: 1, date: "2026-10-09", startHour: 10, duration: 1, wantErr: ErrInvalidDate},
		{name: "today ok", spaceID: 1, clientID: 1, date: "2026-10-10", startHour: 10, duration: 1, wantErr: nil},
		{name: "too early", spaceID: 1, clientID: 1, date: "2026-10-12", startHour: 7, duration: 1, wantErr: ErrInvalidHours},
		{name: "too late start", spaceID: 1, clientID: 1, date: "2026-10-12", startHour: 22, duration: 1, wantErr: ErrInvalidHours},
		{name: "ends after close", spaceID: 1, clientID: 1, date: "2026-10-12", startHour: 21, duration: 2, wantErr: ErrInvalidHours},
		{name: "closing hour ok", spaceID: 1, clientID: 1, date: "2026-10-12", startHour: 21, duration: 1, wantErr: nil},
		{name: "duration zero", spaceID: 1, clientID: 1, date: "2026-10-12", startHour: 10, duration: 0, wantErr: ErrInvalidHours},
		{name: "duration too long", spaceID: 1, clientID: 1, date: "2026-10-12", startHour: 10, duration: 9, wantErr: ErrInvalidHours},
		{name: "unknown space", spaceID: 999, clientID: 1, date: "2026-10-12", startHour: 10, duration: 1, wantErr: ErrSpaceNotFound},
		{name: "unknown client", spaceID: 1, clientID: 999, date: "2026-10-12", startHour: 10, duration: 1, wantErr: ErrClientNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestBookingService()

			_, err := svc.CreateBooking(context.Background(), BookingInput{
				SpaceID: tt.spaceID, ClientID: tt.clientID, Date: tt.date,
				Rate: domain.BookingRateHour, StartHour: tt.startHour, Duration: tt.duration,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateBooking() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnknownRateRejected(t *testing.T) {
	svc := newTestBookingService()
	_, err := svc.CreateBooking(context.Background(), BookingInput{
		SpaceID: 1, ClientID: 1, Date: "2026-10-12", Rate: domain.BookingRate("year"),
	})
	if !errors.Is(err, ErrInvalidHours) {
		t.Errorf("unknown rate error = %v, want ErrInvalidHours", err)
	}
}

func TestUpdateBooking(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 1)
	if err != nil {
		t.Fatalf("CreateBooking() error = %v", err)
	}

	updated, err := svc.UpdateBooking(context.Background(), booking.ID, BookingInput{
		SpaceID: 1, ClientID: 3, Date: "2026-10-14", Rate: domain.BookingRateHour, StartHour: 15, Duration: 3,
	})
	if err != nil {
		t.Fatalf("UpdateBooking() error = %v", err)
	}
	if updated.ClientID != 3 || updated.Date != "2026-10-14" || updated.StartHour != 15 || updated.Duration != 3 {
		t.Errorf("updated = %+v, want client 3 on 2026-10-14 at 15:00 for 3h", updated)
	}
	if updated.Price != 600 { // 200 ₽/ч × 3
		t.Errorf("Price = %d, want 600", updated.Price)
	}

	// Редактирование на занятое время отклоняется.
	if _, err := createHour(t, svc, 1, 4, "2026-10-15", 10, 2); err != nil {
		t.Fatalf("second booking: %v", err)
	}
	if _, err := svc.UpdateBooking(context.Background(), booking.ID, BookingInput{
		SpaceID: 1, ClientID: 3, Date: "2026-10-15", Rate: domain.BookingRateHour, StartHour: 11, Duration: 1,
	}); !errors.Is(err, ErrOverlap) {
		t.Errorf("overlapping update error = %v, want ErrOverlap", err)
	}

	// Несуществующая бронь.
	if _, err := svc.UpdateBooking(context.Background(), 404, BookingInput{SpaceID: 1, ClientID: 1, Date: "2026-10-14", Rate: domain.BookingRateHour, StartHour: 10, Duration: 1}); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("unknown booking error = %v, want ErrBookingNotFound", err)
	}
}

func TestUpdateBookingKeepsItsOwnSlot(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 2)
	if err != nil {
		t.Fatalf("CreateBooking() error = %v", err)
	}

	// Изменение только клиента на том же слоте не должно конфликтовать с самой бронью.
	if _, err := svc.UpdateBooking(context.Background(), booking.ID, BookingInput{
		SpaceID: 1, ClientID: 4, Date: "2026-10-12", Rate: domain.BookingRateHour, StartHour: 10, Duration: 2,
	}); err != nil {
		t.Errorf("same-slot update error = %v, want nil", err)
	}
}

func TestBookingFilterValidation(t *testing.T) {
	tests := []struct {
		name   string
		filter BookingFilter
		want   error
	}{
		{name: "valid filter", filter: BookingFilter{SpaceType: domain.SpaceTypeRoom}},
		{name: "invalid space type", filter: BookingFilter{SpaceType: "unknown"}, want: ErrInvalidFilter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBookingFilter(tt.filter)
			if !errors.Is(err, tt.want) {
				t.Errorf("validateBookingFilter() error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestSearchByOneAndTwoValues проверяет поиск по одному значению и по двум
// синхронизированным значениям из двух таблиц (clients и space_types).
func TestSearchByOneAndTwoValues(t *testing.T) {
	svc := newTestBookingService()

	if _, err := createHour(t, svc, 1, 3, "2026-10-12", 10, 2); err != nil { // Мария, деск
		t.Fatal(err)
	}
	if _, err := createHour(t, svc, 2, 2, "2026-10-12", 12, 1); err != nil { // Студия «Волна», переговорка
		t.Fatal(err)
	}

	// По одному значению: совпадение по имени клиента.
	one, err := svc.BookingsFiltered(context.Background(), BookingFilter{Search: "мария"})
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0].ClientName != "Мария Соколова" {
		t.Fatalf("одно значение: got %+v, want only Мария Соколова", one)
	}

	// По двум синхронизированным значениям: клиент + его тип пространства.
	two, err := svc.BookingsFiltered(context.Background(), BookingFilter{
		ClientID: 2, SpaceType: domain.SpaceTypeRoom,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(two) != 1 || two[0].ClientName != "Студия «Волна»" || two[0].SpaceTypeCode != domain.SpaceTypeRoom {
		t.Fatalf("два значения: got %+v, want only переговорка Студии «Волна»", two)
	}

	// Противоречивые два значения не находят ничего: у Марии нет переговорок.
	none, err := svc.BookingsFiltered(context.Background(), BookingFilter{
		ClientID: 3, SpaceType: domain.SpaceTypeRoom,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("противоречивые значения: got %d bookings, want 0", len(none))
	}
}

// TestClientSpaceTypes проверяет зависимый (синхронизированный) список:
// типы пространств, которые бронировал клиент (JOIN clients → bookings →
// spaces → space_types).
func TestClientSpaceTypes(t *testing.T) {
	svc := newTestBookingService()

	if _, err := createHour(t, svc, 1, 3, "2026-10-12", 10, 1); err != nil { // Мария, деск
		t.Fatal(err)
	}
	if _, err := createHour(t, svc, 3, 3, "2026-10-13", 10, 1); err != nil { // Мария, кабинет
		t.Fatal(err)
	}

	types, err := svc.ClientSpaceTypes(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 2 || types[0].Code != domain.SpaceTypeDesk || types[1].Code != domain.SpaceTypeCabin {
		t.Fatalf("ClientSpaceTypes(3) = %+v, want desk, cabin", types)
	}

	// Клиент без броней — пустой список.
	empty, err := svc.ClientSpaceTypes(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("ClientSpaceTypes(4) = %+v, want empty", empty)
	}

	// Нулевой клиент — пустой список без обращения к хранилищу.
	if zero, err := svc.ClientSpaceTypes(context.Background(), 0); err != nil || zero != nil {
		t.Errorf("ClientSpaceTypes(0) = %+v, %v; want nil, nil", zero, err)
	}

	// Неизвестный клиент — ошибка ErrClientNotFound.
	if _, err := svc.ClientSpaceTypes(context.Background(), 99); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("ClientSpaceTypes(99) error = %v, want ErrClientNotFound", err)
	}
}

func TestBookingStatisticsForFilteredData(t *testing.T) {
	repo := newStubRepo()
	svc := NewBookingService(repo, repo)
	svc.now = func() time.Time { return fixedNow }

	if _, err := createHour(t, svc, 1, 3, "2026-10-12", 10, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := createHour(t, svc, 2, 1, "2026-10-12", 10, 1); err != nil {
		t.Fatal(err)
	}
	// Статус «отменена» остаётся в схеме отдельным значением, поэтому вводим
	// такую бронь напрямую: действие отмены теперь удаляет запись из базы.
	repo.bookings = append(repo.bookings, domain.Booking{
		ID: 3, SpaceID: 1, ClientID: 1, Date: "2026-10-13",
		Rate: domain.BookingRateHour, StartHour: 10, Duration: 1, Price: 200,
		Status: domain.BookingStatusCancelled,
	})

	filter := BookingFilter{Search: "мария", SpaceType: domain.SpaceTypeDesk}
	bookings, err := svc.BookingsFiltered(context.Background(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(bookings) != 1 || bookings[0].ClientName != "Мария Соколова" {
		t.Fatalf("filtered bookings = %+v, want only Мария Соколова", bookings)
	}

	stats, err := svc.Statistics(context.Background(), BookingFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalBookings != 3 || stats.ConfirmedBookings != 2 || stats.CancelledBookings != 1 || stats.Revenue != 1100 {
		t.Errorf("statistics = %+v, want total=3 confirmed=2 cancelled=1 revenue=1100", stats)
	}
}

func TestDatabaseSummary(t *testing.T) {
	repo := newStubRepo()
	svc := NewBookingService(repo, repo)
	svc.now = func() time.Time { return fixedNow }

	if _, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := createHour(t, svc, 1, 4, "2026-10-13", 10, 1); err != nil {
		t.Fatal(err)
	}
	// Легаси-бронь со статусом «отменена»: действие отмены теперь удаляет запись,
	// но схема по-прежнему различает статусы.
	repo.bookings = append(repo.bookings, domain.Booking{
		ID: 3, SpaceID: 1, ClientID: 1, Date: "2026-10-14",
		Rate: domain.BookingRateHour, StartHour: 10, Duration: 1, Price: 200,
		Status: domain.BookingStatusCancelled,
	})

	summary, err := svc.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.Spaces != 3 || summary.Clients != 4 || summary.SpaceTypes != 3 {
		t.Errorf("summary counts = %+v, want spaces=3 clients=4 types=3", summary)
	}
	if summary.Bookings != 3 || summary.Confirmed != 2 || summary.Cancelled != 1 || summary.Revenue != 600 {
		t.Errorf("summary aggregates = %+v, want bookings=3 confirmed=2 cancelled=1 revenue=600", summary)
	}
	if summary.TopSpaceTypeCount == 0 {
		t.Error("TopSpaceTypeCount = 0, want at least one confirmed booking")
	}
}

func TestDeleteBooking(t *testing.T) {
	svc := newTestBookingService()

	booking, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 1)
	if err != nil {
		t.Fatalf("CreateBooking() error = %v", err)
	}

	if err := svc.DeleteBooking(context.Background(), booking.ID); err != nil {
		t.Fatalf("DeleteBooking() error = %v", err)
	}

	// Повторное удаление и несуществующая бронь - одна ошибка.
	if err := svc.DeleteBooking(context.Background(), booking.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("second delete error = %v, want ErrBookingNotFound", err)
	}
	if err := svc.DeleteBooking(context.Background(), 404); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("unknown booking error = %v, want ErrBookingNotFound", err)
	}
}

func TestClientValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   ClientInput
		wantErr error
	}{
		{name: "empty name", input: ClientInput{FullName: "   "}, wantErr: ErrInvalidClientName},
		{name: "long name", input: ClientInput{FullName: string(make([]rune, 81))}, wantErr: ErrInvalidClientName},
		{name: "bad email", input: ClientInput{FullName: "Иван", Email: "not-an-email"}, wantErr: ErrInvalidClientContact},
		{name: "ok without email", input: ClientInput{FullName: "Иван"}, wantErr: nil},
		{name: "ok with email", input: ClientInput{FullName: "Иван", Email: "ivan@example.com"}, wantErr: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestBookingService()
			_, err := svc.CreateClient(context.Background(), tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateClient() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientCreateAndUpdate(t *testing.T) {
	svc := newTestBookingService()

	client, err := svc.CreateClient(context.Background(), ClientInput{FullName: "Новый Клиент", Email: "new@example.com", Company: "ООО «Новый»"})
	if err != nil {
		t.Fatalf("CreateClient() error = %v", err)
	}
	if client.ID == 0 {
		t.Error("created client ID = 0, want assigned id")
	}

	updated, err := svc.UpdateClient(context.Background(), client.ID, ClientInput{FullName: "Изменённый Клиент", Phone: "+7 000 000-00-00"})
	if err != nil {
		t.Fatalf("UpdateClient() error = %v", err)
	}
	if updated.FullName != "Изменённый Клиент" || updated.Phone != "+7 000 000-00-00" {
		t.Errorf("updated client = %+v, want new name and phone", updated)
	}

	if _, err := svc.UpdateClient(context.Background(), 404, ClientInput{FullName: "Кто-то"}); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("unknown client error = %v, want ErrClientNotFound", err)
	}
}

func TestClientDelete(t *testing.T) {
	svc := newTestBookingService()

	// Клиент без броней удаляется.
	created, err := svc.CreateClient(context.Background(), ClientInput{FullName: "Временный Клиент"})
	if err != nil {
		t.Fatalf("CreateClient() error = %v", err)
	}
	if err := svc.DeleteClient(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteClient() error = %v", err)
	}

	clients, err := svc.Clients(context.Background())
	if err != nil {
		t.Fatalf("Clients() error = %v", err)
	}
	for _, client := range clients {
		if client.ID == created.ID {
			t.Error("deleted client is still listed")
		}
	}

	if err := svc.DeleteClient(context.Background(), 404); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("unknown client delete error = %v, want ErrClientNotFound", err)
	}

	// Подтверждённая бронь не мешает удалению: клиент уходит вместе с бронью.
	booking, err := createHour(t, svc, 1, 1, "2026-10-12", 10, 1)
	if err != nil {
		t.Fatalf("CreateBooking() error = %v", err)
	}
	if err := svc.DeleteClient(context.Background(), 1); err != nil {
		t.Fatalf("client with confirmed booking delete error = %v, want nil", err)
	}
	if _, err := svc.BookingByID(context.Background(), booking.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Errorf("booking of deleted client error = %v, want ErrBookingNotFound", err)
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
		if _, err := createHour(t, svc, 1, 1, params.date, params.hour, 1); err != nil {
			t.Fatalf("CreateBooking() error = %v", err)
		}
	}

	bookings, err := svc.Bookings(context.Background())
	if err != nil {
		t.Fatalf("Bookings() error = %v", err)
	}

	want := []string{"2026-10-11:15", "2026-10-12:09", "2026-10-12:10"}
	for i, w := range want {
		got := bookings[i].Booking.Date + ":" + fmtHour(bookings[i].Booking.StartHour)
		if got != w {
			t.Errorf("booking %d = %s, want %s", i, got, w)
		}
	}
}

func fmtHour(hour int) string {
	return time.Date(0, 1, 1, hour, 0, 0, 0, time.UTC).Format("15")
}
