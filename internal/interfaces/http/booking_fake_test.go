package http

import (
	"context"
	"strings"
	"time"

	"coworking/internal/application"
	"coworking/internal/domain"
)

// fakeRepo — тестовое хранилище приложения бронирования в памяти.
type fakeRepo struct {
	spaces        []domain.Space
	spaceTypes    []domain.SpaceType
	clients       []domain.Client
	bookings      []domain.Booking
	nextBookingID int
	nextClientID  int
}

func newFakeRepo() *fakeRepo {
	desk := domain.SpaceType{ID: 1, Code: domain.SpaceTypeDesk, Title: "Деск-место"}
	room := domain.SpaceType{ID: 2, Code: domain.SpaceTypeRoom, Title: "Переговорка"}
	cabin := domain.SpaceType{ID: 3, Code: domain.SpaceTypeCabin, Title: "Кабинет"}

	repo := &fakeRepo{
		spaceTypes: []domain.SpaceType{desk, room, cabin},
		spaces: []domain.Space{
			{ID: 1, Name: "Деск A1", Type: desk, Capacity: 1, PricePerHour: 200, IsActive: true},
			{ID: 2, Name: "Деск A2", Type: desk, Capacity: 1, PricePerHour: 200, IsActive: true},
			{ID: 3, Name: "Переговорка «Амфитеатр»", Type: room, Capacity: 12, PricePerHour: 1800, Deposit: 5000, IsActive: true},
			{ID: 4, Name: "Переговорка «Проект»", Type: room, Capacity: 6, PricePerHour: 1000, Deposit: 3000, IsActive: true},
			{ID: 5, Name: "Кабинет 2–3 №1", Type: cabin, Capacity: 3, PricePerHour: 350, IsActive: true},
		},
		clients: []domain.Client{
			{ID: 1, FullName: "Мария Соколова", Email: "maria@example.com"},
			{ID: 2, FullName: "Студия «Волна»", Company: "Студия «Волна»"},
			{ID: 3, FullName: "ООО «Прогресс»", Company: "ООО «Прогресс»"},
		},
		nextBookingID: 3,
		nextClientID:  4,
	}

	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	repo.bookings = []domain.Booking{
		{ID: 1, SpaceID: 2, ClientID: 1, Date: tomorrow, Rate: domain.BookingRateHour, StartHour: 10, Duration: 2, Price: 400, Status: domain.BookingStatusConfirmed, CreatedAt: time.Now()},
		{ID: 2, SpaceID: 4, ClientID: 2, Date: time.Now().AddDate(0, 0, 2).Format("2006-01-02"), Rate: domain.BookingRateHour, StartHour: 12, Duration: 2, Price: 2000, Deposit: 3000, Status: domain.BookingStatusConfirmed, CreatedAt: time.Now()},
	}
	return repo
}

func (r *fakeRepo) ListSpaces(context.Context) ([]domain.Space, error) {
	return append([]domain.Space(nil), r.spaces...), nil
}

func (r *fakeRepo) ListSpaceTypes(context.Context) ([]domain.SpaceType, error) {
	return append([]domain.SpaceType(nil), r.spaceTypes...), nil
}

func (r *fakeRepo) spaceByID(id int) (domain.Space, bool) {
	for _, space := range r.spaces {
		if space.ID == id {
			return space, true
		}
	}
	return domain.Space{}, false
}

func (r *fakeRepo) clientByID(id int) (domain.Client, bool) {
	for _, client := range r.clients {
		if client.ID == id {
			return client, true
		}
	}
	return domain.Client{}, false
}

func (r *fakeRepo) details(booking domain.Booking) application.BookingDetails {
	space, _ := r.spaceByID(booking.SpaceID)
	client, _ := r.clientByID(booking.ClientID)
	return application.BookingDetails{
		Booking:        booking,
		ClientName:     client.FullName,
		ClientCompany:  client.Company,
		SpaceName:      space.Name,
		SpaceTypeCode:  space.Type.Code,
		SpaceTypeTitle: space.Type.Title,
	}
}

