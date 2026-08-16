package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"xmedia/internal/domain"
)

type mediaLibraryRepo struct{ db *DB }

func scanMediaLibrary(sc interface{ Scan(...any) error }) (*domain.MediaLibrary, error) {
	var (
		m              domain.MediaLibrary
		genres         string
		seasonsJSON    string
		cast           string
		extra          string
		cachedAt       sql.NullString
		lastAccessedAt sql.NullString
	)
	err := sc.Scan(
		&m.ID, &m.ExternalID, &m.ExternalSource, &m.MediaType, &m.Title, &m.TitleOrig,
		&m.PosterURL, &m.BackdropURL, &m.Overview, &m.Year, &m.VoteAvg, &m.VoteCount,
		&genres, &m.Runtime, &m.Seasons, &m.Episodes, &seasonsJSON, &cast, &extra,
		&cachedAt, &lastAccessedAt,
	)
	if err != nil {
		return nil, err
	}
	m.Genres = json.RawMessage(genres)
	m.SeasonsJSON = json.RawMessage(seasonsJSON)
	m.Cast = json.RawMessage(cast)
	m.Extra = json.RawMessage(extra)
	m.CachedAt = parseTS(cachedAt)
	m.LastAccessedAt = nullableTS(lastAccessedAt)
	return &m, nil
}

const mediaLibraryCols = `id, external_id, external_source, media_type, title, title_orig,
	poster_url, backdrop_url, overview, year, vote_avg, vote_count, genres, runtime, seasons, episodes,
	seasons_json, cast, extra, cached_at, last_accessed_at`

func (r *mediaLibraryRepo) Upsert(ctx context.Context, m *domain.MediaLibrary) (int64, error) {
	res, err := r.db.write.ExecContext(ctx, `
		INSERT INTO media_library(
			external_id, external_source, media_type, title, title_orig, poster_url, backdrop_url, overview,
			year, vote_avg, vote_count, genres, runtime, seasons, episodes, seasons_json, cast, extra, cached_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(external_id, external_source) DO UPDATE SET
			media_type=excluded.media_type, title=excluded.title, title_orig=excluded.title_orig,
			poster_url=excluded.poster_url, backdrop_url=excluded.backdrop_url, overview=excluded.overview,
			year=excluded.year, vote_avg=excluded.vote_avg, vote_count=excluded.vote_count,
			genres=excluded.genres, runtime=excluded.runtime, seasons=excluded.seasons, episodes=excluded.episodes,
			seasons_json=excluded.seasons_json, cast=excluded.cast, extra=excluded.extra, cached_at=CURRENT_TIMESTAMP`,
		m.ExternalID, m.ExternalSource, m.MediaType, m.Title, m.TitleOrig, m.PosterURL, m.BackdropURL, m.Overview,
		m.Year, m.VoteAvg, m.VoteCount, string(m.Genres), m.Runtime, m.Seasons, m.Episodes,
		string(m.SeasonsJSON), string(m.Cast), string(m.Extra),
	)
	if err != nil {
		return 0, wrapDB(err)
	}
	id, err := res.LastInsertId()
	return id, wrapDB(err)
}

func (r *mediaLibraryRepo) Get(ctx context.Context, externalID int64, source string) (*domain.MediaLibrary, error) {
	row := r.db.read.QueryRowContext(ctx,
		`SELECT `+mediaLibraryCols+` FROM media_library WHERE external_id=? AND external_source=?`,
		externalID, source)
	m, err := scanMediaLibrary(row)
	if err == sql.ErrNoRows {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	return m, wrapDB(err)
}

// SearchByTitle 按标题模糊查询（LIKE 双向包含，索引匹配器用 §9.2）。
func (r *mediaLibraryRepo) SearchByTitle(ctx context.Context, title string, limit int) ([]*domain.MediaLibrary, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	pattern := "%" + title + "%"
	rows, err := r.db.read.QueryContext(ctx, `
		SELECT `+mediaLibraryCols+` FROM media_library
		WHERE title LIKE ? OR title_orig LIKE ?
		ORDER BY cached_at DESC LIMIT ?`, pattern, pattern, limit)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.MediaLibrary
	for rows.Next() {
		m, err := scanMediaLibrary(rows)
		if err != nil {
			return nil, wrapDB(err)
		}
		out = append(out, m)
	}
	return out, wrapDB(rows.Err())
}

func (r *mediaLibraryRepo) Touch(ctx context.Context, externalID int64, source string) error {
	_, err := r.db.write.ExecContext(ctx,
		`UPDATE media_library SET last_accessed_at=CURRENT_TIMESTAMP WHERE external_id=? AND external_source=?`,
		externalID, source)
	return wrapDB(err)
}

func (r *mediaLibraryRepo) ListForEviction(ctx context.Context, limit int) ([]*domain.MediaLibrary, error) {
	rows, err := r.db.read.QueryContext(ctx,
		`SELECT `+mediaLibraryCols+` FROM media_library ORDER BY last_accessed_at IS NULL, last_accessed_at ASC LIMIT ?`,
		limit)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.MediaLibrary
	for rows.Next() {
		m, err := scanMediaLibrary(rows)
		if err != nil {
			return nil, wrapDB(err)
		}
		out = append(out, m)
	}
	return out, wrapDB(rows.Err())
}

func (r *mediaLibraryRepo) CountTotal(ctx context.Context) (int, error) {
	var n int
	err := r.db.read.QueryRowContext(ctx, `SELECT COUNT(1) FROM media_library`).Scan(&n)
	return n, wrapDB(err)
}

func (r *mediaLibraryRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.write.ExecContext(ctx, `DELETE FROM media_library WHERE id=?`, id)
	return wrapDB(err)
}
