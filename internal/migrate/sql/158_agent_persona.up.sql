-- IS-1096: surface the agent persona on the claim row and the session row so
-- downstream filters (IS-1097 tool-allowlist, IS-1098 claim-filter, IS-1101
-- dashboard) can key off the role the agent claimed/ran as.
--
-- Schema names verified against the live model: there is no zdx_agent_claim
-- or zdx_agent_session table; the claim record lives on zdx_todos (alongside
-- claimed_by / claimed_at / lease_expires_at) and the session record is
-- zdx_claude_sessions. We add `claim_persona` on the todo (distinct from the
-- pre-existing `persona` column, which advertises which persona a todo is
-- INTENDED for) and `persona` on the session.
--
-- Both default to 'dev' per IS-1090 Q4 (awaiting product confirmation); the
-- CLI's NormalizePersona enforces the same default.

ALTER TABLE zdx_todos
    ADD COLUMN claim_persona TEXT NOT NULL DEFAULT 'dev';

ALTER TABLE zdx_claude_sessions
    ADD COLUMN persona TEXT NOT NULL DEFAULT 'dev';
