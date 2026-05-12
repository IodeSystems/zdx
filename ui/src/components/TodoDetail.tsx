import { useState } from 'react'
import { Link, useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  ArrowBack as ArrowBackIcon,
  ArrowUpward as BumpIcon,
} from '@mui/icons-material'
import { useTodoDetail, useCreateIssue, useSetTodoPriority } from '../api'
import { MarkdownContent } from './MarkdownContent'

const KIND_COLORS: Record<string, 'error' | 'warning' | 'info' | 'default'> = {
  'product:triage': 'error',
  plan: 'info',
  dev: 'warning',
  closable: 'info',
  'read:comments': 'default',
  clarify: 'error',
  add: 'warning',
}

function targetLink(slug: string, targetType: string, targetId: string): { to: string; params: Record<string, string> } | null {
  if (targetType === 'issue') return { to: '/project/$slug/issues/$id', params: { slug, id: targetId } }
  if (targetType === 'task') return { to: '/project/$slug/tasks/$id', params: { slug, id: targetId } }
  if (targetType === 'feature') return { to: '/project/$slug/features/$name', params: { slug, name: targetId } }
  return null
}

export function TodoDetail({ slug, todoKey }: { slug: string; todoKey: string }) {
  const router = useRouter()
  const { data, isLoading, error } = useTodoDetail(slug, todoKey)
  const createIssue = useCreateIssue()
  const setPriority = useSetTodoPriority()
  const [issueOpen, setIssueOpen] = useState(false)
  const [issueContext, setIssueContext] = useState('')
  const [createdIssueId, setCreatedIssueId] = useState<number | null>(null)
  const [priorityOpen, setPriorityOpen] = useState(false)
  const [priorityDraft, setPriorityDraft] = useState('')

  if (isLoading) {
    return <Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}><CircularProgress /></Box>
  }
  if (error || !data) {
    return <Typography color="error">Todo not found: {todoKey}</Typography>
  }

  const { todo, reservations, sessions } = data
  const link = targetLink(slug, todo.target_type, todo.target_id)

  const handleOpenIssue = () => {
    const headline = todo.title ? `${todo.title}\n\n` : ''
    setIssueContext(`Reported against todo ${todo.key}\n\n${headline}Todo text:\n${todo.text}\n\nWhat is ambiguous or unclear: `)
    setCreatedIssueId(null)
    setIssueOpen(true)
  }

  const handleSubmitIssue = () => {
    createIssue.mutate(
      { slug, context: issueContext.trim() || undefined, auto_ready: true },
      {
        onSuccess: (issue) => {
          setCreatedIssueId(issue.id)
          setIssueContext('')
        },
      },
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
        Back
      </Button>

      <Typography variant="overline" color="text.secondary">Todo {todo.key}</Typography>
      <Typography variant="h5" sx={{ mb: 1 }}>{todo.title || todo.text}</Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, flexWrap: 'wrap', alignItems: 'center' }}>
        <Chip label={todo.kind} size="small" color={KIND_COLORS[todo.kind] || 'default'} variant="outlined" />
        <Chip label={`priority ${todo.priority}`} size="small" variant="outlined" />
        <Tooltip title="Bump priority (lower number = higher claim order; survives re-evaluate)">
          <span>
            <IconButton
              size="small"
              onClick={() => {
                setPriorityDraft(String(Math.max(1, todo.priority - 1)))
                setPriorityOpen(true)
              }}
              disabled={setPriority.isPending}
              sx={{ p: 0.25 }}
            >
              <BumpIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        {todo.persona && <Chip label={todo.persona} size="small" variant="outlined" />}
        {todo.blocked && todo.reference_issue_id ? (
          <Chip
            label={todo.blocked_reason ? `blocked: ${todo.blocked_reason}` : `blocked → ${todo.reference_issue_id}`}
            size="small"
            color="error"
            variant="outlined"
            component={Link as any}
            to="/project/$slug/issues/$id"
            params={{ slug, id: todo.reference_issue_id }}
            clickable
          />
        ) : todo.blocked ? (
          <Chip
            label={todo.blocked_reason ? `blocked: ${todo.blocked_reason}` : 'blocked'}
            size="small"
            color="error"
            variant="outlined"
          />
        ) : null}
        {(todo.cycle_count ?? 0) > 0 && (
          <Chip label={`cycle ×${todo.cycle_count}`} size="small" color="warning" variant="outlined" />
        )}
        <Chip label={todo.status} size="small" variant="outlined" />
        {link ? (
          <Chip
            label={`${todo.target_type}: ${todo.target_id}`}
            size="small"
            color="primary"
            variant="outlined"
            component={Link as any}
            to={link.to}
            params={link.params}
            clickable
          />
        ) : todo.target_id ? (
          <Chip label={`${todo.target_type}: ${todo.target_id}`} size="small" variant="outlined" />
        ) : null}
        {todo.issue_ref && todo.issue_ref !== todo.target_id && (
          <Chip
            label={todo.issue_ref}
            size="small"
            color="info"
            variant="outlined"
            component={Link as any}
            to="/project/$slug/issues/$id"
            params={{ slug, id: todo.issue_ref }}
            clickable
          />
        )}
      </Box>

      {todo.description && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>Description</Typography>
          <MarkdownContent slug={slug} variant="body1">{todo.description}</MarkdownContent>
        </Box>
      )}

      {todo.instructions ? (
        <Box sx={{ mb: 3, p: 2, bgcolor: 'action.hover', borderRadius: 1 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>Instructions</Typography>
          <MarkdownContent slug={slug} variant="body2">{todo.instructions}</MarkdownContent>
        </Box>
      ) : (
        <Box sx={{ mb: 3, p: 2, border: 1, borderColor: 'divider', borderRadius: 1 }}>
          <MarkdownContent slug={slug} variant="body1">{todo.text}</MarkdownContent>
        </Box>
      )}

      <Box sx={{ mb: 3, display: 'flex', gap: 1 }}>
        <Button size="small" variant="outlined" onClick={handleOpenIssue}>
          File issue about this todo
        </Button>
      </Box>

      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Reservations ({reservations.length})
      </Typography>
      {reservations.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>No reservations yet.</Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, mb: 3 }}>
          {reservations.map(r => {
            const isActive = !r.released_at && new Date(r.lease_expires_at) > new Date()
            const statusLabel = r.released_at ? 'released' : isActive ? 'active' : 'expired'
            const statusColor = isActive ? 'success' : r.released_at ? 'default' : 'warning'
            const inner = (
              <>
                <Chip label={statusLabel} size="small" color={statusColor as any} variant="outlined" />
                {r.session_id && r.session_status && (
                  <Chip
                    label={r.session_status}
                    size="small"
                    color={r.session_status === 'ok' ? 'success' : r.session_status === 'errored' ? 'error' : r.session_status === 'churn' ? 'warning' : 'default'}
                    variant="filled"
                  />
                )}
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {r.session_header || r.claimed_by || '(unknown claimant)'}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {new Date(r.claimed_at).toLocaleString()}
                </Typography>
              </>
            )
            return r.session_id ? (
              <Box
                key={r.id}
                component={Link as any}
                to="/project/$slug/agents/$sessionId"
                params={{ slug, sessionId: String(r.session_id) }}
                sx={{ display: 'flex', gap: 1, alignItems: 'center', textDecoration: 'none', color: 'inherit', '&:hover': { opacity: 0.8 } }}
              >
                {inner}
              </Box>
            ) : (
              <Box key={r.id} sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                {inner}
              </Box>
            )
          })}
        </Box>
      )}

      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Agent Sessions ({sessions.length})
      </Typography>
      {sessions.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>No agent sessions linked to this todo.</Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, mb: 3 }}>
          {sessions.map(s => (
            <Box
              key={s.id}
              component={Link as any}
              to="/project/$slug/agents/$sessionId"
              params={{ slug, sessionId: String(s.id) }}
              sx={{ display: 'flex', gap: 1, alignItems: 'center', textDecoration: 'none', color: 'inherit', py: 0.5, borderBottom: 1, borderColor: 'divider', '&:hover': { opacity: 0.8 } }}
            >
              {s.status && (
                <Chip
                  label={s.status}
                  size="small"
                  color={s.status === 'ok' ? 'success' : s.status === 'errored' ? 'error' : s.status === 'churn' ? 'warning' : 'default'}
                  variant="filled"
                />
              )}
              {!s.status && s.closed_at && <Chip label="closed" size="small" variant="outlined" />}
              {!s.status && !s.closed_at && <Chip label="active" size="small" color="info" variant="outlined" />}
              {s.alias && <Chip label={s.alias} size="small" variant="outlined" />}
              <Typography variant="body2" sx={{ flex: 1 }}>
                {s.header || s.title || s.session_id}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {new Date(s.updated_at).toLocaleString()}
              </Typography>
            </Box>
          ))}
        </Box>
      )}

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3 }}>
        Created: {new Date(todo.created_at).toLocaleString()}
        {todo.resolved_at && <> — Resolved: {new Date(todo.resolved_at).toLocaleString()}</>}
      </Typography>

      <Dialog open={issueOpen} onClose={() => setIssueOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>File issue about this todo</DialogTitle>
        <DialogContent>
          {createdIssueId != null ? (
            <Box sx={{ py: 1 }}>
              <Typography variant="body2" sx={{ mb: 1 }}>Issue created:</Typography>
              <Chip
                label={`IS-${createdIssueId}`}
                color="primary"
                component={Link as any}
                to="/project/$slug/issues/$id"
                params={{ slug, id: `IS-${createdIssueId}` }}
                clickable
              />
            </Box>
          ) : (
            <TextField
              autoFocus
              fullWidth
              multiline
              rows={8}
              sx={{ mt: 1 }}
              value={issueContext}
              onChange={e => setIssueContext(e.target.value)}
              placeholder="Describe what is ambiguous or wrong about this todo..."
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setIssueOpen(false)}>
            {createdIssueId != null ? 'Close' : 'Cancel'}
          </Button>
          {createdIssueId == null && (
            <Button
              onClick={handleSubmitIssue}
              variant="contained"
              disabled={createIssue.isPending || !issueContext.trim()}
            >
              Create issue
            </Button>
          )}
        </DialogActions>
      </Dialog>

      <Dialog open={priorityOpen} onClose={() => setPriorityOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Bump priority</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Lower number = earlier in the claim queue. The next agent
            re-evaluate keeps this value (LEAST clause), so the bump sticks.
            Current: <strong>{todo.priority}</strong>.
          </Typography>
          <TextField
            autoFocus
            type="number"
            label="New priority"
            value={priorityDraft}
            onChange={e => setPriorityDraft(e.target.value)}
            fullWidth
            slotProps={{ htmlInput: { min: 0, max: 1000 } }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPriorityOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            disabled={setPriority.isPending || !priorityDraft.trim() || isNaN(Number(priorityDraft))}
            onClick={() => {
              const n = Math.max(0, Math.floor(Number(priorityDraft)))
              setPriority.mutate(
                { slug, key: todoKey, priority: n },
                { onSuccess: () => setPriorityOpen(false) },
              )
            }}
          >
            Bump
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
