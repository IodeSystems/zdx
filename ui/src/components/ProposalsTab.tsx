import { Link } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Stack,
  Typography,
} from '@mui/material'
import { useProposals, type ProposalItem } from '../api'

const STATUS_COLORS: Record<string, 'warning' | 'success' | 'default' | 'info' | 'secondary'> = {
  proposed: 'warning',
  approved: 'success',
  rejected: 'default',
  snoozed: 'info',
}

const STATUS_RANK: Record<string, number> = { proposed: 0, snoozed: 1, approved: 2, rejected: 3 }

const FILTERS = ['proposed', 'snoozed', 'approved', 'rejected'] as const

function ageLabel(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  if (ms < 0) return 'just now'
  const m = Math.floor(ms / 60000)
  if (m < 1) return 'just now'
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h`
  const d = Math.floor(h / 24)
  if (d < 30) return `${d}d`
  const mo = Math.floor(d / 30)
  if (mo < 12) return `${mo}mo`
  return `${Math.floor(mo / 12)}y`
}

function proposalDisplayTitle(title: string, body: string): string {
  if (title) return title
  if (body) return body.slice(0, 60) + (body.length > 60 ? '…' : '')
  return '(no title)'
}

export function ProposalsTab({
  slug,
  statusFilter,
  onStatusFilter,
}: {
  slug: string
  statusFilter: string | null
  onStatusFilter: (status: string | null) => void
}) {
  const { data, isLoading } = useProposals(slug, statusFilter ?? undefined)

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const items: ProposalItem[] = data ?? []

  const allCountsByStatus = items.reduce((acc, p) => {
    acc[p.status] = (acc[p.status] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {items.length} proposals{statusFilter ? ` — filtered: ${statusFilter}` : ''}
        </Typography>
        <Stack direction="row" spacing={0.5} sx={{ flexWrap: 'wrap' }}>
          {FILTERS
            .slice()
            .sort((a, b) => (STATUS_RANK[a] ?? 99) - (STATUS_RANK[b] ?? 99))
            .map(status => {
              const count = statusFilter === status ? items.length : (allCountsByStatus[status] ?? 0)
              const isActive = statusFilter === status
              return (
                <Chip
                  key={status}
                  label={isActive ? `${status} (${count})` : status}
                  size="small"
                  color={STATUS_COLORS[status] || 'default'}
                  variant={isActive ? 'filled' : 'outlined'}
                  onClick={() => onStatusFilter(isActive ? null : status)}
                  sx={{ cursor: 'pointer' }}
                />
              )
            })}
          {statusFilter && (
            <Chip
              label="clear"
              size="small"
              variant="outlined"
              onClick={() => onStatusFilter(null)}
              sx={{ cursor: 'pointer' }}
            />
          )}
        </Stack>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map(p => (
          <Card key={p.id} variant="outlined">
            <CardActionArea
              component={Link as any}
              to="/project/$slug/proposals/$id"
              params={{ slug, id: String(p.id) }}
            >
              <CardContent sx={{ py: 1.25, display: 'flex', alignItems: 'center', gap: 1 }}>
                <Typography variant="caption" color="text.secondary" sx={{ minWidth: 56 }}>
                  #{p.id}
                </Typography>
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {proposalDisplayTitle(p.title, p.body)}
                </Typography>
                {p.source_type && (
                  <Chip
                    label={p.source_type}
                    size="small"
                    variant="outlined"
                    sx={{ maxWidth: 160 }}
                  />
                )}
                <Typography variant="caption" color="text.secondary" sx={{ minWidth: 40, textAlign: 'right' }}>
                  {ageLabel(p.created_at)}
                </Typography>
                <Chip
                  label={p.status}
                  size="small"
                  color={STATUS_COLORS[p.status] || 'default'}
                  variant="outlined"
                />
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
        {items.length === 0 && !isLoading && (
          <Typography variant="body2" color="text.secondary">No proposals.</Typography>
        )}
      </Box>
    </Box>
  )
}
