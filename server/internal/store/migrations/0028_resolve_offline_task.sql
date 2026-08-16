-- [P0-2] P2 磁力兜底：持久化 115 离线任务 ID，供启动恢复（§28.2）与
-- Resolve Modal 重新接入（§6.5 后台行为）查询使用。
ALTER TABLE resolve_tasks ADD COLUMN offline_task_id TEXT NOT NULL DEFAULT '';
