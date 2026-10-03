// Package domain содержит бизнес-сущности коворкинга.
// Слой чистый: никаких зависимостей от БД, HTTP или фреймворков.
package domain

// ServiceStatus описывает готовность сервиса сайта.
type ServiceStatus string

const (
	// ServiceStatusReady означает, что сервис доступен пользователям.
	ServiceStatusReady ServiceStatus = "ready"
	// ServiceStatusSoon означает, что сервис запланирован и находится в разработке.
	ServiceStatusSoon ServiceStatus = "soon"
)

// Service — сервис, доступный посетителям сайта (виджет, калькулятор, приложение).
type Service struct {
	Slug        string
	Name        string
	Description string
	Group       string
	Status      ServiceStatus
	URL         string
}
