package store

import (
	"context"
	"database/sql"
	"time"

	"xmedia/internal/domain"
)

type pansearchCacheRepo struct{ db *DB }

func (r *pansearchCacheRepo) Get(ctx context.Context, keyword, cloudTypes string) (string, int, *time.Time, error) {
	var (
		results  string
		linkCnt  int
		cachedAt sql.NullString
	)
	err := r.db.read.QueryRowContext(ctx,
		`SELECT results, link_count, cached_at FROM pansearch_cache WHERE keyword=? AND cloud_types=?`,
		keyword, cloudTypes).Scan(&results, &linkCnt, &cachedAt)
	if err == sql.ErrNoRows {
		return "", 0, nil, domain.Errf(domain.CodeNotFound)
	}
	if err != nil {
		return "", 0, nil, wrapDB(err)
	}
	return results, linkCnt, nullableTS(cachedAt), nil
}

func (r *pansearchCacheRepo) Set(ctx context.Context, keyword, cloudTypes, results string, linkCount int) error {
	_, err := r.db.write.ExecContext(ctx, `
		INSERT INTO pansearch_cache(keyword, cloud_types, results, link_count, cached_at)
		VALUES(?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(keyword, cloud_types) DO UPDATE SET
			results=excluded.results, link_count=excluded.link_count, cached_at=CURRENT_TIMESTAMP`,
		keyword, cloudTypes, results, linkCount)
	return wrapDB(err)
}

func (r *pansearchCacheRepo) MarkStale(ctx context.Context, keyword, cloudTypes string) error {
	_, err := r.db.write.ExecContext(ctx,
		`UPDATE pansearch_cache SET link_count=0 WHERE keyword=? AND cloud_types=?`,
		keyword, cloudTypes)
	return wrapDB(err)
}
