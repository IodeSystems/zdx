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
import { useLogEvents, useLogEventsGrouped, useLogEventsTagKeys, useLogEventsTagValues } from '../../../api'

function TagFilterSelect({ slug, tagKey, value, onChange }: {
  slug: string; tagKey: string; value: string; onChange: (v: string) => void
}) {
  const { data } = useLogEventsTagValues(slug, tagKey)
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

const levelColor: Record<string, 'error' | 'warning' | 'info' | 'success' | 'default'> = {
  error: 'error',
  warn: 'warning',
  warning: 'warning',
  info: 'info',
  debug: 'default',
}

function LogsPage() {
  const { slug } = Route.useParams()
  const [filters, setFilters] = useState<Record<string, string>>({})
  const [groupBy, setGroupBy] = useState('')

  const { data: keysData } = useLogEventsTagKeys(slug)
  const tagKeys = keysData?.keys ?? []

  const activeFilters = useMemo(() => {
    const f: Record<string, string> = {}
    for (const [k, v] of Object.entries(filters)) {
      if (v) f[k] = v
    }
    return Object.keys(f).length > 0 ? f : undefined
  }, [filters])

  const { data: logData, isLoading } = useLogEvents(slug, undefined, undefined, activeFilters)
  const { data: groupedData, isLoading: groupLoading } = useLogEventsGrouped(slug, groupBy, activeFilters)

  const data = logData?.items
  const grouped = groupedData?.items

  return (
    <>
      <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>Logs</Typography>

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
                    <TableCell align="right">Count</TableCell>
                    <TableCell>First Seen</TableCell>
                    <TableCell>Last Seen</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {(grouped ?? []).map(item => (
                    <TableRow key={item.group_value} hover>
                      <TableCell>{item.group_value}</TableCell>
                      <TableCell align="right">{item.entry_count}</TableCell>
                      <TableCell sx={{ whiteSpace: 'nowrap' }}>{new Date(item.first_seen).toLocaleString()}</TableCell>
                      <TableCell sx={{ whiteSpace: 'nowrap' }}>{new Date(item.last_seen).toLocaleString()}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </>
      )}

      {isLoading ? <CircularProgress sx={{ m: 4 }} /> :
       (!data || data.length === 0) ? <Typography color="text.secondary">No log events.</Typography> : (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Level</TableCell>
                <TableCell>Message</TableCell>
                <TableCell>Source</TableCell>
                <TableCell>Component</TableCell>
                <TableCell>Tags</TableCell>
                <TableCell>Time</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {data.map(item => (
                <TableRow key={item.id} hover>
                  <TableCell>
                    <Chip label={item.level} size="small"
                      color={levelColor[item.level] ?? 'default'}
                      sx={{ fontSize: '0.7rem', height: 20 }} />
                  </TableCell>
                  <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.8rem', maxWidth: 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {item.message}
                  </TableCell>
                  <TableCell sx={{ fontSize: '0.75rem' }}>{item.source}</TableCell>
                  <TableCell sx={{ fontSize: '0.75rem' }}>{item.component}</TableCell>
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

export const Route = createFileRoute('/project/$slug/logs')({
  component: LogsPage,
})
