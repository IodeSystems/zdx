import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Typography,
} from '@mui/material'
import { useListPlans, type PlanItem } from '../../../../api'

const STATUS_COLORS: Record<string, 'warning' | 'success' | 'default' | 'info'> = {
  draft: 'default',
  active: 'info',
  done: 'success',
  archived: 'default',
}

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

function PlansIndexRoute() {
  const { slug } = Route.useParams()
  const { data, isLoading } = useListPlans(slug)

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const items: PlanItem[] = data ?? []

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {items.length} plan{items.length === 1 ? '' : 's'}
        </Typography>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map(p => (
          <Card key={p.id} variant="outlined">
            <CardActionArea
              component={Link as any}
              to="/project/$slug/plans/$id"
              params={{ slug, id: String(p.id) }}
            >
              <CardContent sx={{ py: 1.25 }}>
                <Typography variant="body2" sx={{ mb: 0.5 }}>
                  {p.title || '(no title)'}
                </Typography>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
                  <Typography variant="caption" color="text.secondary">
                    #{p.id}
                  </Typography>
                  {p.plan_type && (
                    <Chip label={p.plan_type} size="small" variant="outlined" />
                  )}
                  {p.complexity && (
                    <Chip label={p.complexity} size="small" variant="outlined" />
                  )}
                  <Box sx={{ flex: 1 }} />
                  <Typography variant="caption" color="text.secondary">
                    {ageLabel(p.created_at)}
                  </Typography>
                  <Chip
                    label={p.status}
                    size="small"
                    color={STATUS_COLORS[p.status] || 'default'}
                    variant="outlined"
                  />
                </Box>
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
        {items.length === 0 && !isLoading && (
          <Typography variant="body2" color="text.secondary">No plans.</Typography>
        )}
      </Box>
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/plans/')({
  component: PlansIndexRoute,
})
