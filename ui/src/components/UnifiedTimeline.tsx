import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
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
import {
  useComments,
  useHistory,
  useIssueResolutions,
  useMarkCommentsRead,
  useReservationsByIssue,
  useTodosByIssue,
  type CommentItem,
  type HistoryEvent,
  type IssueWorkItem,
  type ReservationItem,
  type ResolutionItem,
  type SoloItem,
} from '../api'
import { MarkdownContent } from './MarkdownContent'

type Kind = 'todo' | 'reservation' | 'resolution' | 'work' | 'history' | 'comment'

const KIND_LABEL: Record<Kind, string> = {
  todo: 'Todos',
  reservation: 'Reservations',
  resolution: 'Resolutions',
  work: 'Work Log',
  history: 'History',
  comment: 'Comments',
}

const KIND_COLOR: Record<Kind, 'success' | 'info' | 'secondary' | 'warning' | 'default' | 'primary'> = {
  todo: 'primary',
  reservation: 'info',
  resolution: 'success',
  work: 'secondary',
  history: 'default',
  comment: 'warning',
}

const KIND_ORDER: Kind[] = ['todo', 'reservation', 'resolution', 'work', 'history', 'comment']

type HistoryGroup = {
  key: string
  at: Date
  agent_id: string
  session_id: string
  user_id: string
  events: HistoryEvent[]
}

type TodoEventKind = 'created' | 'claimed' | 'released' | 'resolved'

type TimelineEvent =
  | { kind: 'todo'; subKind: TodoEventKind; ts: number; payload: SoloItem; releasedBy?: string }
  | { kind: 'reservation'; ts: number; payload: ReservationItem }
  | { kind: 'resolution'; ts: number; payload: ResolutionItem }
  | { kind: 'work'; ts: number; payload: IssueWorkItem; idx: number }
  | { kind: 'history'; ts: number; payload: HistoryGroup }
  | { kind: 'comment'; ts: number; payload: CommentItem; replies: CommentItem[] }

const VALUE_TRUNC = 60

function truncate(s: string): string {
  if (s.length <= VALUE_TRUNC) return s
  return s.slice(0, VALUE_TRUNC) + '…'
}

function eventOldVal(e: HistoryEvent): string {
  return e.kind === 'status' ? (e.from_status ?? '') : (e.old_val ?? '')
}

function eventNewVal(e: HistoryEvent): string {
  return e.kind === 'status' ? (e.to_status ?? '') : (e.new_val ?? '')
}

function hasLongValue(e: HistoryEvent): boolean {
  return eventOldVal(e).length > VALUE_TRUNC || eventNewVal(e).length > VALUE_TRUNC
}

function eventFieldLabel(e: HistoryEvent): string {
  return e.kind === 'status' ? 'status' : (e.field ?? '')
}

function minuteKey(iso: string): string {
  return iso.slice(0, 16)
}

function groupHistory(events: HistoryEvent[]): HistoryGroup[] {
  const groups = new Map<string, HistoryGroup>()
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
  return Array.from(groups.values())
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

const TODO_EVENT_COLOR: Record<TodoEventKind, 'default' | 'primary' | 'success' | 'info' | 'warning'> = {
  created: 'primary',
  claimed: 'info',
  released: 'warning',
  resolved: 'success',
}

const TODO_EVENT_LABEL: Record<TodoEventKind, string> = {
  created: 'created',
  claimed: 'claimed',
  released: 'released',
  resolved: 'resolved',
}

function TodoRow({ t, subKind, releasedBy }: { t: SoloItem; subKind: TodoEventKind; releasedBy?: string }) {
  const eventColor = TODO_EVENT_COLOR[subKind]
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start', flexWrap: 'wrap' }}>
      <Chip label={TODO_EVENT_LABEL[subKind]} size="small" color={eventColor} variant="outlined" />
      {subKind === 'claimed' && t.claimed_by && (
        <Chip label={t.claimed_by} size="small" color="info" variant="filled" sx={{ fontSize: '0.65rem' }} />
      )}
      {subKind === 'released' && releasedBy && (
        <Chip label={releasedBy} size="small" color="warning" variant="filled" sx={{ fontSize: '0.65rem' }} />
      )}
      {t.blocked && (
        <Chip
          label={t.blocked_reason ? `blocked: ${t.blocked_reason}` : 'blocked'}
          size="small"
          color="error"
          variant="outlined"
          sx={{ fontSize: '0.65rem' }}
        />
      )}
      <Box sx={{ flex: 1 }}>
        <Typography variant="body2">{t.title || t.text.slice(0, 80)}</Typography>
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>{t.key}</Typography>
      </Box>
    </Box>
  )
}

