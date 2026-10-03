// Package memory — адаптер хранилища в памяти.
// Позже будет заменён на PostgreSQL-репозиторий с тем же интерфейсом.
package memory

import (
	"context"

	"coworking/internal/application"
	"coworking/internal/domain"
)

// Проверка на этапе компиляции: Repository удовлетворяет порту application.
var _ application.ServiceRepository = (*ServiceRepository)(nil)

// ServiceRepository хранит каталог сервисов в памяти.
type ServiceRepository struct {
	services []domain.Service
}

// NewServiceRepository создаёт хранилище с демонстрационными данными.
func NewServiceRepository() *ServiceRepository {
	return &ServiceRepository{services: seedServices()}
}

// ListServices возвращает копию каталога сервисов.
func (r *ServiceRepository) ListServices(_ context.Context) ([]domain.Service, error) {
	return append([]domain.Service(nil), r.services...), nil
}

// seedServices — стартовый набор сервисов сайта коворкинга.
func seedServices() []domain.Service {
	return []domain.Service{
		{
			Slug:        "booking",
			Name:        "Онлайн-бронирование",
			Description: "Выбор стола или переговорки, расчёт стоимости, проверка свободных часов и оплата брони — приложение к базе данных коворкинга.",
			Group:       "Бронирование",
			Status:      domain.ServiceStatusReady,
			URL:         "/booking",
		},
		{
			Slug:        "free-now",
			Name:        "Свободно сейчас",
			Description: "Живая загрузка офиса: сколько мест свободно по зонам.",
			Group:       "Пространство",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "nearest-room",
			Name:        "Ближайшая переговорка",
			Description: "Подбор переговорки под число участников.",
			Group:       "Пространство",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "rent-calc",
			Name:        "Калькулятор аренды",
			Description: "Стоимость брони: часы, дни и доп. опции.",
			Group:       "Тарифы",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "tariff-compare",
			Name:        "Сравнение тарифов",
			Description: "День, неделя или месяц: что выгоднее.",
			Group:       "Тарифы",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "format-test",
			Name:        "Тест «Формат работы»",
			Description: "Два вопроса — и понятно, какой формат подходит.",
			Group:       "Планирование",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "office-load",
			Name:        "Загрузка офиса по часам",
			Description: "Пиковые часы и тихие окна офиса в течение дня.",
			Group:       "Пространство",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "timer",
			Name:        "Таймер до конца брони",
			Description: "Обратный отсчёт до завершения аренды.",
			Group:       "Бронирование",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "desk-of-the-day",
			Name:        "Стол дня",
			Description: "Случайное место с описанием и деталями.",
			Group:       "Планирование",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "weather",
			Name:        "Погода",
			Description: "Прогноз у офиса: идти пешком или остаться дома.",
			Group:       "Инфраструктура",
			Status:      domain.ServiceStatusReady,
		},
	}
}
