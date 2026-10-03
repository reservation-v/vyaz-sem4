package domain

import "time"

// SpaceType — тип пространства коворкинга.
type SpaceType string

const (
	// SpaceTypeDesk — деск-место в открытой зоне.
	SpaceTypeDesk SpaceType = "desk"
	// SpaceTypeCabin — кабинет для команды.
	SpaceTypeCabin SpaceType = "cabin"
	// SpaceTypeRoom — переговорка.
	SpaceTypeRoom SpaceType = "room"
)

// Space — пространство, которое можно забронировать (стол, кабинет, переговорка).
type Space struct {
	ID           int
	Name         string
	Type         SpaceType
	Capacity     int // сколько человек помещается
	PricePerHour int // рубли за час аренды
	Deposit      int // депозит за переговорку, 0 для столов и кабинетов
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
	ID         int
	SpaceID    int
	ClientName string
	Date       string // дата брони, YYYY-MM-DD
	Rate       BookingRate
	StartHour  int // час начала для почасовой брони, с 8 до 21
	Duration   int // часы для почасовой; дни (1/7/1) для день/неделя/месяц
	Price      int // стоимость аренды
	Deposit    int // депозит (только переговорки)
	Status     BookingStatus
	CreatedAt  time.Time
}
