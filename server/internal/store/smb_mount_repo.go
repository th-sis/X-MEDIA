// [V7 §9.4 UI-first] 容器内 SMB 挂载点仓储 (对应 nas_sources 单条记录).
package store

import (
	"context"
	"database/sql"
	"time"

	"xmedia/internal/domain"
)

type smbMountRepo struct{ db *DB }

func (r *smbMountRepo) Create(ctx context.Context, m *domain.SMBMount) (int64, error) {
	res, err := r.db.write.ExecContext(ctx, `
		INSERT INTO smb_mounts(name, smb_url, remote_path, mount_point, uid, gid)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.Name, m.SMBURL, m.RemotePath, m.MountPoint, m.UID, m.GID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	m.ID = id
	m.State = domain.SMBMountStateUnmounted
	return id, nil
}

func (r *smbMountRepo) Update(ctx context.Context, m *domain.SMBMount) error {
	_, err := r.db.write.ExecContext(ctx, `
		UPDATE smb_mounts
		SET name=?, smb_url=?, remote_path=?, mount_point=?, uid=?, gid=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		m.Name, m.SMBURL, m.RemotePath, m.MountPoint, m.UID, m.GID, m.ID)
	return err
}

func (r *smbMountRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.write.ExecContext(ctx, `DELETE FROM smb_mounts WHERE id=?`, id)
	return err
}

func (r *smbMountRepo) Get(ctx context.Context, id int64) (*domain.SMBMount, error) {
	row := r.db.read.QueryRowContext(ctx, `
		SELECT id, name, smb_url, remote_path, mount_point, uid, gid, state,
		       last_error, last_checked_at, created_at, updated_at
		FROM smb_mounts WHERE id=?`, id)
	return scanSMBMount(row)
}

func (r *smbMountRepo) List(ctx context.Context) ([]*domain.SMBMount, error) {
	rows, err := r.db.read.QueryContext(ctx, `
		SELECT id, name, smb_url, remote_path, mount_point, uid, gid, state,
		       last_error, last_checked_at, created_at, updated_at
		FROM smb_mounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.SMBMount
	for rows.Next() {
		m, err := scanSMBMount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// UpdateRuntime 实时写入 state/last_error (供挂载/卸载流程调用).
func (r *smbMountRepo) UpdateRuntime(ctx context.Context, id int64, state domain.SMBMountState, lastErr string) error {
	_, err := r.db.write.ExecContext(ctx, `
		UPDATE smb_mounts SET state=?, last_error=?, last_checked_at=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		state, lastErr, time.Now().UTC(), id)
	return err
}

func scanSMBMount(r rowScanner) (*domain.SMBMount, error) {
	var m domain.SMBMount
	var state string
	var lastErr sql.NullString
	var lastChecked sql.NullTime
	err := r.Scan(&m.ID, &m.Name, &m.SMBURL, &m.RemotePath, &m.MountPoint,
		&m.UID, &m.GID, &state, &lastErr, &lastChecked,
		&m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	m.State = domain.SMBMountState(state)
	if lastErr.Valid {
		m.LastError = lastErr.String
	}
	if lastChecked.Valid {
		t := lastChecked.Time
		m.LastCheckedAt = &t
	}
	return &m, nil
}
