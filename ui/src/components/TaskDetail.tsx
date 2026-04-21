import { useEffect } from 'react'
import { Link, useRouter } from '@tanstack/react-router'
import { Box, Button, Chip, Tooltip, Typography } from '@mui/material'
import { ArrowBack as ArrowBackIcon, CheckCircle as CheckCircleIcon, LockOpen as LockOpenIcon, PlayArrow as PlayArrowIcon, RadioButtonUnchecked as RadioButtonUncheckedIcon } from '@mui/icons-material'
import { useTask, useTasks, useUpdateTaskStatus, useReleaseTask, useReadyTask, useTaskCodeRefs, useReservationsByTask } from '../api'
import { BlockerQuestionsSection } from './BlockerQuestionsSection'
import { CommentsAndRevisions } from './CommentsAndRevisions'
import { TaskReviewSection } from './TaskReviewSection'
import { CodeRefs } from './CodeRefs'
import { MarkdownContent } from './MarkdownContent'
import { EditHistory } from './EditHistory'
import { ReservationSection } from './ReservationSection'

const STATUS_COLORS: Record<string, 'success' | 'error' | 'warning' | 'default'> = {
  done: 'success',
  blocked: 'error',
}

export function TaskDetail({
  slug,
  taskId,
}: {
  slug: string
  taskId: string
}) {
  const { data: taskData, isLoading } = useTask(slug, taskId)
  const { data: allTasksData } = useTasks(slug)
  const { data: codeRefs } = useTaskCodeRefs(slug, taskId)
  const { data: reservationsData } = useReservationsByTask(slug, taskId)
  const updateStatus = useUpdateTaskStatus()
  const releaseTask = useReleaseTask()
  const readyTask = useReadyTask()
  const router = useRouter()

  const task = taskData
  const numericId = parseInt(taskId.replace(/^TK-/i, ''), 10)

  useEffect(() => {
    if (!task) return
    const headline = task.title || task.text
    document.title = `${taskId}: ${headline} | zdx`
    return () => { document.title = 'zdx' }
  }, [taskId, task?.title, task?.text])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const allTasks = allTasksData?.tasks ?? []
  const blockedByThis = allTasks.filter(t =>
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
        {taskId}: {task.title || task.text}
      </Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip
          label={task.status || 'open'}
          size="small"
          color={STATUS_COLORS[task.status] || 'default'}
          variant="outlined"
        />
        {task.feature && (
          <Chip
            label={task.feature}
            size="small"
            variant="outlined"
            component={Link as any}
            to="/project/$slug/features/$name"
            params={{ slug, name: task.feature }}
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
            to="/project/$slug/issues/$id"
            params={{ slug, id: `IS-${task.issue_id}` }}
            clickable
          />
        )}
        {task.task_group && (
          <Chip
            label={task.task_group}
            size="small"
            variant="outlined"
            color="default"
          />
        )}
        {task.status === 'wip' && (
          <Button
            size="small"
            variant="contained"
            color="primary"
            startIcon={<PlayArrowIcon />}
            onClick={() => readyTask.mutate({ slug, id: task.id })}
            disabled={readyTask.isPending}
          >
            Make ready
          </Button>
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
        ) : task.status !== 'wip' ? (
          <Button
            size="small"
            variant="contained"
            startIcon={<CheckCircleIcon />}
            onClick={() => updateStatus.mutate({ slug, id: task.id, status: 'done', reason: '' })}
            disabled={updateStatus.isPending}
          >
            Mark done
          </Button>
        ) : null}
        {task.claimed_by && task.status !== 'done' && (
          <Tooltip title={`Claimed by ${task.claimed_by}. Release returns the task to ready.`}>
            <Button
              size="small"
              color="warning"
              startIcon={<LockOpenIcon />}
              onClick={() => releaseTask.mutate({ slug, taskId: `TK-${task.id}` })}
              disabled={releaseTask.isPending}
            >
              Release claim
            </Button>
          </Tooltip>
        )}
      </Box>

      {task.claimed_by && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Claimed
          </Typography>
          <Typography variant="body2">
            {task.claimed_by}
            {task.claimed_at && ` · since ${new Date(task.claimed_at).toLocaleString()}`}
            {task.lease_expires_at && ` · lease expires ${new Date(task.lease_expires_at).toLocaleString()}`}
          </Typography>
        </Box>
      )}

      {task.title && task.text && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Implementation Plan
          </Typography>
          <MarkdownContent slug={slug}>{task.text}</MarkdownContent>
        </Box>
      )}

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
          <MarkdownContent slug={slug}>{task.test_plan}</MarkdownContent>
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
                    to="/project/$slug/tasks/$id"
                    params={{ slug, id: ref.toUpperCase() }}
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
                to="/project/$slug/tasks/$id"
                params={{ slug, id: `TK-${t.id}` }}
                clickable
              />
            ))}
          </Box>
        </Box>
      )}

      {task.completed_at && (
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
          Completed: {new Date(task.completed_at).toLocaleString()}
        </Typography>
      )}

      <ReservationSection slug={slug} reservations={reservationsData?.reservations ?? []} />

      <BlockerQuestionsSection slug={slug} targetType="task" targetId={taskId} />

      <TaskReviewSection slug={slug} taskId={taskId} />

      <CodeRefs refs={codeRefs ?? []} slug={slug} />

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3, mb: 3 }}>
        Created: {new Date(task.created_at).toLocaleString()}
      </Typography>

      <EditHistory slug={slug} targetType="task" targetId={taskId} />

      <CommentsAndRevisions slug={slug} targetType="task" targetId={taskId} />
    </Box>
  )
}
