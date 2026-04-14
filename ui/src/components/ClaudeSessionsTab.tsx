import { useState, useRef, useMemo, useEffect } from 'react'
import {
  Box,
  Chip,
  Collapse,
  Typography,
  List,
  ListItemButton,
  ListItemText,
  IconButton,
  CircularProgress,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon, ExpandMore as ExpandMoreIcon, ChevronRight as ChevronRightIcon } from '@mui/icons-material'
import { Link } from '@tanstack/react-router'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'
import {
  useClaudeSessions,
  useInfiniteClaudeSessionEvents,
  useClaudeSessionTokenUsage,
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

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`
  return String(n)
}

function fmtDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60000)}m${Math.round((ms % 60000) / 1000)}s`
}

type ToolDurations = Map<string, number>

function buildToolDurations(events: ClaudeEventItem[]): ToolDurations {
  const toolUseTimestamps = new Map<string, string>()
  const durations = new Map<string, number>()

  for (const event of events) {
    const ev = event.event_json
    if (event.event_type === 'assistant') {
      const msg = ev.message as Record<string, unknown> | undefined
      const content = msg?.content
      if (Array.isArray(content)) {
        for (const block of content) {
          const b = block as Record<string, unknown>
          if (b.type === 'tool_use' && typeof b.id === 'string') {
            const ts = (ev.timestamp as string) || event.created_at
            if (ts) toolUseTimestamps.set(b.id, ts)
          }
        }
      }
    } else if (event.event_type === 'user') {
      const msg = ev.message as Record<string, unknown> | undefined
      const content = msg?.content
      if (Array.isArray(content)) {
        for (const block of content) {
          const b = block as Record<string, unknown>
          if (b.type === 'tool_result' && typeof b.tool_use_id === 'string') {
            const startTs = toolUseTimestamps.get(b.tool_use_id)
            const endTs = (ev.timestamp as string) || event.created_at
            if (startTs && endTs) {
              const ms = new Date(endTs).getTime() - new Date(startTs).getTime()
              if (ms >= 0) durations.set(b.tool_use_id, ms)
            }
          }
        }
      }
    }
  }
  return durations
}

function getToolInfo(event: ClaudeEventItem): { toolName?: string; toolUseId?: string } {
  const ev = event.event_json
  if (event.event_type === 'assistant') {
    const msg = ev.message as Record<string, unknown> | undefined
    const content = msg?.content
    if (Array.isArray(content)) {
      const toolBlock = content.find((c: Record<string, unknown>) => c.type === 'tool_use') as Record<string, unknown> | undefined
      if (toolBlock) return { toolName: toolBlock.name as string, toolUseId: toolBlock.id as string }
    }
  }
  return {}
}

function getContentBlocks(event: ClaudeEventItem): Record<string, unknown>[] {
  const msg = event.event_json.message as Record<string, unknown> | undefined
  const content = msg?.content
  if (typeof content === 'string') return [{ type: 'text', text: content }]
  if (Array.isArray(content)) return content as Record<string, unknown>[]
  return []
}

