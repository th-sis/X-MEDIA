package store

import (
	"context"
	"database/sql"

	"xmedia/internal/domain"
)

type playHistoryRepo struct{ db *DB }

func scanPlayHistory(sc interface{ Scan(...any) error }) (*domain.PlayHistory, error) {
	var (
		h       domain.PlayHistory
		played  sql.NullString
	)
	err := sc.Scan(&h.ID, &h.ExternalID, &h.ExternalSource, &h.MediaType, &h.Title,
		&h.PosterURL, &h.SourceType, &h.Season, &h.Episode, &h.PositionMs, &h.DurationMs, &played)
	if err != nil {
		return nil, err
	}
	h.PlayedAt = parseTS(played)
	return &h, nil
}

const playHistoryCols = `id, external_id, external_source, media_type, title, poster_url,
	source_type, season, episode, position_ms, duration_ms, played_at`

func (r *playHistoryRepo) Upsert(ctx context.Context, h *domain.PlayHistory) error {
	_, err := r.db.write.ExecContext(ctx, `
		INSERT INTO play_history(external_id, external_source, media_type, title, poster_url, source_type,
			season, episode, position_ms, duration_ms, played_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(external_id, external_source, season, episode) DO UPDATE SET
			media_type=excluded.media_type, title=excluded.title, poster_url=excluded.poster_url,
			source_type=excluded.source_type, position_ms=excluded.position_ms,
			duration_ms=excluded.duration_ms, played_at=CURRENT_TIMESTAMP`,
		h.ExternalID, h.ExternalSource, h.MediaType, h.Title, h.PosterURL, h.SourceType,
		h.Season, h.Episode, h.PositionMs, h.DurationMs)
	return wrapDB(err)
}

func (r *playHistoryRepo) Get(ctx context.Context, externalID int64, source string, season, episode int) (*domain.PlayHistory, error) {
	row := r.db.read.QueryRowContext(ctx,
		`SELECT `+playHistoryCols+` FROM play_history WHERE external_id=? AND external_source=? AND season=? AND episode=?`,
		externalID, source, season, episode)
	h, err := scanPlayHistory(row)
	if err == sql.ErrNoRows {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	return h, wrapDB(err)
}

func (r *playHistoryRepo) List(ctx context.Context, limit int) ([]*domain.PlayHistory, error) {
	rows, err := r.db.read.QueryContext(ctx,
		`SELECT `+playHistoryCols+` FROM play_history ORDER BY played_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.PlayHistory
	for rows.Next() {
		h, err := scanPlayHistory(rows)
		if err != nil {
			return nil, wrapDB(err)
		}
		out = append(out, h)
	}
	return out, wrapDB(rows.Err())
}

func (r *playHistoryRepo) ListContinueWatching(ctx context.Context, limit int) ([]*domain.PlayHistory, error) {
	rows, err := r.db.read.QueryContext(ctx, `
		SELECT DISTINCT `+playHistoryCols+` FROM play_history
		WHERE position_ms > 0
		  AND (duration_ms = 0 OR (duration_ms - position_ms) > 120000)
		ORDER BY played_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.PlayHistory
	for rows.Next() {
		h, err := scanPlayHistory(rows)
		if err != nil {
			return nil, wrapDB(err)
		}
		out = append(out, h)
	}
	return out, wrapDB(rows.Err())
}

func (r *playHistoryRepo) DeleteByKey(ctx context.Context, externalID int64, source string, season, episode int) error {
	_, err := r.db.write.ExecContext(ctx,
		`DELETE FROM play_history WHERE external_id=? AND external_source=? AND season=? AND episode=?`,
		externalID, source, season, episode)
	return wrapDB(err)
}

func (r *playHistoryRepo) DeleteAll(ctx context.Context) error {
	_, err := r.db.write.ExecContext(ctx, `DELETE FROM play_history`)
	return wrapDB(err)
}

func (r *playHistoryRepo) HasAny(ctx context.Context, externalID int64, source string) (bool, error) {
	var n int
	err := r.db.read.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM play_history WHERE external_id=? AND external_source=?`,
		externalID, source).Scan(&n)
	return n > 0, wrapDB(err)
}
