-- Начальные данные коворкинга «Место»: одна локация, классификатор типов,
-- каталог пространств, несколько клиентов и демонстрационные брони.

INSERT INTO offices (name, city, address, floor, phone, email)
VALUES ('Место на Пресненской', 'Москва', 'Пресненская наб., 12, башня «Око»', 31, '+7 495 123-45-67', 'hello@mesto.ru');

INSERT INTO space_types (code, title, description)
VALUES
    ('desk',  'Деск-место', 'Рабочее место в открытой зоне с почасовой оплатой'),
    ('cabin', 'Кабинет',    'Отдельный кабинет для команды на несколько человек'),
    ('room',  'Переговорка', 'Комната для встреч и созвонов, бронируется с депозитом');

INSERT INTO spaces (office_id, space_type_id, name, capacity, price_per_hour, deposit)
SELECT o.id, t.id, v.name, v.capacity, v.price_per_hour, v.deposit
FROM (VALUES
    ('desk',  'Деск A1', 1, 200, 0),
    ('desk',  'Деск A2', 1, 200, 0),
    ('desk',  'Деск A3', 1, 200, 0),
    ('desk',  'Деск A4', 1, 200, 0),
    ('desk',  'Деск A5', 1, 200, 0),
    ('desk',  'Деск A6', 1, 200, 0),
    ('desk',  'Деск B1', 1, 200, 0),
    ('desk',  'Деск B2', 1, 200, 0),
    ('desk',  'Деск B3', 1, 200, 0),
    ('desk',  'Деск B4', 1, 200, 0),
    ('desk',  'Деск B5', 1, 200, 0),
    ('desk',  'Деск B6', 1, 200, 0),
    ('desk',  'Деск C1', 1, 200, 0),
    ('desk',  'Деск C2', 1, 200, 0),
    ('desk',  'Деск C3', 1, 200, 0),
    ('desk',  'Деск C4', 1, 200, 0),
    ('desk',  'Деск C5', 1, 200, 0),
    ('desk',  'Деск C6', 1, 200, 0),
    ('desk',  'Деск D1', 1, 200, 0),
    ('desk',  'Деск D2', 1, 200, 0),
    ('desk',  'Деск D3', 1, 200, 0),
    ('desk',  'Деск D4', 1, 200, 0),
    ('desk',  'Деск D5', 1, 200, 0),
    ('desk',  'Деск D6', 1, 200, 0),
    ('cabin', 'Кабинет 2–3 №1', 3, 350, 0),
    ('cabin', 'Кабинет 2–3 №2', 3, 350, 0),
    ('cabin', 'Кабинет 2–3 №3', 3, 350, 0),
    ('cabin', 'Кабинет 4–6 №1', 6, 450, 0),
    ('cabin', 'Кабинет 4–6 №2', 6, 450, 0),
    ('cabin', 'Кабинет 4–6 №3', 6, 450, 0),
    ('room',  'Переговорка «Встреча»', 4, 700, 2000),
    ('room',  'Переговорка «Проект»', 6, 1000, 3000),
    ('room',  'Переговорка «Амфитеатр»', 12, 1800, 5000)
) AS v(type_code, name, capacity, price_per_hour, deposit)
JOIN offices o ON o.name = 'Место на Пресненской'
JOIN space_types t ON t.code = v.type_code
ORDER BY v.name
ON CONFLICT (office_id, name) DO NOTHING;

INSERT INTO clients (full_name, phone, email, company)
VALUES
    ('Мария Соколова', '+7 916 000-11-22', 'maria@example.com', ''),
    ('Студия «Волна»', '+7 495 000-33-44', 'hi@volna.studio', 'Студия «Волна»'),
    ('ООО «Прогресс»', '+7 495 000-55-66', 'office@progress.ru', 'ООО «Прогресс»'),
    ('Илья Морозов', '+7 903 000-77-88', '', '')
ON CONFLICT DO NOTHING;

INSERT INTO bookings (space_id, client_id, book_date, rate, start_hour, duration, price, deposit, status, created_at)
SELECT s.id, c.id, CURRENT_DATE + 1, 'hour', 10, 2, s.price_per_hour * 2, s.deposit, 'confirmed', now() - INTERVAL '1 hour'
FROM spaces s, clients c
WHERE s.name = 'Деск A2' AND c.full_name = 'Мария Соколова';

INSERT INTO bookings (space_id, client_id, book_date, rate, start_hour, duration, price, deposit, status, created_at)
SELECT s.id, c.id, CURRENT_DATE + 2, 'hour', 12, 2, s.price_per_hour * 2, s.deposit, 'confirmed', now() - INTERVAL '30 minutes'
FROM spaces s, clients c
WHERE s.name = 'Переговорка «Проект»' AND c.full_name = 'Студия «Волна»';

INSERT INTO bookings (space_id, client_id, book_date, rate, start_hour, duration, price, deposit, status, created_at)
SELECT s.id, c.id, CURRENT_DATE + 1, 'hour', 14, 1, s.price_per_hour, s.deposit, 'confirmed', now() - INTERVAL '15 minutes'
FROM spaces s, clients c
WHERE s.name = 'Переговорка «Встреча»' AND c.full_name = 'ООО «Прогресс»';
