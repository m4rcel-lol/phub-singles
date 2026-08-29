-- View and click de-duplication.
--
-- A row here is a short-lived, salted HMAC of the requester's address and user
-- agent. It cannot be reversed into an IP, it is never joined against anything,
-- and it is deleted as soon as it expires. Its only job is to stop the same
-- visitor from inflating the counters by reloading the page.

CREATE TABLE IF NOT EXISTS view_events (
    fingerprint  TEXT PRIMARY KEY,
    expires_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_view_events_expires ON view_events (expires_at);
