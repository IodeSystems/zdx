import { Link, useRouter } from '@tanstack/react-router'
import {
  Autocomplete,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon, Edit as EditIcon } from '@mui/icons-material'
import { useCallback, useEffect, useState } from 'react'
import {
  useIssue,
  useTasks,
  useCloseIssue,
  useReadyIssue,
  useEditIssue,
  useSearchIssues,
  useIssueCodeRefs,
  useListFocuses,
  useAddFocusBlocker,
  useRemoveFocusBlocker,
  useAddComment,
  useReservationsByIssueID,
  type IssueItem,
  type IssueWorkItem,
  type TaskItem,
  type FocusItem,
} from '../api'
import { useChannel } from '../hooks/useChannel'
import { BlockerQuestionsSection } from './BlockerQuestionsSection'
import { CodeRefs } from './CodeRefs'
import { MarkdownContent } from './MarkdownContent'
import { UnifiedTimeline } from './UnifiedTimeline'
import { ReservationSection } from './ReservationSection'

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
  issueId,
}: {
  slug: string
  issueId: string
}) {
  const { data, isLoading, refetch } = useIssue(slug, issueId)
  const { data: allTasks, refetch: refetchTasks } = useTasks(slug, { issue: issueId })
  const { data: reservationsData } = useReservationsByIssueID(slug, issueId)
  const { data: codeRefs } = useIssueCodeRefs(slug, issueId)
  const closeIssue = useCloseIssue()
  const readyIssue = useReadyIssue()
  const editIssue = useEditIssue()
  const router = useRouter()
  const [closeOpen, setCloseOpen] = useState(false)
  const [editingTitle, setEditingTitle] = useState(false)
  const [editTitle, setEditTitle] = useState('')
  const [editingContext, setEditingContext] = useState(false)
  const [editContext, setEditContext] = useState('')
  const [previewContext, setPreviewContext] = useState(false)
  const [closeReason, setCloseReason] = useState('')
  const [duplicateOf, setDuplicateOf] = useState<IssueItem | null>(null)
  const [dupSearch, setDupSearch] = useState('')
  const { data: dupResults } = useSearchIssues(slug, dupSearch, closeReason === 'duplicate' && dupSearch.length > 1)
  const { data: allFocuses } = useListFocuses(slug)
  const addFocusBlocker = useAddFocusBlocker()
  const removeFocusBlocker = useRemoveFocusBlocker()
  const [focusInput, setFocusInput] = useState('')
  const addComment = useAddComment()
  const [commentBody, setCommentBody] = useState('')

  const onWsMessage = useCallback(() => {
    refetch()
    refetchTasks()
  }, [refetch, refetchTasks])
  useChannel(`issue:${issueId}`, onWsMessage)

  const issue = data?.issue
  const displayTitle = issue ? issueDisplayTitle(issue.title, issue.context) : ''
  useEffect(() => {
    if (!issue) return
    document.title = `${issueId}: ${displayTitle} | zdx`
    return () => { document.title = 'zdx' }
  }, [issueId, displayTitle, issue])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const linkedTasks: TaskItem[] = allTasks?.tasks ?? []
  const workEntries: IssueWorkItem[] = data?.work ?? []
  const blockingFocuses = (allFocuses ?? []).filter(t => t.blockers.split(',').includes(issueId))
  const blockingFocusIds = new Set(blockingFocuses.map(t => t.id))
  const availableFocuses = (allFocuses ?? []).filter(t => t.status !== 'archived' && !blockingFocusIds.has(t.id))

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
        startIcon={<ArrowBackIcon />}
        size="small"
        sx={{ mb: 2 }}
        onClick={() => router.history.go(-1)}
      >
        Back
      </Button>

      {editingTitle ? (
        <TextField
          fullWidth
          size="small"
          autoFocus
          value={editTitle}
          onChange={e => setEditTitle(e.target.value)}
          onBlur={() => {
            setEditingTitle(false)
            if (editTitle.trim() && editTitle !== issue.title) {
              editIssue.mutate({ slug, id: issue.id, title: editTitle.trim() })
            }
          }}
          onKeyDown={e => {
            if (e.key === 'Enter') { (e.target as HTMLInputElement).blur() }
            if (e.key === 'Escape') { setEditingTitle(false); setEditTitle(issue.title ?? '') }
          }}
          sx={{ mb: 1 }}
          slotProps={{ input: { style: { fontSize: '1.5rem', fontWeight: 400 } } }}
        />
      ) : (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mb: 1 }}>
          <Typography
            variant="h5"
            sx={issue.status === 'wip' ? { cursor: 'pointer', '&:hover': { opacity: 0.7 } } : undefined}
            onClick={issue.status === 'wip' ? () => { setEditTitle(issue.title ?? ''); setEditingTitle(true) } : undefined}
          >
            {issueId}: {displayTitle}
          </Typography>
          {issue.status !== 'wip' && ['open', 'triaged', 'in-progress'].includes(issue.status) && (
            <Tooltip title="Edit title (live issue)">
              <IconButton size="small" onClick={() => { setEditTitle(issue.title ?? ''); setEditingTitle(true) }}>
                <EditIcon fontSize="inherit" />
              </IconButton>
            </Tooltip>
          )}
        </Box>
      )}

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
        {issue.issue_type && (
          <Chip
            label={issue.issue_type}
            size="small"
            variant="outlined"
            color={
              issue.issue_type === 'impl' ? 'secondary' :
              issue.issue_type === 'ask' ? 'info' :
              'default'
            }
          />
        )}
        {((issue.blocked_by_detail ?? (issue.blocked_by ?? []).map((id: string) => ({ id, status: 'open' }))) as { id: string; status: string }[]).map((b) => {
          const closed = b.status === 'closed'
          return (
            <Chip
              key={b.id}
              label={`${closed ? 'depends on' : 'blocked by'}: ${b.id}`}
              size="small"
              variant="outlined"
              color={closed ? 'success' : 'warning'}
              component={Link as any}
              to="/project/$slug/issues/$id"
              params={{ slug, id: b.id }}
              clickable
              sx={{ textDecoration: 'none' }}
            />
          )
        })}
        {issue.duplicate_of && (
          <Chip
            label={`duplicate of: ${issue.duplicate_of}`}
            size="small"
            variant="outlined"
            color="info"
            component={Link as any}
            to="/project/$slug/issues/$id"
            params={{ slug, id: issue.duplicate_of }}
            clickable
            sx={{ textDecoration: 'none' }}
          />
        )}
        {blockingFocuses.map(t => (
          <Chip
            key={t.id}
            label={t.name}
            size="small"
            variant="outlined"
            color="secondary"
            component={Link as any}
            to="/project/$slug/focuses/$name"
            params={{ slug, name: t.name }}
            clickable
            sx={{ textDecoration: 'none' }}
            onDelete={() => removeFocusBlocker.mutate({ slug, focus: `FO-${t.id}`, issue: issueId })}
          />
        ))}
        {issue.status === 'wip' && (
          <Button
            size="small"
            variant="contained"
            color="primary"
            disabled={readyIssue.isPending}
            onClick={() => readyIssue.mutate({ slug, id: issue.id })}
          >
            Make ready
          </Button>
        )}
        {(issue.status === 'open' || issue.status === 'triaged' || issue.status === 'in-progress') && (
          <Button size="small" variant="outlined" color="warning" onClick={() => { setCloseReason(''); setDuplicateOf(null); setDupSearch(''); setCloseOpen(true) }}>
            Close
          </Button>
        )}
      </Box>

      {availableFocuses.length > 0 && (
        <Box sx={{ mb: 2 }}>
          <Autocomplete<FocusItem>
            size="small"
            options={availableFocuses}
            getOptionLabel={(o) => o.name}
            inputValue={focusInput}
            onInputChange={(_, v) => setFocusInput(v)}
            onChange={(_, v) => {
              if (v) {
                addFocusBlocker.mutate({ slug, focus: `FO-${v.id}`, issue: issueId })
                setFocusInput('')
              }
            }}
            value={null}
            renderInput={(params) => <TextField {...params} label="Add focus" placeholder="Search focuses..." />}
            sx={{ maxWidth: 300 }}
            noOptionsText="No focuses"
            isOptionEqualToValue={(o, v) => o.id === v.id}
          />
        </Box>
      )}

      <Dialog open={closeOpen} onClose={() => setCloseOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Close {issue.id}</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            select
            label="Reason"
            value={closeReason}
            onChange={e => { setCloseReason(e.target.value); if (e.target.value !== 'duplicate') { setDuplicateOf(null); setDupSearch('') } }}
            sx={{ mt: 1 }}
            autoFocus
            slotProps={{ select: { native: true } }}
          >
            <option value="">Select reason...</option>
            <option value="done">Done</option>
            <option value="duplicate">Duplicate</option>
            <option value="wontfix">Won't fix</option>
            <option value="invalid">Invalid</option>
          </TextField>
          {closeReason === 'duplicate' && (
            <Autocomplete
              options={(dupResults ?? []).filter(i => `IS-${i.id}` !== issueId)}
              getOptionLabel={(o) => `IS-${o.id}: ${o.title || o.context?.slice(0, 60) || '(no title)'}`}
              value={duplicateOf}
              onChange={(_, v) => setDuplicateOf(v)}
              inputValue={dupSearch}
              onInputChange={(_, v) => setDupSearch(v)}
              renderInput={(params) => <TextField {...params} label="Duplicate of" placeholder="Search issues..." />}
              sx={{ mt: 2 }}
              noOptionsText={dupSearch.length < 2 ? 'Type to search...' : 'No issues found'}
              isOptionEqualToValue={(o, v) => o.id === v.id}
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCloseOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            color="warning"
            disabled={closeIssue.isPending || !closeReason.trim() || (closeReason === 'duplicate' && !duplicateOf)}
            onClick={() => {
              closeIssue.mutate(
                { slug, id: issue.id, reason: closeReason, ...(duplicateOf ? { duplicate_of: `IS-${duplicateOf.id}` } : {}) } as any,
                { onSuccess: () => setCloseOpen(false) },
              )
            }}
          >
            Close
          </Button>
        </DialogActions>
      </Dialog>

      <Box sx={{ mb: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
          <Typography variant="subtitle2" color="text.secondary">
            Context
          </Typography>
          {!editingContext && issue.status !== 'wip' && ['open', 'triaged', 'in-progress'].includes(issue.status) && (
            <Tooltip title="Edit context (live issue)">
              <IconButton size="small" onClick={() => { setEditContext(issue.context ?? ''); setPreviewContext(false); setEditingContext(true) }}>
                <EditIcon fontSize="inherit" />
              </IconButton>
            </Tooltip>
          )}
        </Box>
        {editingContext ? (
          <Box>
            {editingContext && issue.status !== 'wip' && (
              <Chip label="live issue" size="small" color="warning" variant="outlined" sx={{ mb: 1 }} />
            )}
            <Box sx={{ display: 'flex', gap: 1, mb: 1 }}>
              <Button size="small" variant={previewContext ? 'outlined' : 'contained'} onClick={() => setPreviewContext(false)}>Edit</Button>
              <Button size="small" variant={previewContext ? 'contained' : 'outlined'} onClick={() => setPreviewContext(true)}>Preview</Button>
            </Box>
            {previewContext ? (
              <MarkdownContent slug={slug}>{editContext}</MarkdownContent>
            ) : (
              <TextField
                fullWidth
                multiline
                minRows={6}
                size="small"
                autoFocus
                value={editContext}
                onChange={e => setEditContext(e.target.value)}
              />
            )}
            <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
              <Button
                size="small"
                variant="contained"
                disabled={editIssue.isPending}
                onClick={() => {
                  editIssue.mutate({ slug, id: issue.id, context: editContext }, { onSuccess: () => setEditingContext(false) })
                }}
              >
                Save
              </Button>
              <Button size="small" onClick={() => setEditingContext(false)}>Cancel</Button>
            </Box>
          </Box>
        ) : (
          issue.context ? (
            <Box
              sx={issue.status === 'wip' ? { cursor: 'pointer', '&:hover': { opacity: 0.7 } } : undefined}
              onClick={issue.status === 'wip' ? () => { setEditContext(issue.context ?? ''); setPreviewContext(false); setEditingContext(true) } : undefined}
            >
              <MarkdownContent slug={slug}>{issue.context}</MarkdownContent>
            </Box>
          ) : (
            issue.status === 'wip' && (
              <Typography
                color="text.disabled"
                sx={{ cursor: 'pointer', fontStyle: 'italic' }}
                onClick={() => { setEditContext(''); setPreviewContext(false); setEditingContext(true) }}
              >
                Click to add context…
              </Typography>
            )
          )
        )}
      </Box>

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
                to="/project/$slug/tasks/$id"
                params={{ slug, id: `TK-${t.id}` }}
                sx={{ display: 'flex', gap: 1, alignItems: 'center', textDecoration: 'none', color: 'inherit', '&:hover': { opacity: 0.8 } }}
              >
                <Chip label={t.status} size="small" color={t.status === 'done' ? 'success' : 'default'} variant="outlined" />
                <Typography variant="body2">
                  TK-{t.id}: [{t.feature}] {t.title || t.text}
                </Typography>
              </Box>
            ))}
          </Box>
        </Box>
      )}

      <ReservationSection slug={slug} reservations={reservationsData?.reservations ?? []} />

      <BlockerQuestionsSection slug={slug} targetType="issue" targetId={issueId} />

      <CodeRefs refs={codeRefs ?? []} slug={slug} />

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3, mb: 2 }}>
        Created: {new Date(issue.created_at).toLocaleString()}
      </Typography>

      <Box sx={{ mb: 3 }}>
        <TextField
          fullWidth
          multiline
          rows={2}
          size="small"
          placeholder="Add a comment…"
          value={commentBody}
          onChange={e => setCommentBody(e.target.value)}
        />
        <Button
          size="small"
          variant="contained"
          disabled={!commentBody.trim() || addComment.isPending}
          onClick={() => {
            if (!commentBody.trim()) return
            addComment.mutate(
              { slug, target_type: 'issue', target_id: issueId, body: commentBody.trim() },
              { onSuccess: () => setCommentBody('') },
            )
          }}
          sx={{ mt: 1 }}
        >
          Comment
        </Button>
      </Box>

      <UnifiedTimeline slug={slug} issueId={issueId} workEntries={workEntries} />
    </Box>
  )
}
