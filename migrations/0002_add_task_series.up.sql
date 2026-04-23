CREATE TABLE IF NOT EXISTS task_series (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	recurrence_type TEXT NOT NULL,
	recurrence_params JSONB NOT NULL DEFAULT '{}'::jsonb,
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	started_at TIMESTAMPTZ NOT NULL,
	generated_through TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'fk_tasks_task_series'
	) THEN
		ALTER TABLE tasks
			ADD CONSTRAINT fk_tasks_task_series
			FOREIGN KEY (task_series_id) REFERENCES task_series (id)
			ON DELETE SET NULL;
	END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_task_series_active ON task_series (is_active);
CREATE INDEX IF NOT EXISTS idx_task_series_started_at ON task_series (started_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_series_task_date_unique
	ON tasks (task_series_id, task_date)
	WHERE task_series_id IS NOT NULL AND task_date IS NOT NULL;
