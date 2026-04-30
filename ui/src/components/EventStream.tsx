import { useState, useMemo } from 'react'
import {
  Box,
  Button,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import { Forum as ForumIcon, Reply as ReplyIcon } from '@mui/icons-material'
import { useEvents, usePostComment, useReplyToEvent, type EventItem } from '../api'
import { MarkdownContent } from './MarkdownContent'
import { ThreadPanel } from './ThreadPanel'

type RendererProps = { slug: string; event: EventItem }
type Renderer = (p: RendererProps) => React.ReactNode

function getDetail(e: EventItem): Record<string, unknown> {
  const d = e.detail_json
  return d && typeof d === 'object' ? (d as Record<string, unknown>) : {}
}

function CommentEvent({ slug, event }: RendererProps) {
  const body = String(getDetail(event).body ?? '')
  return <MarkdownContent slug={slug}>{body}</MarkdownContent>
}

function RevisionEvent({ event }: RendererProps) {
  const d = getDetail(event)
  const field = String(d.field ?? 'field')
  const fromV = Number(d.from_version ?? 0)
  const toV = Number(d.to_version ?? 0)
  return (
    <Typography variant="body2" color="text.secondary">
      {field} updated (v{fromV} → v{toV})
    </Typography>
  )
}

function StatusChangeEvent({ event }: RendererProps) {
  const d = getDetail(event)
  const kind = String(d.target_kind ?? '')
  const from = String(d.from_status ?? '')
  const to = String(d.to_status ?? '')
  return (
    <Typography variant="body2" color="text.secondary">
      {kind} {from} → {to}
    </Typography>
  )
}

const EVENT_RENDERERS: Record<string, Renderer> = {
  comment: CommentEvent,
  revision: RevisionEvent,
  status_change: StatusChangeEvent,
}

// Reactions are hidden from the UI per IS-615 spec.
const HIDDEN_EVENT_TYPES = new Set(['reaction'])

function FallbackEvent({ event }: RendererProps) {
  const summary = (() => {
    const s = event.summary_json
    if (s && typeof s === 'object') {
      const body = (s as Record<string, unknown>).body
      if (typeof body === 'string' && body) return body
    }
    return event.event_type
  })()
  return (
    <Typography variant="body2" color="text.secondary">
      {summary}
    </Typography>
  )
}

function eventLabel(e: EventItem): string {
  const author = e.author || 'system'
  const ts = new Date(e.created_at).toLocaleString()
  return `${author} — ${ts}`
}

function EventRow({
  slug,
  event,
  canReply,
  onReply,
  isReplyOpen,
  onCloseReply,
  replying,
  replyValue,
  onReplyValueChange,
  onSubmitReply,
}: {
  slug: string
  event: EventItem
  canReply: boolean
  onReply: () => void
  isReplyOpen: boolean
  onCloseReply: () => void
  replying: boolean
  replyValue: string
  onReplyValueChange: (v: string) => void
  onSubmitReply: () => void
}) {
  const Renderer = EVENT_RENDERERS[event.event_type] ?? FallbackEvent
  return (
    <Box id={`E-${event.id}`} sx={{ mb: 1.5, borderLeft: 2, borderColor: 'success.light', pl: 1.5 }}>
      <Stack direction="row" spacing={1} sx={{ mb: 0.25, alignItems: 'center' }}>
        <Typography variant="caption" color="text.secondary">
          {eventLabel(event)}
        </Typography>
        {canReply && !isReplyOpen && (
          <Tooltip title="Reply">
            <IconButton size="small" onClick={onReply}>
              <ReplyIcon fontSize="inherit" />
            </IconButton>
          </Tooltip>
        )}
      </Stack>
      <Box>
        <Renderer slug={slug} event={event} />
      </Box>
      {isReplyOpen && (
        <Box sx={{ mt: 1 }}>
          <TextField
            fullWidth
            multiline
            rows={2}
            size="small"
            placeholder="Reply…"
            value={replyValue}
            onChange={e => onReplyValueChange(e.target.value)}
          />
          <Stack direction="row" spacing={1} sx={{ mt: 0.5 }}>
            <Button
              size="small"
              variant="contained"
              disabled={!replyValue.trim() || replying}
              onClick={onSubmitReply}
            >
              Reply
            </Button>
            <Button size="small" onClick={onCloseReply}>
              Cancel
            </Button>
          </Stack>
        </Box>
      )}
    </Box>
  )
}

export function EventStream({
  slug,
  targetType,
  targetId,
}: {
  slug: string
  targetType: string
  targetId: string
}) {
  const [activeThreadId, setActiveThreadId] = useState<number | null>(null)
  const [panelOpen, setPanelOpen] = useState(false)
  const [body, setBody] = useState('')
  const [replyTo, setReplyTo] = useState<number | null>(null)
  const [replyBody, setReplyBody] = useState('')

  const { data } = useEvents(slug, targetType, targetId, activeThreadId)
  const events = data?.events ?? []

  const { data: allData } = useEvents(slug, targetType, targetId, null)
  const allEvents = allData?.events ?? []

  const hasThreads = useMemo(() => allEvents.some(e => e.thread_id != null), [allEvents])

  const visibleEvents = events.filter(e => !HIDDEN_EVENT_TYPES.has(e.event_type))

  const postComment = usePostComment()
  const replyMutation = useReplyToEvent()

  const handleAddComment = () => {
    if (!body.trim()) return
    postComment.mutate(
      { slug, target_type: targetType, target_id: targetId, body: body.trim() },
      { onSuccess: () => setBody('') },
    )
  }

  const handleSubmitReply = () => {
    if (replyTo == null || !replyBody.trim()) return
    replyMutation.mutate(
      { slug, target_type: targetType, target_id: targetId, event_id: replyTo, body: replyBody.trim() },
      {
        onSuccess: () => {
          setReplyBody('')
          setReplyTo(null)
        },
      },
    )
  }

  return (
    <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start' }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack direction="row" sx={{ mb: 1, alignItems: 'center', justifyContent: 'space-between' }}>
          <Typography variant="subtitle2" color="text.secondary">
            Activity ({visibleEvents.length})
            {activeThreadId != null && (
              <Typography component="span" variant="caption" sx={{ ml: 1 }}>
                (thread #{activeThreadId})
              </Typography>
            )}
          </Typography>
          {hasThreads && (
            <Tooltip title="Threads">
              <IconButton size="small" onClick={() => setPanelOpen(o => !o)} aria-label="Toggle threads">
                <ForumIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          )}
        </Stack>

        {activeThreadId != null && (
          <Button size="small" onClick={() => setActiveThreadId(null)} sx={{ mb: 1 }}>
            Show all
          </Button>
        )}

        {visibleEvents.map(e => {
          const canReply = activeThreadId == null && e.thread_id == null
          return (
            <EventRow
              key={e.id}
              slug={slug}
              event={e}
              canReply={canReply}
              onReply={() => {
                setReplyTo(e.id)
                setReplyBody('')
              }}
              isReplyOpen={replyTo === e.id}
              onCloseReply={() => setReplyTo(null)}
              replying={replyMutation.isPending}
              replyValue={replyBody}
              onReplyValueChange={setReplyBody}
              onSubmitReply={handleSubmitReply}
            />
          )
        })}

        <Box sx={{ mt: 2 }}>
          <TextField
            fullWidth
            multiline
            rows={2}
            size="small"
            placeholder="Add a comment…"
            value={body}
            onChange={e => setBody(e.target.value)}
          />
          <Button
            size="small"
            variant="contained"
            disabled={!body.trim() || postComment.isPending}
            onClick={handleAddComment}
            sx={{ mt: 1 }}
          >
            Comment
          </Button>
        </Box>
      </Box>

      {panelOpen && hasThreads && (
        <ThreadPanel
          slug={slug}
          targetType={targetType}
          targetId={targetId}
          activeThreadId={activeThreadId}
          onSelectThread={id => setActiveThreadId(id)}
          onClose={() => setPanelOpen(false)}
        />
      )}
    </Box>
  )
}
