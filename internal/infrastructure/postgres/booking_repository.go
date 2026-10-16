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
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	_ application.BookingRepository = (*Repository)(nil)
	_ application.ClientRepository  = (*Repository)(nil)
)

// Repository реализует хранение пространств, клиентов и броней в PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository открывает пул подключений PostgreSQL.
func NewRepository(ctx context.Context, databaseURL string) (*Repository, error) {
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
	return &Repository{pool: pool}, nil
}

// Close освобождает подключения к базе данных.
func (r *Repository) Close() {
	r.pool.Close()
}

const spaceColumns = `s.id, s.office_id, s.name, s.capacity, s.price_per_hour, s.deposit, s.is_active,
	t.id, t.code, t.title, t.description`

func (r *Repository) ListSpaces(ctx context.Context) ([]domain.Space, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+spaceColumns+`
		FROM spaces s
		JOIN space_types t ON t.id = s.space_type_id
		ORDER BY s.name, s.id`)
	if err != nil {
		return nil, fmt.Errorf("query spaces: %w", err)
	}
	defer rows.Close()

	spaces := make([]domain.Space, 0)
	for rows.Next() {
		space, err := scanSpace(rows)
		if err != nil {
			return nil, err
		}
		spaces = append(spaces, space)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spaces: %w", err)
	}
	return spaces, nil
}

func (r *Repository) ListSpaceTypes(ctx context.Context) ([]domain.SpaceType, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, code, title, description FROM space_types ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query space types: %w", err)
	}
	defer rows.Close()

	types := make([]domain.SpaceType, 0)
	for rows.Next() {
		var (
			item     domain.SpaceType
			codeText string
		)
		if err := rows.Scan(&item.ID, &codeText, &item.Title, &item.Description); err != nil {
			return nil, fmt.Errorf("scan space type: %w", err)
		}
		item.Code = domain.SpaceTypeCode(codeText)
		types = append(types, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate space types: %w", err)
	}
	return types, nil
}

func (r *Repository) ListClients(ctx context.Context) ([]domain.Client, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, full_name, phone, email, company, created_at
		FROM clients
		ORDER BY full_name, id`)
	if err != nil {
		return nil, fmt.Errorf("query clients: %w", err)
	}
	defer rows.Close()

	clients := make([]domain.Client, 0)
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clients: %w", err)
	}
	return clients, nil
}

func (r *Repository) ClientByID(ctx context.Context, id int) (domain.Client, error) {
	const query = `SELECT id, full_name, phone, email, company, created_at FROM clients WHERE id = $1`
	client, err := scanClient(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Client{}, application.ErrClientNotFound
		}
		return domain.Client{}, fmt.Errorf("query client %d: %w", id, err)
	}
	return client, nil
}

func (r *Repository) SaveClient(ctx context.Context, client domain.Client) (domain.Client, error) {
	const query = `
		INSERT INTO clients (full_name, phone, email, company)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	if err := r.pool.QueryRow(ctx, query, client.FullName, client.Phone, client.Email, client.Company).
		Scan(&client.ID, &client.CreatedAt); err != nil {
		return domain.Client{}, fmt.Errorf("insert client: %w", err)
	}
	return client, nil
}

func (r *Repository) UpdateClient(ctx context.Context, client domain.Client) (domain.Client, error) {
	const query = `
		UPDATE clients
		SET full_name = $2, phone = $3, email = $4, company = $5
		WHERE id = $1
		RETURNING created_at`
	if err := r.pool.QueryRow(ctx, query, client.ID, client.FullName, client.Phone, client.Email, client.Company).
		Scan(&client.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Client{}, application.ErrClientNotFound
		}
		return domain.Client{}, fmt.Errorf("update client %d: %w", client.ID, err)
	}
	return client, nil
}

// DeleteClient удаляет клиента вместе с его бронями. Целостность обеспечивает
// внешний ключ bookings.client_id ON DELETE CASCADE: брони уходят вслед за
// клиентом, предварительная отмена не нужна.
func (r *Repository) DeleteClient(ctx context.Context, id int) error {
	command, err := r.pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete client %d: %w", id, err)
	}
	if command.RowsAffected() == 0 {
		return application.ErrClientNotFound
	}
	return nil
}

