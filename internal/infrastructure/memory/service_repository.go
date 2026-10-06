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

// NewServiceRepository создаёт хранилище с каталогом сервисов сайта.
func NewServiceRepository() *ServiceRepository {
	return &ServiceRepository{services: seedServices()}
}

// ListServices возвращает копию каталога сервисов.
func (r *ServiceRepository) ListServices(_ context.Context) ([]domain.Service, error) {
	return append([]domain.Service(nil), r.services...), nil
}

// seedServices — каталог сервисов сайта «Место».
//
// Первый сервис — собственное приложение к базе данных коворкинга (Лаба 3).
// Остальные — интеграции с внешними готовыми сервисами сторонних разработчиков.
// У каждого свой провайдер, формат ответа и способ подключения: JSON через fetch,
// простой текст, изображение, POST-форма — чтобы показать разные подходы интеграции.
func seedServices() []domain.Service {
	return []domain.Service{
		{
			Slug:        "booking",
			Name:        "Онлайн-бронирование",
			Description: "Наше приложение к базе данных коворкинга: выбор стола или переговорки, проверка свободных часов, расчёт стоимости с депозитом и отмена брони.",
			Group:       "База данных",
			Status:      domain.ServiceStatusReady,
			URL:         "/booking",
		},
		{
			Slug:        "weather",
			Name:        "Погода у офиса",
			Description: "Текущая погода в Москва-Сити от сервиса wttr.in: температура, ветер и осадки. Помогает решить, идти пешком или остаться работать дома.",
			Group:       "Инфраструктура",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "currency",
			Name:        "Курс валют",
			Description: "Официальные курсы доллара, евро и юаня к рублю от Банка России (cbr-xml-daily.ru). Нужно гостям и командам, которые платят в валюте.",
			Group:       "Финансы",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "translate",
			Name:        "Переводчик RU↔EN",
			Description: "Перевод фразы с русского на английский и обратно через сервис MyMemory. Выручает в общении с иностранными резидентами и гостями.",
			Group:       "Коммуникации",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "qr-code",
			Name:        "QR-код",
			Description: "Генератор QR-кодов (goqr.me): закодируйте ссылку на бронь, адрес офиса или пароль от гостевого Wi-Fi и покажите гостю с телефона.",
			Group:       "Инфраструктура",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "workday",
			Name:        "Рабочий день или выходной",
			Description: "Быстрая проверка, рабочий сегодня день или выходной, через сервис isdayoff.ru: показываем статус и ближайшее изменение графика.",
			Group:       "Расписание",
			Status:      domain.ServiceStatusReady,
		},
		{
			Slug:        "world-time",
			Name:        "Время в филиалах",
			Description: "Точное местное время в Москве, Пекине и Лондоне (timeapi.io) — удобно, если клиенты заказывают переговорку из другого часового пояса.",
			Group:       "Расписание",
			Status:      domain.ServiceStatusReady,
		},
	}
}
