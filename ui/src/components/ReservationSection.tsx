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
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, mb: 3 }}>
          {reservations.map(r => {
            const isActive = !r.released_at && new Date(r.lease_expires_at) > new Date()
            const statusLabel = r.released_at ? 'released' : isActive ? 'active' : 'expired'
            const statusColor = isActive ? 'success' : r.released_at ? 'default' : 'warning'
            const inner = (
              <>
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
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {r.session_header || r.claimed_by || '(unknown claimant)'}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {new Date(r.claimed_at).toLocaleString()}
                </Typography>
              </>
            )
            return r.session_id ? (
              <Box
                key={r.id}
                component={Link as any}
                to="/project/$slug/agents/$sessionId"
                params={{ slug, sessionId: String(r.session_id) }}
                sx={{
                  display: 'flex',
                  gap: 1,
                  alignItems: 'center',
                  textDecoration: 'none',
                  color: 'inherit',
                  '&:hover': { opacity: 0.8 },
                }}
              >
                {inner}
              </Box>
            ) : (
              <Box key={r.id} sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                {inner}
              </Box>
            )
          })}
        </Box>
      )}
    </>
  )
}
