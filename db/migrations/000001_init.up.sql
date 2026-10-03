-- Минимальная схема БД сервиса бронирования (Лаба 1: проект; Лаба 3: подключение).
-- Две таблицы: каталог пространств и брони. Без платежей и клиентов-аккаунтов —
-- для рабочего варианта достаточно имени клиента в брони.

CREATE TABLE spaces (
    id             serial PRIMARY KEY,
    name           text NOT NULL,
    type           text NOT NULL CHECK (type IN ('desk', 'cabin', 'room')),
    capacity       int  NOT NULL CHECK (capacity > 0),
    price_per_hour int  NOT NULL CHECK (price_per_hour >= 0),
    deposit        int  NOT NULL DEFAULT 0 CHECK (deposit >= 0)
);

CREATE TABLE bookings (
    id          serial PRIMARY KEY,
    space_id    int  NOT NULL REFERENCES spaces (id),
    client_name text NOT NULL,
    book_date   date NOT NULL,
    rate        text NOT NULL DEFAULT 'hour'
                CHECK (rate IN ('hour', 'day', 'week', 'month')),
    start_hour  int  NOT NULL DEFAULT 0 CHECK (start_hour BETWEEN 0 AND 21),
    duration    int  NOT NULL CHECK (duration BETWEEN 1 AND 8),
    price       int  NOT NULL CHECK (price >= 0),
    deposit     int  NOT NULL DEFAULT 0 CHECK (deposit >= 0),
    status      text NOT NULL DEFAULT 'confirmed'
                CHECK (status IN ('confirmed', 'cancelled')),
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Быстрый поиск пересечений броней одного пространства в один день.
CREATE INDEX bookings_space_date_idx ON bookings (space_id, book_date, start_hour);