import { createFileRoute, Link } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import {
  Box,
  Chip,
  CircularProgress,
  Collapse,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material'
import { KeyboardArrowDown, KeyboardArrowUp } from '@mui/icons-material'
import { useTests, useTestHistory } from '../../../../api'
import type { TestItem } from '../../../../api'

const STATUS_COLOR: Record<string, 'success' | 'error' | 'default' | 'warning'> = {
  pass: 'success',
  fail: 'error',
  skip: 'warning',
}

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

function TestHistoryRow({ slug, test }: { slug: string; test: TestItem }) {
  const [open, setOpen] = useState(false)
  const { data, isLoading } = useTestHistory(slug, test.name, open)
  const history = data?.history ?? []

  return (
    <>
      <TableRow hover>
        <TableCell sx={{ width: 32, p: 0, pl: 1 }}>
          <IconButton size="small" onClick={() => setOpen(!open)}>
            {open ? <KeyboardArrowUp fontSize="small" /> : <KeyboardArrowDown fontSize="small" />}
          </IconButton>
        </TableCell>
        <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>
          <Link
            to="/project/$slug/tests/$id"
            params={{ slug, id: String(test.id) }}
            style={{ color: 'inherit', textDecoration: 'none' }}
            onMouseOver={(e) => { (e.currentTarget as HTMLElement).style.textDecoration = 'underline' }}
            onMouseOut={(e) => { (e.currentTarget as HTMLElement).style.textDecoration = 'none' }}
          >
            {test.name}
          </Link>
        </TableCell>
        <TableCell>
          <Chip label={test.layer} size="small" variant="outlined" sx={{ fontSize: '0.75rem', height: 20 }} />
        </TableCell>
        <TableCell>
          <Chip label={test.status} size="small" color={STATUS_COLOR[test.status] ?? 'default'} />
        </TableCell>
        <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{formatDuration(test.duration_ms)}</TableCell>
        <TableCell sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>
          {test.last_run_at ? formatRelativeTime(test.last_run_at) : '—'}
        </TableCell>
      </TableRow>
      <TableRow>
        <TableCell colSpan={6} sx={{ py: 0, borderBottom: open ? undefined : 'none' }}>
          <Collapse in={open} timeout="auto" unmountOnExit>
            <Box sx={{ py: 1, pl: 4 }}>
              {isLoading ? (
                <CircularProgress size={16} />
              ) : history.length === 0 ? (
                <Typography variant="body2" color="text.secondary">No history recorded.</Typography>
              ) : (
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>Status</TableCell>
                      <TableCell>Duration</TableCell>
                      <TableCell>Run At</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {history.map((h) => (
                      <TableRow key={h.id}>
                        <TableCell>
                          <Chip label={h.status} size="small" color={STATUS_COLOR[h.status] ?? 'default'} />
                        </TableCell>
                        <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                          {formatDuration(h.duration_ms)}
                        </TableCell>
                        <TableCell sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>
                          {formatRelativeTime(h.run_at)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </Box>
          </Collapse>
        </TableCell>
      </TableRow>
    </>
  )
}

const emptyArray: TestItem[] = []
function TestsPage() {
  const { slug } = Route.useParams()
  const { data, isLoading } = useTests(slug)
  const [filterComponent, setFilterComponent] = useState('')
  const [filterLayer, setFilterLayer] = useState('')
  const [filterStatus, setFilterStatus] = useState('')

  const tests = data?.tests ?? emptyArray

  const components = useMemo(() => [...new Set(tests.map(t => t.component))].sort(), [tests])
  const layers = useMemo(() => [...new Set(tests.map(t => t.layer))].sort(), [tests])
  const statuses = useMemo(() => [...new Set(tests.map(t => t.status))].sort(), [tests])

  const filtered = useMemo(() => tests.filter(t =>
    (!filterComponent || t.component === filterComponent) &&
    (!filterLayer || t.layer === filterLayer) &&
    (!filterStatus || t.status === filterStatus)
  ), [tests, filterComponent, filterLayer, filterStatus])

  const grouped = useMemo(() => {
    const m = new Map<string, typeof filtered>()
    for (const t of filtered) {
      const list = m.get(t.component) ?? []
      list.push(t)
      m.set(t.component, list)
    }
    return [...m.entries()].sort((a, b) => a[0].localeCompare(b[0]))
  }, [filtered])

  const hasFilters = filterComponent || filterLayer || filterStatus

  const passingCount = tests.filter(t => t.status === 'pass').length
  const failingCount = tests.filter(t => t.status === 'fail').length

  return (
    <>
      <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>Tests</Typography>

      {!isLoading && tests.length > 0 && (
        <Box sx={{ display: 'flex', gap: 1.5, mb: 2, alignItems: 'center' }}>
          <Chip label={`${tests.length} total`} size="small" variant="outlined" />
          {passingCount > 0 && <Chip label={`${passingCount} pass`} size="small" color="success" variant="outlined" />}
          {failingCount > 0 && <Chip label={`${failingCount} fail`} size="small" color="error" variant="outlined" />}
        </Box>
      )}

      {(components.length > 1 || layers.length > 1 || statuses.length > 1) && (
        <Box sx={{ display: 'flex', gap: 1, mb: 2, flexWrap: 'wrap', alignItems: 'center' }}>
          {components.length > 1 && (
            <FormControl size="small" sx={{ minWidth: 140 }}>
              <InputLabel>Component</InputLabel>
              <Select value={filterComponent} label="Component" onChange={e => setFilterComponent(e.target.value)}>
                <MenuItem value="">All</MenuItem>
                {components.map(c => <MenuItem key={c} value={c}>{c}</MenuItem>)}
              </Select>
            </FormControl>
          )}
          {layers.length > 1 && (
            <FormControl size="small" sx={{ minWidth: 120 }}>
              <InputLabel>Layer</InputLabel>
              <Select value={filterLayer} label="Layer" onChange={e => setFilterLayer(e.target.value)}>
                <MenuItem value="">All</MenuItem>
                {layers.map(l => <MenuItem key={l} value={l}>{l}</MenuItem>)}
              </Select>
            </FormControl>
          )}
          {statuses.length > 1 && (
            <FormControl size="small" sx={{ minWidth: 120 }}>
              <InputLabel>Status</InputLabel>
              <Select value={filterStatus} label="Status" onChange={e => setFilterStatus(e.target.value)}>
                <MenuItem value="">All</MenuItem>
                {statuses.map(s => <MenuItem key={s} value={s}>{s}</MenuItem>)}
              </Select>
            </FormControl>
          )}
          {hasFilters && (
            <Chip label="Clear" size="small" onDelete={() => { setFilterComponent(''); setFilterLayer(''); setFilterStatus('') }} />
          )}
        </Box>
      )}

      {isLoading ? <CircularProgress sx={{ m: 4 }} /> :
       tests.length === 0 ? <Typography color="text.secondary">No tests registered.</Typography> : (
        <>
          {grouped.map(([component, items]) => (
            <Box key={component} sx={{ mb: 3 }}>
              <Typography variant="subtitle2" sx={{ mb: 0.5, fontWeight: 600 }}>{component}</Typography>
              <TableContainer component={Paper} variant="outlined">
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ width: 32 }} />
                      <TableCell>Name</TableCell>
                      <TableCell>Layer</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Duration</TableCell>
                      <TableCell>Last Run</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {items.map(t => (
                      <TestHistoryRow key={t.id} slug={slug} test={t} />
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </Box>
          ))}
        </>
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/tests/')({
  component: TestsPage,
})
