-- The public profile moves from "/" to "/<handle>"; "/" becomes a landing page.
-- The handle is stored lowercase and validated by the API before it lands here.

ALTER TABLE profile ADD COLUMN username TEXT NOT NULL DEFAULT '';

UPDATE profile SET username = 'creator' WHERE id = 1 AND username = '';
