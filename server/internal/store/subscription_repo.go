package store

import (
	"context"
	"database/sql"

	"xmedia/internal/domain"
)

type subscriptionRepo struct{ db *DB }

func scanSubscription(sc interface{ Scan(...any) error }) (*domain.Subscription, error) {
	var (
		s          domain.Subscription
		status     string
		lastSearch sql.NullString
		created    sql.NullString
		updated    sql.NullString
	)
	err := sc.Scan(&s.ID, &s.ExternalID, &s.ExternalSource, &s.MediaType, &s.Title, &s.Year,
		&s.PosterURL, &status, &s.AutoRuleID, &lastSearch, &s.SearchCount, &s.MaxSearches,
		&s.ResultSource, &s.ResultAccountID, &s.ResultPath, &created, &updated)
	if err != nil {
		return nil, err
	}
	s.Status = domain.SubStatus(status)
	s.LastSearchAt = nullableTS(lastSearch)
	s.CreatedAt = parseTS(created)
	s.UpdatedAt = parseTS(updated)
	return &s, nil
}

const subscriptionCols = `id, external_id, external_source, media_type, title, year, poster_url, status,
	auto_rule_id, last_search_at, search_count, max_searches, result_source, result_account_id, result_path,
	created_at, updated_at`

func (r *subscriptionRepo) Add(ctx context.Context, s *domain.Subscription) (int64, error) {
	res, err := r.db.write.ExecContext(ctx, `
		INSERT INTO subscriptions(external_id, external_source, media_type, title, year, poster_url, status, max_searches)
		VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(external_id, external_source) DO NOTHING`,
		s.ExternalID, s.ExternalSource, s.MediaType, s.Title, s.Year, s.PosterURL, string(s.Status), s.MaxSearches)
	if err != nil {
		return 0, wrapDB(err)
	}
	id, err := res.LastInsertId()
	return id, wrapDB(err)
}

func (r *subscriptionRepo) Remove(ctx context.Context, externalID int64, source string) error {
	_, err := r.db.write.ExecContext(ctx,
		`DELETE FROM subscriptions WHERE external_id=? AND external_source=?`, externalID, source)
	return wrapDB(err)
}

func (r *subscriptionRepo) List(ctx context.Context) ([]*domain.Subscription, error) {
	rows, err := r.db.read.QueryContext(ctx,
		`SELECT `+subscriptionCols+` FROM subscriptions ORDER BY created_at DESC`)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.Subscription
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			return nil, wrapDB(err)
		}
		out = append(out, s)
	}
	return out, wrapDB(rows.Err())
}

func (r *subscriptionRepo) UpdateStatus(ctx context.Context, id int64, status domain.SubStatus, resultSource string, resultAccountID int64, resultPath string) error {
	_, err := r.db.write.ExecContext(ctx, `
		UPDATE subscriptions SET status=?, result_source=?, result_account_id=?, result_path=?,
			last_search_at=CURRENT_TIMESTAMP, search_count=search_count+1, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		string(status), resultSource, resultAccountID, resultPath, id)
	return wrapDB(err)
}

func (r *subscriptionRepo) Exists(ctx context.Context, externalID int64, source string) (bool, error) {
	var n int
	err := r.db.read.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM subscriptions WHERE external_id=? AND external_source=?`,
		externalID, source).Scan(&n)
	return n > 0, wrapDB(err)
}

func (r *subscriptionRepo) ActiveCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.read.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM subscriptions WHERE status IN ('watching','found')`).Scan(&n)
	return n, wrapDB(err)
}

// TouchSearch 记录一次自动搜寻：search_count+1 并刷新 last_search_at（§20）。
func (r *subscriptionRepo) TouchSearch(ctx context.Context, id int64) error {
	_, err := r.db.write.ExecContext(ctx, `
		UPDATE subscriptions SET last_search_at=CURRENT_TIMESTAMP,
			search_count=search_count+1, updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return wrapDB(err)
}