// bookingColumns — общий список полей с соединением клиентов и пространств.
const bookingColumns = `b.id, b.space_id, b.client_id, b.book_date::text, b.rate, b.start_hour,
	b.duration, b.price, b.deposit, b.status, b.created_at,
	c.full_name, c.company, s.name, t.code, t.title`

// bookingFilterClause — общие условия поиска.
// $1 — поиск по одному значению (клиент ИЛИ компания ИЛИ пространство);
// $2 — клиент (id) для поиска по двум синхронизированным значениям;
// $3 — тип пространства.
const bookingFilterClause = `
	WHERE ($1 = '' OR c.full_name ILIKE '%' || $1 || '%' OR c.company ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')
	  AND ($2 = 0 OR b.client_id = $2)
	  AND ($3 = '' OR t.code = $3)`

func bookingFilterArgs(filter application.BookingFilter) []any {
	return []any{
		filter.Search,
		filter.ClientID,
		string(filter.SpaceType),
	}
}

func (r *Repository) SearchBookings(ctx context.Context, filter application.BookingFilter) ([]application.BookingDetails, error) {
	query := `
		SELECT ` + bookingColumns + `
		FROM bookings b
		JOIN clients c ON c.id = b.client_id
		JOIN spaces s ON s.id = b.space_id
		JOIN space_types t ON t.id = s.space_type_id` + bookingFilterClause + `
		  AND ($4 = 0 OR b.space_id = $4)
		ORDER BY b.book_date, b.start_hour, b.created_at`
	rows, err := r.pool.Query(ctx, query, append(bookingFilterArgs(filter), filter.SpaceID)...)
	if err != nil {
		return nil, fmt.Errorf("search bookings: %w", err)
	}
	defer rows.Close()
	return scanBookingDetails(rows)
}

func (r *Repository) BookingByID(ctx context.Context, id int) (application.BookingDetails, error) {
	query := `SELECT ` + bookingColumns + `
		FROM bookings b
		JOIN clients c ON c.id = b.client_id
		JOIN spaces s ON s.id = b.space_id
		JOIN space_types t ON t.id = s.space_type_id
		WHERE b.id = $1`
	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return application.BookingDetails{}, fmt.Errorf("query booking %d: %w", id, err)
	}
	defer rows.Close()

	details, err := scanBookingDetails(rows)
	if err != nil {
		return application.BookingDetails{}, fmt.Errorf("query booking %d: %w", id, err)
	}
	if len(details) == 0 {
		return application.BookingDetails{}, application.ErrBookingNotFound
	}
	return details[0], nil
}

func (r *Repository) BookingStatistics(ctx context.Context, filter application.BookingFilter) (application.BookingStatistics, error) {
	summaryQuery := `
		WITH filtered AS (
			SELECT b.status, b.price
			FROM bookings b
			JOIN clients c ON c.id = b.client_id
			JOIN spaces s ON s.id = b.space_id
			JOIN space_types t ON t.id = s.space_type_id` + bookingFilterClause + `
		)
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'confirmed'),
		       COUNT(*) FILTER (WHERE status = 'cancelled'),
		       COALESCE(SUM(price) FILTER (WHERE status = 'confirmed'), 0)
		FROM filtered`
	var stats application.BookingStatistics
	if err := r.pool.QueryRow(ctx, summaryQuery, bookingFilterArgs(filter)...).
		Scan(&stats.TotalBookings, &stats.ConfirmedBookings, &stats.CancelledBookings, &stats.Revenue); err != nil {
		return application.BookingStatistics{}, fmt.Errorf("query booking statistics: %w", err)
	}

	byTypeQuery := `
		SELECT t.code, t.title, COUNT(*), COALESCE(SUM(b.price) FILTER (WHERE b.status = 'confirmed'), 0)
		FROM bookings b
		JOIN clients c ON c.id = b.client_id
		JOIN spaces s ON s.id = b.space_id
		JOIN space_types t ON t.id = s.space_type_id` + bookingFilterClause + `
		GROUP BY t.code, t.title
		ORDER BY t.code`
	rows, err := r.pool.Query(ctx, byTypeQuery, bookingFilterArgs(filter)...)
	if err != nil {
		return application.BookingStatistics{}, fmt.Errorf("query booking statistics by type: %w", err)
	}
	defer rows.Close()

	stats.BySpaceType = make([]application.BookingTypeStatistics, 0)
	for rows.Next() {
		var (
			item     application.BookingTypeStatistics
			codeText string
		)
		if err := rows.Scan(&codeText, &item.Title, &item.Count, &item.Revenue); err != nil {
			return application.BookingStatistics{}, fmt.Errorf("scan booking statistics by type: %w", err)
		}
		item.SpaceType = domain.SpaceTypeCode(codeText)
		stats.BySpaceType = append(stats.BySpaceType, item)
	}
	if err := rows.Err(); err != nil {
		return application.BookingStatistics{}, fmt.Errorf("iterate booking statistics by type: %w", err)
	}
	return stats, nil
}

