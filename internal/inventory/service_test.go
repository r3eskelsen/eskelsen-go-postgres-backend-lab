package inventory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		req   CreateRequest
		valid bool
	}{
		{"valid", "demo-key", CreateRequest{1, 1}, true},
		{"max-key", strings.Repeat("x", 128), CreateRequest{1, 1}, true},
		{"empty-key", "", CreateRequest{1, 1}, false},
		{"long-key", strings.Repeat("x", 129), CreateRequest{1, 1}, false},
		{"space", "bad key", CreateRequest{1, 1}, false},
		{"unicode", "caf\u00e9", CreateRequest{1, 1}, false},
		{"zero-product", "key", CreateRequest{0, 1}, false},
		{"negative-product", "key", CreateRequest{-1, 1}, false},
		{"zero-quantity", "key", CreateRequest{1, 0}, false},
		{"negative-quantity", "key", CreateRequest{1, -1}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(c.key, c.req)
			if c.valid && err != nil {
				t.Fatal(err)
			}
			if !c.valid && !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected invalid request, got %v", err)
			}
		})
	}
}

func TestHTTPValidation(t *testing.T) {
	// Nil DB proves these requests are rejected before database access.
	handler := NewHandler(&Service{})
	cases := []struct {
		name, method, path, body, key string
		status                        int
	}{
		{"health", "GET", "/healthz", "", "", 200},
		{"bad-product", "GET", "/products/no", "", "", 400},
		{"bad-reservation", "GET", "/reservations/no", "", "", 400},
		{"bad-cancel", "POST", "/reservations/no/cancel", "", "", 400},
		{"missing-key", "POST", "/reservations", `{}`, "", 400},
		{"malformed", "POST", "/reservations", `{`, "key", 400},
		{"unknown-field", "POST", "/reservations",
			`{"product_id":1,"quantity":1,"extra":true}`, "key", 400},
		{"trailing-json", "POST", "/reservations",
			`{"product_id":1,"quantity":1} {}`, "key", 400},
		{"zero-quantity", "POST", "/reservations",
			`{"product_id":1,"quantity":0}`, "key", 400},
		{"oversized", "POST", "/reservations",
			strings.Repeat(" ", 4097), "key", 413},
		{"wrong-method", "DELETE", "/reservations", "", "", 405},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
			if c.key != "" {
				req.Header.Set("Idempotency-Key", c.key)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != c.status {
				t.Fatalf("status=%d want=%d body=%s",
					rec.Code, c.status, rec.Body.String())
			}
		})
	}
}

func TestIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL required: integration tests must not silently skip")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid database configuration")
	}
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot create pool")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal("database unavailable")
	}
	s := &Service{DB: pool}
	runID, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	key := func(name string) string { return runID + "-" + name }
	product := func(name string, quantity int32) int64 {
		t.Helper()
		var id int64
		err := pool.QueryRow(ctx, `
INSERT INTO products(sku, initial_quantity, available_quantity)
VALUES ($1,$2,$2) RETURNING id`,
			key(name), quantity).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	available := func(id int64, want int32) {
		t.Helper()
		p, err := s.GetProduct(ctx, id)
		if err != nil || p.Available != want {
			t.Fatalf("available=%d want=%d err=%v", p.Available, want, err)
		}
	}

	t.Run("HTTPCreateReadCancelReplay", func(t *testing.T) {
		id := product("http", 5)
		server := httptest.NewServer(NewHandler(s))
		defer server.Close()
		client := &http.Client{Timeout: 10 * time.Second}
		create := func() []byte {
			t.Helper()
			req, err := http.NewRequestWithContext(ctx, "POST",
				server.URL+"/reservations",
				strings.NewReader(fmt.Sprintf(`{"product_id":%d,"quantity":2}`, id)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Idempotency-Key", key("http-create"))
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(resp.Body); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != 201 {
				t.Fatalf("status=%d body=%s", resp.StatusCode, buf.String())
			}
			return buf.Bytes()
		}
		first := create()
		second := create()
		if !bytes.Equal(first, second) {
			t.Fatal("HTTP replay differs")
		}
		available(id, 3)

		var reservationID string
		err := pool.QueryRow(ctx,
			`SELECT id::text FROM reservations WHERE product_id=$1`, id).Scan(&reservationID)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/products/" + fmt.Sprint(id), "/reservations/" + reservationID, "/readyz"} {
			resp, err := client.Get(server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("GET %s: %d", path, resp.StatusCode)
			}
		}
		for i := 0; i < 2; i++ {
			req, err := http.NewRequestWithContext(ctx, "POST",
				server.URL+"/reservations/"+reservationID+"/cancel", nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("cancel status=%d", resp.StatusCode)
			}
		}
		available(id, 5)
		if !bytes.Equal(first, create()) {
			t.Fatal("historical creation replay changed after cancellation")
		}
		available(id, 5)
	})

	t.Run("Overselling100Requests25Units", func(t *testing.T) {
		id := product("oversell", 25)
		start := make(chan struct{})
		results := make(chan error, 100)
		for i := 0; i < 100; i++ {
			go func(i int) {
				<-start
				_, err := s.Create(ctx, key(fmt.Sprintf("stock-%d", i)),
					CreateRequest{id, 1})
				results <- err
			}(i)
		}
		close(start)
		success, conflicts, unexpected := 0, 0, 0
		for i := 0; i < 100; i++ {
			err := <-results
			switch {
			case err == nil:
				success++
			case errors.Is(err, ErrStock):
				conflicts++
			default:
				unexpected++
			}
		}
		t.Logf("success=%d stock_conflicts=%d unexpected=%d",
			success, conflicts, unexpected)
		if success != 25 || conflicts != 75 || unexpected != 0 {
			t.Fatal("unexpected concurrent reservation results")
		}
		available(id, 0)
	})

	t.Run("ConcurrentIdempotencyAndCancellation", func(t *testing.T) {
		id := product("idempotency", 20)
		type result struct {
			body []byte
			err  error
		}
		results := make(chan result, 20)
		start := make(chan struct{})
		for i := 0; i < 20; i++ {
			go func() {
				<-start
				body, err := s.Create(ctx, key("same-key"), CreateRequest{id, 3})
				results <- result{body, err}
			}()
		}
		close(start)
		var first []byte
		for i := 0; i < 20; i++ {
			r := <-results
			if r.err != nil {
				t.Fatal(r.err)
			}
			if first == nil {
				first = r.body
			}
			if !bytes.Equal(first, r.body) {
				t.Fatal("replay mismatch")
			}
		}
		available(id, 17)
		var reservationID string
		var count int
		err := pool.QueryRow(ctx, `
SELECT count(*), min(id::text) FROM reservations WHERE product_id=$1`,
			id).Scan(&count, &reservationID)
		if err != nil || count != 1 {
			t.Fatalf("count=%d err=%v", count, err)
		}
		_, err = s.Create(ctx, key("same-key"), CreateRequest{id, 4})
		if !errors.Is(err, ErrKeyConflict) {
			t.Fatalf("conflict: %v", err)
		}

		cancellations := make(chan error, 20)
		cancelStart := make(chan struct{})
		for i := 0; i < 20; i++ {
			go func() {
				<-cancelStart
				r, err := s.Cancel(ctx, reservationID)
				if err == nil && r.Status != "cancelled" {
					err = errors.New("incorrect cancellation state")
				}
				cancellations <- err
			}()
		}
		close(cancelStart)
		for i := 0; i < 20; i++ {
			if err := <-cancellations; err != nil {
				t.Fatal(err)
			}
		}
		available(id, 20)
		t.Log("20 identical creates -> 1 reservation; 20 cancellations -> 1 stock return")
	})

	t.Run("RollbackAndRetry", func(t *testing.T) {
		id := product("rollback", 10)
		injected := errors.New("injected failure")
		_, err := s.create(ctx, key("rollback"), CreateRequest{id, 2},
			func() error { return injected })
		if !errors.Is(err, injected) {
			t.Fatalf("fault not observed: %v", err)
		}
		available(id, 10)
		var count int
		err = pool.QueryRow(ctx, `
SELECT count(*) FROM idempotency_requests WHERE request_key=$1`,
			key("rollback")).Scan(&count)
		if err != nil || count != 0 {
			t.Fatalf("partial key count=%d err=%v", count, err)
		}
		err = pool.QueryRow(ctx, `
SELECT count(*) FROM reservations WHERE product_id=$1`, id).Scan(&count)
		if err != nil || count != 0 {
			t.Fatalf("partial reservation count=%d err=%v", count, err)
		}
		if _, err := s.Create(ctx, key("rollback"), CreateRequest{id, 2}); err != nil {
			t.Fatal(err)
		}
		available(id, 8)
	})

	t.Run("BlockedRowDeadlineAndRetry", func(t *testing.T) {
		id := product("timeout", 10)
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer rollback(tx)
		var locked int64
		if err := tx.QueryRow(ctx,
			`SELECT id FROM products WHERE id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
			t.Fatal(err)
		}
		short, done := context.WithTimeout(ctx, 200*time.Millisecond)
		begin := time.Now()
		_, err = s.Create(short, key("timeout"), CreateRequest{id, 2})
		elapsed := time.Since(begin)
		expired := errors.Is(short.Err(), context.DeadlineExceeded)
		done()
		if err == nil || !expired || elapsed > 4*time.Second {
			t.Fatalf("deadline did not bound wait: elapsed=%s err=%v", elapsed, err)
		}
		rollback(tx)
		available(id, 10)
		if _, err := s.Create(ctx, key("timeout"), CreateRequest{id, 2}); err != nil {
			t.Fatal(err)
		}
		available(id, 8)
		t.Logf("blocked operation returned after %s", elapsed)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := s.Create(ctx, key("missing"), CreateRequest{9223372036854775807, 1})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("create: %v", err)
		}
		missingID, err := newID()
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Cancel(ctx, missingID)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("cancel: %v", err)
		}
	})

	t.Run("FinalSQLInvariants", func(t *testing.T) {
		var violations int
		err := pool.QueryRow(ctx, `
SELECT count(*) FROM (
SELECT p.id FROM products p
LEFT JOIN reservations r ON r.product_id=p.id
GROUP BY p.id
HAVING p.available_quantity +
COALESCE(SUM(r.quantity) FILTER (WHERE r.status='active'),0)
<> p.initial_quantity
) invalid`).Scan(&violations)
		if err != nil || violations != 0 {
			t.Fatalf("inventory violations=%d err=%v", violations, err)
		}
		err = pool.QueryRow(ctx, `
SELECT count(*) FROM idempotency_requests
WHERE response_status IS NULL`).Scan(&violations)
		if err != nil || violations != 0 {
			t.Fatalf("incomplete idempotency records=%d err=%v", violations, err)
		}
		t.Log("inventory conservation violations=0; incomplete idempotency records=0")
	})
}
