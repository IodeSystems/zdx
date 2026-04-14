import { createFileRoute } from '@tanstack/react-router'
import { useState, useMemo } from 'react'
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
import { useTimed, useTimedGrouped, useTimedTagKeys, useTimedTagValues } from '../../../api'

function TagFilterSelect({ slug, tagKey, value, onChange }: {
  slug: string; tagKey: string; value: string; onChange: (v: string) => void
}) {
  const { data } = useTimedTagValues(slug, tagKey)
  const values = data?.values ?? []
  return (
    <FormControl size="small" sx={{ minWidth: 140 }}>
      <InputLabel>{tagKey}</InputLabel>
      <Select value={value} label={tagKey} onChange={e => onChange(e.target.value as string)}>
        <MenuItem value="">All</MenuItem>
        {values.map(v => <MenuItem key={v} value={v}>{v}</MenuItem>)}
      </Select>
    </FormControl>
  )
}

function TimingsPage() {
  const { slug } = Route.useParams()
  const [filters, setFilters] = useState<Record<string, string>>({})
  const [groupBy, setGroupBy] = useState('')

  const { data: keysData } = useTimedTagKeys(slug)
  const tagKeys = keysData?.keys ?? []

  const activeFilters = useMemo(() => {
    const f: Record<string, string> = {}
    for (const [k, v] of Object.entries(filters)) {
      if (v) f[k] = v
    }
    return Object.keys(f).length > 0 ? f : undefined
  }, [filters])

  const { data: timedData, isLoading } = useTimed(slug, undefined, undefined, activeFilters)
  const { data: groupedData, isLoading: groupLoading } = useTimedGrouped(
    slug, groupBy, activeFilters
  )

  const data = timedData?.items
  const grouped = groupedData?.items

  return (
    <>
      <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>Timings</Typography>

      {tagKeys.length > 0 && (
        <Box sx={{ display: 'flex', gap: 1, mb: 2, flexWrap: 'wrap', alignItems: 'center' }}>
          {tagKeys.map(k => (
            <TagFilterSelect
              key={k} slug={slug} tagKey={k} value={filters[k] ?? ''}
              onChange={v => setFilters(prev => ({ ...prev, [k]: v }))}
            />
          ))}
          <FormControl size="small" sx={{ minWidth: 140 }}>
            <InputLabel>Group by</InputLabel>
            <Select value={groupBy} label="Group by" onChange={e => setGroupBy(e.target.value as string)}>
              <MenuItem value="">None</MenuItem>
              {tagKeys.map(k => <MenuItem key={k} value={k}>{k}</MenuItem>)}
            </Select>
          </FormControl>
          {activeFilters && (
            <Chip label="Clear filters" size="small" onDelete={() => { setFilters({}); setGroupBy('') }} />
          )}
        </Box>
      )}

      {groupBy && (
        <>
          <Typography variant="subtitle2" sx={{ mb: 1 }}>Grouped by: {groupBy}</Typography>
          {groupLoading ? <CircularProgress size={20} /> : (
            <TableContainer component={Paper} variant="outlined" sx={{ mb: 3 }}>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>{groupBy}</TableCell>
                    <TableCell align="right">Entries</TableCell>
                    <TableCell align="right">Max (ms)</TableCell>
                    <TableCell align="right">Avg (ms)</TableCell>
                    <TableCell align="right">Total Count</TableCell>
                    <TableCell align="right">Total (ms)</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {(grouped ?? []).map(item => (
                    <TableRow key={item.group_value} hover>
                      <TableCell>{item.group_value}</TableCell>
                      <TableCell align="right">{item.entry_count}</TableCell>
                      <TableCell align="right">{item.max_ms}</TableCell>
                      <TableCell align="right">{item.avg_ms}</TableCell>
                      <TableCell align="right">{item.sum_count}</TableCell>
                      <TableCell align="right">{item.sum_total_ms}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </>
      )}

      {isLoading ? <CircularProgress sx={{ m: 4 }} /> :
       (!data || data.length === 0) ? <Typography color="text.secondary">No timed entries.</Typography> : (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell align="right">Max (ms)</TableCell>
                <TableCell align="right">Avg (ms)</TableCell>
                <TableCell align="right">Count</TableCell>
                <TableCell align="right">Total (ms)</TableCell>
                <TableCell>Source</TableCell>
                <TableCell>Tags</TableCell>
                <TableCell>Last Seen</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {data.map(item => (
                <TableRow key={item.id} hover>
                  <TableCell>{item.name}</TableCell>
                  <TableCell align="right">{item.duration_ms}</TableCell>
                  <TableCell align="right">{item.avg_ms}</TableCell>
                  <TableCell align="right">{item.count}</TableCell>
                  <TableCell align="right">{item.total_ms}</TableCell>
                  <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>{item.source}</TableCell>
                  <TableCell sx={{ maxWidth: 200 }}>
                    <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                      {Object.entries(item.context_json ?? {}).map(([k, v]) => (
                        <Chip key={k} label={`${k}=${v}`} size="small" variant="outlined"
                          sx={{ fontSize: '0.7rem', height: 20 }} />
                      ))}
                    </Box>
                  </TableCell>
                  <TableCell sx={{ whiteSpace: 'nowrap' }}>{new Date(item.created_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/timings')({
  component: TimingsPage,
})
