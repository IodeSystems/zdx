import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material'
import { Add as AddIcon, AutoAwesome as GenerateIcon, TrendingUp, TrendingDown, TrendingFlat } from '@mui/icons-material'
import { useState, useMemo } from 'react'
import { useJournalEntries, useCreateJournalEntry, useGenerateJournalEntry, useChurnSessions, useExtractPatternFromSession, type JournalEntryItem, type ClaudeSessionItem } from '../api'
import { MarkdownContent } from './MarkdownContent'

interface MetricDelta {
  name: string
  prev: number
  curr: number
  diff: number
}

const METRIC_LABELS: Record<string, string> = {
  go_files: 'Go files',
  go_loc: 'Go LOC',
  test_files: 'Test files',
  test_functions: 'Test fns',
  migrations: 'Migrations',
  sql_query_files: 'SQL queries',
  ts_files: 'TS files',
  tsx_files: 'TSX files',
  ts_loc: 'TS/TSX LOC',
  git_commits_since: 'Commits',
  git_files_changed_since: 'Files changed',
}

function MetricsGrid({ stateJson, changelogJson }: { stateJson: string; changelogJson: string }) {
  const { metrics, deltas } = useMemo(() => {
    let metrics: Record<string, number> = {}
    let deltas: MetricDelta[] = []
    try { metrics = JSON.parse(stateJson) } catch { /* empty */ }
    try { deltas = JSON.parse(changelogJson) } catch { /* empty */ }
    return { metrics, deltas }
  }, [stateJson, changelogJson])

  const deltaMap = useMemo(() => {
    const m = new Map<string, MetricDelta>()
    for (const d of deltas) m.set(d.name, d)
    return m
  }, [deltas])

  const keys = Object.keys(metrics).filter(k => typeof metrics[k] === 'number')
  if (keys.length === 0) return null

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(130px, 1fr))', gap: 0.75, mt: 1, mb: 0.5 }}>
      {keys.map(k => {
        const delta = deltaMap.get(k)
        const diff = delta?.diff ?? 0
        return (
          <Box key={k} sx={{ px: 1, py: 0.5, borderRadius: 1, bgcolor: 'action.hover', display: 'flex', flexDirection: 'column' }}>
            <Typography variant="caption" color="text.secondary" sx={{ fontSize: '0.65rem', lineHeight: 1.2 }}>
              {METRIC_LABELS[k] ?? k}
            </Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Typography variant="body2" sx={{ fontWeight: 600, fontSize: '0.85rem' }}>
                {metrics[k].toLocaleString()}
              </Typography>
              {diff !== 0 && (
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
                  {diff > 0 ? (
                    <TrendingUp sx={{ fontSize: 14, color: 'success.main' }} />
                  ) : (
                    <TrendingDown sx={{ fontSize: 14, color: 'error.main' }} />
                  )}
                  <Typography variant="caption" sx={{ fontSize: '0.7rem', color: diff > 0 ? 'success.main' : 'error.main' }}>
                    {diff > 0 ? '+' : ''}{diff}
                  </Typography>
                </Box>
              )}
              {diff === 0 && delta && (
                <TrendingFlat sx={{ fontSize: 14, color: 'text.disabled' }} />
              )}
            </Box>
          </Box>
        )
      })}
    </Box>
  )
}

