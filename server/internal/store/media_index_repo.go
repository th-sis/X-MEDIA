package store

import (
	"context"
	"database/sql"
	"time"

	"xmedia/internal/domain"
)

type mediaIndexRepo struct{ db *DB }

func scanMediaIndex(sc interface{ Scan(...any) error }) (*domain.MediaIndex, error) {
	var (
		m            domain.MediaIndex
		matchStatus  string
		urlExpires   sql.NullString
		lastPlayedAt sql.NullString
		createdAt    sql.NullString
		updatedAt    sql.NullString
	)
	err := sc.Scan(
		&m.ID, &m.ExternalID, &m.ExternalSource, &m.Season, &m.Episode,
		&m.MediaType, &m.Title, &m.OriginalName, &m.Year,
		&m.SourceType, &m.AccountID, &m.FilePath, &m.FileID,
		&m.FileSize, &m.FileFormat, &matchStatus, &m.MatchScore,
		&m.StreamURL, &urlExpires, &lastPlayedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.MatchStatus = domain.MatchStatus(matchStatus)
	m.URLExpires = nullableTS(urlExpires)
	m.LastPlayedAt = nullableTS(lastPlayedAt)
	m.CreatedAt = parseTS(createdAt)
	m.UpdatedAt = parseTS(updatedAt)
	return &m, nil
}

const mediaIndexCols = `id, external_id, external_source, season, episode, media_type, title, original_name, year,
	source_type, account_id, file_path, file_id, file_size, file_format, match_status, match_score,
	stream_url, url_expires, last_played_at, created_at, updated_at`

func (r *mediaIndexRepo) Upsert(ctx context.Context, m *domain.MediaIndex) (int64, error) {
	res, err := r.db.write.ExecContext(ctx, `
		INSERT INTO media_index(
			external_id, external_source, season, episode, media_type, title, original_name, year,
			source_type, account_id, file_path, file_id, file_size, file_format, match_status, match_score,
			stream_url, url_expires, last_played_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(source_type, file_path) DO UPDATE SET
			external_id=excluded.external_id, external_source=excluded.external_source,
			season=excluded.season, episode=excluded.episode, media_type=excluded.media_type,
			title=excluded.title, original_name=excluded.original_name, year=excluded.year,
			account_id=excluded.account_id, file_id=excluded.file_id, file_size=excluded.file_size,
			file_format=excluded.file_format, match_status=excluded.match_status, match_score=excluded.match_score,
			stream_url=excluded.stream_url, url_expires=excluded.url_expires, updated_at=CURRENT_TIMESTAMP`,
		m.ExternalID, m.ExternalSource, m.Season, m.Episode, m.MediaType, m.Title, m.OriginalName, m.Year,
		m.SourceType, m.AccountID, m.FilePath, m.FileID, m.FileSize, m.FileFormat, string(m.MatchStatus), m.MatchScore,
		m.StreamURL, tsValue(timeOrZero(m.URLExpires)),
	)
	if err != nil {
		return 0, wrapDB(err)
	}
	id, err := res.LastInsertId()
	return id, wrapDB(err)
}

func (r *mediaIndexRepo) FindBest(ctx context.Context, externalID int64, source string, season, episode int) (*domain.MediaIndex, error) {
	row := r.db.read.QueryRowContext(ctx, `
		SELECT `+mediaIndexCols+` FROM media_index
		WHERE external_id=? AND external_source=?
		  AND (season=? OR season=0)
		  AND (episode=? OR episode=0)
		ORDER BY season DESC, episode DESC
		LIMIT 1`, externalID, source, season, episode)
	m, err := scanMediaIndex(row)
	if err == sql.ErrNoRows {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	return m, wrapDB(err)
}

func (r *mediaIndexRepo) AvailableKeys(ctx context.Context, items []domain.AvailabilityKey) ([]domain.AvailabilityKey, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]domain.AvailabilityKey, 0)
	for _, it := range items {
		row := r.db.read.QueryRowContext(ctx, `
			SELECT COUNT(1) FROM media_index
			WHERE external_id=? AND external_source=?
			  AND (season=? OR season=0) AND (episode=? OR episode=0)
			LIMIT 1`, it.ExternalID, it.ExternalSource, it.Season, it.Episode)
		var n int
		if err := row.Scan(&n); err != nil {
			return nil, wrapDB(err)
		}
		if n > 0 {
			out = append(out, it)
		}
	}
	return out, nil
}

func (r *mediaIndexRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.read.QueryRowContext(ctx, `SELECT COUNT(1) FROM media_index`).Scan(&n)
	return n, wrapDB(err)
}

func (r *mediaIndexRepo) ListBySource(ctx context.Context, sourceType string, accountID int64) ([]*domain.MediaIndex, error) {
	rows, err := r.db.read.QueryContext(ctx, `
		SELECT `+mediaIndexCols+` FROM media_index WHERE source_type=? AND account_id=?`,
		sourceType, accountID)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.MediaIndex
	for rows.Next() {
		m, err := scanMediaIndex(rows)
		if err != nil {
			return nil, wrapDB(err)
		}
		out = append(out, m)
	}
	return out, wrapDB(rows.Err())
}

func (r *mediaIndexRepo) DeleteBySourcePath(ctx context.Context, sourceType, filePath string) error {
	_, err := r.db.write.ExecContext(ctx,
		`DELETE FROM media_index WHERE source_type=? AND file_path=?`, sourceType, filePath)
	return wrapDB(err)
}

// timeOrZero 把 *time.Time 归一为 time.Time（nil 视为零值）。
func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
