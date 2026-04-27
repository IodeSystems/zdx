import { Link } from '@tanstack/react-router'
import { Box, Chip, Typography } from '@mui/material'
import type { ReservationItem } from '../api'

export function ReservationSection({
  slug,
  reservations,
}: {
  slug: string
  reservations: ReservationItem[]
}) {
  return (
    <>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Reservations ({reservations.length})
      </Typography>
      {reservations.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
          No reservations yet.
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mb: 3 }}>
          {reservations.map(r => {
            const isActive = !r.released_at && new Date(r.lease_expires_at) > new Date()
            const statusLabel = r.released_at ? 'released' : isActive ? 'active' : 'expired'
            const statusColor = isActive ? 'success' : r.released_at ? 'default' : 'warning'
            const inner = (
              <Box>
                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap' }}>
                  <Chip label={statusLabel} size="small" color={statusColor as any} variant="outlined" />
                  {r.session_id && r.session_status && (
                    <Chip
                      label={r.session_status}
                      size="small"
                      color={
                        r.session_status === 'ok'
                          ? 'success'
                          : r.session_status === 'errored'
                            ? 'error'
                            : r.session_status === 'churn'
                              ? 'warning'
                              : 'default'
                      }
                      variant="filled"
                    />
                  )}
                  <Typography variant="caption" color="text.secondary">
                    {r.session_alias || r.claimed_by || '(unknown claimant)'}
                  </Typography>
                </Box>
                {r.session_header && (
                  <Typography variant="body2" sx={{ mt: 0.5 }}>{r.session_header}</Typography>
                )}
                <Box sx={{ display: 'flex', gap: 2, mt: 0.5, flexWrap: 'wrap' }}>
                  <Typography variant="caption" color="text.disabled">
                    acquired: {new Date(r.claimed_at).toLocaleString()}
                  </Typography>
                  {r.released_at && (
                    <Typography variant="caption" color="text.disabled">
                      released: {new Date(r.released_at).toLocaleString()}
                    </Typography>
                  )}
                </Box>
              </Box>
            )
            return r.session_id ? (
              <Box
                key={r.id}
                component={Link as any}
                to="/project/$slug/agents/$sessionId"
                params={{ slug, sessionId: String(r.session_id) }}
                sx={{
                  display: 'block',
                  textDecoration: 'none',
                  color: 'inherit',
                  '&:hover': { opacity: 0.8 },
                }}
              >
                {inner}
              </Box>
            ) : (
              <Box key={r.id}>{inner}</Box>
            )
          })}
        </Box>
      )}
    </>
  )
}
