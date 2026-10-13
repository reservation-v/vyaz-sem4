-- Схема БД коворкинга «Место» (Лаба 3).
-- Пять таблиц, 36 атрибутов:
--   offices     (7)  - локации коворкинга;
--   space_types (4)  - классификатор типов пространств;
--   spaces      (8)  - конкретные рабочие места, кабинеты и переговорки;
--   clients     (6)  - клиенты (люди и команды), которые бронируют;
--   bookings    (11) - брони пространств клиентами.
-- Контроль значений выполняется ограничениями CHECK, NOT NULL, UNIQUE и внешними ключами.

CREATE TABLE offices (
    id      serial PRIMARY KEY,
    name    text NOT NULL,
    city    text NOT NULL,
    address text NOT NULL,
    floor   int  NOT NULL CHECK (floor > 0),
    phone   text NOT NULL,
    email   text NOT NULL CHECK (email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$')
);

CREATE TABLE space_types (
    id          serial PRIMARY KEY,
    code        text NOT NULL UNIQUE CHECK (code ~ '^[a-z_]+$'),
    title       text NOT NULL CHECK (char_length(btrim(title)) > 0),
    description text NOT NULL DEFAULT ''
);

CREATE TABLE spaces (
    id             serial PRIMARY KEY,
    office_id      int  NOT NULL REFERENCES offices (id) ON DELETE RESTRICT,
    space_type_id  int  NOT NULL REFERENCES space_types (id) ON DELETE RESTRICT,
    name           text NOT NULL CHECK (char_length(btrim(name)) > 0),
    capacity       int  NOT NULL CHECK (capacity > 0),
    price_per_hour int  NOT NULL CHECK (price_per_hour >= 0),
    deposit        int  NOT NULL DEFAULT 0 CHECK (deposit >= 0),
    is_active      boolean NOT NULL DEFAULT true,
    UNIQUE (office_id, name)
);

CREATE TABLE clients (
    id         serial PRIMARY KEY,
    full_name  text NOT NULL CHECK (char_length(btrim(full_name)) BETWEEN 1 AND 100),
    phone      text NOT NULL DEFAULT '',
    email      text NOT NULL DEFAULT ''
               CHECK (email = '' OR email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'),
    company    text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE bookings (
    id          serial PRIMARY KEY,
    space_id    int  NOT NULL REFERENCES spaces (id) ON DELETE RESTRICT,
    client_id   int  NOT NULL REFERENCES clients (id) ON DELETE RESTRICT,
    book_date   date NOT NULL,
    rate        text NOT NULL DEFAULT 'hour'
                CHECK (rate IN ('hour', 'day', 'week', 'month')),
    start_hour  int  NOT NULL DEFAULT 8 CHECK (start_hour BETWEEN 8 AND 21),
    duration    int  NOT NULL,
    price       int  NOT NULL CHECK (price >= 0),
    deposit     int  NOT NULL DEFAULT 0 CHECK (deposit >= 0),
    status      text NOT NULL DEFAULT 'confirmed'
                CHECK (status IN ('confirmed', 'cancelled')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- Контроль значений: длительность согласована с тарифом.
    CHECK (
        (rate = 'hour'  AND duration BETWEEN 1 AND 8) OR
        (rate = 'day'   AND duration = 1) OR
        (rate = 'week'  AND duration = 7) OR
        (rate = 'month' AND duration = 1)
    )
);

-- Быстрый поиск пересечений броней одного пространства в один день.
CREATE INDEX bookings_space_date_idx ON bookings (space_id, book_date, start_hour);
CREATE INDEX bookings_date_status_idx ON bookings (book_date, status);
CREATE INDEX bookings_client_idx ON bookings (client_id);
