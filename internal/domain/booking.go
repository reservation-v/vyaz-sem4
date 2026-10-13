package domain

import "time"

// SpaceTypeCode — код типа пространства (классификатор space_types).
type SpaceTypeCode string

const (
	// SpaceTypeDesk — деск-место в открытой зоне.
	SpaceTypeDesk SpaceTypeCode = "desk"
	// SpaceTypeCabin — кабинет для команды.
	SpaceTypeCabin SpaceTypeCode = "cabin"
	// SpaceTypeRoom — переговорка.
	SpaceTypeRoom SpaceTypeCode = "room"
)

// SpaceType — классификатор типов пространств (таблица space_types).
type SpaceType struct {
	ID          int
	Code        SpaceTypeCode
	Title       string
	Description string
}

// Office — локация коворкинга (таблица offices).
type Office struct {
	ID      int
	Name    string
	City    string
	Address string
	Floor   int
	Phone   string
	Email   string
}

// Space — пространство, которое можно забронировать (стол, кабинет, переговорка).
type Space struct {
	ID           int
	OfficeID     int
	Name         string
	Type         SpaceType // тип-классификатор, подтягивается соединением с space_types
	Capacity     int       // сколько человек помещается
	PricePerHour int       // рубли за час аренды
	Deposit      int       // депозит за переговорку, 0 для столов и кабинетов
	IsActive     bool      // доступно ли пространство для новых броней
}

// Client — клиент коворкинга (таблица clients).
type Client struct {
	ID        int
	FullName  string
	Phone     string
	Email     string
	Company   string
	CreatedAt time.Time
}

// BookingRate — тариф брони: почасовая, подневная, понедельная, помесячная.
type BookingRate string

const (
	BookingRateHour  BookingRate = "hour"  // почасовая
	BookingRateDay   BookingRate = "day"   // на день (деск-места)
	BookingRateWeek  BookingRate = "week"  // на неделю (деск-места)
	BookingRateMonth BookingRate = "month" // на месяц (деск-места)
)

// BookingStatus — статус брони.
type BookingStatus string

const (
	// BookingStatusConfirmed — бронь подтверждена и действует.
	BookingStatusConfirmed BookingStatus = "confirmed"
	// BookingStatusCancelled — бронь отменена.
	BookingStatusCancelled BookingStatus = "cancelled"
)

// Booking — бронь пространства на отрезок времени.
type Booking struct {
	ID        int
	SpaceID   int
	ClientID  int
	Date      string // дата брони, YYYY-MM-DD
	Rate      BookingRate
	StartHour int // час начала для почасовой брони, с 8 до 21
	Duration  int // часы для почасовой; 1/7/1 для день/неделя/месяц
	Price     int // стоимость аренды
	Deposit   int // депозит (только переговорки)
	Status    BookingStatus
	CreatedAt time.Time
}
