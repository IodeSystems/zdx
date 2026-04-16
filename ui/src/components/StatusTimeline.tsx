import { Box, Chip, Typography } from '@mui/material'
import { Link } from '@tanstack/react-router'
import { useStatusEvents } from '../api'

export function StatusTimeline({
  slug,
  targetType,
  targetId,
}: {
  slug: string
  targetType: 'issue' | 'task'
  targetId: string
}) {
  const { data } = useStatusEvents(targetType, targetId)
  const events = data?.events ?? []

  if (events.length === 0) {
    return (
      <Box sx={{ mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
          Status history
        </Typography>
        <Typography variant="caption" color="text.disabled">
          No status changes recorded yet.
        </Typography>
      </Box>
    )
  }

  return (
    <Box sx={{ mb: 2 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        Status history ({events.length})
      </Typography>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
        {events.map((e) => {
          const created = new Date(e.created_at)
          return (
            <Box
              key={e.id}
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
              <Chip
                label={e.from_status || '—'}
                size="small"
                variant="outlined"
                sx={{ fontSize: '0.65rem' }}
              />
              <Typography variant="caption">→</Typography>
              <Chip label={e.to_status} size="small" color="primary" sx={{ fontSize: '0.65rem' }} />
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
                  to="/project/$slug/claude/$sessionId"
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
