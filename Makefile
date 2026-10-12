# Makefile для локальной работы с сайтом коворкинга «Место».
# Основные действия: поднять, опустить, удалить.

# Пароль PostgreSQL (переопределяется: make up POSTGRES_PASSWORD=...).
POSTGRES_PASSWORD ?= devsecret
export POSTGRES_PASSWORD

COMPOSE ?= docker compose

.PHONY: up down clean

up: ## Поднять БД и сайт в контейнерах
	$(COMPOSE) up -d --build
	@echo "Сайт: http://localhost:8081/"

down: ## Остановить и удалить контейнеры (данные БД сохраняются)
	$(COMPOSE) down

clean: ## Удалить контейнеры, том БД и образ
	$(COMPOSE) down -v --rmi local
	@echo "Локальное окружение очищено."
