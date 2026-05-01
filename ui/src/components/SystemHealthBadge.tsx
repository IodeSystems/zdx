import { useState } from 'react'
import {
  Box,
  Chip,
  IconButton,
  Popover,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import { WarningAmber as WarningAmberIcon } from '@mui/icons-material'
import { useHealthStatus } from '../api'

function formatSince(since?: string): string {
  if (!since) return ''
  const d = new Date(since)
  if (Number.isNaN(d.getTime())) return since
  return d.toLocaleString()
}

function chipColor(state: string): 'success' | 'warning' | 'default' {
  if (state === 'ok') return 'success'
  if (state === 'unconfigured') return 'default'
  return 'warning'
}

export function SystemHealthBadge() {
  const { data } = useHealthStatus()
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null)

  if (!data || data.status === 'ok') return null

  const subsystems = Object.entries(data.subsystems ?? {})

  return (
    <>
      <Tooltip title="System health degraded — click for details">
        <IconButton
          color="inherit"
          size="small"
          onClick={(e) => setAnchorEl(e.currentTarget)}
        >
          <WarningAmberIcon color="warning" fontSize="small" />
        </IconButton>
      </Tooltip>
      <Popover
        open={Boolean(anchorEl)}
        anchorEl={anchorEl}
        onClose={() => setAnchorEl(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <Box sx={{ p: 2, minWidth: 320, maxWidth: 480 }}>
          <Typography variant="subtitle2" sx={{ mb: 1 }}>
            System health: {data.status}
          </Typography>
          <Stack spacing={1.5}>
            {subsystems.map(([name, s]) => (
              <Box key={name}>
                <Stack direction="row" spacing={1} sx={{ mb: 0.5, alignItems: 'center' }}>
                  <Typography variant="body2" sx={{ fontWeight: 500 }}>{name}</Typography>
                  <Chip label={s.state} size="small" color={chipColor(s.state)} />
                </Stack>
                {s.since && (
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                    since {formatSince(s.since)}
                  </Typography>
                )}
                {s.reason && (
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                    {s.reason}
                  </Typography>
                )}
              </Box>
            ))}
          </Stack>
        </Box>
      </Popover>
    </>
  )
}
