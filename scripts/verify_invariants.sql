\set ON_ERROR_STOP on

-- A nonzero result is an invariant violation.
SELECT
    p.id,
    p.sku,
    p.initial_quantity,
    p.available_quantity,
    COALESCE(SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0)
        AS active_reserved_quantity
FROM products p
LEFT JOIN reservations r ON r.product_id = p.id
GROUP BY p.id
HAVING
    p.available_quantity
    + COALESCE(SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0)
    <> p.initial_quantity;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM products p
        LEFT JOIN reservations r ON r.product_id = p.id
        GROUP BY p.id
        HAVING p.available_quantity
            + COALESCE(
                SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0
              )
            <> p.initial_quantity
    ) THEN
        RAISE EXCEPTION 'Inventory conservation invariant violated';
    END IF;

    IF EXISTS (
        SELECT 1 FROM idempotency_requests
        WHERE response_status IS NULL
    ) THEN
        RAISE EXCEPTION 'Incomplete committed idempotency record';
    END IF;
END $$;