function EntryCard({ entry, prev, isTech, slug }: { entry: JournalEntryItem; prev?: JournalEntryItem; isTech: boolean; slug: string }) {
  const [expanded, setExpanded] = useState(false)

  const hasMetrics = isTech && entry.state_json && entry.state_json !== '{}'

  const fields: { label: string; key: keyof JournalEntryItem }[] = [
    { label: 'Assessment', key: 'assessment' },
    { label: 'Concerns', key: 'concerns' },
    { label: 'Next Steps', key: 'next' },
  ]

  return (
    <Card variant="outlined" sx={{ mb: 1 }}>
      <CardContent
        sx={{ py: 1.25, '&:last-child': { pb: 1.25 }, cursor: 'pointer' }}
        onClick={() => setExpanded(e => !e)}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>{entry.date}</Typography>
          {entry.baseline && <Chip label="baseline" size="small" color="info" sx={{ fontSize: '0.7rem' }} />}
          {hasMetrics && <Chip label="metrics" size="small" color="success" sx={{ fontSize: '0.65rem', height: 18 }} />}
          <Typography variant="body2" color="text.secondary" sx={{ flex: 1, ml: 1 }}>
            {entry.tldr}
          </Typography>
        </Box>
        {expanded && (
          <Box sx={{ mt: 1.5, display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {hasMetrics && (
              <MetricsGrid stateJson={entry.state_json} changelogJson={entry.changelog_json} />
            )}
            {fields.map(f => {
              const val = entry[f.key] as string
              if (!val) return null
              const prevVal = prev?.[f.key] as string | undefined
              const changed = prev && prevVal !== val
              return (
                <Box key={f.key}>
                  <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
                    {f.label}
                    {changed && (
                      <Chip label="changed" size="small" color="warning" sx={{ fontSize: '0.65rem', ml: 0.5, height: 18 }} />
                    )}
                  </Typography>
                  <Box sx={{ mt: 0.25 }}>
                    <MarkdownContent slug={slug}>{val}</MarkdownContent>
                  </Box>
                </Box>
              )
            })}
          </Box>
        )}
      </CardContent>
    </Card>
  )
}

interface CheckinForm {
  tldr: string
  assessment: string
  concerns: string
  next: string
}

const emptyForm: CheckinForm = { tldr: '', assessment: '', concerns: '', next: '' }

function ChurnSessionCard({ session, slug }: { session: ClaudeSessionItem; slug: string }) {
  const extract = useExtractPatternFromSession()
  const [patternName, setPatternName] = useState<string | null>(null)

  const handleExtract = async () => {
    const pattern = await extract.mutateAsync({ slug, sessionId: session.id })
    setPatternName(pattern.name)
  }

  return (
    <Card variant="outlined" sx={{ mb: 1 }}>
      <CardContent sx={{ py: 1.25, '&:last-child': { pb: 1.25 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
          <Chip label="churn" size="small" color="warning" sx={{ fontSize: '0.65rem', height: 18 }} />
          <Typography variant="caption" color="text.secondary">{session.created_at?.slice(0, 10)}</Typography>
          <Typography variant="body2" sx={{ fontWeight: 600, flex: 1 }}>{session.header || session.title}</Typography>
        </Box>
        {session.summary && (
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>{session.summary}</Typography>
        )}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          {patternName ? (
            <Chip label={`Pattern: ${patternName}`} size="small" color="success" sx={{ fontSize: '0.7rem' }} />
          ) : (
            <Button
              size="small"
              variant="outlined"
              onClick={handleExtract}
              disabled={extract.isPending}
            >
              {extract.isPending ? 'Extracting...' : 'Extract Pattern'}
            </Button>
          )}
          {extract.isError && (
            <Typography variant="caption" color="error">{extract.error.message}</Typography>
          )}
        </Box>
      </CardContent>
    </Card>
  )
}

function ChurnReviewPanel({ slug }: { slug: string }) {
  const { data, isLoading } = useChurnSessions(slug)
  const sessions = data?.sessions ?? []

  if (isLoading) return null
  if (sessions.length === 0) return null

  return (
    <Box sx={{ mb: 3 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Churn Sessions ({sessions.length})
      </Typography>
      {sessions.map(s => (
        <ChurnSessionCard key={s.id} session={s} slug={slug} />
      ))}
    </Box>
  )
}

export function JournalTab({ slug }: { slug: string }) {
  const [role, setRole] = useState<'owner' | 'tech'>('owner')
  const { data: entries, isLoading } = useJournalEntries(slug, role)
  const create = useCreateJournalEntry()
  const generate = useGenerateJournalEntry()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<CheckinForm>(emptyForm)

  const handleSave = async () => {
    const today = new Date().toISOString().slice(0, 10)
    await create.mutateAsync({ slug, role, date: today, ...form })
    setForm(emptyForm)
    setDialogOpen(false)
  }

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const sorted = entries ?? []

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 2, gap: 2 }}>
        <Tabs value={role} onChange={(_, v) => setRole(v)} sx={{ minHeight: 36 }}>
          <Tab label="Owner" value="owner" sx={{ minHeight: 36, py: 0 }} />
          <Tab label="Tech" value="tech" sx={{ minHeight: 36, py: 0 }} />
        </Tabs>
        <Box sx={{ flex: 1 }} />
        <Button
          size="small"
          startIcon={<GenerateIcon />}
          onClick={() => generate.mutate({ slug, role })}
          disabled={generate.isPending}
        >
          {generate.isPending ? 'Generating...' : 'Generate'}
        </Button>
        <Button size="small" startIcon={<AddIcon />} onClick={() => setDialogOpen(true)}>
          Check-in
        </Button>
      </Box>
      {role === 'tech' && <ChurnReviewPanel slug={slug} />}
      {sorted.length === 0 && (
        <Typography variant="body2" color="text.secondary">No standup entries yet.</Typography>
      )}
      {sorted.map((entry, i) => (
        <EntryCard key={entry.date + i} entry={entry} prev={sorted[i + 1]} isTech={role === 'tech'} slug={slug} />
      ))}
      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>New Check-in ({role})</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '8px !important' }}>
          <TextField
            label="TL;DR"
            value={form.tldr}
            onChange={e => setForm({ ...form, tldr: e.target.value })}
            size="small"
            fullWidth
            autoFocus
          />
          <TextField
            label="Assessment"
            value={form.assessment}
            onChange={e => setForm({ ...form, assessment: e.target.value })}
            size="small"
            fullWidth
            multiline
            minRows={3}
          />
          <TextField
            label="Concerns"
            value={form.concerns}
            onChange={e => setForm({ ...form, concerns: e.target.value })}
            size="small"
            fullWidth
            multiline
            minRows={2}
          />
          <TextField
            label="Next Steps"
            value={form.next}
            onChange={e => setForm({ ...form, next: e.target.value })}
            size="small"
            fullWidth
            multiline
            minRows={2}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleSave} variant="contained" disabled={!form.tldr.trim()}>Save</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
