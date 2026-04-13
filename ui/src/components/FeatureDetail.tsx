import { Link, useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useFeature, useTasks, type TaskItem, type SpecItem } from '../api'

type FeatureTask = TaskItem
type Spec = SpecItem

function TaskIcon({ status }: { status: string }) {
  const icon = status === 'done' ? '\u2713' : status === 'blocked' ? '\u2717' : '\u25CB'
  const color = status === 'done' ? 'success.main' : status === 'blocked' ? 'error.main' : 'warning.main'
  return <Typography component="span" sx={{ color, fontWeight: 600, mr: 1 }}>{icon}</Typography>
}

export function FeatureDetail({
  slug,
  componentSlug,
  name,
}: {
  slug: string
  componentSlug: string
  name: string
}) {
  const { data: feature, isLoading } = useFeature(slug, name)
  const { data: tasksData } = useTasks(slug, { feature: name })
  const router = useRouter()

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const tasks: FeatureTask[] = tasksData ?? []
  const specList: Spec[] = feature?.specs ?? []

  if (!feature) {
    return (
      <Box>
        <Button
          startIcon={<ArrowBackIcon />}
          size="small"
          sx={{ mb: 2 }}
          onClick={() => router.history.go(-1)}
        >
          Back
        </Button>
        <Typography color="error">Feature {name} not found.</Typography>
      </Box>
    )
  }

  // Group tasks by status for a quick visual breakdown.
  const grouped = tasks.reduce((acc, t) => {
    const k = t.status || 'pending'
    ;(acc[k] ||= []).push(t)
    return acc
  }, {} as Record<string, FeatureTask[]>)

  return (
    <Box>
      <Button
        component={Link as any}
        to="/project/$slug/$component/features"
        params={{ slug, component: componentSlug }}
        startIcon={<ArrowBackIcon />}
        size="small"
        sx={{ mb: 2 }}
      >
        Back to features
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        {feature.name}
      </Typography>

      {feature.component && (
        <Box sx={{ mb: 1 }}>
          <Chip label={feature.component} size="small" variant="outlined" />
        </Box>
      )}

      {feature.what && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            What
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>{feature.what}</Typography>
        </Box>
      )}

      {feature.why && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Why
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>{feature.why}</Typography>
        </Box>
      )}

      {feature.done_when && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Done when
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>{feature.done_when}</Typography>
        </Box>
      )}

      {feature.description && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Description
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>{feature.description}</Typography>
        </Box>
      )}

      {specList.length > 0 && (
        <Box sx={{ mb: 2, mt: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Specs ({specList.length})
          </Typography>
          {specList.map((s: Spec) => (
            <Box key={s.id} sx={{ py: 0.75, borderBottom: 1, borderColor: 'divider' }}>
              <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
                <Chip label={s.kind} size="small" variant="outlined" color="info" />
                <Typography variant="body2">{s.description}</Typography>
              </Box>
            </Box>
          ))}
        </Box>
      )}

      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1, mt: 2 }}>
        Tasks ({tasks.length})
      </Typography>
      {tasks.length === 0 ? (
        <Typography variant="body2" color="text.secondary">No tasks.</Typography>
      ) : (
        Object.entries(grouped).map(([status, sTasks]) => (
          <Box key={status} sx={{ mb: 2 }}>
            <Typography variant="caption" color="text.secondary" sx={{ textTransform: 'uppercase', display: 'block', mb: 0.5 }}>
              {status} ({sTasks.length})
            </Typography>
            {sTasks.map(t => (
              <Box key={t.id} sx={{ display: 'flex', alignItems: 'flex-start', py: 0.5, borderBottom: 1, borderColor: 'divider' }}>
                <TaskIcon status={t.status} />
                <Box sx={{ flex: 1 }}>
                  <Link
                    to="/project/$slug/$component/tasks/$id"
                    params={{ slug, component: componentSlug, id: `TK-${t.id}` }}
                    style={{ textDecoration: 'none', color: 'inherit' }}
                  >
                    <Typography variant="body2">{t.text}</Typography>
                  </Link>
                  {t.reason && (
                    <Typography variant="caption" color="warning.main">{t.reason}</Typography>
                  )}
                </Box>
              </Box>
            ))}
          </Box>
        ))
      )}
    </Box>
  )
}
