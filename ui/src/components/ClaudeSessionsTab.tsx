import { useState, useRef, useCallback } from 'react'
import {
  Box,
  Chip,
  Typography,
  List,
  ListItemButton,
  ListItemText,
  IconButton,
  CircularProgress,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { Link } from '@tanstack/react-router'
import {
  useClaudeSessions,
  useClaudeSessionEvents,
  type ClaudeSessionItem,
  type ClaudeEventItem,
} from '../api'

function fmtDate(ts: string) {
  return ts ? ts.slice(0, 16).replace('T', ' ') : ''
}

const EVENT_COLORS: Record<string, string> = {
  user: '#2196f3',
  assistant: '#4caf50',
  attachment: '#ff9800',
  'ai-title': '#9c27b0',
  'queue-operation': '#607d8b',
}

function EventRow({ event }: { event: ClaudeEventItem }) {
  const [expanded, setExpanded] = useState(false)
  const color = EVENT_COLORS[event.event_type] ?? '#888'

  const summary = getSummary(event)

  return (
    <Box
      sx={{
        py: 0.5,
        px: 1,
        borderLeft: 3,
        borderColor: color,
        mb: 0.5,
        cursor: 'pointer',
        '&:hover': { bgcolor: 'action.hover' },
        fontFamily: 'monospace',
        fontSize: '0.8rem',
      }}
      onClick={() => setExpanded(!expanded)}
    >
      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
        <Chip label={event.event_type} size="small" sx={{ bgcolor: color, color: '#fff', height: 20, fontSize: '0.7rem' }} />
        <Typography variant="caption" color="text.secondary" sx={{ flexShrink: 0 }}>
          #{event.seq}
        </Typography>
        <Typography variant="body2" color="text.secondary" noWrap sx={{ flex: 1, fontSize: '0.8rem' }}>
          {summary}
        </Typography>
      </Box>
      {expanded && (
        <Box
          component="pre"
          sx={{
            mt: 1,
            p: 1,
            bgcolor: 'background.default',
            borderRadius: 1,
            overflow: 'auto',
            maxHeight: 400,
            fontSize: '0.75rem',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
          }}
        >
          {JSON.stringify(event.event_json, null, 2)}
        </Box>
      )}
    </Box>
  )
}

function getSummary(event: ClaudeEventItem): string {
  const ev = event.event_json
  if (event.event_type === 'user' || event.event_type === 'assistant') {
    const msg = ev.message as Record<string, unknown> | undefined
    if (msg) {
      const content = msg.content
      if (typeof content === 'string') return content.slice(0, 120)
      if (Array.isArray(content)) {
        const text = content.find((c: Record<string, unknown>) => c.type === 'text')
        if (text && typeof text.text === 'string') return (text.text as string).slice(0, 120)
      }
    }
  }
  if (event.event_type === 'ai-title') {
    const title = ev.title as string | undefined
    return title ?? ''
  }
  if (event.event_type === 'queue-operation') {
    return `${ev.operation ?? ''}`
  }
  return ''
}

function SessionDetail({
  slug,
  session,
  onBack,
  componentSlug,
}: {
  slug: string
  session: ClaudeSessionItem
  onBack: () => void
  componentSlug: string
}) {
  const PAGE_SIZE = 200
  const [offset, setOffset] = useState(0)
  const [prevSessionId, setPrevSessionId] = useState(session.id)
  if (prevSessionId !== session.id) {
    setPrevSessionId(session.id)
    setOffset(0)
  }
  const { data, isLoading } = useClaudeSessionEvents(slug, session.id, PAGE_SIZE, offset)
  const containerRef = useRef<HTMLDivElement>(null)

  const events = data?.events ?? []
  const total = data?.total ?? 0

  const loadMore = useCallback(() => {
    if (offset + PAGE_SIZE < total) {
      setOffset((o) => o + PAGE_SIZE)
    }
  }, [offset, total])

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
        <IconButton size="small" onClick={onBack}>
          <ArrowBackIcon fontSize="small" />
        </IconButton>
        <Typography variant="subtitle2">
          {session.title || session.session_id.slice(0, 8)}
        </Typography>
        {session.issue_id && (
          <Link
            to="/project/$slug/$component/issues/$id"
            params={{ slug, component: componentSlug, id: session.issue_id }}
            style={{ textDecoration: 'none' }}
          >
            <Chip label={session.issue_id} size="small" variant="outlined" sx={{ cursor: 'pointer' }} />
          </Link>
        )}
        <Typography variant="caption" color="text.secondary">
          {total} events
        </Typography>
      </Box>

      <Box
        ref={containerRef}
        sx={{
          flex: 1,
          overflow: 'auto',
          display: 'flex',
          flexDirection: 'column-reverse',
        }}
      >
        <Box>
          {isLoading && (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 2 }}>
              <CircularProgress size={24} />
            </Box>
          )}
          {events.map((e) => (
            <EventRow key={e.id} event={e} />
          ))}
          {offset + PAGE_SIZE < total && (
            <Box sx={{ textAlign: 'center', py: 1 }}>
              <Chip label="Load more" size="small" onClick={loadMore} sx={{ cursor: 'pointer' }} />
            </Box>
          )}
        </Box>
      </Box>
    </Box>
  )
}

export function ClaudeSessionsTab({ slug, componentSlug = 'all' }: { slug: string; componentSlug?: string }) {
  const { data: sessions = [], isLoading } = useClaudeSessions(slug)
  const [selected, setSelected] = useState<ClaudeSessionItem | null>(null)

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  if (selected) {
    return (
      <SessionDetail
        slug={slug}
        session={selected}
        onBack={() => setSelected(null)}
        componentSlug={componentSlug}
      />
    )
  }

  return (
    <Box>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 2 }}>
        {sessions.length} Claude {sessions.length === 1 ? 'session' : 'sessions'}
      </Typography>
      {sessions.length === 0 ? (
        <Typography variant="body2" color="text.secondary">No Claude sessions recorded.</Typography>
      ) : (
        <List dense disablePadding>
          {sessions.map((s) => (
            <ListItemButton key={s.id} onClick={() => setSelected(s)} sx={{ borderBottom: 1, borderColor: 'divider' }}>
              <ListItemText
                primary={s.title || s.session_id.slice(0, 12)}
                secondary={
                  <Box component="span" sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                    <Typography component="span" variant="caption" color="text.secondary">
                      {fmtDate(s.created_at)}
                    </Typography>
                    {s.issue_id && <Chip label={s.issue_id} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.7rem' }} />}
                    <Typography component="span" variant="caption" color="text.disabled">
                      {s.event_count} events
                    </Typography>
                  </Box>
                }
              />
            </ListItemButton>
          ))}
        </List>
      )}
    </Box>
  )
}
