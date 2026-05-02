import { useState } from 'react'
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Chip,
  CircularProgress,
  Typography,
} from '@mui/material'
import { ExpandMore as ExpandMoreIcon } from '@mui/icons-material'
import { Link } from '@tanstack/react-router'
import { useIncompleteReports, type IncompleteReportGroup } from '../api'

const REASONS = [
  'capability_gap',
  'ambiguous_spec',
  'external_dep',
  'needs_decision',
  'permission_denied',
  'environment_broken',
  'preexisting_test_failure',
  'flaky_test',
] as const

const REASON_COLORS: Record<string, 'default' | 'primary' | 'secondary' | 'error' | 'info' | 'success' | 'warning'> = {
  capability_gap: 'warning',
  ambiguous_spec: 'info',
  external_dep: 'secondary',
  needs_decision: 'primary',
  permission_denied: 'error',
  environment_broken: 'error',
  preexisting_test_failure: 'error',
  flaky_test: 'warning',
}

function relativeTime(iso: string): string {
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return iso
  const diffMs = Date.now() - t
  const sec = Math.round(diffMs / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.round(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.round(min / 60)
  if (hr < 24) return `${hr}h ago`
  const day = Math.round(hr / 24)
  if (day < 30) return `${day}d ago`
  return new Date(t).toLocaleDateString()
}

function truncateFingerprint(fp: string): string {
  if (fp.length <= 14) return fp
  return `${fp.slice(0, 8)}…${fp.slice(-4)}`
}

function groupKey(g: IncompleteReportGroup): string {
  return `${g.reason}:${g.evidence_fingerprint}`
}

export function BlockedWorkPanel({ slug }: { slug: string }) {
  const [activeReason, setActiveReason] = useState<string | null>(null)
  const { data, isLoading, isFetching } = useIncompleteReports(slug, activeReason ?? undefined)

  const groups = data ?? []

  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 2 }}>Blocked work</Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Cascading blockers grouped by (reason, evidence). Each row collapses many todos hitting the same
        structural issue.
      </Typography>

      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 3, alignItems: 'center' }}>
        <Chip
          label="all"
          size="small"
          onClick={() => setActiveReason(null)}
          color={activeReason === null ? 'primary' : 'default'}
          variant={activeReason === null ? 'filled' : 'outlined'}
        />
        {REASONS.map(r => (
          <Chip
            key={r}
            label={r}
            size="small"
            onClick={() => setActiveReason(r)}
            color={activeReason === r ? (REASON_COLORS[r] ?? 'primary') : 'default'}
            variant={activeReason === r ? 'filled' : 'outlined'}
          />
        ))}
        {isFetching && <CircularProgress size={16} sx={{ ml: 1 }} />}
      </Box>

      {isLoading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
          <CircularProgress size={24} />
        </Box>
      ) : groups.length === 0 ? (
        <Box sx={{ textAlign: 'center', py: 6, color: 'text.secondary' }}>
          <Typography variant="body1">No blocked work — nice.</Typography>
        </Box>
      ) : (
        groups.map(g => (
          <BlockedGroupRow key={groupKey(g)} slug={slug} group={g} />
        ))
      )}
    </Box>
  )
}

function BlockedGroupRow({ slug, group: g }: { slug: string; group: IncompleteReportGroup }) {
  const keys = g.affected_todo_keys ?? []
  // Server orders by most-recent first within a group via aggregation; pick the first key as "most recent affected".
  const viewKey = keys[0]
  return (
    <Accordion disableGutters variant="outlined" sx={{ '&:before': { display: 'none' }, mb: 1 }}>
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flex: 1, minWidth: 0 }}>
          <Chip
            label={g.reason}
            size="small"
            color={REASON_COLORS[g.reason] ?? 'default'}
            sx={{ flexShrink: 0 }}
          />
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {g.total_count} report{g.total_count === 1 ? '' : 's'} across {keys.length} todo{keys.length === 1 ? '' : 's'}
          </Typography>
          <Box sx={{ flex: 1 }} />
          <Typography
            variant="caption"
            sx={{ fontFamily: 'monospace', color: 'text.secondary', flexShrink: 0 }}
            title={g.evidence_fingerprint}
          >
            {truncateFingerprint(g.evidence_fingerprint)}
          </Typography>
          <Typography variant="caption" color="text.secondary" sx={{ flexShrink: 0 }}>
            {relativeTime(g.last_seen)}
          </Typography>
        </Box>
      </AccordionSummary>
      <AccordionDetails>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 2 }}>
          {keys.length === 0 ? (
            <Typography variant="caption" color="text.secondary">
              No affected todos.
            </Typography>
          ) : (
            keys.map(k => (
              <Chip
                key={k}
                label={k}
                size="small"
                variant="outlined"
                component={Link as any}
                to="/project/$slug/tasks/$id"
                params={{ slug, id: k }}
                clickable
                sx={{ fontFamily: 'monospace' }}
              />
            ))
          )}
        </Box>
        <Box sx={{ display: 'flex', gap: 1 }}>
          {viewKey && (
            <Button
              size="small"
              variant="outlined"
              component={Link as any}
              to="/project/$slug/tasks/$id"
              params={{ slug, id: viewKey }}
            >
              View suggestion
            </Button>
          )}
        </Box>
        {/* Note: structured 'apply suggested action' (POST /api/dx/incomplete-reports/apply) is a follow-up. */}
      </AccordionDetails>
    </Accordion>
  )
}
