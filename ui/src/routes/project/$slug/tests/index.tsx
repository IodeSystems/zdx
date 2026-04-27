import { createFileRoute, Link } from '@tanstack/react-router'
import { useMemo, useState, useDeferredValue } from 'react'
import {
  Box,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  InputAdornment,
  LinearProgress,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import { Clear as ClearIcon, Search as SearchIcon } from '@mui/icons-material'
import { useTests } from '../../../../api'
import type { TestItem } from '../../../../api'

const STATUS_COLOR: Record<string, 'success' | 'error' | 'default' | 'warning'> = {
  pass: 'success',
  fail: 'error',
  skip: 'warning',
}

const SLOW_TEST_LIMIT = 10
const SLOW_TEST_MIN_MS = 100

function formatDuration(ms: number): string {
  if (ms === 0) return '—'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function formatRelativeTime(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const mins = Math.floor(diffMs / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  return `${days}d ago`
}

function isDeployLine(branch: string): boolean {
  return branch === 'main' || branch === 'master'
}

function BranchBadge({ branch }: { branch: string }) {
  if (!branch) return null
  const deploy = isDeployLine(branch)
  return (
    <Tooltip title={deploy ? 'Deploy line' : `Dev branch: ${branch}`}>
      <Chip
        label={deploy ? 'main' : branch.length > 16 ? branch.slice(0, 14) + '…' : branch}
        size="small"
        variant={deploy ? 'filled' : 'outlined'}
        color={deploy ? 'primary' : 'default'}
        sx={{ fontSize: '0.7rem', height: 18, fontFamily: 'monospace' }}
      />
    </Tooltip>
  )
}

function TestCard({ slug, test, highlight }: { slug: string; test: TestItem; highlight?: 'fail' | 'slow' | null }) {
  const lastFailedAgo = test.last_failed_at ? formatRelativeTime(test.last_failed_at) : null
  const failedOnDeploy = test.last_failed_branch ? isDeployLine(test.last_failed_branch) : false

  return (
    <Paper
      variant="outlined"
      sx={{
        p: 1.25,
        borderLeft: highlight === 'fail' ? '3px solid' : highlight === 'slow' ? '3px solid' : undefined,
        borderLeftColor: highlight === 'fail' ? 'error.main' : highlight === 'slow' ? 'warning.main' : undefined,
        '&:hover': { bgcolor: 'action.hover' },
      }}
    >
      <Link
        to="/project/$slug/tests/$id"
        params={{ slug, id: String(test.id) }}
        style={{ color: 'inherit', textDecoration: 'none' }}
      >
        <Stack spacing={0.75}>
          <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1, flexWrap: 'wrap' }}>
            <Typography
              variant="body2"
              sx={{
                fontFamily: 'monospace',
                fontSize: '0.85rem',
                fontWeight: 500,
                wordBreak: 'break-word',
                flex: 1,
                minWidth: 0,
              }}
            >
              {test.name}
            </Typography>
            <Chip label={test.status} size="small" color={STATUS_COLOR[test.status] ?? 'default'} />
          </Box>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, alignItems: 'center' }}>
            <Chip label={test.component} size="small" variant="outlined" sx={{ fontSize: '0.7rem', height: 18 }} />
            <Chip label={test.layer} size="small" variant="outlined" sx={{ fontSize: '0.7rem', height: 18 }} />
            {test.last_run_branch && <BranchBadge branch={test.last_run_branch} />}
            <Box sx={{ flex: 1 }} />
            <Typography variant="caption" sx={{ fontFamily: 'monospace', color: 'text.secondary' }}>
              {formatDuration(test.duration_ms)}
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5, fontSize: '0.75rem', color: 'text.secondary' }}>
            {test.last_run_at && (
              <Box component="span">
                Last run: {formatRelativeTime(test.last_run_at)}
              </Box>
            )}
            {lastFailedAgo && (
              <Tooltip title={`Failed on ${test.last_failed_branch || 'unknown branch'}`}>
                <Box
                  component="span"
                  sx={{
                    color: failedOnDeploy ? 'error.main' : 'warning.main',
                    fontWeight: failedOnDeploy ? 600 : 500,
                  }}
                >
                  Failed: {lastFailedAgo}
                </Box>
              </Tooltip>
            )}
          </Box>
        </Stack>
      </Link>
    </Paper>
  )
}

