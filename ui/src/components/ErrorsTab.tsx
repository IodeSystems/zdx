import { useState } from 'react'
import {
  Box,
  Card,
  CardContent,
  Chip,
  Collapse,
  IconButton,
  Tab,
  Tabs,
  Typography,
} from '@mui/material'
import { ExpandMore as ExpandMoreIcon } from '@mui/icons-material'
import { useErrors, useSlowQueries, type ErrorReportItem, type SlowQueryItem } from '../api'

function fmtDate(ts: string) {
  return ts ? ts.slice(0, 10) : ''
}

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

export function ErrorsTab({ slug, componentSlug: _componentSlug }: { slug: string; componentSlug?: string }) {
  const [tab, setTab] = useState(0)
  const { data: errors = [], isLoading: errLoading } = useErrors(slug)
  const { data: queries = [], isLoading: qLoading } = useSlowQueries(slug)

  return (
    <Box>
      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 2, borderBottom: 1, borderColor: 'divider' }}>
        <Tab label={`Errors (${errors.length})`} />
        <Tab label={`Slow queries (${queries.length})`} />
      </Tabs>

      {tab === 0 && (
        <>
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
    </Box>
  )
}
