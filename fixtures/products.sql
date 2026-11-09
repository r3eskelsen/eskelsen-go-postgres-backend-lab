-- Synthetic fixtures for a freshly migrated, empty demo database.
-- Deliberately fails on duplicate SKUs rather than resetting inventory.
BEGIN;
INSERT INTO products (sku, initial_quantity, available_quantity)
VALUES
    ('DEMO-NOTEBOOK', 25, 25),
    ('DEMO-PENCIL', 100, 100),
    ('DEMO-EMPTY', 0, 0);
COMMIT;