function MetricTile({ label, value, sub, color }: { label: string; value: string | number; sub?: string; color?: string }) {
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 1.5,
        flex: '1 1 130px',
        minWidth: 130,
        display: 'flex',
        flexDirection: 'column',
        gap: 0.25,
      }}
    >
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: 'uppercase', letterSpacing: 0.5, fontSize: '0.65rem' }}>
        {label}
      </Typography>
      <Typography variant="h6" sx={{ fontWeight: 600, lineHeight: 1.2, color: color || 'text.primary' }}>
        {value}
      </Typography>
      {sub && (
        <Typography variant="caption" color="text.secondary" sx={{ fontSize: '0.7rem' }}>
          {sub}
        </Typography>
      )}
    </Paper>
  )
}

function Dashboard({ slug, tests }: { slug: string; tests: TestItem[] }) {
  const total = tests.length
  const passing = tests.filter(t => t.status === 'pass').length
  const failing = tests.filter(t => t.status === 'fail').length
  const skipped = tests.filter(t => t.status === 'skip').length
  const passRate = total > 0 ? Math.round((passing / total) * 100) : 0

  const deployTests = tests.filter(t => t.last_run_branch && isDeployLine(t.last_run_branch))
  const deployFailing = deployTests.filter(t => t.status === 'fail').length

  const totalDuration = tests.reduce((acc, t) => acc + (t.duration_ms || 0), 0)

  const failingTests = useMemo(
    () =>
      tests
        .filter(t => t.status === 'fail')
        .sort((a, b) => {
          const ad = a.last_failed_at ? new Date(a.last_failed_at).getTime() : 0
          const bd = b.last_failed_at ? new Date(b.last_failed_at).getTime() : 0
          return bd - ad
        }),
    [tests]
  )

  const slowTests = useMemo(
    () =>
      [...tests]
        .filter(t => t.duration_ms >= SLOW_TEST_MIN_MS)
        .sort((a, b) => b.duration_ms - a.duration_ms)
        .slice(0, SLOW_TEST_LIMIT),
    [tests]
  )

  return (
    <Stack spacing={3}>
      <Box>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5 }}>
          <MetricTile label="Total" value={total} sub={`${passing} pass · ${failing} fail${skipped ? ` · ${skipped} skip` : ''}`} />
          <MetricTile
            label="Pass rate"
            value={`${passRate}%`}
            color={passRate >= 95 ? 'success.main' : passRate >= 80 ? 'warning.main' : 'error.main'}
          />
          <MetricTile
            label="Failing"
            value={failing}
            color={failing === 0 ? 'success.main' : 'error.main'}
            sub={failing === 0 ? 'all green' : undefined}
          />
          <MetricTile
            label="Deploy line"
            value={deployTests.length === 0 ? '—' : deployFailing === 0 ? 'green' : `${deployFailing} fail`}
            color={deployTests.length === 0 ? 'text.secondary' : deployFailing === 0 ? 'success.main' : 'error.main'}
            sub={deployTests.length === 0 ? 'no main runs' : `${deployTests.length} tests on main`}
          />
          <MetricTile label="Total time" value={formatDuration(totalDuration)} sub="last run sum" />
        </Box>
        {total > 0 && (
          <Box sx={{ mt: 1.5 }}>
            <LinearProgress
              variant="determinate"
              value={passRate}
              color={passRate >= 95 ? 'success' : passRate >= 80 ? 'warning' : 'error'}
              sx={{ height: 6, borderRadius: 3 }}
            />
          </Box>
        )}
      </Box>

      <Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
            Failing tests
          </Typography>
          <Chip
            label={failingTests.length}
            size="small"
            color={failingTests.length === 0 ? 'success' : 'error'}
            variant={failingTests.length === 0 ? 'outlined' : 'filled'}
          />
        </Box>
        {failingTests.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No failing tests. 🎉
          </Typography>
        ) : (
          <Stack spacing={1}>
            {failingTests.map(t => (
              <TestCard key={t.id} slug={slug} test={t} highlight="fail" />
            ))}
          </Stack>
        )}
      </Box>

      <Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
            Slowest tests
          </Typography>
          <Chip label={`top ${slowTests.length}`} size="small" variant="outlined" />
        </Box>
        {slowTests.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No durations recorded.
          </Typography>
        ) : (
          <Stack spacing={1}>
            {slowTests.map(t => (
              <TestCard key={t.id} slug={slug} test={t} highlight="slow" />
            ))}
          </Stack>
        )}
      </Box>
    </Stack>
  )
}

