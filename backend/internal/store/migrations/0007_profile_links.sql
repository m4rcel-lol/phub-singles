-- Links are part of a user's page, not a global shared list.  Preserve the
-- existing list by assigning it to the account that owned the legacy page.
ALTER TABLE links ADD COLUMN user_id INTEGER REFERENCES users (id) ON DELETE CASCADE;

UPDATE links
SET user_id = (SELECT user_id FROM profile WHERE id = 1)
WHERE user_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_links_user_position ON links (user_id, position, id);
