import { useMemo } from 'react'
import { Box, IconButton, Stack, Tooltip, Typography } from '@mui/material'
import { Close as CloseIcon } from '@mui/icons-material'
import { useEvents, type EventItem } from '../api'

const PANEL_WIDTH = 280

type ThreadSummary = {
  id: number
  rootEventId: number | null
  title: string
  lastActivity: string
  messageCount: number
}

function eventBodyText(e: EventItem): string {
  const s = e.summary_json
  if (s && typeof s === 'object') {
    const body = (s as Record<string, unknown>).body
    if (typeof body === 'string' && body.trim()) return body.trim()
  }
  const d = e.detail_json
  if (d && typeof d === 'object') {
    const body = (d as Record<string, unknown>).body
    if (typeof body === 'string' && body.trim()) return body.trim()
  }
  return e.event_type
}

function truncate(s: string, n: number): string {
  if (s.length <= n) return s
  return s.slice(0, n - 1) + '…'
}

export function ThreadPanel({
  slug,
  targetType,
  targetId,
  activeThreadId,
  onSelectThread,
  onClose,
}: {
  slug: string
  targetType: string
  targetId: string
  activeThreadId: number | null
  onSelectThread: (id: number | null) => void
  onClose: () => void
}) {
  const { data } = useEvents(slug, targetType, targetId, null)
  const events = data?.events ?? []

  const threads = useMemo<ThreadSummary[]>(() => {
    const byThread = new Map<number, EventItem[]>()
    for (const e of events) {
      if (e.thread_id == null) continue
      const arr = byThread.get(e.thread_id) ?? []
      arr.push(e)
      byThread.set(e.thread_id, arr)
    }
    const eventById = new Map(events.map(e => [e.id, e]))
    const out: ThreadSummary[] = []
    for (const [tid, msgs] of byThread.entries()) {
      msgs.sort((a, b) => a.created_at.localeCompare(b.created_at))
      const root = events.find(e => e.thread_id === tid && e.id < msgs[0].id) ?? eventById.get(msgs[0].id)
      const title = root ? truncate(eventBodyText(root), 60) : `Thread #${tid}`
      const last = msgs[msgs.length - 1].created_at
      out.push({
        id: tid,
        rootEventId: root?.id ?? null,
        title,
        lastActivity: last,
        messageCount: msgs.length,
      })
    }
    out.sort((a, b) => b.lastActivity.localeCompare(a.lastActivity))
    return out
  }, [events])

  return (
    <Box
      sx={{
        width: PANEL_WIDTH,
        flexShrink: 0,
        borderLeft: 1,
        borderColor: 'divider',
        pl: 2,
        ml: 1,
      }}
    >
      <Stack direction="row" sx={{ mb: 1, alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography variant="subtitle2" color="text.secondary">
          Threads ({threads.length})
        </Typography>
        <Tooltip title="Close">
          <IconButton size="small" onClick={onClose} aria-label="Close threads">
            <CloseIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Stack>

      <Box
        component="button"
        onClick={() => onSelectThread(null)}
        sx={{
          display: 'block',
          width: '100%',
          textAlign: 'left',
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          p: 0.5,
          mb: 1,
          color: activeThreadId == null ? 'primary.main' : 'text.secondary',
          fontWeight: activeThreadId == null ? 600 : 400,
        }}
      >
        Show all
      </Box>

      <Stack spacing={0.5}>
        {threads.map(t => {
          const active = activeThreadId === t.id
          return (
            <Box
              key={t.id}
              component="button"
              onClick={() => onSelectThread(t.id)}
              sx={{
                display: 'block',
                width: '100%',
                textAlign: 'left',
                background: active ? 'action.selected' : 'none',
                border: 1,
                borderColor: active ? 'primary.light' : 'divider',
                borderRadius: 1,
                cursor: 'pointer',
                p: 1,
              }}
            >
              <Typography variant="body2" sx={{ fontWeight: active ? 600 : 500 }}>
                {t.title}
              </Typography>
              <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                {new Date(t.lastActivity).toLocaleString()} — {t.messageCount} msg
              </Typography>
            </Box>
          )
        })}
        {threads.length === 0 && (
          <Typography variant="caption" color="text.secondary">
            No threads yet.
          </Typography>
        )}
      </Stack>
    </Box>
  )
}
