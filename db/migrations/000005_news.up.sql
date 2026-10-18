-- Лаба 4: новости коворкинга и подключённые RSS-ленты.
-- Свои новости хранятся в news, список внешних источников - в news_sources.
-- Схема лабы 3 (offices, space_types, spaces, clients, bookings) не меняется.

CREATE TABLE news (
    id           serial PRIMARY KEY,
    title        text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    summary      text NOT NULL DEFAULT '' CHECK (char_length(summary) <= 2000),
    published_at timestamptz NOT NULL DEFAULT now(),
    is_published boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- Лента всегда выбирает опубликованные новости по убыванию даты.
CREATE INDEX news_published_idx ON news (is_published, published_at DESC);

CREATE TABLE news_sources (
    id         serial PRIMARY KEY,
    title      text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 100),
    url        text NOT NULL UNIQUE CHECK (url ~ '^https?://'),
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Источник по умолчанию: тег «коворкинг» на vc.ru.
INSERT INTO news_sources (title, url)
VALUES ('vc.ru · коворкинг', 'https://vc.ru/rss/tag/coworking');

-- Свои новости, которые раньше были зашиты в шаблон news.html.
INSERT INTO news (title, summary, published_at)
VALUES
    (
        'Скидка 20 % на первый месяц',
        'Новые резиденты получают скидку на первый месяц аренды деск-места или кабинета. Акция действует до конца октября.',
        '2026-10-01 10:00:00+03'
    ),
    (
        'Митапы по четвергам',
        'Каждый четверг в 19:00 - открытые встречи резидентов в лаунже: короткие доклады, вопросы и нетворкинг с чаем.',
        '2026-09-15 19:00:00+03'
    ),
    (
        'Место открылось',
        '31-й этаж башни «Око» принял первых резидентов: 24 стола, 6 кабинетов и 3 переговорки с видом на Москва-Сити.',
        '2026-09-01 10:00:00+03'
    );
