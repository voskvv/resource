BEGIN TRANSACTION;
ALTER TABLE volumes
    DROP COLUMN is_active;
COMMIT TRANSACTION;