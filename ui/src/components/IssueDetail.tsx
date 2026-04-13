import { Link, useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { useState } from 'react'
import {
  useIssue,
  useTasks,
  useCloseIssue,
  type IssueWorkItem,
  type TaskItem,
} from '../api'

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

export function IssueDetail({
  slug,
  componentSlug,
  issueId,
}: {
  slug: string
  componentSlug: string
  issueId: string
}) {
  const { data, isLoading } = useIssue(slug, issueId)
  const { data: allTasks } = useTasks(slug, { issue: issueId })
  const closeIssue = useCloseIssue()
  const router = useRouter()
  const [closeOpen, setCloseOpen] = useState(false)
  const [closeReason, setCloseReason] = useState('')

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const issue = data?.issue
  const linkedTasks: TaskItem[] = allTasks ?? []
  const workEntries: IssueWorkItem[] = data?.work ?? []

  if (!issue) {
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
        <Typography color="error">Issue {issueId} not found.</Typography>
      </Box>
    )
  }

  const pLabel = priorityLabel(issue.priority)

  return (
    <Box>
      <Button
        component={Link as any}
        to="/project/$slug/$component/issues"
        params={{ slug, component: componentSlug }}
        startIcon={<ArrowBackIcon />}
        size="small"
        sx={{ mb: 2 }}
      >
        Back to issues
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        {issue.id}: {issueDisplayTitle(issue.title, issue.context)}
      </Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip
          label={pLabel}
          size="small"
          color={PRIORITY_COLORS[pLabel] || 'default'}
        />
        <Chip
          label={issue.status}
          size="small"
          color={STATUS_COLORS[issue.status] || 'default'}
          variant="outlined"
        />
        {issue.component && (
          <Chip label={issue.component} size="small" variant="outlined" />
        )}
        {issue.blocked_by && (
          <Chip label={`blocked by: ${issue.blocked_by}`} size="small" variant="outlined" color="warning" />
        )}
        {(issue.status === 'open' || issue.status === 'triaged' || issue.status === 'in-progress') && (
          <Button size="small" variant="outlined" color="warning" onClick={() => { setCloseReason(''); setCloseOpen(true) }}>
            Close
          </Button>
        )}
      </Box>

      <Dialog open={closeOpen} onClose={() => setCloseOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Close {issue.id}</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            label="Reason"
            multiline
            rows={3}
            value={closeReason}
            onChange={e => setCloseReason(e.target.value)}
            sx={{ mt: 1 }}
            autoFocus
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCloseOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            color="warning"
            disabled={closeIssue.isPending || !closeReason.trim()}
            onClick={() => {
              closeIssue.mutate(
                { slug, id: issue.id, reason: closeReason },
                { onSuccess: () => setCloseOpen(false) },
              )
            }}
          >
            Close
          </Button>
        </DialogActions>
      </Dialog>

      {issue.context && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Context
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
            {issue.context}
          </Typography>
        </Box>
      )}

      {linkedTasks.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Tasks ({linkedTasks.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {linkedTasks.map(t => (
              <Box
                key={t.id}
                component={Link as any}
                to="/project/$slug/$component/tasks/$id"
                params={{ slug, component: componentSlug, id: t.id }}
                sx={{ display: 'flex', gap: 1, alignItems: 'center', textDecoration: 'none', color: 'inherit', '&:hover': { opacity: 0.8 } }}
              >
                <Chip label={t.status} size="small" color={t.status === 'done' ? 'success' : 'default'} variant="outlined" />
                <Typography variant="body2">
                  {t.id}: [{t.feature}] {t.text}
                </Typography>
              </Box>
            ))}
          </Box>
        </Box>
      )}

      {workEntries.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Work Log ({workEntries.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {workEntries.map((e, i) => (
              <Box key={i} sx={{ borderLeft: 2, borderColor: 'divider', pl: 1.5 }}>
                <Typography variant="caption" color="text.secondary">
                  {e.created_at} — {e.agent}
                </Typography>
                {e.note && (
                  <Typography variant="body2">{e.note}</Typography>
                )}
              </Box>
            ))}
          </Box>
        </Box>
      )}

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3 }}>
        Created: {issue.created_at}
      </Typography>
    </Box>
  )
}
