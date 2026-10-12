package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"coworking/internal/application"
	"coworking/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ application.BookingRepository = (*BookingRepository)(nil)

// BookingRepository реализует хранение каталога и броней в PostgreSQL.
type BookingRepository struct {
	pool *pgxpool.Pool
}

// NewBookingRepository открывает пул подключений PostgreSQL.
func NewBookingRepository(ctx context.Context, databaseURL string) (*BookingRepository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return &BookingRepository{pool: pool}, nil
}

// Close освобождает подключения к базе данных.
func (r *BookingRepository) Close() {
	r.pool.Close()
}

func (r *BookingRepository) ListSpaces(ctx context.Context) ([]domain.Space, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, type, capacity, price_per_hour, deposit
		FROM spaces
		ORDER BY type, name, id`)
	if err != nil {
		return nil, fmt.Errorf("query spaces: %w", err)
	}
	defer rows.Close()

	spaces := make([]domain.Space, 0)
	for rows.Next() {
		var space domain.Space
		if err := rows.Scan(&space.ID, &space.Name, &space.Type, &space.Capacity, &space.PricePerHour, &space.Deposit); err != nil {
			return nil, fmt.Errorf("scan space: %w", err)
		}
		spaces = append(spaces, space)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spaces: %w", err)
	}
	return spaces, nil
}

func (r *BookingRepository) ListBookings(ctx context.Context) ([]domain.Booking, error) {
	return r.SearchBookings(ctx, application.BookingFilter{})
}

func (r *BookingRepository) SearchBookings(ctx context.Context, filter application.BookingFilter) ([]domain.Booking, error) {
	query := `
		SELECT b.id, b.space_id, b.client_name, b.book_date::text, b.rate,
		       b.start_hour, b.duration, b.price, b.deposit, b.status, b.created_at
		FROM bookings b
		JOIN spaces s ON s.id = b.space_id
		WHERE ($1 = '' OR b.client_name ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR s.type = $2)
		  AND ($3 = '' OR b.status = $3)
		  AND ($4::date IS NULL OR b.book_date >= $4::date)
		  AND ($5::date IS NULL OR b.book_date <= $5::date)
		ORDER BY b.book_date, b.start_hour, b.created_at`
	rows, err := r.pool.Query(ctx, query,
		filter.Search,
		string(filter.SpaceType),
		string(filter.Status),
		emptyToNil(filter.DateFrom),
		emptyToNil(filter.DateTo),
	)
	if err != nil {
		return nil, fmt.Errorf("search bookings: %w", err)
	}
	defer rows.Close()
	return scanBookings(rows)
}

func (r *BookingRepository) BookingStatistics(ctx context.Context, filter application.BookingFilter) (application.BookingStatistics, error) {
	const query = `
		WITH filtered AS (
			SELECT b.status, b.price, s.type
			FROM bookings b
			JOIN spaces s ON s.id = b.space_id
			WHERE ($1 = '' OR b.client_name ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')
			  AND ($2 = '' OR s.type = $2)
			  AND ($3 = '' OR b.status = $3)
			  AND ($4::date IS NULL OR b.book_date >= $4::date)
			  AND ($5::date IS NULL OR b.book_date <= $5::date)
		)
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'confirmed'),
		       COUNT(*) FILTER (WHERE status = 'cancelled'),
		       COALESCE(SUM(price) FILTER (WHERE status = 'confirmed'), 0)
		FROM filtered`
	var stats application.BookingStatistics
	if err := r.pool.QueryRow(ctx, query,
		filter.Search,
		string(filter.SpaceType),
		string(filter.Status),
		emptyToNil(filter.DateFrom),
		emptyToNil(filter.DateTo),
	).Scan(&stats.TotalBookings, &stats.ConfirmedBookings, &stats.CancelledBookings, &stats.Revenue); err != nil {
		return application.BookingStatistics{}, fmt.Errorf("query booking statistics: %w", err)
	}

	byTypeRows, err := r.pool.Query(ctx, `
		SELECT s.type, COUNT(*), COALESCE(SUM(b.price) FILTER (WHERE b.status = 'confirmed'), 0)
		FROM bookings b
		JOIN spaces s ON s.id = b.space_id
		WHERE ($1 = '' OR b.client_name ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR s.type = $2)
		  AND ($3 = '' OR b.status = $3)
		  AND ($4::date IS NULL OR b.book_date >= $4::date)
		  AND ($5::date IS NULL OR b.book_date <= $5::date)
		GROUP BY s.type
		ORDER BY s.type`,
		filter.Search,
		string(filter.SpaceType),
		string(filter.Status),
		emptyToNil(filter.DateFrom),
		emptyToNil(filter.DateTo),
	)
	if err != nil {
		return application.BookingStatistics{}, fmt.Errorf("query booking statistics by type: %w", err)
	}
	defer byTypeRows.Close()
	stats.BySpaceType = make([]application.BookingTypeStatistics, 0)
	for byTypeRows.Next() {
		var item application.BookingTypeStatistics
		if err := byTypeRows.Scan(&item.SpaceType, &item.Count, &item.Revenue); err != nil {
			return application.BookingStatistics{}, fmt.Errorf("scan booking statistics by type: %w", err)
		}
		stats.BySpaceType = append(stats.BySpaceType, item)
	}
	if err := byTypeRows.Err(); err != nil {
		return application.BookingStatistics{}, fmt.Errorf("iterate booking statistics by type: %w", err)
	}
	return stats, nil
}

func (r *BookingRepository) SaveBooking(ctx context.Context, booking domain.Booking) (domain.Booking, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin save booking transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM spaces WHERE id = $1)`, booking.SpaceID).Scan(&exists); err != nil {
		return domain.Booking{}, fmt.Errorf("check booking space: %w", err)
	}
	if !exists {
		return domain.Booking{}, fmt.Errorf("%w: id %d", application.ErrSpaceNotFound, booking.SpaceID)
	}
	var lockedSpaceID int
	if err := tx.QueryRow(ctx, `SELECT id FROM spaces WHERE id = $1 FOR UPDATE`, booking.SpaceID).Scan(&lockedSpaceID); err != nil {
		return domain.Booking{}, fmt.Errorf("lock booking space: %w", err)
	}

	conflict, err := hasOverlappingBooking(ctx, tx, booking)
	if err != nil {
		return domain.Booking{}, err
	}
	if conflict {
		return domain.Booking{}, fmt.Errorf("%w: space id %d", application.ErrOverlap, booking.SpaceID)
	}

	query := `INSERT INTO bookings (space_id, client_name, book_date, rate, start_hour, duration, price, deposit, status)
		VALUES ($1, $2, $3::date, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`
	if err := tx.QueryRow(ctx, query,
		booking.SpaceID, booking.ClientName, booking.Date, string(booking.Rate), booking.StartHour,
		booking.Duration, booking.Price, booking.Deposit, string(booking.Status),
	).Scan(&booking.ID, &booking.CreatedAt); err != nil {
		return domain.Booking{}, fmt.Errorf("insert booking: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("commit booking transaction: %w", err)
	}
	return booking, nil
}

