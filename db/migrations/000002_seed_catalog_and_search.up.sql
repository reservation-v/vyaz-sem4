-- Populate the initial coworking catalog once and support server-side booking search.
INSERT INTO spaces (name, type, capacity, price_per_hour, deposit)
VALUES
    ('Деск A1', 'desk', 1, 200, 0),
    ('Деск A2', 'desk', 1, 200, 0),
    ('Деск A3', 'desk', 1, 200, 0),
    ('Деск A4', 'desk', 1, 200, 0),
    ('Деск A5', 'desk', 1, 200, 0),
    ('Деск A6', 'desk', 1, 200, 0),
    ('Деск B1', 'desk', 1, 200, 0),
    ('Деск B2', 'desk', 1, 200, 0),
    ('Деск B3', 'desk', 1, 200, 0),
    ('Деск B4', 'desk', 1, 200, 0),
    ('Деск B5', 'desk', 1, 200, 0),
    ('Деск B6', 'desk', 1, 200, 0),
    ('Деск C1', 'desk', 1, 200, 0),
    ('Деск C2', 'desk', 1, 200, 0),
    ('Деск C3', 'desk', 1, 200, 0),
    ('Деск C4', 'desk', 1, 200, 0),
    ('Деск C5', 'desk', 1, 200, 0),
    ('Деск C6', 'desk', 1, 200, 0),
    ('Деск D1', 'desk', 1, 200, 0),
    ('Деск D2', 'desk', 1, 200, 0),
    ('Деск D3', 'desk', 1, 200, 0),
    ('Деск D4', 'desk', 1, 200, 0),
    ('Деск D5', 'desk', 1, 200, 0),
    ('Деск D6', 'desk', 1, 200, 0),
    ('Кабинет 2–3 №1', 'cabin', 3, 350, 0),
    ('Кабинет 2–3 №2', 'cabin', 3, 350, 0),
    ('Кабинет 2–3 №3', 'cabin', 3, 350, 0),
    ('Кабинет 4–6 №1', 'cabin', 6, 450, 0),
    ('Кабинет 4–6 №2', 'cabin', 6, 450, 0),
    ('Кабинет 4–6 №3', 'cabin', 6, 450, 0),
    ('Переговорка «Встреча»', 'room', 4, 700, 2000),
    ('Переговорка «Проект»', 'room', 6, 1000, 3000),
    ('Переговорка «Амфитеатр»', 'room', 12, 1800, 5000)
ON CONFLICT DO NOTHING;

INSERT INTO bookings (space_id, client_name, book_date, rate, start_hour, duration, price, deposit, status, created_at)
SELECT s.id, 'Мария Соколова', CURRENT_DATE + 1, 'hour', 10, 2, 400, 0, 'confirmed', now() - INTERVAL '1 hour'
FROM spaces s
WHERE s.name = 'Деск A2'
  AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.client_name = 'Мария Соколова' AND b.space_id = s.id AND b.book_date = CURRENT_DATE + 1);

INSERT INTO bookings (space_id, client_name, book_date, rate, start_hour, duration, price, deposit, status, created_at)
SELECT s.id, 'Студия «Волна»', CURRENT_DATE + 2, 'hour', 12, 2, 2000, 3000, 'confirmed', now() - INTERVAL '30 minutes'
FROM spaces s
WHERE s.name = 'Переговорка «Проект»'
  AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.client_name = 'Студия «Волна»' AND b.space_id = s.id AND b.book_date = CURRENT_DATE + 2);

CREATE INDEX IF NOT EXISTS bookings_date_status_idx ON bookings (book_date, status);
CREATE INDEX IF NOT EXISTS bookings_space_status_date_idx ON bookings (space_id, status, book_date, start_hour);
