import { Box, Chip, Tooltip, Typography } from '@mui/material'
import { Link } from '@tanstack/react-router'
import { useHistory, type HistoryEvent } from '../api'

const VALUE_TRUNC = 60

function truncate(s: string): string {
  if (s.length <= VALUE_TRUNC) return s
  return s.slice(0, VALUE_TRUNC) + '…'
}

function StatusRow({ e }: { e: HistoryEvent }) {
  return (
    <>
      <Typography variant="caption" sx={{ minWidth: 60 }}>
        status
      </Typography>
      <Chip
        label={e.from_status || '—'}
        size="small"
        variant="outlined"
        sx={{ fontSize: '0.65rem' }}
      />
      <Typography variant="caption">→</Typography>
      <Chip
        label={e.to_status || '—'}
        size="small"
        color="primary"
        sx={{ fontSize: '0.65rem' }}
      />
    </>
  )
}

function FieldRow({ e }: { e: HistoryEvent }) {
  const oldVal = e.old_val ?? ''
  const newVal = e.new_val ?? ''
  const oldTrunc = truncate(oldVal)
  const newTrunc = truncate(newVal)
  return (
    <>
      <Typography variant="caption" sx={{ minWidth: 60 }}>
        {e.field}
      </Typography>
      <Tooltip title={oldVal} disableHoverListener={oldVal === oldTrunc}>
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
          '{oldTrunc}'
        </Typography>
      </Tooltip>
      <Typography variant="caption">→</Typography>
      <Tooltip title={newVal} disableHoverListener={newVal === newTrunc}>
        <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
          '{newTrunc}'
        </Typography>
      </Tooltip>
    </>
  )
}

export function EditHistory({
  slug,
  targetType,
  targetId,
}: {
  slug: string
  targetType: 'issue' | 'task'
  targetId: string
}) {
  const { data } = useHistory(targetType, targetId)
  const events = data?.events ?? []

  if (events.length === 0) {
    return (
      <Box sx={{ mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
          Edit history
        </Typography>
        <Typography variant="caption" color="text.disabled">
          No edits recorded yet.
        </Typography>
      </Box>
    )
  }

  return (
    <Box sx={{ mb: 2 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        Edit history ({events.length})
      </Typography>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
        {events.map((e) => {
          const created = new Date(e.created_at)
          return (
            <Box
              key={`${e.kind}:${e.id}`}
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                borderLeft: 2,
                borderColor: 'divider',
                pl: 1.5,
                flexWrap: 'wrap',
              }}
            >
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ minWidth: 120 }}
                title={created.toISOString()}
              >
                {created.toLocaleString()}
              </Typography>
              {e.kind === 'status' ? <StatusRow e={e} /> : <FieldRow e={e} />}
              {e.agent_id && (
                <Chip
                  label={`agent:${e.agent_id}`}
                  size="small"
                  variant="outlined"
                  sx={{ fontSize: '0.65rem' }}
                />
              )}
              {e.session_id && (
                <Link
                  to="/project/$slug/agents/$sessionId"
                  params={{ slug, sessionId: e.session_id }}
                  style={{ textDecoration: 'none' }}
                >
                  <Chip
                    label={`session:${e.session_id.slice(0, 8)}`}
                    size="small"
                    variant="outlined"
                    clickable
                    sx={{ fontSize: '0.65rem' }}
                  />
                </Link>
              )}
              {e.user_id && (
                <Typography variant="caption" color="text.disabled">
                  user:{e.user_id}
                </Typography>
              )}
            </Box>
          )
        })}
      </Box>
    </Box>
  )
}
