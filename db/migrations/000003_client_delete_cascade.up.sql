-- Удаление клиента вместе с его бронями.
-- Требование преподавателя: запись клиента удаляется без условий, поэтому брони
-- уходят вместе с ним. Внешний ключ переводится в ON DELETE CASCADE, чтобы БД
-- сама поддерживала целостность и удаление не требовало предварительной отмены.

ALTER TABLE bookings
    DROP CONSTRAINT bookings_client_id_fkey,
    ADD CONSTRAINT bookings_client_id_fkey
        FOREIGN KEY (client_id) REFERENCES clients (id) ON DELETE CASCADE;
