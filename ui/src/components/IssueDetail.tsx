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
  TextField,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useCallback, useEffect, useState } from 'react'
import {
  useIssue,
  useTasks,
  useCloseIssue,
  useSearchIssues,
  useIssueCodeRefs,
  useIssueResolutions,
  useListThemes,
  useAddThemeBlocker,
  useRemoveThemeBlocker,
  useClaudeSessionsByIssue,
  type IssueItem,
  type IssueWorkItem,
  type TaskItem,
  type ThemeItem,
} from '../api'
import { useChannel } from '../hooks/useChannel'
import { BlockerQuestionsSection } from './BlockerQuestionsSection'
import { CommentsAndRevisions } from './CommentsAndRevisions'
import { CodeRefs } from './CodeRefs'
import { MarkdownContent } from './MarkdownContent'
import { StatusTimeline } from './StatusTimeline'

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
  const { data: codeRefs } = useIssueCodeRefs(slug, issueId)
  const { data: resolutions } = useIssueResolutions(slug, issueId)
  const { data: sessionsData } = useClaudeSessionsByIssue(slug, issueId)
  const closeIssue = useCloseIssue()
  const router = useRouter()
  const [closeOpen, setCloseOpen] = useState(false)
  const [closeReason, setCloseReason] = useState('')
  const [duplicateOf, setDuplicateOf] = useState<IssueItem | null>(null)
  const [dupSearch, setDupSearch] = useState('')
  const { data: dupResults } = useSearchIssues(slug, dupSearch, closeReason === 'duplicate' && dupSearch.length > 1)
  const { data: allThemes } = useListThemes(slug)
  const addThemeBlocker = useAddThemeBlocker()
  const removeThemeBlocker = useRemoveThemeBlocker()
  const [themeInput, setThemeInput] = useState('')

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
  const blockingThemes = (allThemes ?? []).filter(t => t.blockers.split(',').includes(issueId))
  const blockingThemeIds = new Set(blockingThemes.map(t => t.id))
  const availableThemes = (allThemes ?? []).filter(t => t.status !== 'archived' && !blockingThemeIds.has(t.id))

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

      <Typography variant="h5" sx={{ mb: 1 }}>
        {issueId}: {displayTitle}
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
        {issue.issue_type && (
          <Chip label={issue.issue_type} size="small" variant="outlined" color={issue.issue_type === 'impl' ? 'secondary' : 'default'} />
        )}
        {(((issue.blocked_by ?? []) as unknown) as string[]).map((bid: string) => (
          <Chip
            key={bid}
            label={`blocked by: ${bid}`}
            size="small"
            variant="outlined"
            color="warning"
            component={Link as any}
            to="/project/$slug/issues/$id"
            params={{ slug, id: bid }}
            clickable
            sx={{ textDecoration: 'none' }}
          />
        ))}
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
        {blockingThemes.map(t => (
          <Chip
            key={t.id}
            label={t.name}
            size="small"
            variant="outlined"
            color="secondary"
            component={Link as any}
            to="/project/$slug/themes/$name"
            params={{ slug, name: t.name }}
            clickable
            sx={{ textDecoration: 'none' }}
            onDelete={() => removeThemeBlocker.mutate({ slug, theme: `TH-${t.id}`, issue: issueId })}
          />
        ))}
        {(issue.status === 'open' || issue.status === 'triaged' || issue.status === 'in-progress') && (
          <Button size="small" variant="outlined" color="warning" onClick={() => { setCloseReason(''); setDuplicateOf(null); setDupSearch(''); setCloseOpen(true) }}>
            Close
          </Button>
        )}
      </Box>

      {availableThemes.length > 0 && (
        <Box sx={{ mb: 2 }}>
          <Autocomplete<ThemeItem>
            size="small"
            options={availableThemes}
            getOptionLabel={(o) => o.name}
            inputValue={themeInput}
            onInputChange={(_, v) => setThemeInput(v)}
            onChange={(_, v) => {
              if (v) {
                addThemeBlocker.mutate({ slug, theme: `TH-${v.id}`, issue: issueId })
                setThemeInput('')
              }
            }}
            value={null}
            renderInput={(params) => <TextField {...params} label="Add theme" placeholder="Search themes..." />}
            sx={{ maxWidth: 300 }}
            noOptionsText="No themes"
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

      {issue.context && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Context
          </Typography>
          <MarkdownContent slug={slug}>{issue.context}</MarkdownContent>
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
                to="/project/$slug/tasks/$id"
                params={{ slug, id: t.id }}
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

      <BlockerQuestionsSection slug={slug} targetType="issue" targetId={issueId} />

      <CodeRefs refs={codeRefs ?? []} />

      {(resolutions ?? []).length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Resolutions ({resolutions!.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {resolutions!.map(r => (
              <Box key={r.id} sx={{ borderLeft: 2, borderColor: r.reverted ? 'error.main' : 'success.main', pl: 1.5 }}>
                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap' }}>
                  <Chip label={r.source} size="small" variant="outlined" />
                  {r.branch_of_origin && <Chip label={r.branch_of_origin} size="small" variant="outlined" color="info" />}
                  {r.reverted && <Chip label={`reverted by ${r.reverted_by?.slice(0, 10)}`} size="small" color="error" />}
                  <Typography variant="caption" color="text.secondary">
                    {new Date(r.resolved_at).toLocaleString()} — {r.author}
                  </Typography>
                </Box>
                <Box sx={{ mt: 0.5 }}>
                  {r.commits.map(c => (
                    <Typography key={c.sha} variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                      {c.sha.slice(0, 10)}
                    </Typography>
                  ))}
                </Box>
              </Box>
            ))}
          </Box>
        </Box>
      )}

      {(sessionsData?.sessions ?? []).length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Claude Sessions ({sessionsData!.sessions.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {sessionsData!.sessions.map(s => (
              <Box
                key={s.id}
                component={Link as any}
                to="/project/$slug/agents/$sessionId"
                params={{ slug, sessionId: String(s.id) }}
                sx={{ display: 'flex', gap: 1, alignItems: 'center', textDecoration: 'none', color: 'inherit', '&:hover': { opacity: 0.8 } }}
              >
                <Chip
                  label={s.status || 'pending'}
                  size="small"
                  color={s.status === 'ok' ? 'success' : s.status === 'errored' ? 'error' : s.status === 'churn' ? 'warning' : 'default'}
                  variant="outlined"
                />
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {s.header || s.title || s.session_id.slice(0, 8)}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {new Date(s.created_at).toLocaleString()}
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
                  {new Date(e.created_at).toLocaleString()} — {e.agent}
                </Typography>
                {e.note && (
                  <Typography variant="body2">{e.note}</Typography>
                )}
              </Box>
            ))}
          </Box>
        </Box>
      )}

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3, mb: 3 }}>
        Created: {new Date(issue.created_at).toLocaleString()}
      </Typography>

      <StatusTimeline slug={slug} targetType="issue" targetId={issueId} />

      <CommentsAndRevisions slug={slug} targetType="issue" targetId={issueId} />
    </Box>
  )
}
