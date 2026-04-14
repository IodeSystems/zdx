import { useState, useMemo } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Collapse,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  Typography,
} from '@mui/material'
import { ExpandMore as ExpandMoreIcon } from '@mui/icons-material'
import { useErrors, useSlowQueries, useTimed, useClearErrors, useReportError, useErrorEvents, useErrorEventsGrouped, useErrorEventsTagKeys, useErrorEventsTagValues, type ErrorReportItem, type SlowQueryItem, type TimedItem } from '../api'
import { fmtDate } from '../utils/date'

function ErrorRow({ e }: { e: ErrorReportItem }) {
  const [open, setOpen] = useState(false)
  return (
    <Card variant="outlined" sx={{ mb: 0.5 }}>
      <CardContent sx={{ py: 1, '&:last-child': { pb: 1 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="caption" color="text.secondary" sx={{ minWidth: 72 }}>
            {fmtDate(e.created_at)}
          </Typography>
          <Typography variant="body2" sx={{ fontWeight: 500, flex: 1, fontFamily: 'monospace' }}>
            {e.error_name || '(unnamed)'}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {e.source}
          </Typography>
          {e.endpoint && (
            <Chip label={e.endpoint} size="small" variant="outlined" sx={{ maxWidth: 180 }} />
          )}
          {e.stack_trace && (
            <IconButton
              size="small"
              onClick={() => setOpen(o => !o)}
              sx={{ transform: open ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }}
            >
              <ExpandMoreIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
        {e.stack_trace && (
          <Collapse in={open}>
            <Box
              component="pre"
              sx={{
                mt: 1,
                p: 1,
                bgcolor: 'action.hover',
                borderRadius: 1,
                fontSize: '0.72rem',
                overflowX: 'auto',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
              }}
            >
              {e.stack_trace}
            </Box>
          </Collapse>
        )}
      </CardContent>
    </Card>
  )
}

function SlowQueryRow({ q }: { q: SlowQueryItem }) {
  const [open, setOpen] = useState(false)
  return (
    <Card variant="outlined" sx={{ mb: 0.5 }}>
      <CardContent sx={{ py: 1, '&:last-child': { pb: 1 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="caption" color="text.secondary" sx={{ minWidth: 72 }}>
            {fmtDate(q.created_at)}
          </Typography>
          <Chip
            label={`${q.duration_ms}ms`}
            size="small"
            color={q.duration_ms > 1000 ? 'error' : q.duration_ms > 300 ? 'warning' : 'default'}
          />
          {q.endpoint && (
            <Chip label={q.endpoint} size="small" variant="outlined" sx={{ maxWidth: 180 }} />
          )}
          <Typography variant="body2" sx={{ flex: 1, fontFamily: 'monospace', fontSize: '0.8rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {q.sql_text}
          </Typography>
          <IconButton
            size="small"
            onClick={() => setOpen(o => !o)}
            sx={{ transform: open ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }}
          >
            <ExpandMoreIcon fontSize="small" />
          </IconButton>
        </Box>
        <Collapse in={open}>
          <Box component="pre" sx={{ mt: 1, p: 1, bgcolor: 'action.hover', borderRadius: 1, fontSize: '0.72rem', overflowX: 'auto', whiteSpace: 'pre-wrap' }}>
            {q.sql_text}
            {q.explain_json && `\n\nEXPLAIN:\n${q.explain_json}`}
          </Box>
        </Collapse>
      </CardContent>
    </Card>
  )
}

function TimedRow({ t }: { t: TimedItem }) {
  return (
    <Card variant="outlined" sx={{ mb: 0.5 }}>
      <CardContent sx={{ py: 1, '&:last-child': { pb: 1 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="caption" color="text.secondary" sx={{ minWidth: 72 }}>
            {fmtDate(t.created_at)}
          </Typography>
          <Chip
            label={`${t.duration_ms}ms`}
            size="small"
            color={t.duration_ms > 1000 ? 'error' : t.duration_ms > 300 ? 'warning' : 'default'}
          />
          <Typography variant="body2" sx={{ flex: 1, fontFamily: 'monospace', fontSize: '0.8rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {t.name}
          </Typography>
          {t.source && t.source !== t.name && (
            <Typography variant="caption" color="text.secondary">
              {t.source}
            </Typography>
          )}
        </Box>
      </CardContent>
    </Card>
  )
}

function ErrorEventTagFilterSelect({ slug, tagKey, value, onChange }: {
  slug: string; tagKey: string; value: string; onChange: (v: string) => void
}) {
  const { data } = useErrorEventsTagValues(slug, tagKey)
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

export function ErrorsTab({ slug }: { slug: string }) {
  const [tab, setTab] = useState(0)
  const { data: errData, isLoading: errLoading } = useErrors(slug)
  const { data: qData, isLoading: qLoading } = useSlowQueries(slug)
  const { data: timedData, isLoading: timedLoading } = useTimed(slug)
  const errors = errData?.errors ?? []
  const queries = qData?.queries ?? []
  const timed = timedData?.items ?? []
  const clearErrors = useClearErrors(slug)
  const reportError = useReportError(slug)

  const [eeFilters, setEeFilters] = useState<Record<string, string>>({})
  const [eeGroupBy, setEeGroupBy] = useState('')
  const { data: eeKeysData } = useErrorEventsTagKeys(slug)
  const eeTagKeys = eeKeysData?.keys ?? []
  const eeActiveFilters = useMemo(() => {
    const f: Record<string, string> = {}
    for (const [k, v] of Object.entries(eeFilters)) {
      if (v) f[k] = v
    }
    return Object.keys(f).length > 0 ? f : undefined
  }, [eeFilters])
  const { data: eeData, isLoading: eeLoading } = useErrorEvents(slug, undefined, undefined, eeActiveFilters)
  const { data: eeGroupedData, isLoading: eeGroupLoading } = useErrorEventsGrouped(slug, eeGroupBy, eeActiveFilters)
  const errorEvents = eeData?.items ?? []
  const eeGrouped = eeGroupedData?.items

  return (
    <Box>
      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 2, borderBottom: 1, borderColor: 'divider' }}>
        <Tab label={`Errors (${errors.length})`} />
        <Tab label={`Slow queries (${queries.length})`} />
        <Tab label={`Timed (${timed.length})`} />
        <Tab label={`Error Events (${errorEvents.length})`} />
      </Tabs>

      {tab === 0 && (
        <>
          <Box sx={{ display: 'flex', gap: 1, mb: 1 }}>
            <Button size="small" variant="outlined" color="warning"
              onClick={() => reportError.mutate({ source: 'server', errorName: 'Test server error (simulated)' })}>
              Simulate server error
            </Button>
            <Button size="small" variant="outlined" color="warning"
              onClick={() => reportError.mutate({ source: 'client', errorName: 'Test client error (simulated)' })}>
              Simulate client error
            </Button>
            {errors.length > 0 && (
              <Button size="small" variant="outlined" color="error"
                onClick={() => clearErrors.mutate()}>
                Clear errors
              </Button>
            )}
          </Box>
          {errLoading && !errors.length && (
            <Typography color="text.secondary">Loading...</Typography>
          )}
          {!errLoading && errors.length === 0 && (
            <Typography color="text.secondary">No error reports.</Typography>
          )}
          {errors.map(e => <ErrorRow key={e.id} e={e} />)}
        </>
      )}

      {tab === 1 && (
        <>
          {qLoading && !queries.length && (
            <Typography color="text.secondary">Loading...</Typography>
          )}
          {!qLoading && queries.length === 0 && (
            <Typography color="text.secondary">No slow queries.</Typography>
          )}
          {queries.map(q => <SlowQueryRow key={q.id} q={q} />)}
        </>
      )}

      {tab === 2 && (
        <>
          {timedLoading && !timed.length && (
            <Typography color="text.secondary">Loading...</Typography>
          )}
          {!timedLoading && timed.length === 0 && (
            <Typography color="text.secondary">No timed records.</Typography>
          )}
          {timed.map(t => <TimedRow key={t.id} t={t} />)}
        </>
      )}

      {tab === 3 && (
        <>
          {eeTagKeys.length > 0 && (
            <Box sx={{ display: 'flex', gap: 1, mb: 2, flexWrap: 'wrap', alignItems: 'center' }}>
              {eeTagKeys.map(k => (
                <ErrorEventTagFilterSelect
                  key={k} slug={slug} tagKey={k} value={eeFilters[k] ?? ''}
                  onChange={v => setEeFilters(prev => ({ ...prev, [k]: v }))}
                />
              ))}
              <FormControl size="small" sx={{ minWidth: 140 }}>
                <InputLabel>Group by</InputLabel>
                <Select value={eeGroupBy} label="Group by" onChange={e => setEeGroupBy(e.target.value as string)}>
                  <MenuItem value="">None</MenuItem>
                  {eeTagKeys.map(k => <MenuItem key={k} value={k}>{k}</MenuItem>)}
                </Select>
              </FormControl>
              {(eeActiveFilters || eeGroupBy) && (
                <Chip label="Clear filters" size="small" onDelete={() => { setEeFilters({}); setEeGroupBy('') }} />
              )}
            </Box>
          )}
          {eeGroupBy && (
            <>
              <Typography variant="subtitle2" sx={{ mb: 1 }}>Grouped by: {eeGroupBy}</Typography>
              {eeGroupLoading ? <CircularProgress size={20} /> : (
                <TableContainer component={Paper} variant="outlined" sx={{ mb: 3 }}>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>{eeGroupBy}</TableCell>
                        <TableCell align="right">Count</TableCell>
                        <TableCell>First Seen</TableCell>
                        <TableCell>Last Seen</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {(eeGrouped ?? []).map(item => (
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
          {eeLoading && !errorEvents.length && (
            <Typography color="text.secondary">Loading...</Typography>
          )}
          {!eeLoading && errorEvents.length === 0 && (
            <Typography color="text.secondary">No error events from SDK ingest.</Typography>
          )}
          <TableContainer component={Paper} variant="outlined">
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell>Message</TableCell>
                  <TableCell>Source</TableCell>
                  <TableCell>Component</TableCell>
                  <TableCell>Tags</TableCell>
                  <TableCell>Time</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {errorEvents.map(item => (
                  <TableRow key={item.id} hover>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>{item.name}</TableCell>
                    <TableCell sx={{ maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{item.message}</TableCell>
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
        </>
      )}
    </Box>
  )
}
