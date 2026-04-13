import { useState } from 'react'
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
import { useIssues, type IssueResp } from '../api'

type Issue = IssueResp

function priorityLabel(p: string): string {
  if (!p) return 'untriaged'
  return { '1': 'urgent', '2': 'high', '3': 'medium', '4': 'low' }[p] ?? p
}

function issueDisplayTitle(title: string, context: string): string {
  if (title) return title
  if (context) return context.slice(0, 60) + (context.length > 60 ? '…' : '')
  return '(no title)'
}

const PRIORITY_COLORS: Record<string, 'error' | 'warning' | 'default' | 'info'> = {
  urgent: 'error',
  high: 'warning',
  medium: 'info',
  low: 'default',
}

const STATUS_COLORS: Record<string, 'warning' | 'info' | 'secondary' | 'success' | 'default'> = {
  open: 'warning',
  triaged: 'info',
  'in-progress': 'secondary',
  done: 'success',
  closed: 'success',
}

const STATUS_RANK: Record<string, number> = { open: 0, triaged: 1, 'in-progress': 2, done: 3, closed: 4 }

export function IssuesTab({ slug, componentSlug = 'all' }: { slug: string; componentSlug?: string }) {
  const [statusFilter, setStatusFilter] = useState<string | null>(null)
  const { data, isLoading } = useIssues(slug)

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const allItems: Issue[] = data ?? []

  const component = componentSlug === 'all' ? '' : componentSlug
  const componentFiltered = component ? allItems.filter(i => i.component === component) : allItems

  const items = statusFilter
    ? componentFiltered.filter(i => i.status === statusFilter)
    : componentFiltered

  const statusCounts = componentFiltered.reduce((acc, i) => {
    acc[i.status] = (acc[i.status] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  function toggleFilter(status: string) {
    setStatusFilter(prev => prev === status ? null : status)
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {statusFilter ? `${items.length} of ${componentFiltered.length}` : `${componentFiltered.length}`} issues
          {componentSlug !== 'all' && ` (component=${componentSlug})`}
          {statusFilter && ` — filtered: ${statusFilter}`}
        </Typography>
        <Stack direction="row" spacing={0.5} sx={{ flexWrap: 'wrap' }}>
          {Object.entries(statusCounts)
            .sort(([a], [b]) => (STATUS_RANK[a] ?? 99) - (STATUS_RANK[b] ?? 99))
            .map(([status, count]) => (
              <Chip
                key={status}
                label={`${status} (${count})`}
                size="small"
                color={STATUS_COLORS[status] || 'default'}
                variant={statusFilter === status ? 'filled' : 'outlined'}
                onClick={() => toggleFilter(status)}
                sx={{ cursor: 'pointer' }}
              />
            ))}
          {statusFilter && (
            <Chip
              label="clear"
              size="small"
              variant="outlined"
              onClick={() => setStatusFilter(null)}
              sx={{ cursor: 'pointer' }}
            />
          )}
        </Stack>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map(i => {
          const pLabel = priorityLabel(i.priority)
          return (
            <Card key={i.id} variant="outlined">
              <CardActionArea
                component={Link as any}
                to="/project/$slug/$component/issues/$id"
                params={{ slug, component: componentSlug, id: i.id }}
              >
                <CardContent sx={{ py: 1.25, display: 'flex', alignItems: 'center', gap: 1 }}>
                  <Chip
                    label={pLabel}
                    size="small"
                    color={PRIORITY_COLORS[pLabel] || 'default'}
                    sx={{ minWidth: 70 }}
                  />
                  <Typography variant="body2" sx={{ flex: 1 }}>
                    {i.id}: {issueDisplayTitle(i.title, i.context)}
                  </Typography>
                  <Chip
                    label={i.status}
                    size="small"
                    color={STATUS_COLORS[i.status] || 'default'}
                    variant="outlined"
                  />
                </CardContent>
              </CardActionArea>
            </Card>
          )
        })}
        {items.length === 0 && !isLoading && (
          <Typography variant="body2" color="text.secondary">No issues.</Typography>
        )}
      </Box>
    </Box>
  )
}
