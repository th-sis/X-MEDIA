package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"xmedia/internal/domain"
)

// nasSourceRepo 是 NAS 媒体源仓储实现（[V7 §9.4+ 扩展]）。
// 命名对齐 accountRepo，便于 code review 复用既有心智。
type nasSourceRepo struct{ db *DB }

// newNASSourceRepo 构造仓储；调用方由 store.New 注入到 Store.NASSources。
func newNASSourceRepo(db *DB) *nasSourceRepo {
	return &nasSourceRepo{db: db}
}

// nasSourceCols 是 SELECT 子句的权威列表；scan 时按此顺序消费。
const nasSourceCols = `id, name, path, enabled, file_count, last_accessibility, last_checked_at, created_at, updated_at`

func (r *nasSourceRepo) Create(ctx context.Context, s *domain.NASSource) (int64, error) {
	res, err := r.db.write.ExecContext(ctx,
		`INSERT INTO nas_sources(name, path, enabled) VALUES (?,?,?)`,
		s.Name, s.Path, boolToInt(s.Enabled))
	if err != nil {
		return 0, wrapDB(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, wrapDB(err)
	}
	return id, nil
}

func (r *nasSourceRepo) Update(ctx context.Context, s *domain.NASSource) error {
	res, err := r.db.write.ExecContext(ctx,
		`UPDATE nas_sources
		 SET name=?, path=?, enabled=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		s.Name, s.Path, boolToInt(s.Enabled), s.ID)
	if err != nil {
		return wrapDB(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return wrapDB(err)
	}
	if n == 0 {
		return domain.Errf(domain.CodeNotFound)
	}
	return nil
}

func (r *nasSourceRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.write.ExecContext(ctx, `DELETE FROM nas_sources WHERE id=?`, id)
	if err != nil {
		return wrapDB(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return wrapDB(err)
	}
	if n == 0 {
		return domain.Errf(domain.CodeNotFound)
	}
	return nil
}

func (r *nasSourceRepo) Get(ctx context.Context, id int64) (*domain.NASSource, error) {
	row := r.db.read.QueryRowContext(ctx, `SELECT `+nasSourceCols+` FROM nas_sources WHERE id=?`, id)
	s, err := scanNASSource(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.Errf(domain.CodeNotFound)
		}
		return nil, err
	}
	return s, nil
}

func (r *nasSourceRepo) List(ctx context.Context) ([]*domain.NASSource, error) {
	rows, err := r.db.read.QueryContext(ctx, `SELECT `+nasSourceCols+` FROM nas_sources ORDER BY id`)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.NASSource
	for rows.Next() {
		s, err := scanNASSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, wrapDB(rows.Err())
}

func (r *nasSourceRepo) ListEnabled(ctx context.Context) ([]*domain.NASSource, error) {
	rows, err := r.db.read.QueryContext(ctx, `SELECT `+nasSourceCols+` FROM nas_sources WHERE enabled=1 ORDER BY id`)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.NASSource
	for rows.Next() {
		s, err := scanNASSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, wrapDB(rows.Err())
}

func (r *nasSourceRepo) PathTaken(ctx context.Context, path string, excludeID int64) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, nil
	}
	var one int
	var err error
	if excludeID > 0 {
		err = r.db.read.QueryRowContext(ctx,
			`SELECT 1 FROM nas_sources WHERE path=? AND id<>? LIMIT 1`, path, excludeID).Scan(&one)
	} else {
		err = r.db.read.QueryRowContext(ctx,
			`SELECT 1 FROM nas_sources WHERE path=? LIMIT 1`, path).Scan(&one)
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, wrapDB(err)
	}
	return true, nil
}

func (r *nasSourceRepo) NameTaken(ctx context.Context, name string, excludeID int64) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, nil
	}
	var one int
	var err error
	if excludeID > 0 {
		err = r.db.read.QueryRowContext(ctx,
			`SELECT 1 FROM nas_sources WHERE LOWER(name)=LOWER(?) AND id<>? LIMIT 1`, name, excludeID).Scan(&one)
	} else {
		err = r.db.read.QueryRowContext(ctx,
			`SELECT 1 FROM nas_sources WHERE LOWER(name)=LOWER(?) LIMIT 1`, name).Scan(&one)
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, wrapDB(err)
	}
	return true, nil
}

func (r *nasSourceRepo) UpdateHealth(ctx context.Context, id int64, accessibility domain.NASAccessibility, fileCount int64, at time.Time) error {
	res, err := r.db.write.ExecContext(ctx,
		`UPDATE nas_sources
		 SET file_count=?, last_accessibility=?, last_checked_at=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		fileCount, string(accessibility), at.UTC().Format(tsLayout), id)
	if err != nil {
		return wrapDB(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return wrapDB(err)
	}
	if n == 0 {
		return domain.Errf(domain.CodeNotFound)
	}
	return nil
}

// scanNASSource 把 SELECT 结果映射成 domain.NASSource。
// 支持 *sql.Row（Get）和 *sql.Rows（List），二者都实现 rowScanner。
func scanNASSource(s rowScanner) (*domain.NASSource, error) {
	var (
		out       domain.NASSource
		enabled   int
		accessStr string
		checked   sql.NullString
		created   sql.NullString
		updated   sql.NullString
	)
	err := s.Scan(
		&out.ID, &out.Name, &out.Path, &enabled, &out.FileCount,
		&accessStr, &checked, &created, &updated,
	)
	if err != nil {
		return nil, wrapDB(err)
	}
	out.Enabled = enabled != 0
	switch domain.NASAccessibility(accessStr) {
	case domain.NASAccessibilityOK:
		out.LastAccessibility = domain.NASAccessibilityOK
	case domain.NASAccessibilityNotAccessible:
		out.LastAccessibility = domain.NASAccessibilityNotAccessible
	default:
		out.LastAccessibility = domain.NASAccessibilityUnknown
	}
	// [bug fix] 复用 store 包内的 nullableTS 助手（支持多种 layout）；
	// 修复之前 time.Parse 单 layout 解析失败导致 LastCheckedAt/UpdatedAt 等字段为空。
	out.LastCheckedAt = nullableTS(checked)
	out.CreatedAt = parseTS(created)
	out.UpdatedAt = parseTS(updated)
	return &out, nil
}
