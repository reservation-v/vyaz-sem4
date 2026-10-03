// Package application содержит use cases (сценарии) приложения.
// Здесь объявляются порты, которые реализует infrastructure.
package application

import (
	"context"

	"coworking/internal/domain"
)

// ServiceRepository — порт хранилища сервисов (DIP:
// интерфейс принадлежит application, реализация — infrastructure).
type ServiceRepository interface {
	ListServices(ctx context.Context) ([]domain.Service, error)
}

// ServiceCatalog — use case: просмотр каталога сервисов сайта.
type ServiceCatalog struct {
	repo ServiceRepository
}

// NewServiceCatalog создаёт сценарий каталога с переданным хранилищем.
func NewServiceCatalog(repo ServiceRepository) *ServiceCatalog {
	return &ServiceCatalog{repo: repo}
}

// ListServices возвращает все сервисы сайта в порядке, заданном хранилищем.
func (c *ServiceCatalog) ListServices(ctx context.Context) ([]domain.Service, error) {
	return c.repo.ListServices(ctx)
}