function ReservationRow({ r, slug }: { r: ReservationItem; slug: string }) {
  const isActive = !r.released_at && new Date(r.lease_expires_at) > new Date()
  const statusLabel = r.released_at ? 'released' : isActive ? 'active' : 'expired'
  const statusColor: 'success' | 'default' | 'warning' = isActive ? 'success' : r.released_at ? 'default' : 'warning'
  const inner = (
    <Box>
      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip label={statusLabel} size="small" color={statusColor} variant="outlined" />
        {r.session_id && r.session_status && (
          <Chip
            label={r.session_status || 'session'}
            size="small"
            color={r.session_status === 'ok' ? 'success' : r.session_status === 'errored' ? 'error' : r.session_status === 'churn' ? 'warning' : 'default'}
            variant="filled"
          />
        )}
        <Typography variant="caption" color="text.secondary">{r.claimed_by}</Typography>
      </Box>
      {r.session_header && (
        <Typography variant="body2" sx={{ mt: 0.5 }}>{r.session_header}</Typography>
      )}
      {r.todo_text && (
        <Box sx={{ mt: 0.5 }}>
          <MarkdownContent slug={slug}>{r.todo_text}</MarkdownContent>
        </Box>
      )}
      <Box sx={{ display: 'flex', gap: 2, mt: 0.5, flexWrap: 'wrap' }}>
        <Typography variant="caption" color="text.disabled">
          acquired: {new Date(r.claimed_at).toLocaleString()}
        </Typography>
        {r.released_at && (
          <Typography variant="caption" color="text.disabled">
            released: {new Date(r.released_at).toLocaleString()}
          </Typography>
        )}
      </Box>
    </Box>
  )
  return r.session_id ? (
    <Box
      component={Link as any}
      to="/project/$slug/agents/$sessionId"
      params={{ slug, sessionId: String(r.session_id) }}
      sx={{ display: 'block', textDecoration: 'none', color: 'inherit', '&:hover': { opacity: 0.8 } }}
    >
      {inner}
    </Box>
  ) : (
    <Box>{inner}</Box>
  )
}

function ResolutionRow({ r }: { r: ResolutionItem }) {
  return (
    <Box>
      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip label={r.source} size="small" variant="outlined" />
        {r.branch_of_origin && <Chip label={r.branch_of_origin} size="small" variant="outlined" color="info" />}
        {r.reverted && <Chip label={`reverted by ${r.reverted_by?.slice(0, 10)}`} size="small" color="error" />}
        <Typography variant="caption" color="text.secondary">
          {r.author}
        </Typography>
      </Box>
      <Box sx={{ mt: 0.5 }}>
        {r.commits.map(c => (
          <Typography key={c.sha} variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
            {c.sha.slice(0, 10)}
          </Typography>
        ))}
      </Box>
    </Box>
  )
}

function WorkRow({ e }: { e: IssueWorkItem }) {
  return (
    <Box>
      <Typography variant="caption" color="text.secondary">
        {e.agent}
      </Typography>
      {e.note && <Typography variant="body2">{e.note}</Typography>}
    </Box>
  )
}

