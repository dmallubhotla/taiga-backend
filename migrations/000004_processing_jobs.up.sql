-- Which extraction code built a workout's derived data; workouts from a file whose version isn't the current one get reprocessed.
ALTER TABLE workouts ADD COLUMN processor_version VARCHAR(64) NOT NULL DEFAULT '';

-- Background work, claimed by the in-process worker.
CREATE TABLE jobs (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users (id),
  kind VARCHAR(32) NOT NULL,
  workout_id INTEGER REFERENCES workouts (id) ON DELETE CASCADE,
  -- queued, running, done or dead
  state VARCHAR(16) NOT NULL DEFAULT 'queued',
  attempts INTEGER NOT NULL DEFAULT 0,
  run_at TIMESTAMP WITH TIME ZONE NOT NULL,
  -- A running job whose lease has passed was dropped by a crashed worker and can be claimed again.
  lease_until TIMESTAMP WITH TIME ZONE,
  last_error TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX idx_jobs_state_run_at ON jobs (state, run_at);

CREATE INDEX idx_jobs_user_id ON jobs (user_id, id);

-- At most one pending job per kind and workout, so enqueueing twice is a no-op.
CREATE UNIQUE INDEX idx_jobs_pending ON jobs (kind, user_id, COALESCE(workout_id, 0))
WHERE
  state IN ('queued', 'running');
