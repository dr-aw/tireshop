package postgres

import (
	"context"
	"encoding/json/v2"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"tireshop/internal/requests"
)

var ErrVersion = errors.New("stale version")
var ErrStock = errors.New("insufficient stock")

type AdminItem struct {
	UnitLabel    string `json:"unit_label"`
	ProductID    string `json:"product_id"`
	Name         string `json:"name"`
	Quantity     int64  `json:"quantity"`
	UnitPriceRub int64  `json:"unit_price_rub"`
}
type AdminRequest struct {
	ID           string          `json:"id"`
	CustomerName string          `json:"customer_name"`
	Phone        string          `json:"phone"`
	Comment      string          `json:"comment"`
	Status       requests.Status `json:"status"`
	Version      int64           `json:"version"`
	TotalRub     int64           `json:"total_rub"`
	CreatedAt    time.Time       `json:"created_at"`
	Items        []AdminItem     `json:"items"`
}
type AdminEvent struct {
	Actor     string       `json:"actor"`
	CreatedAt time.Time    `json:"created_at"`
	Comment   string       `json:"comment"`
	Before    AdminRequest `json:"before"`
	After     AdminRequest `json:"after"`
}
type AdminDetail struct {
	Request AdminRequest `json:"request"`
	History []AdminEvent `json:"history"`
}
type AdminEdit struct {
	Version int64           `json:"version"`
	Status  requests.Status `json:"status"`
	Comment string          `json:"comment"`
	Items   []struct {
		ProductID string `json:"product_id"`
		Quantity  int64  `json:"quantity"`
	} `json:"items,omitzero"`
}

const adminColumns = `request_id::text,customer_name,phone,comment,status,version,total_rub,created_at`

func scanRequest(row pgx.Row) (AdminRequest, error) {
	var v AdminRequest
	err := row.Scan(&v.ID, &v.CustomerName, &v.Phone, &v.Comment, &v.Status, &v.Version, &v.TotalRub, &v.CreatedAt)
	v.Items = []AdminItem{}
	return v, err
}
func loadAdmin(ctx context.Context, tx pgx.Tx, id string, lock bool) (AdminRequest, error) {
	sql := `SELECT ` + adminColumns + ` FROM requests.requests WHERE request_id=$1::uuid`
	if lock {
		sql += ` FOR UPDATE`
	}
	v, err := scanRequest(tx.QueryRow(ctx, sql, id))
	if err != nil {
		return v, err
	}
	rows, err := tx.Query(ctx, `SELECT product_id::text,name,quantity,unit_price_rub,unit_label FROM requests.request_items WHERE request_id=$1::uuid ORDER BY product_id`, id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var item AdminItem
		if err := rows.Scan(&item.ProductID, &item.Name, &item.Quantity, &item.UnitPriceRub, &item.UnitLabel); err != nil {
			return v, err
		}
		v.Items = append(v.Items, item)
	}
	return v, rows.Err()
}
func (s *Store) AdminList(ctx context.Context, offset int) ([]AdminRequest, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+adminColumns+` FROM requests.requests ORDER BY created_at DESC,request_id DESC LIMIT 50 OFFSET $1`, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminRequest{}
	for rows.Next() {
		v, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) AdminGet(ctx context.Context, id string) (AdminDetail, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return AdminDetail{}, err
	}
	defer tx.Rollback(ctx)
	v, err := loadAdmin(ctx, tx, id, false)
	if err != nil {
		return AdminDetail{}, err
	}
	out := AdminDetail{Request: v, History: []AdminEvent{}}
	rows, err := tx.Query(ctx, `SELECT actor,created_at,comment,before_snapshot,after_snapshot FROM requests.request_history WHERE request_id=$1::uuid ORDER BY event_id DESC LIMIT 100`, id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var event AdminEvent
		var before, after []byte
		if err := rows.Scan(&event.Actor, &event.CreatedAt, &event.Comment, &before, &after); err != nil {
			return out, err
		}
		if err := json.Unmarshal(before, &event.Before); err != nil {
			return out, err
		}
		if err := json.Unmarshal(after, &event.After); err != nil {
			return out, err
		}
		out.History = append(out.History, event)
	}
	return out, rows.Err()
}

