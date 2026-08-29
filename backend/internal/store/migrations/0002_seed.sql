-- Seed data so a fresh deployment already looks like a finished page.
-- Every statement is idempotent: re-running the migration set never duplicates
-- rows, and edits made through the admin panel are never overwritten.

INSERT OR IGNORE INTO profile (id, display_name, tagline, bio, avatar_url)
VALUES (
    1,
    'pornhub.singles',
    'Every link. One page.',
    'Independent creator. New drops weekly. Everything official lives right here — if it is not on this page, it is not me.',
    ''
);

INSERT INTO links (title, url, icon, enabled, position)
SELECT 'Latest drop', 'https://example.com/latest', '🔥', 1, 0
WHERE NOT EXISTS (SELECT 1 FROM links);

INSERT INTO links (title, url, icon, enabled, position)
SELECT 'Subscribe', 'https://example.com/subscribe', '⭐', 1, 1
WHERE (SELECT COUNT(*) FROM links) = 1;

INSERT INTO links (title, url, icon, enabled, position)
SELECT 'Instagram', 'https://instagram.com/', '📸', 1, 2
WHERE (SELECT COUNT(*) FROM links) = 2;

INSERT INTO links (title, url, icon, enabled, position)
SELECT 'X / Twitter', 'https://x.com/', '𝕏', 1, 3
WHERE (SELECT COUNT(*) FROM links) = 3;

INSERT INTO links (title, url, icon, enabled, position)
SELECT 'Wishlist', 'https://example.com/wishlist', '🎁', 1, 4
WHERE (SELECT COUNT(*) FROM links) = 4;

INSERT INTO links (title, url, icon, enabled, position)
SELECT 'Business enquiries', 'mailto:hello@pornhub.singles', '✉️', 1, 5
WHERE (SELECT COUNT(*) FROM links) = 5;
