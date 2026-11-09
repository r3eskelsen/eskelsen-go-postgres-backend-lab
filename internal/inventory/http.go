package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

var uuidPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, ErrInvalid):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, ErrStock):
		status, code = http.StatusConflict, "insufficient_stock"
	case errors.Is(err, ErrKeyConflict):
		status, code = http.StatusConflict, "idempotency_conflict"
	case errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, context.Canceled):
		status, code = http.StatusServiceUnavailable, "operation_timeout"
	}
	writeJSON(w, status, map[string]string{"error": code})
}

func bounded(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		fn(w, r.WithContext(ctx))
	}
}

func NewHandler(s *Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := s.DB.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable,
				map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	mux.HandleFunc("GET /products/{id}", bounded(
		func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil || id <= 0 {
				writeError(w, ErrInvalid)
				return
			}
			p, err := s.GetProduct(r.Context(), id)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, p)
		},
	))

	mux.HandleFunc("POST /reservations", bounded(
		func(w http.ResponseWriter, r *http.Request) {
			keys := r.Header.Values("Idempotency-Key")
			if len(keys) != 1 {
				writeError(w, ErrInvalid)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			defer r.Body.Close()
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			var req CreateRequest
			if err := decoder.Decode(&req); err != nil {
				writeDecodeError(w, err)
				return
			}
			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				writeDecodeError(w, err)
				return
			}
			body, err := s.Create(r.Context(), keys[0], req)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, body)
		},
	))

	mux.HandleFunc("GET /reservations/{id}", bounded(
		func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if !uuidPattern.MatchString(id) {
				writeError(w, ErrInvalid)
				return
			}
			reservation, err := s.GetReservation(r.Context(), id)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, reservation)
		},
	))

	mux.HandleFunc("POST /reservations/{id}/cancel", bounded(
		func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if !uuidPattern.MatchString(id) {
				writeError(w, ErrInvalid)
				return
			}
			reservation, err := s.Cancel(r.Context(), id)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, reservation)
		},
	))
	return mux
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge,
			map[string]string{"error": "request_too_large"})
		return
	}
	writeError(w, ErrInvalid)
}
