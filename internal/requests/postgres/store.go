// Package postgres persists requests using a pool restricted to the requests
// runtime role. It does not implement HTTP or employee authentication.
package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tireshop/internal/requests"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create commits the idempotency result and request together. A successful
// return may represent a business conflict; inspect Result.Code. Retrying an
// uncertain commit must use the same key and input.
func (s *Store) Create(ctx context.Context, key string, input Submission) (Result, error) {
	if !uuidPattern.MatchString(key) {
		return Result{}, ErrInvalidSubmission
	}
	input, fingerprint, err := normalize(input)
	if err != nil {
		return Result{}, err
	}
	key = strings.ToLower(key)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("begin request: %w", err)
	}
	defer func() {
		// Release the connection even if the request context has already expired.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	insert, err := tx.Exec(ctx, `
		INSERT INTO requests.request_submissions (idempotency_key, payload_hash)
		VALUES ($1::uuid, $2) ON CONFLICT (idempotency_key) DO NOTHING`, key, fingerprint[:])
	if err != nil {
		return Result{}, fmt.Errorf("claim submission key: %w", err)
	}
	if insert.RowsAffected() == 0 {
		// ON CONFLICT waits for the competing transaction. At READ COMMITTED this
		// next statement gets a new snapshot and sees its completed result.
		var storedHash, response []byte
		if err := tx.QueryRow(ctx, `SELECT payload_hash, response FROM requests.request_submissions
			WHERE idempotency_key = $1::uuid`, key).Scan(&storedHash, &response); err != nil {
			return Result{}, fmt.Errorf("read submission result: %w", err)
		}
		if !bytes.Equal(storedHash, fingerprint[:]) {
			return Result{}, ErrIdempotencyConflict
		}
		if len(response) == 0 {
			return Result{}, ErrIncompleteResult
		}
		var result Result
		if err := json.Unmarshal(response, &result); err != nil {
			return Result{}, fmt.Errorf("decode submission result: %w", err)
		}
		return result, nil
	}

	products, err := loadProducts(ctx, tx, input.Items)
	if err != nil {
		return Result{}, err
	}
	quote, quoteErr := requests.PrepareQuote(input.Items, products)
	result := Result{}
	switch {
	case quoteErr == nil:
		result.Code = "created"
		result.RequestID, err = insertRequest(ctx, tx, input, quote)
		if err != nil {
			return Result{}, err
		}
	case errors.Is(quoteErr, requests.ErrPriceChanged):
		result = Result{Code: "price_changed", Quote: &quote}
	case errors.Is(quoteErr, requests.ErrProductUnavailable):
		result.Code = "product_unavailable"
	case errors.Is(quoteErr, requests.ErrInvalidItems), errors.Is(quoteErr, requests.ErrAmountOverflow):
		result.Code = "invalid_items"
	default:
		return Result{}, quoteErr
	}

	response, err := json.Marshal(result)
	if err != nil {
		return Result{}, err
	}
	var requestID any
	if result.RequestID != "" {
		requestID = result.RequestID
	}
	if _, err = tx.Exec(ctx, `UPDATE requests.request_submissions
		SET response = $2::jsonb, request_id = $3::uuid WHERE idempotency_key = $1::uuid`,
		key, response, requestID); err != nil {
		return Result{}, fmt.Errorf("complete submission: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit request: %w", err)
	}
	return result, nil
}

func loadProducts(ctx context.Context, tx pgx.Tx, items []requests.RequestedItem) (map[string]requests.Product, error) {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ProductID
	}
	rows, err := tx.Query(ctx, `SELECT product_id::text, name, unit_price_rub, is_available
		FROM catalog.request_products_v1 WHERE product_id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("read catalog contract: %w", err)
	}
	defer rows.Close()
	products := make(map[string]requests.Product, len(items))
	for rows.Next() {
		var id string
		var product requests.Product
		if err := rows.Scan(&id, &product.Name, &product.UnitPriceRub, &product.Available); err != nil {
			return nil, err
		}
		products[id] = product
	}
	return products, rows.Err()
}

func insertRequest(ctx context.Context, tx pgx.Tx, input Submission, quote requests.Quote) (string, error) {
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO requests.requests (customer_name, phone, comment, total_rub)
		VALUES ($1, $2, $3, $4) RETURNING request_id::text`,
		input.CustomerName, input.Phone, input.Comment, quote.TotalRub).Scan(&id); err != nil {
		return "", fmt.Errorf("insert request: %w", err)
	}
	for _, item := range quote.Items {
		if _, err := tx.Exec(ctx, `INSERT INTO requests.request_items
			(request_id, product_id, name, quantity, unit_price_rub, total_rub)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`,
			id, item.ProductID, item.Name, item.Quantity, item.UnitPriceRub, item.TotalRub); err != nil {
			return "", fmt.Errorf("insert request item: %w", err)
		}
	}
	return id, nil
}
