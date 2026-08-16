package store

import (
	"context"
	"database/sql"

	"xmedia/internal/domain"
)

type resolveTaskRepo struct{ db *DB }

func scanResolveTask(sc interface{ Scan(...any) error }) (*domain.ResolveTask, error) {
	var (
		t       domain.ResolveTask
		status  string
		stage   string
		created sql.NullString
		updated sql.NullString
	)
	err := sc.Scan(&t.ID, &t.ExternalID, &t.ExternalSource, &t.MediaType, &t.Title, &t.Year,
		&t.Season, &t.Episode, &status, &stage, &t.StageDetail, &t.ProgressPct,
		&t.ResultSource, &t.ResultFileID, &t.ResultAccountID, &t.ResultFilePath, &t.OfflineTaskID, &t.ErrorMsg,
		&created, &updated)
	if err != nil {
		return nil, err
	}
	t.Status = domain.ResolveStatus(status)
	t.Stage = domain.ResolveStage(stage)
	t.CreatedAt = parseTS(created)
	t.UpdatedAt = parseTS(updated)
	return &t, nil
}

const resolveTaskCols = `id, external_id, external_source, media_type, title, year, season, episode,
	status, stage, stage_detail, progress_pct, result_source, result_file_id, result_account_id,
	result_file_path, offline_task_id, error_msg, created_at, updated_at`

func (r *resolveTaskRepo) Create(ctx context.Context, t *domain.ResolveTask) (int64, error) {
	res, err := r.db.write.ExecContext(ctx, `
		INSERT INTO resolve_tasks(external_id, external_source, media_type, title, year, season, episode,
			status, stage, stage_detail, progress_pct)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		t.ExternalID, t.ExternalSource, t.MediaType, t.Title, t.Year, t.Season, t.Episode,
		string(t.Status), string(t.Stage), t.StageDetail, t.ProgressPct)
	if err != nil {
		return 0, wrapDB(err)
	}
	id, err := res.LastInsertId()
	return id, wrapDB(err)
}

func (r *resolveTaskRepo) Get(ctx context.Context, id int64) (*domain.ResolveTask, error) {
	row := r.db.read.QueryRowContext(ctx, `SELECT `+resolveTaskCols+` FROM resolve_tasks WHERE id=?`, id)
	t, err := scanResolveTask(row)
	if err == sql.ErrNoRows {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	return t, wrapDB(err)
}

func (r *resolveTaskRepo) FindActiveByKey(ctx context.Context, externalID int64, source string, season, episode int) (*domain.ResolveTask, error) {
	row := r.db.read.QueryRowContext(ctx, `
		SELECT `+resolveTaskCols+` FROM resolve_tasks
		WHERE external_id=? AND external_source=? AND season=? AND episode=?
		  AND status IN ('pending','running')
		ORDER BY id DESC LIMIT 1`, externalID, source, season, episode)
	t, err := scanResolveTask(row)
	if err == sql.ErrNoRows {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	return t, wrapDB(err)
}

func (r *resolveTaskRepo) Update(ctx context.Context, t *domain.ResolveTask) error {
	_, err := r.db.write.ExecContext(ctx, `
		UPDATE resolve_tasks SET status=?, stage=?, stage_detail=?, progress_pct=?,
			result_source=?, result_file_id=?, result_account_id=?, result_file_path=?,
			offline_task_id=?, error_msg=?,
			updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		string(t.Status), string(t.Stage), t.StageDetail, t.ProgressPct,
		t.ResultSource, t.ResultFileID, t.ResultAccountID, t.ResultFilePath, t.OfflineTaskID, t.ErrorMsg, t.ID)
	return wrapDB(err)
}

func (r *resolveTaskRepo) ListActive(ctx context.Context) ([]*domain.ResolveTask, error) {
	rows, err := r.db.read.QueryContext(ctx, `
		SELECT `+resolveTaskCols+` FROM resolve_tasks WHERE status IN ('pending','running')`)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []*domain.ResolveTask
	for rows.Next() {
		t, err := scanResolveTask(rows)
		if err != nil {
			return nil, wrapDB(err)
		}
		out = append(out, t)
	}
	return out, wrapDB(rows.Err())
}