// Edit holds the request lock while invoking the catalog-owned command. Both
// services' changes share the same commit, including version and audit history.
func (s *Store) AdminEdit(ctx context.Context, id, actor string, in AdminEdit) (AdminRequest, error) {
	in.Comment = strings.TrimSpace(in.Comment)
	if in.Version < 1 || utf8.RuneCountInString(in.Comment) > 2000 || len(in.Items) > 100 {
		return AdminRequest{}, ErrInvalidSubmission
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AdminRequest{}, err
	}
	defer tx.Rollback(ctx)
	before, err := loadAdmin(ctx, tx, id, true)
	if err != nil {
		return before, err
	}
	if before.Version != in.Version {
		return before, ErrVersion
	}
	if _, err := requests.ChangeStatus(before.Status, in.Status, in.Comment); err != nil {
		return before, ErrInvalidSubmission
	}
	after := before
	if in.Items != nil {
		if before.Status != requests.Created || len(in.Items) == 0 {
			return before, ErrInvalidSubmission
		}
		old := map[string]AdminItem{}
		for _, item := range before.Items {
			old[item.ProductID] = item
		}
		items := make([]AdminItem, 0, len(in.Items))
		seen := map[string]bool{}
		var total int64
		for _, input := range in.Items {
			id := strings.ToLower(input.ProductID)
			if !uuidPattern.MatchString(id) || seen[id] || input.Quantity < 1 || input.Quantity > 10000 {
				return before, ErrInvalidSubmission
			}
			seen[id] = true
			item, exists := old[id]
			if !exists {
				item.ProductID = id
				err = tx.QueryRow(ctx, `SELECT name,unit_price_rub,unit_label FROM catalog.request_products_v1 WHERE product_id=$1::uuid AND is_available`, id).Scan(&item.Name, &item.UnitPriceRub, &item.UnitLabel)
				if errors.Is(err, pgx.ErrNoRows) {
					return before, ErrInvalidSubmission
				}
				if err != nil {
					return before, err
				}
			}
			item.Quantity = input.Quantity
			if item.UnitPriceRub > requests.MaxAmount/item.Quantity || total > requests.MaxAmount-item.UnitPriceRub*item.Quantity {
				return before, ErrInvalidSubmission
			}
			total += item.UnitPriceRub * item.Quantity
			items = append(items, item)
		}
		after.Items = items
		after.TotalRub = total
		if _, err = tx.Exec(ctx, `DELETE FROM requests.request_items WHERE request_id=$1::uuid`, id); err != nil {
			return before, err
		}
		for _, item := range items {
			if _, err = tx.Exec(ctx, `INSERT INTO requests.request_items(request_id,product_id,name,quantity,unit_price_rub,total_rub,unit_label) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7)`, id, item.ProductID, item.Name, item.Quantity, item.UnitPriceRub, item.Quantity*item.UnitPriceRub, item.UnitLabel); err != nil {
				return before, err
			}
		}
	}
	if before.Status == requests.Created && in.Status == requests.Delivery {
		ids := []string{}
		quantities := []int64{}
		for _, item := range after.Items {
			ids = append(ids, item.ProductID)
			quantities = append(quantities, item.Quantity)
		}
		if _, err = tx.Exec(ctx, `SELECT catalog.dispatch_v1($1::uuid[],$2::bigint[])`, ids, quantities); err != nil {
			var pgerr *pgconn.PgError
			if errors.As(err, &pgerr) && pgerr.Code == "P0001" {
				return before, ErrStock
			}
			return before, err
		}
	}
	after.Status = in.Status
	after.Version++
	if _, err = tx.Exec(ctx, `UPDATE requests.requests SET status=$2,total_rub=$3,version=version+1 WHERE request_id=$1::uuid`, id, after.Status, after.TotalRub); err != nil {
		return before, err
	}
	b, err := json.Marshal(before)
	if err != nil {
		return before, err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return before, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO requests.request_history(request_id,actor,comment,before_snapshot,after_snapshot) VALUES($1::uuid,$2,$3,$4::jsonb,$5::jsonb)`, id, actor, in.Comment, b, a); err != nil {
		return before, err
	}
	if err = tx.Commit(ctx); err != nil {
		return before, err
	}
	return after, nil
}
