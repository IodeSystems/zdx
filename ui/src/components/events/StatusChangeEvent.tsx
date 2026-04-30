import { Chip, Stack, Typography } from '@mui/material'
import type { EventItem } from '../../api'

type Props = { event: EventItem; slug: string }

type StatusChangeDetail = {
  target_kind?: string
  from_status?: string
  to_status?: string
  actor?: string
}

const STATUS_COLORS: Record<string, 'warning' | 'success' | 'default' | 'info' | 'secondary'> = {
  proposed: 'warning',
  approved: 'success',
  rejected: 'default',
  snoozed: 'info',
  open: 'warning',
  active: 'info',
  closed: 'secondary',
  resolved: 'success',
  pending: 'default',
  ready: 'info',
  wip: 'warning',
  done: 'success',
  cancelled: 'default',
  draft: 'default',
}

export function StatusChangeEvent({ event }: Props) {
  const d = event.detail_json as StatusChangeDetail | null ?? {}
  const kind = String(d.target_kind ?? '')
  const from = String(d.from_status ?? '')
  const to = String(d.to_status ?? '')

  return (
    <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
      <Typography variant="body2" color="text.secondary">
        {kind ? `${kind} status` : 'Status'} changed from
      </Typography>
      <Chip label={from || '—'} size="small" variant="outlined" color={STATUS_COLORS[from] ?? 'default'} />
      <Typography variant="body2" color="text.secondary">→</Typography>
      <Chip label={to || '—'} size="small" color={STATUS_COLORS[to] ?? 'default'} />
    </Stack>
  )
}
