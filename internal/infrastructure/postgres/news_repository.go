package postgres

import (
	"context"
	"errors"
	"fmt"

	"coworking/internal/application"
	"coworking/internal/domain"

	"github.com/jackc/pgx/v5/pgconn"
)

// Проверка на этапе компиляции: Repository реализует порт новостей.
var _ application.NewsRepository = (*Repository)(nil)

// ListNews возвращает опубликованные свои новости по убыванию даты.
func (r *Repository) ListNews(ctx context.Context, limit int) ([]domain.NewsItem, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, title, summary, published_at
		FROM news
		WHERE is_published
		ORDER BY published_at DESC, id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list news: %w", err)
	}
	defer rows.Close()

	var items []domain.NewsItem
	for rows.Next() {
		item := domain.NewsItem{Own: true, Source: "Место"}
		if err := rows.Scan(&item.ID, &item.Title, &item.Summary, &item.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan news: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate news: %w", err)
	}
	return items, nil
}

// SaveNews сохраняет свою новость и возвращает её с присвоенным ID.
func (r *Repository) SaveNews(ctx context.Context, item domain.NewsItem) (domain.NewsItem, error) {
	const query = `
		INSERT INTO news (title, summary, published_at)
		VALUES ($1, $2, $3)
		RETURNING id`

	if err := r.pool.QueryRow(ctx, query, item.Title, item.Summary, item.PublishedAt).Scan(&item.ID); err != nil {
		return domain.NewsItem{}, fmt.Errorf("insert news: %w", err)
	}

	item.Own = true
	if item.Source == "" {
		item.Source = "Место"
	}
	return item, nil
}

// DeleteNews удаляет свою новость по ID.
func (r *Repository) DeleteNews(ctx context.Context, id int) error {
	command, err := r.pool.Exec(ctx, `DELETE FROM news WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete news %d: %w", id, err)
	}
	if command.RowsAffected() == 0 {
		return application.ErrNewsNotFound
	}
	return nil
}

// ListNewsSources возвращает активные подключённые RSS-ленты.
func (r *Repository) ListNewsSources(ctx context.Context) ([]domain.NewsSource, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, url, is_active
		FROM news_sources
		WHERE is_active
		ORDER BY title, id`)
	if err != nil {
		return nil, fmt.Errorf("list news sources: %w", err)
	}
	defer rows.Close()

	var sources []domain.NewsSource
	for rows.Next() {
		var source domain.NewsSource
		if err := rows.Scan(&source.ID, &source.Title, &source.URL, &source.IsActive); err != nil {
			return nil, fmt.Errorf("scan news source: %w", err)
		}
		sources = append(sources, source)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate news sources: %w", err)
	}
	return sources, nil
}

// SaveNewsSource подключает новую RSS-ленту.
func (r *Repository) SaveNewsSource(ctx context.Context, source domain.NewsSource) (domain.NewsSource, error) {
	const query = `
		INSERT INTO news_sources (title, url, is_active)
		VALUES ($1, $2, $3)
		RETURNING id`

	err := r.pool.QueryRow(ctx, query, source.Title, source.URL, source.IsActive).Scan(&source.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.NewsSource{}, application.ErrDuplicateNewsSource
		}
		return domain.NewsSource{}, fmt.Errorf("insert news source: %w", err)
	}
	return source, nil
}

// DeleteNewsSource отключает RSS-ленту.
func (r *Repository) DeleteNewsSource(ctx context.Context, id int) error {
	command, err := r.pool.Exec(ctx, `DELETE FROM news_sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete news source %d: %w", id, err)
	}
	if command.RowsAffected() == 0 {
		return application.ErrNewsSourceNotFound
	}
	return nil
}