func (r *Repository) DatabaseSummary(ctx context.Context) (application.DatabaseSummary, error) {
	const query = `
		SELECT (SELECT COUNT(*) FROM offices),
		       (SELECT COUNT(*) FROM space_types),
		       (SELECT COUNT(*) FROM spaces),
		       (SELECT COUNT(*) FROM clients),
		       (SELECT COUNT(*) FROM bookings),
		       (SELECT COUNT(*) FROM bookings WHERE status = 'confirmed'),
		       (SELECT COUNT(*) FROM bookings WHERE status = 'cancelled'),
		       (SELECT COALESCE(SUM(price), 0) FROM bookings WHERE status = 'confirmed')`
	var summary application.DatabaseSummary
	if err := r.pool.QueryRow(ctx, query).Scan(
		&summary.Offices, &summary.SpaceTypes, &summary.Spaces, &summary.Clients,
		&summary.Bookings, &summary.Confirmed, &summary.Cancelled, &summary.Revenue,
	); err != nil {
		return application.DatabaseSummary{}, fmt.Errorf("query database summary: %w", err)
	}

	const topQuery = `
		SELECT t.title, COUNT(*)
		FROM bookings b
		JOIN spaces s ON s.id = b.space_id
		JOIN space_types t ON t.id = s.space_type_id
		WHERE b.status = 'confirmed'
		GROUP BY t.title
		ORDER BY COUNT(*) DESC, t.title
		LIMIT 1`
	if err := r.pool.QueryRow(ctx, topQuery).Scan(&summary.TopSpaceTypeTitle, &summary.TopSpaceTypeCount); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return application.DatabaseSummary{}, fmt.Errorf("query top space type: %w", err)
	}
	return summary, nil
}

