package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"tireshop/internal/requests/postgres"
)

func pageOffset(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("offset")
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 100000 {
		fail(w, 400, "validation_error", "Некорректная страница")
		return 0, false
	}
	return n, true
}
func adminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		fail(w, 404, "not_found", "Запись не найдена")
	case errors.Is(err, postgres.ErrVersion):
		fail(w, 409, "version_conflict", "Запись уже изменена. Обновите данные перед новой правкой")
	case errors.Is(err, postgres.ErrStock):
		fail(w, 409, "insufficient_stock", "Недостаточно товара на складе. Статус и остатки не изменены")
	case errors.Is(err, postgres.ErrInvalidSubmission):
		fail(w, 422, "validation_error", "Проверьте состав, статус и комментарий отмены")
	default:
		fail(w, 503, "unavailable", "Результат операции не подтверждён. Обновите запись перед повтором")
	}
}
func (a adminGate) requestRoutes(mux *http.ServeMux) {
	store := postgres.New(a.pool)
	mux.HandleFunc("GET /api/admin/requests", a.protect(func(w http.ResponseWriter, r *http.Request, actor string) {
		offset, ok := pageOffset(w, r)
		if !ok {
			return
		}
		items, err := store.AdminList(r.Context(), offset)
		if err != nil {
			adminError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	}))
	mux.HandleFunc("GET /api/admin/requests/{id}", a.protect(func(w http.ResponseWriter, r *http.Request, actor string) {
		id := r.PathValue("id")
		if !uuid.MatchString(id) {
			fail(w, 400, "validation_error", "Некорректный номер заявки")
			return
		}
		v, err := store.AdminGet(r.Context(), id)
		if err != nil {
			adminError(w, err)
			return
		}
		writeJSON(w, 200, v)
	}))
	mux.HandleFunc("PUT /api/admin/requests/{id}", a.protect(func(w http.ResponseWriter, r *http.Request, actor string) {
		id := r.PathValue("id")
		if !uuid.MatchString(id) {
			fail(w, 400, "validation_error", "Некорректный номер заявки")
			return
		}
		var in postgres.AdminEdit
		if !adminBody(w, r, &in) {
			return
		}
		v, err := store.AdminEdit(r.Context(), id, actor, in)
		if err != nil {
			adminError(w, err)
			return
		}
		writeJSON(w, 200, v)
	}))
}
