package store

import (
	"context"
	"database/sql"

	"xmedia/internal/domain"
)

type favoriteRepo struct{ db *DB }

func scanFavorite(sc interface{ Scan(...any) error }) (*domain.Favorite, error) {
	var (
		f       domain.Favorite
		created sql.NullString
	)
	err := sc.Scan(&f.ID, &f.ExternalID, &f.ExternalSource, &f.MediaType, &f.Title, &f.PosterURL, &f.Year, &created)
	if err != nil {
		return nil, err
	}
	f.CreatedAt = parseTS(created)
	return &f, nil
}

func (r *favoriteRepo) Add(ctx context.Context, f *domain.Favorite) (int64, error) {
	res, err := r.db.write.ExecContext(ctx, `
		INSERT INTO favorites(external_id, external_source, media_type, title, poster_url, year)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(external_id, external_source) DO NOTHING`,
		f.ExternalID, f.ExternalSource, f.MediaType, f.Title, f.PosterURL, f.Year)
	if err != nil {
		return 0, wrapDB(err)
	}
	id, err := res.LastInsertId()
	return id, wrapDB(err)
}

func (r *favoriteRepo) Remove(ctx context.Context, externalID int64, source string) error {
	_, err := r.db.write.ExecContext(ctx,
		`DELETE FROM favorites WHERE external_id=? AND external_source=?`, externalID, source)
	return wrapDB(err)
}

func (r *favoriteRepo) List(ctx context.Context) ([]*domain.Favorite, error) {
	rows, err := r.db.read.QueryContext(ctx,
		`SELECT id, external_id, external_source, media_type, title, poster_url, year, created_at
		 FROM favorites ORDER BY created_at DESC`)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.Favorite
	for rows.Next() {
		f, err := scanFavorite(rows)
		if err != nil {
			return nil, wrapDB(err)
		}
		out = append(out, f)
	}
	return out, wrapDB(rows.Err())
}

func (r *favoriteRepo) Exists(ctx context.Context, externalID int64, source string) (bool, error) {
	var n int
	err := r.db.read.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM favorites WHERE external_id=? AND external_source=?`,
		externalID, source).Scan(&n)
	return n > 0, wrapDB(err)
}
