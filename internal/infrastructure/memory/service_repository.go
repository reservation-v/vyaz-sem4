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
			Description: "Выбор офиса, стола или переговорки, даты и времени, расчёт стоимости и оплата брони — приложение к базе данных коворкинга.",
			Group:       "Бронирование",
			Status:      domain.ServiceStatusReady,
			URL:         "http://localhost:8080",
		},
		{
			Slug:        "free-now",
			Name:        "Свободно сейчас",
			Description: "Живая загрузка офисов: сколько мест свободно в эту минуту.",
			Group:       "Пространство",
			Status:      domain.ServiceStatusSoon,
		},
		{
			Slug:        "nearest-room",
			Name:        "Ближайшая переговорка",
			Description: "Подбор переговорки под нужное время и число участников.",
			Group:       "Пространство",
			Status:      domain.ServiceStatusSoon,
		},
		{
			Slug:        "rent-calc",
			Name:        "Калькулятор аренды",
			Description: "Стоимость брони: часы на тариф плюс дополнительные опции.",
			Group:       "Тарифы",
			Status:      domain.ServiceStatusSoon,
		},
		{
			Slug:        "tariff-compare",
			Name:        "Сравнение тарифов",
			Description: "День, неделя или месяц: что выгоднее при вашем графике.",
			Group:       "Тарифы",
			Status:      domain.ServiceStatusSoon,
		},
		{
			Slug:        "format-test",
			Name:        "Тест «Формат работы»",
			Description: "Пять вопросов, чтобы подобрать стол, кабинет или лаунж.",
			Group:       "Планирование",
			Status:      domain.ServiceStatusSoon,
		},
		{
			Slug:        "office-load",
			Name:        "Загрузка офиса по часам",
			Description: "Пиковые часы и тихие окна каждого офиса в течение дня.",
			Group:       "Пространство",
			Status:      domain.ServiceStatusSoon,
		},
		{
			Slug:        "timer",
			Name:        "Таймер до конца брони",
			Description: "Обратный отсчёт до завершения аренды места или переговорки.",
			Group:       "Бронирование",
			Status:      domain.ServiceStatusSoon,
		},
		{
			Slug:        "desk-of-the-day",
			Name:        "Стол дня",
			Description: "Случайное место недели с описанием и деталями.",
			Group:       "Планирование",
			Status:      domain.ServiceStatusSoon,
		},
		{
			Slug:        "weather",
			Name:        "Погода",
			Description: "Прогноз у каждого офиса: идти пешком или остаться дома.",
			Group:       "Инфраструктура",
			Status:      domain.ServiceStatusSoon,
		},
	}
}
