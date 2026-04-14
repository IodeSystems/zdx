import { Link, useRouter } from '@tanstack/react-router'
import { Box, Button, Chip, Typography } from '@mui/material'
import { ArrowBack as ArrowBackIcon, CheckCircle as CheckCircleIcon, RadioButtonUnchecked as RadioButtonUncheckedIcon } from '@mui/icons-material'
import { useTasks, useUpdateTaskStatus, useTaskCodeRefs } from '../api'
import { CommentsAndRevisions } from './CommentsAndRevisions'
import { CodeRefs } from './CodeRefs'

const STATUS_COLORS: Record<string, 'success' | 'error' | 'warning' | 'default'> = {
  done: 'success',
  blocked: 'error',
}

export function TaskDetail({
  slug,
  componentSlug,
  taskId,
}: {
  slug: string
  componentSlug: string
  taskId: string
}) {
  const { data, isLoading } = useTasks(slug)
  const { data: codeRefs } = useTaskCodeRefs(slug, taskId)
  const updateStatus = useUpdateTaskStatus()
  const router = useRouter()

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const tasks = data?.tasks ?? []
  // taskId is "TK-N" format; TaskItem.id is numeric
  const numericId = parseInt(taskId.replace(/^TK-/i, ''), 10)
  const task = tasks.find(t => t.id === numericId)
  const blockedByThis = tasks.filter(t =>
    t.id !== numericId &&
    t.depends
      .split(/[\s,]+/)
      .filter(Boolean)
      .some(ref => ref.toUpperCase() === taskId.toUpperCase())
  )

  if (!task) {
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
        <Typography color="error">Task {taskId} not found.</Typography>
      </Box>
    )
  }

  return (
    <Box>
      <Button
        startIcon={<ArrowBackIcon />}
        size="small"
        sx={{ mb: 2 }}
        onClick={() => router.history.go(-1)}
      >
        Back to tasks
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        {task.id}: {task.text}
      </Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip
          label={task.status || 'open'}
          size="small"
          color={STATUS_COLORS[task.status] || 'default'}
          variant="outlined"
        />
        {componentSlug && componentSlug !== 'all' && (
          <Chip
            label={componentSlug}
            size="small"
            variant="outlined"
            color="secondary"
            component={Link as any}
            to="/project/$slug/$component"
            params={{ slug, component: componentSlug }}
            clickable
          />
        )}
        {task.feature && (
          <Chip
            label={task.feature}
            size="small"
            variant="outlined"
            component={Link as any}
            to="/project/$slug/$component/features/$name"
            params={{ slug, component: componentSlug, name: task.feature }}
            clickable
          />
        )}
        {task.issue_id && (
          <Chip
            label={`IS-${task.issue_id}`}
            size="small"
            variant="outlined"
            color="info"
            component={Link as any}
            to="/project/$slug/$component/issues/$id"
            params={{ slug, component: componentSlug, id: `IS-${task.issue_id}` }}
            clickable
          />
        )}
        {task.status === 'done' ? (
          <Button
            size="small"
            startIcon={<RadioButtonUncheckedIcon />}
            onClick={() => updateStatus.mutate({ slug, id: task.id, status: 'open', reason: '' })}
            disabled={updateStatus.isPending}
          >
            Mark undone
          </Button>
        ) : (
          <Button
            size="small"
            variant="contained"
            startIcon={<CheckCircleIcon />}
            onClick={() => updateStatus.mutate({ slug, id: task.id, status: 'done', reason: '' })}
            disabled={updateStatus.isPending}
          >
            Mark done
          </Button>
        )}
      </Box>

      {task.reason && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Reason
          </Typography>
          <Typography variant="body2" color="warning.main">
            {task.reason}
          </Typography>
        </Box>
      )}

      {task.test_plan && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Test Plan
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
            {task.test_plan}
          </Typography>
        </Box>
      )}

      {task.test_refs && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Test Refs
          </Typography>
          <Typography variant="body2">{task.test_refs}</Typography>
        </Box>
      )}

      {task.depends && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Blockers
          </Typography>
          <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
            {task.depends.split(/[\s,]+/).filter(Boolean).map(ref => {
              const tkMatch = ref.match(/^TK-(\d+)$/i)
              if (tkMatch) {
                return (
                  <Chip
                    key={ref}
                    label={ref.toUpperCase()}
                    size="small"
                    color="warning"
                    variant="outlined"
                    component={Link as any}
                    to="/project/$slug/$component/tasks/$id"
                    params={{ slug, component: componentSlug, id: ref.toUpperCase() }}
                    clickable
                  />
                )
              }
              return <Chip key={ref} label={ref} size="small" variant="outlined" />
            })}
          </Box>
        </Box>
      )}

      {blockedByThis.length > 0 && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Blocking
          </Typography>
          <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
            {blockedByThis.map(t => (
              <Chip
                key={t.id}
                label={`TK-${t.id}`}
                size="small"
                color="default"
                variant="outlined"
                component={Link as any}
                to="/project/$slug/$component/tasks/$id"
                params={{ slug, component: componentSlug, id: `TK-${t.id}` }}
                clickable
              />
            ))}
          </Box>
        </Box>
      )}

      {task.completed_at && (
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
          Completed: {task.completed_at}
        </Typography>
      )}

      <CodeRefs refs={codeRefs ?? []} />

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3, mb: 3 }}>
        Created: {task.created_at}
      </Typography>

      <CommentsAndRevisions slug={slug} targetType="task" targetId={taskId} />
    </Box>
  )
}
