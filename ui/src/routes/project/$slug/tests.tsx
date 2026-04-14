import { createFileRoute } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import {
  Box,
  Chip,
  CircularProgress,
  FormControl,
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
import { useTests } from '../../../api'

const STATUS_COLOR: Record<string, 'success' | 'error' | 'default' | 'warning'> = {
  pass: 'success',
  fail: 'error',
  skip: 'warning',
}

function TestsPage() {
  const { slug } = Route.useParams()
  const { data, isLoading } = useTests(slug)
  const [filterComponent, setFilterComponent] = useState('')
  const [filterLayer, setFilterLayer] = useState('')
  const [filterStatus, setFilterStatus] = useState('')

  const tests = data?.tests ?? []

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
                      <TableCell>Name</TableCell>
                      <TableCell>Layer</TableCell>
                      <TableCell>Status</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {items.map(t => (
                      <TableRow key={t.id} hover>
                        <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{t.name}</TableCell>
                        <TableCell>
                          <Chip label={t.layer} size="small" variant="outlined" sx={{ fontSize: '0.75rem', height: 20 }} />
                        </TableCell>
                        <TableCell>
                          <Chip label={t.status} size="small" color={STATUS_COLOR[t.status] ?? 'default'} />
                        </TableCell>
                      </TableRow>
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

export const Route = createFileRoute('/project/$slug/tests')({
  component: TestsPage,
})