func (r *Repository) SaveBooking(ctx context.Context, booking domain.Booking) (domain.Booking, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin save booking transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSpace(ctx, tx, booking.SpaceID); err != nil {
		return domain.Booking{}, err
	}

	conflict, err := hasOverlappingBooking(ctx, tx, booking, 0)
	if err != nil {
		return domain.Booking{}, err
	}
	if conflict {
		return domain.Booking{}, fmt.Errorf("%w: space id %d", application.ErrOverlap, booking.SpaceID)
	}

	const query = `INSERT INTO bookings (space_id, client_id, book_date, rate, start_hour, duration, price, deposit, status)
		VALUES ($1, $2, $3::date, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`
	if err := tx.QueryRow(ctx, query,
		booking.SpaceID, booking.ClientID, booking.Date, string(booking.Rate), booking.StartHour,
		booking.Duration, booking.Price, booking.Deposit, string(booking.Status),
	).Scan(&booking.ID, &booking.CreatedAt); err != nil {
		return domain.Booking{}, fmt.Errorf("insert booking: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("commit booking transaction: %w", err)
	}
	return booking, nil
}

func (r *Repository) UpdateBooking(ctx context.Context, booking domain.Booking) (domain.Booking, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin update booking transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSpace(ctx, tx, booking.SpaceID); err != nil {
		return domain.Booking{}, err
	}

	conflict, err := hasOverlappingBooking(ctx, tx, booking, booking.ID)
	if err != nil {
		return domain.Booking{}, err
	}
	if conflict {
		return domain.Booking{}, fmt.Errorf("%w: space id %d", application.ErrOverlap, booking.SpaceID)
	}

	const query = `UPDATE bookings
		SET space_id = $2, client_id = $3, book_date = $4::date, rate = $5,
		    start_hour = $6, duration = $7, price = $8, deposit = $9, status = $10
		WHERE id = $1
		RETURNING created_at`
	if err := tx.QueryRow(ctx, query,
		booking.ID, booking.SpaceID, booking.ClientID, booking.Date, string(booking.Rate), booking.StartHour,
		booking.Duration, booking.Price, booking.Deposit, string(booking.Status),
	).Scan(&booking.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Booking{}, application.ErrBookingNotFound
		}
		return domain.Booking{}, fmt.Errorf("update booking %d: %w", booking.ID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("commit update booking transaction: %w", err)
	}
	return booking, nil
}

// DeleteBooking удаляет бронь из таблицы. Отмена брони выполняется удалением,
// поэтому запись исчезает из списка и перестаёт учитываться в счётчиках.
func (r *Repository) DeleteBooking(ctx context.Context, id int) error {
	command, err := r.pool.Exec(ctx, `DELETE FROM bookings WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete booking %d: %w", id, err)
	}
	if command.RowsAffected() == 0 {
		return application.ErrBookingNotFound
	}
	return nil
}

// lockSpace проверяет, что пространство существует, активно, и берёт по нему блокировку.
func lockSpace(ctx context.Context, tx pgx.Tx, spaceID int) error {
	var isActive bool
	if err := tx.QueryRow(ctx, `SELECT is_active FROM spaces WHERE id = $1 FOR UPDATE`, spaceID).Scan(&isActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: id %d", application.ErrSpaceNotFound, spaceID)
		}
		return fmt.Errorf("lock booking space: %w", err)
	}
	if !isActive {
		return fmt.Errorf("%w: id %d", application.ErrSpaceNotFound, spaceID)
	}
	return nil
}

func hasOverlappingBooking(ctx context.Context, tx pgx.Tx, booking domain.Booking, excludeID int) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM bookings b
			WHERE b.space_id = $1
			  AND b.status = 'confirmed'
			  AND b.id <> $7
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
	parsedDate, err := time.Parse("2006-01-02", booking.Date)
	if err != nil {
		return false, fmt.Errorf("parse booking date: %w", err)
	}

	endDate := parsedDate.AddDate(0, 0, 1)
	switch booking.Rate {
	case domain.BookingRateWeek:
		endDate = parsedDate.AddDate(0, 0, 7)
	case domain.BookingRateMonth:
		endDate = parsedDate.AddDate(0, 1, 0)
	}

	var conflict bool
	if err := tx.QueryRow(ctx, query, booking.SpaceID, booking.Date, endDate.Format("2006-01-02"),
		string(booking.Rate), booking.StartHour, booking.Duration, excludeID).Scan(&conflict); err != nil {
		return false, fmt.Errorf("check booking overlap: %w", err)
	}
	return conflict, nil
}

func scanSpace(row pgx.Row) (domain.Space, error) {
	var (
		space    domain.Space
		codeText string
	)
	if err := row.Scan(
		&space.ID, &space.OfficeID, &space.Name, &space.Capacity, &space.PricePerHour, &space.Deposit, &space.IsActive,
		&space.Type.ID, &codeText, &space.Type.Title, &space.Type.Description,
	); err != nil {
		return domain.Space{}, fmt.Errorf("scan space: %w", err)
	}
	space.Type.Code = domain.SpaceTypeCode(codeText)
	return space, nil
}

func scanClient(row pgx.Row) (domain.Client, error) {
	var client domain.Client
	if err := row.Scan(&client.ID, &client.FullName, &client.Phone, &client.Email, &client.Company, &client.CreatedAt); err != nil {
		return domain.Client{}, err
	}
	return client, nil
}

func scanBookingDetails(rows pgx.Rows) ([]application.BookingDetails, error) {
	details := make([]application.BookingDetails, 0)
	for rows.Next() {
		var (
			item     application.BookingDetails
			rateText string
			statText string
			codeText string
		)
		if err := rows.Scan(
			&item.Booking.ID, &item.Booking.SpaceID, &item.Booking.ClientID, &item.Booking.Date,
			&rateText, &item.Booking.StartHour, &item.Booking.Duration, &item.Booking.Price,
			&item.Booking.Deposit, &statText, &item.Booking.CreatedAt,
			&item.ClientName, &item.ClientCompany, &item.SpaceName, &codeText, &item.SpaceTypeTitle,
		); err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		item.Booking.Rate = domain.BookingRate(rateText)
		item.Booking.Status = domain.BookingStatus(statText)
		item.SpaceTypeCode = domain.SpaceTypeCode(codeText)
		details = append(details, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookings: %w", err)
	}
	return details, nil
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
