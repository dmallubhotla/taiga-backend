-- Remove profile fields from users table (SQLite doesn't support DROP COLUMN in older versions)
-- This would require table recreation in SQLite < 3.35.0
-- For simplicity, we'll use SQLite 3.35.0+ syntax
ALTER TABLE users DROP COLUMN is_active;
ALTER TABLE users DROP COLUMN avatar_url;
ALTER TABLE users DROP COLUMN bio;