func (r *fakeRepo) SearchBookings(_ context.Context, filter application.BookingFilter) ([]application.BookingDetails, error) {
	out := make([]application.BookingDetails, 0, len(r.bookings))
	for _, booking := range r.bookings {
		detail := r.details(booking)
		space, _ := r.spaceByID(booking.SpaceID)

		if filter.SpaceID != 0 && booking.SpaceID != filter.SpaceID {
			continue
		}
		if filter.Status != "" && booking.Status != filter.Status {
			continue
		}
		if filter.SpaceType != "" && space.Type.Code != filter.SpaceType {
			continue
		}
		if filter.DateFrom != "" && booking.Date < filter.DateFrom {
			continue
		}
		if filter.DateTo != "" && booking.Date > filter.DateTo {
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

func (r *fakeRepo) BookingByID(_ context.Context, id int) (application.BookingDetails, error) {
	for _, booking := range r.bookings {
		if booking.ID == id {
			return r.details(booking), nil
		}
	}
	return application.BookingDetails{}, application.ErrBookingNotFound
}

func (r *fakeRepo) BookingStatistics(ctx context.Context, filter application.BookingFilter) (application.BookingStatistics, error) {
	details, err := r.SearchBookings(ctx, filter)
	if err != nil {
		return application.BookingStatistics{}, err
	}
	stats := application.BookingStatistics{TotalBookings: len(details)}
	byType := map[domain.SpaceTypeCode]*application.BookingTypeStatistics{}
	for _, detail := range details {
		entry := byType[detail.SpaceTypeCode]
		if entry == nil {
			entry = &application.BookingTypeStatistics{SpaceType: detail.SpaceTypeCode, Title: detail.SpaceTypeTitle}
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

func (r *fakeRepo) DatabaseSummary(context.Context) (application.DatabaseSummary, error) {
	summary := application.DatabaseSummary{
		Offices:    1,
		SpaceTypes: len(r.spaceTypes),
		Spaces:     len(r.spaces),
		Clients:    len(r.clients),
		Bookings:   len(r.bookings),
	}
	topByType := map[string]int{}
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

func (r *fakeRepo) SaveBooking(_ context.Context, booking domain.Booking) (domain.Booking, error) {
	booking.ID = r.nextBookingID
	r.nextBookingID++
	booking.CreatedAt = time.Now()
	r.bookings = append(r.bookings, booking)
	return booking, nil
}

func (r *fakeRepo) UpdateBooking(_ context.Context, booking domain.Booking) (domain.Booking, error) {
	for i := range r.bookings {
		if r.bookings[i].ID == booking.ID {
			booking.CreatedAt = r.bookings[i].CreatedAt
			r.bookings[i] = booking
			return booking, nil
		}
	}
	return domain.Booking{}, application.ErrBookingNotFound
}

func (r *fakeRepo) CancelBooking(_ context.Context, id int) error {
	for i := range r.bookings {
		if r.bookings[i].ID == id && r.bookings[i].Status == domain.BookingStatusConfirmed {
			r.bookings[i].Status = domain.BookingStatusCancelled
			return nil
		}
	}
	return application.ErrBookingNotFound
}

func (r *fakeRepo) ListClients(context.Context) ([]domain.Client, error) {
	return append([]domain.Client(nil), r.clients...), nil
}

func (r *fakeRepo) ClientByID(_ context.Context, id int) (domain.Client, error) {
	if client, ok := r.clientByID(id); ok {
		return client, nil
	}
	return domain.Client{}, application.ErrClientNotFound
}

func (r *fakeRepo) SaveClient(_ context.Context, client domain.Client) (domain.Client, error) {
	client.ID = r.nextClientID
	r.nextClientID++
	client.CreatedAt = time.Now()
	r.clients = append(r.clients, client)
	return client, nil
}

func (r *fakeRepo) UpdateClient(_ context.Context, client domain.Client) (domain.Client, error) {
	for i := range r.clients {
		if r.clients[i].ID == client.ID {
			client.CreatedAt = r.clients[i].CreatedAt
			r.clients[i] = client
			return client, nil
		}
	}
	return domain.Client{}, application.ErrClientNotFound
}

func (r *fakeRepo) DeleteClient(_ context.Context, id int) error {
	found := false
	for i := range r.clients {
		if r.clients[i].ID == id {
			found = true
			r.clients = append(r.clients[:i], r.clients[i+1:]...)
			break
		}
	}
	if !found {
		return application.ErrClientNotFound
	}

	kept := r.bookings[:0]
	for _, booking := range r.bookings {
		if booking.ClientID == id && booking.Status == domain.BookingStatusCancelled {
			continue
		}
		kept = append(kept, booking)
	}
	r.bookings = kept
	return nil
}
