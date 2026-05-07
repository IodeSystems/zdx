import { Link, useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Typography,
} from '@mui/material'
import {
  ArrowBack as ArrowBackIcon,
} from '@mui/icons-material'
import { useGoals, type GoalItem } from '../api'
import { EventStream } from './EventStream'

const STATUS_COLORS: Record<string, 'success' | 'warning' | 'default'> = {
  active: 'success',
  paused: 'warning',
  archived: 'default',
}

function fmtTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

export function GoalDetail({ slug, goalId }: { slug: string; goalId: number }) {
  const router = useRouter()
  const { data: goals, isLoading } = useGoals(slug)

  const goal = goals?.find((g: GoalItem) => g.id === goalId)

  if (isLoading) {
    return <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}><CircularProgress /></Box>
  }

  if (!goal) {
    return (
      <Box sx={{ p: 2 }}>
        <Link
          to="/project/$slug/goals"
          params={{ slug }}
          style={{ textDecoration: 'none' }}
        >
          <Button size="small" startIcon={<ArrowBackIcon fontSize="small" />}>Back</Button>
        </Link>
        <Typography sx={{ mt: 2, color: 'text.secondary' }}>Goal #{goalId} not found.</Typography>
      </Box>
    )
  }

  return (
    <Box sx={{ maxWidth: 720, mx: 'auto', p: 2 }}>
      <Box sx={{ mb: 2 }}>
        <Button
          size="small"
          startIcon={<ArrowBackIcon fontSize="small" />}
          onClick={() => router.history.go(-1)}
        >
          Back
        </Button>
      </Box>

      <Box sx={{ mb: 3 }}>
        <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>
          Goal #{goal.id}: {goal.title}
        </Typography>
        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 1 }}>
          <Chip
            label={goal.status}
            size="small"
            color={STATUS_COLORS[goal.status] ?? 'default'}
          />
          <Chip label={`P${goal.priority}`} size="small" variant="outlined" />
        </Box>
        {goal.description && (
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            {goal.description}
          </Typography>
        )}
        <Typography variant="caption" color="text.disabled">
          {goal.created_at && `Created ${fmtTime(goal.created_at)}`}
          {goal.created_at && goal.updated_at ? ' · ' : ''}
          {goal.updated_at && `Updated ${fmtTime(goal.updated_at)}`}
        </Typography>
      </Box>

      <EventStream slug={slug} targetType="goal" targetId={String(goal.id)} />
    </Box>
  )
}
