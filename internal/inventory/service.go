package inventory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid     = errors.New("invalid request")
	ErrNotFound    = errors.New("resource not found")
	ErrStock       = errors.New("insufficient stock")
	ErrKeyConflict = errors.New("idempotency key conflict")
)

type CreateRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int32 `json:"quantity"`
}

type Reservation struct {
	ID          string     `json:"id"`
	ProductID   int64      `json:"product_id"`
	Quantity    int32      `json:"quantity"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	CancelledAt *time.Time `json:"cancelled_at"`
}

type Product struct {
	ID        int64  `json:"id"`
	SKU       string `json:"sku"`
	Initial   int32  `json:"initial_quantity"`
	Available int32  `json:"available_quantity"`
}

type Service struct {
	DB *pgxpool.Pool
}

func Validate(key string, req CreateRequest) error {
	if len(key) == 0 || len(key) > 128 ||
		req.ProductID <= 0 || req.Quantity <= 0 {
		return ErrInvalid
	}
	// ASCII keys avoid character-count differences between Go and SQL.
	for i := range key {
		if key[i] < 33 || key[i] > 126 {
			return ErrInvalid
		}
	}
	return nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// rollback uses an independent bounded context when the request has expired.
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

const reservationColumns = `id::text, product_id, quantity, status, created_at, cancelled_at`

type scanner interface {
	Scan(...any) error
}

func scanReservation(row scanner) (Reservation, error) {
	var r Reservation
	err := row.Scan(
		&r.ID, &r.ProductID, &r.Quantity, &r.Status,
		&r.CreatedAt, &r.CancelledAt,
	)
	return r, err
}

// Create stores only successful operations for replay.
// A commit error may be ambiguous: callers should retry using the same key.
func (s *Service) Create(
	ctx context.Context, key string, req CreateRequest,
) (json.RawMessage, error) {
	return s.create(ctx, key, req, nil)
}

// afterDecrement is an internal transaction fault-injection seam for tests.
// It is never exposed through the HTTP API.
func (s *Service) create(
	ctx context.Context,
	key string,
	req CreateRequest,
	afterDecrement func() error,
) (json.RawMessage, error) {
	if err := Validate(key, req); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	fingerprint := sha256.Sum256(canonical)

	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return nil, err
	}
	defer rollback(tx)

	tag, err := tx.Exec(ctx, `
INSERT INTO idempotency_requests
(operation, request_key, request_fingerprint)
VALUES ('create_reservation', $1, $2)
ON CONFLICT (operation, request_key) DO NOTHING`,
		key, fingerprint[:])
	if err != nil {
		return nil, err
	}

	if tag.RowsAffected() == 0 {
		var same bool
		var status *int32
		var body []byte
		err = tx.QueryRow(ctx, `
SELECT request_fingerprint = $2, response_status, response_body
FROM idempotency_requests
WHERE operation = 'create_reservation' AND request_key = $1`,
			key, fingerprint[:]).Scan(&same, &status, &body)
		if err != nil {
			return nil, err
		}
		if !same {
			return nil, ErrKeyConflict
		}
		if status == nil || *status != 201 || len(body) == 0 {
			return nil, errors.New("incomplete idempotency record")
		}
		// Read-only replay; deferred rollback releases this transaction.
		return json.RawMessage(body), nil
	}

	var available int32
	err = tx.QueryRow(ctx, `
SELECT available_quantity FROM products WHERE id = $1 FOR UPDATE`,
		req.ProductID).Scan(&available)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if available < req.Quantity {
		return nil, ErrStock
	}

	_, err = tx.Exec(ctx, `
UPDATE products
SET available_quantity = available_quantity - $2
WHERE id = $1`, req.ProductID, req.Quantity)
	if err != nil {
		return nil, err
	}
	if afterDecrement != nil {
		if err = afterDecrement(); err != nil {
			return nil, err
		}
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	r, err := scanReservation(tx.QueryRow(ctx, `
INSERT INTO reservations (id, product_id, quantity)
VALUES ($1, $2, $3)
RETURNING `+reservationColumns, id, req.ProductID, req.Quantity))
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	// Return the same JSONB representation on initial and replay responses.
	var stored []byte
	err = tx.QueryRow(ctx, `
UPDATE idempotency_requests
SET response_status = 201, response_body = $2::jsonb,
    reservation_id = $3
WHERE operation = 'create_reservation' AND request_key = $1
RETURNING response_body`, key, string(body), id).Scan(&stored)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return json.RawMessage(stored), nil
}

func (s *Service) Cancel(ctx context.Context, id string) (Reservation, error) {
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return Reservation{}, err
	}
	defer rollback(tx)

	r, err := scanReservation(tx.QueryRow(ctx,
		`SELECT `+reservationColumns+`
 FROM reservations WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, ErrNotFound
	}
	if err != nil {
		return Reservation{}, err
	}
	if r.Status == "cancelled" {
		return r, nil
	}

	var productID int64
	err = tx.QueryRow(ctx,
		`SELECT id FROM products WHERE id = $1 FOR UPDATE`,
		r.ProductID).Scan(&productID)
	if err != nil {
		return Reservation{}, err
	}
	_, err = tx.Exec(ctx, `
UPDATE products
SET available_quantity = available_quantity + $2
WHERE id = $1`, r.ProductID, r.Quantity)
	if err != nil {
		return Reservation{}, err
	}
	r, err = scanReservation(tx.QueryRow(ctx, `
UPDATE reservations
SET status = 'cancelled', cancelled_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING `+reservationColumns, id))
	if err != nil {
		return Reservation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Reservation{}, err
	}
	return r, nil
}

func (s *Service) GetReservation(
	ctx context.Context, id string,
) (Reservation, error) {
	r, err := scanReservation(s.DB.QueryRow(ctx,
		`SELECT `+reservationColumns+` FROM reservations WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, ErrNotFound
	}
	return r, err
}

func (s *Service) GetProduct(ctx context.Context, id int64) (Product, error) {
	var p Product
	err := s.DB.QueryRow(ctx, `
SELECT id, sku, initial_quantity, available_quantity
FROM products WHERE id = $1`, id).Scan(
		&p.ID, &p.SKU, &p.Initial, &p.Available,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	return p, err
}
