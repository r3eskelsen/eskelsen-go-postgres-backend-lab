-- Destructive rollback for the isolated demo database only.
-- Never execute automatically.
BEGIN;
DROP TABLE idempotency_requests;
DROP TABLE reservations;
DROP TABLE products;
COMMIT;
