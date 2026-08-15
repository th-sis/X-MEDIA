package store

import (
	"context"
	"database/sql"

	"xmedia/internal/domain"
)

type searchHistoryRepo struct{ db *DB }

func (r *searchHistoryRepo) Add(ctx context.Context, keyword string) error {
	_, err := r.db.write.ExecContext(ctx, `
		INSERT INTO search_history(keyword, searched_at) VALUES(?, CURRENT_TIMESTAMP)
		ON CONFLICT(keyword) DO UPDATE SET searched_at=CURRENT_TIMESTAMP`, keyword)
	return wrapDB(err)
}

func (r *searchHistoryRepo) List(ctx context.Context, limit int) ([]*domain.SearchHistory, error) {
	rows, err := r.db.read.QueryContext(ctx,
		`SELECT id, keyword, searched_at FROM search_history ORDER BY searched_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.SearchHistory
	for rows.Next() {
		var (
			h       domain.SearchHistory
			searched sql.NullString
		)
		if err := rows.Scan(&h.ID, &h.Keyword, &searched); err != nil {
			return nil, wrapDB(err)
		}
		h.SearchedAt = parseTS(searched)
		out = append(out, &h)
	}
	return out, wrapDB(rows.Err())
}

func (r *searchHistoryRepo) Clear(ctx context.Context) error {
	_, err := r.db.write.ExecContext(ctx, `DELETE FROM search_history`)
	return wrapDB(err)
}