function SearchResults({ slug, tests, query }: { slug: string; tests: TestItem[]; query: string }) {
  const q = query.trim().toLowerCase()
  const filtered = useMemo(
    () =>
      tests.filter(
        t =>
          t.name.toLowerCase().includes(q) ||
          t.component.toLowerCase().includes(q) ||
          t.layer.toLowerCase().includes(q) ||
          (t.last_run_branch && t.last_run_branch.toLowerCase().includes(q))
      ),
    [tests, q]
  )

  const grouped = useMemo(() => {
    const m = new Map<string, TestItem[]>()
    for (const t of filtered) {
      const list = m.get(t.component) ?? []
      list.push(t)
      m.set(t.component, list)
    }
    return [...m.entries()].sort((a, b) => a[0].localeCompare(b[0]))
  }, [filtered])

  if (filtered.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No tests match “{query}”.
      </Typography>
    )
  }

  return (
    <Stack spacing={3}>
      <Typography variant="body2" color="text.secondary">
        {filtered.length} {filtered.length === 1 ? 'match' : 'matches'} for “{query}”
      </Typography>
      {grouped.map(([component, items]) => (
        <Box key={component}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 0.75 }}>
            {component}
          </Typography>
          <Divider sx={{ mb: 1 }} />
          <Stack spacing={1}>
            {items.map(t => (
              <TestCard key={t.id} slug={slug} test={t} />
            ))}
          </Stack>
        </Box>
      ))}
    </Stack>
  )
}

function TestsPage() {
  const { slug } = Route.useParams()
  const { data, isLoading } = useTests(slug)
  const [search, setSearch] = useState('')
  const deferredSearch = useDeferredValue(search)

  const tests = data?.tests ?? []
  const isSearching = deferredSearch.trim().length > 0

  return (
    <Stack spacing={2}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <Typography variant="h6" sx={{ fontWeight: 600, mr: 1 }}>
          Tests
        </Typography>
        <Box sx={{ flex: 1, minWidth: 200, maxWidth: { xs: '100%', sm: 360 } }}>
          <TextField
            fullWidth
            size="small"
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search tests…"
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" />
                  </InputAdornment>
                ),
                endAdornment: search ? (
                  <InputAdornment position="end">
                    <IconButton size="small" onClick={() => setSearch('')} edge="end">
                      <ClearIcon fontSize="small" />
                    </IconButton>
                  </InputAdornment>
                ) : null,
              },
            }}
          />
        </Box>
      </Box>

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : tests.length === 0 ? (
        <Typography color="text.secondary">No tests registered.</Typography>
      ) : isSearching ? (
        <SearchResults slug={slug} tests={tests} query={deferredSearch} />
      ) : (
        <Dashboard slug={slug} tests={tests} />
      )}
    </Stack>
  )
}

export const Route = createFileRoute('/project/$slug/tests/')({
  component: TestsPage,
})
