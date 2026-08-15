package store

import (
	"context"
	"time"
)

type rateLimitRepo struct{ db *DB }

func (r *rateLimitRepo) Count(ctx context.Context, clientIP string, windowStart time.Time) (int, error) {
	var n int
	err := r.db.read.QueryRowContext(ctx,
		`SELECT request_count FROM resolve_rate_limits WHERE client_ip=? AND window_start=?`,
		clientIP, tsValue(windowStart)).Scan(&n)
	if err != nil {
		// 无记录视为 0
		return 0, nil
	}
	return n, nil
}

func (r *rateLimitRepo) Increment(ctx context.Context, clientIP string, windowStart time.Time) error {
	_, err := r.db.write.ExecContext(ctx, `
		INSERT INTO resolve_rate_limits(client_ip, window_start, request_count)
		VALUES(?,?,1)
		ON CONFLICT(client_ip, window_start) DO UPDATE SET request_count=request_count+1`,
		clientIP, tsValue(windowStart))
	return wrapDB(err)
}

func (r *rateLimitRepo) Cleanup(ctx context.Context, before time.Time) error {
	_, err := r.db.write.ExecContext(ctx,
		`DELETE FROM resolve_rate_limits WHERE window_start < ?`, tsValue(before))
	return wrapDB(err)
}