func hasOverlappingBooking(ctx context.Context, tx pgx.Tx, booking domain.Booking) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM bookings b
			WHERE b.space_id = $1
			  AND b.status = 'confirmed'
			  AND b.book_date < $3::date
			  AND $2::date < CASE b.rate
				WHEN 'week' THEN b.book_date + 7
				WHEN 'month' THEN (b.book_date + INTERVAL '1 month')::date
				ELSE b.book_date + 1
			  END
			AND CASE
				WHEN $4::text = 'hour' AND b.rate = 'hour' AND b.book_date = $2::date
				THEN b.start_hour < $5::int + $6::int AND $5::int < b.start_hour + b.duration
				WHEN $4::text = 'hour' AND b.rate = 'hour'
				THEN FALSE
				ELSE TRUE
			  END
		)`
	var conflict bool
	endDate := booking.Date
	parsedDate, err := time.Parse("2006-01-02", booking.Date)
	if err != nil {
		return false, fmt.Errorf("parse booking date: %w", err)
	}
	switch booking.Rate {
	case domain.BookingRateWeek:
		endDate = parsedDate.AddDate(0, 0, 7).Format("2006-01-02")
	case domain.BookingRateMonth:
		endDate = parsedDate.AddDate(0, 1, 0).Format("2006-01-02")
	default:
		endDate = parsedDate.AddDate(0, 0, 1).Format("2006-01-02")
	}
	if err := tx.QueryRow(ctx, query, booking.SpaceID, booking.Date, endDate, string(booking.Rate), booking.StartHour, booking.Duration).Scan(&conflict); err != nil {
		return false, fmt.Errorf("check booking overlap: %w", err)
	}
	return conflict, nil
}

func (r *BookingRepository) BookingByID(ctx context.Context, id int) (domain.Booking, error) {
	const query = `SELECT id, space_id, client_name, book_date::text, rate, start_hour, duration,
		price, deposit, status, created_at FROM bookings WHERE id = $1`
	var booking domain.Booking
	if err := r.pool.QueryRow(ctx, query, id).Scan(
		&booking.ID, &booking.SpaceID, &booking.ClientName, &booking.Date, &booking.Rate,
		&booking.StartHour, &booking.Duration, &booking.Price, &booking.Deposit, &booking.Status, &booking.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Booking{}, application.ErrBookingNotFound
		}
		return domain.Booking{}, fmt.Errorf("query booking %d: %w", id, err)
	}
	return booking, nil
}

func (r *BookingRepository) DeleteBooking(ctx context.Context, id int) error {
	command, err := r.pool.Exec(ctx, `UPDATE bookings SET status = 'cancelled' WHERE id = $1 AND status = 'confirmed'`, id)
	if err != nil {
		return fmt.Errorf("cancel booking %d: %w", id, err)
	}
	if command.RowsAffected() == 0 {
		return application.ErrBookingNotFound
	}
	return nil
}

func emptyToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func scanBookings(rows pgx.Rows) ([]domain.Booking, error) {
	bookings := make([]domain.Booking, 0)
	for rows.Next() {
		var booking domain.Booking
		if err := rows.Scan(
			&booking.ID, &booking.SpaceID, &booking.ClientName, &booking.Date, &booking.Rate,
			&booking.StartHour, &booking.Duration, &booking.Price, &booking.Deposit, &booking.Status, &booking.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		bookings = append(bookings, booking)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookings: %w", err)
	}
	return bookings, nil
}

// MigrationURI переводит DATABASE_URL в формат URI, ожидаемый драйвером migrate.
func MigrationURI(databaseURL string) (string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "", fmt.Errorf("parse database URL for migrations: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", fmt.Errorf("unsupported database URL scheme %q", parsed.Scheme)
	}
	parsed.Scheme = "pgx5"
	query := parsed.Query()
	query.Set("x-multi-statement", "true")
	parsed.RawQuery = query.Encode()
	return strings.TrimSpace(parsed.String()), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
