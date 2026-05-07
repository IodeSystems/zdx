import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Paper,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  Refresh as RefreshIcon,
  ArrowUpward as BumpIcon,
} from '@mui/icons-material'
import {
  useSolo,
  useBlockerQuestions,
  useEvaluateSolo,
  useApplySolo,
  useUnblockAllTodos,
  useSetTodoPriority,
  type SoloItem,
  type EvaluateDiff,
} from '../api'
import { MarkdownContent } from './MarkdownContent'
import { ChoiceAnswerForm } from './ChoiceAnswerForm'

const KIND_COLORS: Record<string, 'error' | 'warning' | 'info' | 'default'> = {
  triage: 'error',
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

export function TodoRow({ slug, item }: { slug: string; item: SoloItem }) {
  const link = targetLink(slug, item.target_type, item.target_id)
  const todoHref = `/project/${slug}/todos/${encodeURIComponent(item.key)}`
  const hasFooter = (item.issue_ref && item.issue_ref !== item.target_id) || item.blocked || item.persona
  const setPriority = useSetTodoPriority()
  const [priorityOpen, setPriorityOpen] = useState(false)
  const [priorityDraft, setPriorityDraft] = useState('')
  return (
    <Box sx={{ py: 0.75, borderBottom: 1, borderColor: 'divider' }}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mb: 0.5, alignItems: 'center' }}>
        <Chip
          label={item.kind}
          size="small"
          color={KIND_COLORS[item.kind] || 'default'}
          variant="outlined"
        />
        <Chip label={`p${item.priority}`} size="small" variant="outlined" />
        <Tooltip title="Bump priority (lower = earlier; survives re-evaluate)">
          <span>
            <IconButton
              size="small"
              onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                setPriorityDraft(String(Math.max(0, item.priority - 1)))
                setPriorityOpen(true)
              }}
              disabled={setPriority.isPending}
              sx={{ p: 0.25 }}
            >
              <BumpIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        {link ? (
          <Chip
            label={item.target_id}
            size="small"
            variant="outlined"
            color="primary"
            component={Link as any}
            to={link.to}
            params={link.params}
            clickable
          />
        ) : item.target_id ? (
          <Chip label={item.target_id} size="small" variant="outlined" />
        ) : null}
      </Box>
      <Box
        component="a"
        href={todoHref}
        sx={{ display: 'block', textDecoration: 'none', color: 'inherit', '&:hover .todo-title': { textDecoration: 'underline' } }}
      >
        <Typography variant="body2" className="todo-title">
          {item.title || item.text}
        </Typography>
        {item.description && (
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
            {item.description}
          </Typography>
        )}
      </Box>
      {hasFooter && (
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mt: 0.5 }}>
          {item.issue_ref && item.issue_ref !== item.target_id && (
            <Chip
              label={item.issue_ref}
              size="small"
              variant="outlined"
              color="info"
              component={Link as any}
              to="/project/$slug/issues/$id"
              params={{ slug, id: item.issue_ref }}
              clickable
            />
          )}
          {item.blocked && item.reference_issue_id ? (
            <Chip
              label={item.blocked_reason ? `blocked: ${item.blocked_reason}` : `blocked → ${item.reference_issue_id}`}
              size="small"
              color="error"
              variant="outlined"
              component={Link as any}
              to="/project/$slug/issues/$id"
              params={{ slug, id: item.reference_issue_id }}
              clickable
            />
          ) : item.blocked ? (
            <Chip
              label={item.blocked_reason ? `blocked: ${item.blocked_reason}` : 'blocked'}
              size="small"
              color="error"
              variant="outlined"
            />
          ) : null}
          {item.persona && <Chip label={item.persona} size="small" variant="outlined" sx={{ fontSize: '0.7rem' }} />}
        </Box>
      )}
      <Dialog open={priorityOpen} onClose={() => setPriorityOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Bump priority</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Lower number = earlier in the claim queue. The next solo
            re-evaluate keeps this value (LEAST clause), so the bump sticks.
            Current: <strong>{item.priority}</strong>.
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
                { slug, key: item.key, priority: n },
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

function EvaluateDialog({ open, diff, onClose, onApply, applying }: {
  open: boolean
  diff: EvaluateDiff | null
  onClose: () => void
  onApply: () => void
  applying: boolean
}) {
  if (!diff) return null
  const hasChanges = (diff.added?.length ?? 0) > 0 || (diff.removed?.length ?? 0) > 0 || (diff.changed?.length ?? 0) > 0
  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>Queue Re-evaluation</DialogTitle>
      <DialogContent>
        {!hasChanges ? (
          <Typography color="text.secondary">No changes — queue is up to date ({diff.unchanged?.length ?? 0} items).</Typography>
        ) : (
          <Box>
            {diff.added.length > 0 && (
              <Box sx={{ mb: 2 }}>
                <Typography variant="subtitle2" color="success.main" sx={{ mb: 0.5 }}>
                  + {diff.added.length} new
                </Typography>
                <List dense disablePadding>
                  {diff.added.map(a => (
                    <ListItem key={a.key} disablePadding sx={{ py: 0.25 }}>
                      <Chip label={a.kind} size="small" sx={{ mr: 1 }} color={KIND_COLORS[a.kind] || 'default'} />
                      <ListItemText primary={a.title || a.text} secondary={a.target_id} />
                    </ListItem>
                  ))}
                </List>
              </Box>
            )}
            {diff.removed.length > 0 && (
              <Box sx={{ mb: 2 }}>
                <Typography variant="subtitle2" color="error.main" sx={{ mb: 0.5 }}>
                  - {diff.removed.length} removed
                </Typography>
                <List dense disablePadding>
                  {diff.removed.map(r => (
                    <ListItem key={r.key} disablePadding sx={{ py: 0.25 }}>
                      <Chip label={r.kind} size="small" sx={{ mr: 1 }} variant="outlined" />
                      <ListItemText primary={r.title || r.text} secondary={r.target_id} />
                    </ListItem>
                  ))}
                </List>
              </Box>
            )}
            {diff.changed.length > 0 && (
              <Box sx={{ mb: 2 }}>
                <Typography variant="subtitle2" color="warning.main" sx={{ mb: 0.5 }}>
                  ~ {diff.changed.length} changed
                </Typography>
                <List dense disablePadding>
                  {diff.changed.map(ch => (
                    <ListItem key={ch.after.key} disablePadding sx={{ py: 0.25 }}>
                      <Chip label={ch.after.kind} size="small" sx={{ mr: 1 }} color="warning" />
                      <ListItemText primary={ch.after.title || ch.after.text} secondary={`priority: ${ch.before.priority} → ${ch.after.priority}`} />
                    </ListItem>
                  ))}
                </List>
              </Box>
            )}
            <Typography variant="body2" color="text.secondary">
              {diff.unchanged?.length ?? 0} unchanged items
            </Typography>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        {hasChanges && (
          <Button onClick={onApply} variant="contained" disabled={applying}>
            {applying ? 'Applying...' : 'Apply Changes'}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  )
}

export function QueueView({ slug }: { slug: string }) {
  const { data, isLoading } = useSolo(slug)
  const { data: bqData } = useBlockerQuestions(slug, 'pending')
  const evaluate = useEvaluateSolo()
  const apply = useApplySolo()
  const unblockAll = useUnblockAllTodos()
  const [showBlocked, setShowBlocked] = useState(false)
  const [diffOpen, setDiffOpen] = useState(false)
  const [diff, setDiff] = useState<EvaluateDiff | null>(null)

  const handleEvaluate = async () => {
    const result = await evaluate.mutateAsync({ slug })
    setDiff(result)
    setDiffOpen(true)
  }

  const handleApply = async () => {
    if (!diff) return
    const items = [
      ...(diff.added ?? []),
      ...(diff.changed ?? []).map(ch => ch.after),
      ...(diff.unchanged ?? []),
    ]
    await apply.mutateAsync({ slug, items })
    setDiffOpen(false)
    setDiff(null)
  }

  if (isLoading) return <Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}><CircularProgress /></Box>

  const pendingQuestions = bqData?.questions ?? []
  const allItems: SoloItem[] = data ?? []
  const visibleItems = showBlocked ? allItems : allItems.filter(i => !i.blocked)
  const blockedCount = allItems.filter(i => i.blocked).length
  const [pick, ...rest] = visibleItems

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        <Typography variant="h6">Queue</Typography>
        <Button
          size="small"
          startIcon={<RefreshIcon />}
          onClick={handleEvaluate}
          disabled={evaluate.isPending}
        >
          {evaluate.isPending ? 'Evaluating...' : 'Re-evaluate'}
        </Button>
        {blockedCount > 0 && (
          <>
            <FormControlLabel
              control={<Switch size="small" checked={showBlocked} onChange={(_, v) => setShowBlocked(v)} />}
              label={<Typography variant="body2">Show blocked ({blockedCount})</Typography>}
            />
            {showBlocked && (
              <Button
                size="small"
                color="warning"
                variant="outlined"
                onClick={() => unblockAll.mutate({ slug })}
                disabled={unblockAll.isPending}
              >
                {unblockAll.isPending ? 'Unblocking...' : 'Unblock all'}
              </Button>
            )}
          </>
        )}
        <Typography variant="body2" color="text.secondary" sx={{ ml: 'auto' }}>
          {allItems.length} items
        </Typography>
      </Box>

      {pendingQuestions.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="warning.main" sx={{ mb: 1 }}>
            Questions Blocking Progress ({pendingQuestions.length})
          </Typography>
          {pendingQuestions.map(q => (
            <Paper key={q.id} variant="outlined" sx={{ p: 2, mb: 1.5, borderColor: 'warning.main' }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                {(() => {
                  const link = targetLink(slug, q.target_type, q.target_id)
                  return link ? (
                    <Chip
                      label={`${q.target_type}:${q.target_id}`}
                      size="small"
                      color="primary"
                      variant="outlined"
                      component={Link as any}
                      to={link.to}
                      params={link.params}
                      clickable
                    />
                  ) : (
                    <Chip label={`${q.target_type}:${q.target_id}`} size="small" color="primary" variant="outlined" />
                  )
                })()}
                <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.75rem' }}>
                  BQ-{q.id}
                </Typography>
              </Box>
              <MarkdownContent slug={slug} variant="body1">{q.context}</MarkdownContent>
              <ChoiceAnswerForm slug={slug} question={q} />
            </Paper>
          ))}
        </Box>
      )}

      {!pick && pendingQuestions.length === 0 && (
        <Typography color="text.secondary">Nothing to do — all clear. Try re-evaluating to refresh.</Typography>
      )}

      {pick && (
        <Box sx={{ mb: 3, p: 2, border: 1, borderColor: 'primary.main', borderRadius: 1 }}>
          <TodoRow slug={slug} item={pick} />
        </Box>
      )}

      {rest.length > 0 && (
        <>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Upcoming ({rest.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column' }}>
            {rest.map((item) => (
              <TodoRow key={item.key} slug={slug} item={item} />
            ))}
          </Box>
        </>
      )}

      <EvaluateDialog
        open={diffOpen}
        diff={diff}
        onClose={() => setDiffOpen(false)}
        onApply={handleApply}
        applying={apply.isPending}
      />
    </Box>
  )
}
