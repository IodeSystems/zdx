import { Link } from '@tanstack/react-router'
import {
  Box,
  Chip,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon, TrendingUp, TrendingDown, TrendingFlat } from '@mui/icons-material'
import { useMemo } from 'react'
import { useJournalEntry } from '../api'
import { CommentsAndRevisions } from './CommentsAndRevisions'
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

function Section({ label, content, slug }: { label: string; content: string; slug: string }) {
  if (!content) return null
  return (
    <Box sx={{ mb: 2 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        {label}
      </Typography>
      <MarkdownContent slug={slug}>{content}</MarkdownContent>
    </Box>
  )
}

export function JournalDetail({ slug, entryId }: { slug: string; entryId: string }) {
  const { data: entry, isLoading, isError } = useJournalEntry(slug, entryId)

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (isError || !entry) return <Typography color="error">Entry not found.</Typography>

  const hasMetrics = entry.state_json && entry.state_json !== '{}'

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
        <Link to="/project/$slug/journal" params={{ slug }} style={{ textDecoration: 'none', color: 'inherit', display: 'flex', alignItems: 'center' }}>
          <ArrowBackIcon fontSize="small" />
        </Link>
        <Typography variant="h6" sx={{ fontWeight: 600 }}>
          Standup — {entry.date}
        </Typography>
        {entry.baseline && <Chip label="baseline" size="small" color="info" />}
      </Box>

      {entry.tldr && (
        <Typography variant="body1" sx={{ mb: 2, fontStyle: 'italic', color: 'text.secondary' }}>
          {entry.tldr}
        </Typography>
      )}

      {hasMetrics && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>Metrics</Typography>
          <MetricsGrid stateJson={entry.state_json} changelogJson={entry.changelog_json} />
        </Box>
      )}

      <Section label="Assessment" content={entry.assessment} slug={slug} />
      <Section label="Concerns" content={entry.concerns} slug={slug} />
      <Section label="Next Steps" content={entry.next} slug={slug} />

      <Box sx={{ mt: 3 }}>
        <CommentsAndRevisions slug={slug} targetType="journal" targetId={entryId} />
      </Box>
    </Box>
  )
}
