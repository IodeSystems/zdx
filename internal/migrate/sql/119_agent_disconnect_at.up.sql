ALTER TABLE zdx_agents ADD COLUMN disconnect_at timestamptz NULL;
CREATE INDEX zdx_agents_disconnect_at_idx ON zdx_agents (disconnect_at) WHERE disconnect_at IS NOT NULL;
