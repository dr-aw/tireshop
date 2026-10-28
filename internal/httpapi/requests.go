package httpapi

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"tireshop/internal/requests"
	"tireshop/internal/requests/postgres"
)

type itemInput struct {
	ProductID            string `json:"product_id"`
	Quantity             *int64 `json:"quantity"`
	ExpectedUnitPriceRub *int64 `json:"expected_unit_price_rub"`
}
type submissionInput struct {
	CustomerName string      `json:"customer_name"`
	Phone        string      `json:"phone"`
	Comment      string      `json:"comment"`
	Items        []itemInput `json:"items"`
}

// Requests keeps public submissions separate from session-protected admin APIs.
func Requests(pool *pgxpool.Pool, allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()
	health(mux, pool)
	admin := adminGate{pool: pool, origins: allowedOrigins}
	admin.routes(mux)
	admin.requestRoutes(mux)
	store := postgres.New(pool)
	limits := &limiter{clients: make(map[string]bucket)}
	inflight := make(chan struct{}, 8)
	mux.HandleFunc("POST /api/requests", func(w http.ResponseWriter, r *http.Request) {
		// Browsers must use our same-origin frontend. Non-browser API clients may
		// omit Origin; JSON and the idempotency key are still required.
		if origin := r.Header.Get("Origin"); origin != "" {
			allowed := false
			for _, candidate := range allowedOrigins {
				allowed = allowed || origin == candidate
			}
			if !allowed {
				fail(w, 403, "origin_denied", "Источник запроса не разрешён")
				return
			}
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			fail(w, 415, "unsupported_media_type", "Ожидается JSON")
			return
		}
		if !limits.allow(r.RemoteAddr) {
			w.Header().Set("Retry-After", "1")
			fail(w, 429, "rate_limited", "Слишком много запросов. Повторите чуть позже")
			return
		}
		select {
		case inflight <- struct{}{}:
			defer func() { <-inflight }()
		default:
			w.Header().Set("Retry-After", "1")
			fail(w, 503, "unavailable", "Сервис занят. Повторите отправку чуть позже")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				fail(w, 413, "body_too_large", "Заявка слишком большая")
			} else {
				fail(w, 400, "invalid_json", "Не удалось прочитать заявку")
			}
			return
		}
		var input submissionInput
		// v2 rejects duplicate JSON members; unknown fields are also rejected so
		// misspelled prices cannot silently become zero. Pointers detect omissions.
		if json.Unmarshal(body, &input, json.RejectUnknownMembers(true)) != nil {
			fail(w, 400, "invalid_json", "Проверьте формат заявки")
			return
		}
		submission := postgres.Submission{CustomerName: input.CustomerName, Phone: input.Phone, Comment: input.Comment}
		for _, item := range input.Items {
			if item.Quantity == nil || item.ExpectedUnitPriceRub == nil {
				fail(w, 422, "validation_error", "Укажите количество и цену товара")
				return
			}
			submission.Items = append(submission.Items, requests.RequestedItem{ProductID: item.ProductID, Quantity: *item.Quantity, ExpectedUnitPriceRub: *item.ExpectedUnitPriceRub})
		}
		// Start the DB budget before pool acquisition. Queueing, SQL and COMMIT
		// share this deadline; a timeout never proves that a commit did not happen.
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		result, err := store.Create(ctx, r.Header.Get("Idempotency-Key"), submission)
		switch {
		case errors.Is(err, postgres.ErrInvalidSubmission):
			fail(w, 422, "validation_error", "Проверьте контакты, количество и ключ отправки")
			return
		case errors.Is(err, postgres.ErrIdempotencyConflict):
			fail(w, 409, "idempotency_conflict", "Эта попытка уже использована для других данных")
			return
		case err != nil:
			// Do not expose SQL errors or log contact data. The frontend retains
			// the exact attempt and retries with its existing key after recovery.
			w.Header().Set("Retry-After", "1")
			fail(w, 503, "unavailable", "Не удалось подтвердить приём. Повторите ту же отправку")
			return
		}
		switch result.Code {
		case "created":
			writeJSON(w, 201, map[string]string{"request_id": result.RequestID, "message": "Заявка получена"})
		case "price_changed":
			type priceItem struct {
				ProductID    string `json:"product_id"`
				Name         string `json:"name"`
				Quantity     int64  `json:"quantity"`
				UnitPriceRub int64  `json:"unit_price_rub"`
			}
			items := make([]priceItem, 0, len(result.Quote.Items))
			for _, item := range result.Quote.Items {
				items = append(items, priceItem{item.ProductID, item.Name, item.Quantity, item.UnitPriceRub})
			}
			writeJSON(w, 409, struct {
				Error    apiError    `json:"error"`
				Items    []priceItem `json:"items"`
				TotalRub int64       `json:"total_rub"`
			}{apiError{"price_changed", "Цена изменилась. Подтвердите новую сумму"}, items, result.Quote.TotalRub})
		case "product_unavailable":
			fail(w, 409, "product_unavailable", "Один из товаров больше недоступен. Измените состав заявки")
		case "invalid_items":
			fail(w, 422, "validation_error", "Проверьте товары и сумму заявки")
		default:
			fail(w, 503, "unavailable", "Не удалось подтвердить приём заявки")
		}
	})
	return secure(mux)
}