function HistoryRow({
  g,
  slug,
  onDiff,
}: {
  g: HistoryGroup
  slug: string
  onDiff: (e: HistoryEvent) => void
}) {
  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', mb: 0.5 }}>
        {g.agent_id && (
          <Chip label={`agent:${g.agent_id}`} size="small" variant="outlined" sx={{ fontSize: '0.65rem' }} />
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
        {g.events.map((e) => {
          if (e.kind === 'status') {
            return (
              <Box key={`status:${e.id}`} sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
                <Typography variant="caption" sx={{ minWidth: 60 }}>status</Typography>
                <Chip label={e.from_status || '—'} size="small" variant="outlined" sx={{ fontSize: '0.65rem' }} />
                <Typography variant="caption">→</Typography>
                <Chip label={e.to_status || '—'} size="small" color="primary" sx={{ fontSize: '0.65rem' }} />
              </Box>
            )
          }
          const oldVal = e.old_val ?? ''
          const newVal = e.new_val ?? ''
          const oldTrunc = truncate(oldVal)
          const newTrunc = truncate(newVal)
          return (
            <Box key={`field:${e.id}`} sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
              <Typography variant="caption" sx={{ minWidth: 60 }}>{e.field}</Typography>
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
              {hasLongValue(e) && (
                <IconButton size="small" onClick={() => onDiff(e)} title="View diff">
                  <DiffIcon fontSize="inherit" />
                </IconButton>
              )}
            </Box>
          )
        })}
      </Box>
    </Box>
  )
}

function CommentRow({ c, replies, slug }: { c: CommentItem; replies: CommentItem[]; slug: string }) {
  const renderOne = (cm: CommentItem, depth: number, kids: CommentItem[], byParent: Map<number, CommentItem[]>) => {
    const displayAuthor = cm.author_alias
      ? `${cm.author_alias} (${cm.author || 'anonymous'})`
      : (cm.author || 'anonymous')
    return (
      <Box
        key={cm.id}
        id={`C-${cm.id}`}
        sx={{ ml: depth * 3, borderRadius: 1, transition: 'background-color 0.5s', '&.comment-hash-highlight': { backgroundColor: 'warning.light' } }}
      >
        <Box sx={{ borderLeft: 2, borderColor: cm.unread ? 'info.main' : 'success.light', pl: 1.5 }}>
          <Typography variant="caption" sx={{ color: cm.unread ? '#1976d2' : '#2e7d32' }}>
            {displayAuthor}
          </Typography>
          <Box sx={{ mt: 0.25 }}>
            <MarkdownContent slug={slug}>{cm.body}</MarkdownContent>
          </Box>
        </Box>
        {kids.map(child => renderOne(child, depth + 1, byParent.get(child.id) ?? [], byParent))}
      </Box>
    )
  }
  const byParent = new Map<number, CommentItem[]>()
  for (const r of replies) {
    if (r.parent_id) {
      const arr = byParent.get(r.parent_id) ?? []
      arr.push(r)
      byParent.set(r.parent_id, arr)
    }
  }
  const directReplies = byParent.get(c.id) ?? []
  return renderOne(c, 0, directReplies, byParent)
}

export function UnifiedTimeline({
  slug,
  issueId,
  workEntries,
}: {
  slug: string
  issueId: string
  workEntries: IssueWorkItem[]
}) {
  const { data: reservationsData } = useReservationsByIssue(slug, issueId)
  const { data: resolutions } = useIssueResolutions(slug, issueId)
  const { data: historyData } = useHistory('issue', issueId)
  const { data: commentsData } = useComments(slug, 'issue', issueId)
  const { data: todosData } = useTodosByIssue(slug, issueId)
  const markRead = useMarkCommentsRead()

  const reservations = reservationsData?.reservations ?? []
  const resols = resolutions ?? []
  const histEvents = historyData?.events ?? []
  const comments = commentsData?.comments ?? []
  const todos = todosData?.todos ?? []

  const [active, setActive] = useState<Set<Kind>>(new Set(KIND_ORDER))
  const [diffEvent, setDiffEvent] = useState<HistoryEvent | null>(null)
  const hasMarkedRead = useRef(false)
  const hasScrolledToHash = useRef(false)

  const hasUnread = comments.some(c => c.unread)
  useEffect(() => {
    if (hasUnread && !hasMarkedRead.current) {
      hasMarkedRead.current = true
      markRead.mutate({ slug, target_type: 'issue', target_id: issueId })
    }
  }, [hasUnread, slug, issueId])

  useEffect(() => {
    if (hasScrolledToHash.current || comments.length === 0) return
    const m = window.location.hash.match(/^#C-(\d+)$/)
    if (!m) return
    const el = document.getElementById(`C-${m[1]}`)
    if (!el) return
    hasScrolledToHash.current = true
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    el.classList.add('comment-hash-highlight')
    const t = setTimeout(() => el.classList.remove('comment-hash-highlight'), 2000)
    return () => clearTimeout(t)
  }, [comments.length])

  const events = useMemo<TimelineEvent[]>(() => {
    const out: TimelineEvent[] = []
    const todoById = new Map(todos.map(t => [String(t.id), t]))
    for (const t of todos) {
      out.push({ kind: 'todo', subKind: 'created', ts: new Date(t.created_at).getTime(), payload: t })
      if (t.claimed_at) {
        out.push({ kind: 'todo', subKind: 'claimed', ts: new Date(t.claimed_at).getTime(), payload: t })
      }
      if (t.resolved_at) {
        out.push({ kind: 'todo', subKind: 'resolved', ts: new Date(t.resolved_at).getTime(), payload: t })
      }
    }
    for (const r of reservations) {
      out.push({ kind: 'reservation', ts: new Date(r.claimed_at).getTime(), payload: r })
      if (r.target_type === 'todo' && r.released_at) {
        const todo = todoById.get(r.target_id)
        if (todo) {
          out.push({ kind: 'todo', subKind: 'released', ts: new Date(r.released_at).getTime(), payload: todo, releasedBy: r.claimed_by })
        }
      }
    }
    for (const r of resols) {
      out.push({ kind: 'resolution', ts: new Date(r.resolved_at).getTime(), payload: r })
    }
    workEntries.forEach((e, idx) => {
      out.push({ kind: 'work', ts: new Date(e.created_at).getTime(), payload: e, idx })
    })
    for (const g of groupHistory(histEvents)) {
      out.push({ kind: 'history', ts: g.at.getTime(), payload: g })
    }
    const topLevel = comments.filter(c => !c.parent_id)
    for (const c of topLevel) {
      out.push({ kind: 'comment', ts: new Date(c.created_at).getTime(), payload: c, replies: comments })
    }
    out.sort((a, b) => b.ts - a.ts)
    return out
  }, [todos, reservations, resols, workEntries, histEvents, comments])

  const visible = events.filter(e => active.has(e.kind))

  const counts = useMemo(() => {
    const c: Record<Kind, number> = { todo: 0, reservation: 0, resolution: 0, work: 0, history: 0, comment: 0 }
    for (const e of events) c[e.kind]++
    return c
  }, [events])

  const allOn = active.size === KIND_ORDER.length
  const toggleKind = (k: Kind) => {
    setActive(prev => {
      if (prev.size === KIND_ORDER.length) return new Set([k])
      const next = new Set(prev)
      if (next.has(k)) next.delete(k)
      else next.add(k)
      if (next.size === 0) return new Set(KIND_ORDER)
      return next
    })
  }

  if (events.length === 0) {
    return (
      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
          Timeline
        </Typography>
        <Typography variant="caption" color="text.disabled">
          No activity yet.
        </Typography>
      </Box>
    )
  }

  return (
    <Box sx={{ mb: 3 }}>
      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap', mb: 1 }}>
        <Typography variant="subtitle2" color="text.secondary">
          Timeline ({events.length})
        </Typography>
        <Chip
          label="All"
          size="small"
          variant={allOn ? 'filled' : 'outlined'}
          color={allOn ? 'primary' : 'default'}
          onClick={() => setActive(new Set(KIND_ORDER))}
          clickable
        />
        {KIND_ORDER.map(k => {
          const on = active.has(k) && !allOn
          return (
            <Chip
              key={k}
              label={`${KIND_LABEL[k]} (${counts[k]})`}
              size="small"
              variant={on ? 'filled' : 'outlined'}
              color={on ? KIND_COLOR[k] : 'default'}
              onClick={() => toggleKind(k)}
              disabled={counts[k] === 0}
              clickable
            />
          )
        })}
      </Box>

      {visible.length === 0 ? (
        <Typography variant="caption" color="text.disabled">
          No events match the current filter.
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {visible.map(ev => {
            const ts = new Date(ev.ts)
            const key =
              ev.kind === 'todo' ? `todo:${ev.payload.id}:${ev.subKind}` :
              ev.kind === 'comment' ? `comment:${ev.payload.id}` :
              ev.kind === 'reservation' ? `reservation:${ev.payload.id}` :
              ev.kind === 'resolution' ? `resolution:${ev.payload.id}` :
              ev.kind === 'work' ? `work:${ev.idx}` :
              `history:${ev.payload.key}`
            return (
              <Box
                key={key}
                sx={{
                  borderLeft: 2,
                  borderColor: 'divider',
                  pl: 1.5,
                  py: 0.5,
                }}
              >
                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap', mb: 0.5 }}>
                  <Chip
                    label={KIND_LABEL[ev.kind]}
                    size="small"
                    color={KIND_COLOR[ev.kind]}
                    variant="outlined"
                    sx={{ fontSize: '0.65rem' }}
                  />
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    title={ts.toISOString()}
                  >
                    {ts.toLocaleString()}
                  </Typography>
                </Box>
                {ev.kind === 'todo' && <TodoRow t={ev.payload} subKind={ev.subKind} releasedBy={'releasedBy' in ev ? ev.releasedBy : undefined} />}
                {ev.kind === 'reservation' && <ReservationRow r={ev.payload} slug={slug} />}
                {ev.kind === 'resolution' && <ResolutionRow r={ev.payload} />}
                {ev.kind === 'work' && <WorkRow e={ev.payload} />}
                {ev.kind === 'history' && <HistoryRow g={ev.payload} slug={slug} onDiff={setDiffEvent} />}
                {ev.kind === 'comment' && (
                  <CommentRow c={ev.payload} replies={ev.replies} slug={slug} />
                )}
              </Box>
            )
          })}
        </Box>
      )}

      {diffEvent && <DiffModal event={diffEvent} onClose={() => setDiffEvent(null)} />}
    </Box>
  )
}