function AgentBlock({ input }: { input: Record<string, unknown> }) {
  const [showPrompt, setShowPrompt] = useState(false)
  const desc = (input.description as string) || 'Agent'
  const prompt = input.prompt as string | undefined
  const subType = input.subagent_type as string | undefined

  return (
    <Box sx={{ mt: 0.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
        {subType && <Chip label={subType} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.65rem' }} />}
        <Typography variant="caption" color="text.secondary">{desc}</Typography>
      </Box>
      {prompt && (
        <>
          <Box
            onClick={(e) => { e.stopPropagation(); setShowPrompt(!showPrompt) }}
            sx={{ display: 'flex', alignItems: 'center', cursor: 'pointer', mt: 0.5, color: 'text.secondary', '&:hover': { color: 'text.primary' } }}
          >
            {showPrompt ? <ExpandMoreIcon sx={{ fontSize: 16 }} /> : <ChevronRightIcon sx={{ fontSize: 16 }} />}
            <Typography variant="caption">Prompt</Typography>
          </Box>
          <Collapse in={showPrompt}>
            <Box sx={{ mt: 0.5, p: 1, bgcolor: 'action.hover', borderRadius: 1, fontSize: '0.75rem', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 300, overflow: 'auto', fontFamily: 'monospace' }}>
              {prompt}
            </Box>
          </Collapse>
        </>
      )}
    </Box>
  )
}

function RichContent({ event }: { event: ClaudeEventItem }) {
  const blocks = getContentBlocks(event)
  if (blocks.length === 0) {
    return (
      <Box component="pre" sx={{ m: 0, p: 1, fontSize: '0.75rem', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 400, overflow: 'auto' }}>
        {JSON.stringify(event.event_json, null, 2)}
      </Box>
    )
  }

  return (
    <>
      {blocks.map((block, i) => {
        if (block.type === 'text' && typeof block.text === 'string') {
          return (
            <Box key={i} sx={{ fontSize: '0.8rem', '& pre': { m: 0 }, '& p': { m: 0, mb: 0.5 }, '& table': { fontSize: '0.75rem', borderCollapse: 'collapse', '& td, & th': { border: '1px solid', borderColor: 'divider', px: 1, py: 0.25 } } }}>
              <Markdown remarkPlugins={[remarkGfm]} components={{
                code({ className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '')
                  const code = String(children).replace(/\n$/, '')
                  if (match) {
                    return <SyntaxHighlighter style={oneDark} language={match[1]} PreTag="div" customStyle={{ fontSize: '0.75rem', margin: 0, borderRadius: 4 }}>{code}</SyntaxHighlighter>
                  }
                  return <code className={className} {...props} style={{ background: 'rgba(0,0,0,0.1)', padding: '1px 4px', borderRadius: 3, fontSize: '0.75rem' }}>{children}</code>
                }
              }}>
                {block.text}
              </Markdown>
            </Box>
          )
        }
        if (block.type === 'tool_use') {
          const name = block.name as string
          const input = block.input as Record<string, unknown>
          return (
            <Box key={i} sx={{ mt: 0.5, p: 1, bgcolor: 'background.default', borderRadius: 1, border: 1, borderColor: 'divider' }}>
              <Typography variant="caption" sx={{ fontWeight: 600 }}>{name}</Typography>
              {name === 'Bash' && typeof input.command === 'string' ? (
                <SyntaxHighlighter style={oneDark} language="bash" PreTag="div" customStyle={{ fontSize: '0.75rem', margin: '4px 0 0', borderRadius: 4 }}>{input.command}</SyntaxHighlighter>
              ) : name === 'Edit' ? (
                <Box sx={{ fontSize: '0.75rem', fontFamily: 'monospace', mt: 0.5 }}>
                  <Typography variant="caption" color="text.secondary">{input.file_path as string}</Typography>
                  {typeof input.old_string === 'string' && (
                    <Box sx={{ bgcolor: 'error.dark', color: '#fff', p: 0.5, borderRadius: 0.5, mt: 0.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 150, overflow: 'auto' }}>- {input.old_string}</Box>
                  )}
                  {typeof input.new_string === 'string' && (
                    <Box sx={{ bgcolor: 'success.dark', color: '#fff', p: 0.5, borderRadius: 0.5, mt: 0.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 150, overflow: 'auto' }}>+ {input.new_string}</Box>
                  )}
                </Box>
              ) : name === 'Read' ? (
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>{input.file_path as string}</Typography>
              ) : name === 'Write' ? (
                <Box sx={{ mt: 0.5 }}>
                  <Typography variant="caption" color="text.secondary">{input.file_path as string}</Typography>
                  {typeof input.content === 'string' && (
                    <SyntaxHighlighter style={oneDark} language="text" PreTag="div" customStyle={{ fontSize: '0.7rem', margin: '4px 0 0', borderRadius: 4, maxHeight: 200 }}>{input.content.slice(0, 2000)}</SyntaxHighlighter>
                  )}
                </Box>
              ) : name === 'Agent' ? (
                <AgentBlock input={input} />
              ) : name === 'Grep' || name === 'Glob' ? (
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>{input.pattern as string}{input.path ? ` in ${input.path}` : ''}</Typography>
              ) : (
                <Box component="pre" sx={{ m: 0, mt: 0.5, fontSize: '0.7rem', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 200, overflow: 'auto' }}>
                  {JSON.stringify(input, null, 2)}
                </Box>
              )}
            </Box>
          )
        }
        if (block.type === 'tool_result') {
          const content = block.content
          if (typeof content === 'string') {
            return (
              <Box key={i} component="pre" sx={{ m: 0, mt: 0.5, p: 1, bgcolor: 'background.default', borderRadius: 1, fontSize: '0.7rem', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 300, overflow: 'auto' }}>
                {content.slice(0, 5000)}
              </Box>
            )
          }
          if (Array.isArray(content)) {
            return (
              <Box key={i}>
                {(content as Record<string, unknown>[]).map((c, j) => {
                  if (c.type === 'text' && typeof c.text === 'string') {
                    return <Box key={j} component="pre" sx={{ m: 0, mt: 0.5, p: 1, bgcolor: 'background.default', borderRadius: 1, fontSize: '0.7rem', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 300, overflow: 'auto' }}>{(c.text as string).slice(0, 5000)}</Box>
                  }
                  return null
                })}
              </Box>
            )
          }
        }
        return null
      })}
    </>
  )
}

function EventRow({ event, toolDurations }: { event: ClaudeEventItem; toolDurations: ToolDurations }) {
  const [expanded, setExpanded] = useState(false)
  const color = EVENT_COLORS[event.event_type] ?? '#888'

  const summary = getSummary(event)
  const { toolName, toolUseId } = getToolInfo(event)
  const duration = toolUseId ? toolDurations.get(toolUseId) : undefined

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
        {toolName && (
          <Chip label={toolName} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.65rem' }} />
        )}
        {duration !== undefined && (
          <Typography variant="caption" sx={{ color: duration > 30000 ? 'error.main' : duration > 5000 ? 'warning.main' : 'text.secondary', flexShrink: 0 }}>
            {fmtDuration(duration)}
          </Typography>
        )}
        <Typography variant="body2" color="text.secondary" noWrap sx={{ flex: 1, fontSize: '0.8rem' }}>
          {summary}
        </Typography>
      </Box>
      {expanded && (
        <Box sx={{ mt: 1, maxHeight: 500, overflow: 'auto' }}>
          <RichContent event={event} />
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
}: {
  slug: string
  session: ClaudeSessionItem
  onBack: () => void
}) {
  const sentinelRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useInfiniteClaudeSessionEvents(slug, session.id)
  const { data: tokenUsage } = useClaudeSessionTokenUsage(slug, session.id)

  const allEvents = useMemo(
    () => data?.pages.flatMap((p) => p.events) ?? [],
    [data]
  )
  const total = data?.pages[0]?.total ?? 0
  const toolDurations = useMemo(() => buildToolDurations(allEvents), [allEvents])

  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasNextPage && !isFetchingNextPage) {
          fetchNextPage()
        }
      },
      { root: containerRef.current, threshold: 0.1 }
    )
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

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
            to="/project/$slug/issues/$id"
            params={{ slug, id: session.issue_id }}
            style={{ textDecoration: 'none' }}
          >
            <Chip label={session.issue_id} size="small" variant="outlined" sx={{ cursor: 'pointer' }} />
          </Link>
        )}
        <Typography variant="caption" color="text.secondary">
          {allEvents.length} / {total} events
        </Typography>
      </Box>
      {tokenUsage && (tokenUsage.input_tokens > 0 || tokenUsage.output_tokens > 0) && (
        <Box sx={{ display: 'flex', gap: 0.5, mb: 1, flexWrap: 'wrap' }}>
          <Chip label={`In: ${fmtTokens(tokenUsage.input_tokens)}`} size="small" sx={{ height: 20, fontSize: '0.7rem' }} />
          <Chip label={`Out: ${fmtTokens(tokenUsage.output_tokens)}`} size="small" sx={{ height: 20, fontSize: '0.7rem' }} />
          {tokenUsage.cache_read_input_tokens > 0 && (
            <Chip label={`Cache read: ${fmtTokens(tokenUsage.cache_read_input_tokens)}`} size="small" variant="outlined" sx={{ height: 20, fontSize: '0.7rem' }} />
          )}
          {tokenUsage.cache_creation_input_tokens > 0 && (
            <Chip label={`Cache write: ${fmtTokens(tokenUsage.cache_creation_input_tokens)}`} size="small" variant="outlined" sx={{ height: 20, fontSize: '0.7rem' }} />
          )}
        </Box>
      )}

      <Box
        ref={containerRef}
        sx={{
          flex: 1,
          overflow: 'auto',
        }}
      >
        {allEvents.map((e) => (
          <EventRow key={e.id} event={e} toolDurations={toolDurations} />
        ))}
        {hasNextPage && (
          <Box ref={sentinelRef} sx={{ display: 'flex', justifyContent: 'center', py: 2 }}>
            <CircularProgress size={20} />
          </Box>
        )}
        {isLoading && !hasNextPage && (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 2 }}>
            <CircularProgress size={24} />
          </Box>
        )}
      </Box>
    </Box>
  )
}

export function ClaudeSessionsTab({ slug }: { slug: string }) {
  const { data: sessData, isLoading } = useClaudeSessions(slug)
  const sessions = sessData?.sessions ?? []
  const [selected, setSelected] = useState<ClaudeSessionItem | null>(null)

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  if (selected) {
    return (
      <SessionDetail
        slug={slug}
        session={selected}
        onBack={() => setSelected(null)}
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
