BEGIN;

CREATE TABLE products (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sku text NOT NULL UNIQUE,
    initial_quantity integer NOT NULL,
    available_quantity integer NOT NULL,
    CONSTRAINT products_sku_valid
        CHECK (length(sku) BETWEEN 1 AND 80),
    CONSTRAINT products_initial_nonnegative
        CHECK (initial_quantity >= 0),
    CONSTRAINT products_available_valid
        CHECK (
            available_quantity >= 0
            AND available_quantity <= initial_quantity
        )
);

CREATE TABLE reservations (
    id uuid PRIMARY KEY,
    product_id bigint NOT NULL REFERENCES products(id),
    quantity integer NOT NULL CHECK (quantity > 0),
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'cancelled')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cancelled_at timestamptz,
    CONSTRAINT reservations_cancelled_timestamp_valid
        CHECK (
            (status = 'active' AND cancelled_at IS NULL)
            OR
            (status = 'cancelled' AND cancelled_at IS NOT NULL)
        )
);

CREATE INDEX reservations_active_product_idx
    ON reservations(product_id)
    WHERE status = 'active';

CREATE TABLE idempotency_requests (
    operation text NOT NULL,
    request_key text NOT NULL,
    request_fingerprint bytea NOT NULL,
    response_status integer,
    response_body jsonb,
    reservation_id uuid REFERENCES reservations(id),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (operation, request_key),
    CONSTRAINT idempotency_operation_valid
        CHECK (operation = 'create_reservation'),
    CONSTRAINT idempotency_key_valid
        CHECK (length(request_key) BETWEEN 1 AND 128),
    CONSTRAINT idempotency_fingerprint_valid
        CHECK (octet_length(request_fingerprint) = 32),
    CONSTRAINT idempotency_response_complete
        CHECK (
            (
                response_status IS NULL
                AND response_body IS NULL
                AND reservation_id IS NULL
            )
            OR
            (
                response_status IS NOT NULL
                AND response_status = 201
                AND response_body IS NOT NULL
                AND reservation_id IS NOT NULL
            )
        )
);

COMMIT;
