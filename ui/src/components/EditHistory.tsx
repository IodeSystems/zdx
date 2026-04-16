import { useState } from 'react'
import {
  Box,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Tooltip,
  Typography,
} from '@mui/material'
import { Close as CloseIcon, CompareArrows as DiffIcon } from '@mui/icons-material'
import { Link } from '@tanstack/react-router'
import { useHistory, type HistoryEvent } from '../api'

const VALUE_TRUNC = 60

function truncate(s: string): string {
  if (s.length <= VALUE_TRUNC) return s
  return s.slice(0, VALUE_TRUNC) + '…'
}

function eventFieldLabel(e: HistoryEvent): string {
  return e.kind === 'status' ? 'status' : (e.field ?? '')
}

function eventOldVal(e: HistoryEvent): string {
  return e.kind === 'status' ? (e.from_status ?? '') : (e.old_val ?? '')
}

function eventNewVal(e: HistoryEvent): string {
  return e.kind === 'status' ? (e.to_status ?? '') : (e.new_val ?? '')
}

function DiffModal({ event, onClose }: { event: HistoryEvent; onClose: () => void }) {
  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span>{eventFieldLabel(event)}</span>
        <IconButton size="small" onClick={onClose}>
          <CloseIcon fontSize="small" />
        </IconButton>
      </DialogTitle>
      <DialogContent>
        <Box sx={{ mb: 1 }}>
          <Typography variant="caption" color="text.secondary">Before</Typography>
          <Box sx={{ bgcolor: 'error.light', p: 1, borderRadius: 1, mt: 0.5, whiteSpace: 'pre-wrap' }}>
            <Typography variant="body2">{eventOldVal(event) || '(empty)'}</Typography>
          </Box>
        </Box>
        <Box>
          <Typography variant="caption" color="text.secondary">After</Typography>
          <Box sx={{ bgcolor: 'success.light', p: 1, borderRadius: 1, mt: 0.5, whiteSpace: 'pre-wrap' }}>
            <Typography variant="body2">{eventNewVal(event) || '(empty)'}</Typography>
          </Box>
        </Box>
      </DialogContent>
    </Dialog>
  )
}

function StatusRow({ e }: { e: HistoryEvent }) {
  return (
    <>
      <Typography variant="caption" sx={{ minWidth: 60 }}>
        status
      </Typography>
      <Chip
        label={e.from_status || '—'}
        size="small"
        variant="outlined"
        sx={{ fontSize: '0.65rem' }}
      />
      <Typography variant="caption">→</Typography>
      <Chip
        label={e.to_status || '—'}
        size="small"
        color="primary"
        sx={{ fontSize: '0.65rem' }}
      />
    </>
  )
}

function FieldRow({ e }: { e: HistoryEvent }) {
  const oldVal = e.old_val ?? ''
  const newVal = e.new_val ?? ''
  const oldTrunc = truncate(oldVal)
  const newTrunc = truncate(newVal)
  return (
    <>
      <Typography variant="caption" sx={{ minWidth: 60 }}>
        {e.field}
      </Typography>
      <Tooltip title={oldVal} disableHoverListener={oldVal === oldTrunc}>
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
          '{oldTrunc}'
        </Typography>
      </Tooltip>
      <Typography variant="caption">→</Typography>
      <Tooltip title={newVal} disableHoverListener={newVal === newTrunc}>
        <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
          '{newTrunc}'
        </Typography>
      </Tooltip>
    </>
  )
}

type Group = {
  key: string
  at: Date
  agent_id: string
  session_id: string
  user_id: string
  events: HistoryEvent[]
}

function minuteKey(iso: string): string {
  return iso.slice(0, 16)
}

function groupEvents(events: HistoryEvent[]): Group[] {
  const groups = new Map<string, Group>()
  for (const e of events) {
    const k = `${e.agent_id}|${e.session_id}|${e.user_id}|${minuteKey(e.created_at)}`
    const existing = groups.get(k)
    if (existing) {
      existing.events.push(e)
    } else {
      groups.set(k, {
        key: k,
        at: new Date(e.created_at),
        agent_id: e.agent_id,
        session_id: e.session_id,
        user_id: e.user_id,
        events: [e],
      })
    }
  }
  return Array.from(groups.values()).sort((a, b) => b.at.getTime() - a.at.getTime())
}

function hasLongValue(e: HistoryEvent): boolean {
  const oldVal = eventOldVal(e)
  const newVal = eventNewVal(e)
  return oldVal.length > VALUE_TRUNC || newVal.length > VALUE_TRUNC
}

export function EditHistory({
  slug,
  targetType,
  targetId,
}: {
  slug: string
  targetType: 'issue' | 'task'
  targetId: string
}) {
  const { data } = useHistory(targetType, targetId)
  const [diffEvent, setDiffEvent] = useState<HistoryEvent | null>(null)
  const events = data?.events ?? []

  if (events.length === 0) {
    return (
      <Box sx={{ mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
          History
        </Typography>
        <Typography variant="caption" color="text.disabled">
          No history recorded yet.
        </Typography>
      </Box>
    )
  }

  const groups = groupEvents(events)

  return (
    <Box sx={{ mb: 2 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        History ({events.length})
      </Typography>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {groups.map((g) => (
          <Box
            key={g.key}
            sx={{
              borderLeft: 2,
              borderColor: 'divider',
              pl: 1.5,
              py: 0.5,
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', mb: 0.5 }}>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ minWidth: 120 }}
                title={g.at.toISOString()}
              >
                {g.at.toLocaleString()}
              </Typography>
              {g.agent_id && (
                <Chip
                  label={`agent:${g.agent_id}`}
                  size="small"
                  variant="outlined"
                  sx={{ fontSize: '0.65rem' }}
                />
              )}
              {g.session_id && (
                <Link
                  to="/project/$slug/agents/$sessionId"
                  params={{ slug, sessionId: g.session_id }}
                  style={{ textDecoration: 'none' }}
                >
                  <Chip
                    label={`session:${g.session_id.slice(0, 8)}`}
                    size="small"
                    variant="outlined"
                    clickable
                    sx={{ fontSize: '0.65rem' }}
                  />
                </Link>
              )}
              {g.user_id && (
                <Typography variant="caption" color="text.disabled">
                  user:{g.user_id}
                </Typography>
              )}
            </Box>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25, pl: 1 }}>
              {g.events.map((e) => (
                <Box
                  key={`${e.kind}:${e.id}`}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1,
                    flexWrap: 'wrap',
                  }}
                >
                  {e.kind === 'status' ? <StatusRow e={e} /> : <FieldRow e={e} />}
                  {e.kind === 'field' && hasLongValue(e) && (
                    <IconButton size="small" onClick={() => setDiffEvent(e)} title="View diff">
                      <DiffIcon fontSize="inherit" />
                    </IconButton>
                  )}
                </Box>
              ))}
            </Box>
          </Box>
        ))}
      </Box>
      {diffEvent && <DiffModal event={diffEvent} onClose={() => setDiffEvent(null)} />}
    </Box>
  )
}
