DROP INDEX IF EXISTS bookings_space_status_date_idx;
DROP INDEX IF EXISTS bookings_date_status_idx;

DELETE FROM bookings
WHERE client_name IN ('Мария Соколова', 'Студия «Волна»');

DELETE FROM spaces
WHERE name IN (
    'Деск A1', 'Деск A2', 'Деск A3', 'Деск A4', 'Деск A5', 'Деск A6',
    'Деск B1', 'Деск B2', 'Деск B3', 'Деск B4', 'Деск B5', 'Деск B6',
    'Деск C1', 'Деск C2', 'Деск C3', 'Деск C4', 'Деск C5', 'Деск C6',
    'Деск D1', 'Деск D2', 'Деск D3', 'Деск D4', 'Деск D5', 'Деск D6',
    'Кабинет 2–3 №1', 'Кабинет 2–3 №2', 'Кабинет 2–3 №3',
    'Кабинет 4–6 №1', 'Кабинет 4–6 №2', 'Кабинет 4–6 №3',
    'Переговорка «Встреча»', 'Переговорка «Проект»', 'Переговорка «Амфитеатр»'
)
AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.space_id = spaces.id);
