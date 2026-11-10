-- One activity_file row per user per content; uploads rely on this to dedupe without racing.
-- An index rather than a table constraint so the same statement works on sqlite.
CREATE UNIQUE INDEX idx_activity_file_user_hash ON activity_file (user_id, file_repo_hash);
