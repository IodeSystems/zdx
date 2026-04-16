-- Todos become reservable: add lease expiry for atomic claiming.
ALTER TABLE zdx_todos ADD COLUMN lease_expires_at timestamptz;
