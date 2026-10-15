-- Возврат к сохранению истории: подтверждённая бронь снова запрещает удаление
-- клиента (внешний ключ ON DELETE RESTRICT).

ALTER TABLE bookings
    DROP CONSTRAINT bookings_client_id_fkey,
    ADD CONSTRAINT bookings_client_id_fkey
        FOREIGN KEY (client_id) REFERENCES clients (id) ON DELETE RESTRICT;
