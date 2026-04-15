import { useState, useRef, useMemo, useEffect, useCallback } from 'react'
import {
  Box,
  Chip,
  Collapse,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
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
  useClaudeSession,
  useInfiniteClaudeSessionEvents,
  useClaudeSessionTokenUsage,
  useClaudeSessionTokenUsageByAgent,
  type ClaudeSessionItem,
  type ClaudeEventItem,
  type AgentTokenUsage,
} from '../api'
import { useChannel } from '../hooks/useChannel'
import { fmtDate } from '../utils/date'

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

type ToolResultContent = string | Record<string, unknown>[]
type ToolResultMap = Map<string, ToolResultContent>
type ToolDurations = Map<string, number>

function buildToolDurations(events: ClaudeEventItem[]): ToolDurations {
  const toolUseTs = new Map<string, string>()
  const toolResultTs = new Map<string, string>()
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
            if (ts) toolUseTs.set(b.id, ts)
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
            const ts = (ev.timestamp as string) || event.created_at
            if (ts) toolResultTs.set(b.tool_use_id, ts)
          }
        }
      }
    }
  }

  for (const [id, startTs] of toolUseTs) {
    const endTs = toolResultTs.get(id)
    if (endTs) {
      const ms = new Date(endTs).getTime() - new Date(startTs).getTime()
      if (ms >= 0) durations.set(id, ms)
    }
  }
  return durations
}

function buildToolResultMap(events: ClaudeEventItem[]): { resultMap: ToolResultMap; toolResultEventIds: Set<number> } {
  const resultMap = new Map<string, ToolResultContent>()
  const toolResultEventIds = new Set<number>()

  for (const event of events) {
    if (event.event_type !== 'user') continue
    const blocks = getContentBlocks(event)
    const allToolResults = blocks.every((b) => b.type === 'tool_result')
    if (!allToolResults || blocks.length === 0) continue

    toolResultEventIds.add(event.id)
    for (const block of blocks) {
      if (block.type === 'tool_result' && typeof block.tool_use_id === 'string') {
        const content = block.content
        if (typeof content === 'string') {
          resultMap.set(block.tool_use_id as string, content)
        } else if (Array.isArray(content)) {
          resultMap.set(block.tool_use_id as string, content as Record<string, unknown>[])
        }
      }
    }
  }
  return { resultMap, toolResultEventIds }
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

function ToolResultInline({ content }: { content: ToolResultContent }) {
  if (typeof content === 'string') {
    return (
      <Box component="pre" sx={{ m: 0, mt: 0.5, p: 1, bgcolor: 'background.default', borderRadius: 1, fontSize: '0.7rem', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 300, overflow: 'auto' }}>
        {content.slice(0, 5000)}
      </Box>
    )
  }
  if (Array.isArray(content)) {
    return (
      <>
        {content.map((c, j) => {
          if (c.type === 'text' && typeof c.text === 'string') {
            return <Box key={j} component="pre" sx={{ m: 0, mt: 0.5, p: 1, bgcolor: 'background.default', borderRadius: 1, fontSize: '0.7rem', whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 300, overflow: 'auto' }}>{(c.text as string).slice(0, 5000)}</Box>
          }
          return null
        })}
      </>
    )
  }
  return null
}

function RichContent({ event, toolResultMap }: { event: ClaudeEventItem; toolResultMap?: ToolResultMap }) {
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
          const toolUseId = block.id as string | undefined
          const inlineResult = toolUseId && toolResultMap ? toolResultMap.get(toolUseId) : undefined
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
              {inlineResult !== undefined && <ToolResultInline content={inlineResult} />}
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

function EventRow({ event, toolDurations, toolResultMap }: { event: ClaudeEventItem; toolDurations: ToolDurations; toolResultMap: ToolResultMap }) {
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
          <RichContent event={event} toolResultMap={toolResultMap} />
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

const AGENT_TYPE_COLORS: Record<string, string> = {
  Explore: '#2196f3',
  'code-reviewer': '#9c27b0',
  'general-purpose': '#4caf50',
  Plan: '#ff9800',
}

function AgentBreakdown({ agents }: { agents: AgentTokenUsage[] }) {
  if (agents.length <= 1) return null

  return (
    <Box sx={{ mb: 1 }}>
      <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5, display: 'block' }}>
        Token usage by agent
      </Typography>
      <TableContainer>
        <Table size="small" sx={{ '& td, & th': { px: 1, py: 0.25, fontSize: '0.75rem', border: 'none' } }}>
          <TableHead>
            <TableRow sx={{ '& th': { color: 'text.secondary', fontWeight: 600 } }}>
              <TableCell>Agent</TableCell>
              <TableCell align="right">Input</TableCell>
              <TableCell align="right">Output</TableCell>
              <TableCell align="right">Cache Read</TableCell>
              <TableCell align="right">Cache Write</TableCell>
              <TableCell align="right">Events</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {agents.map((a, i) => {
              const label = a.agent_id ? (a.agent_description || a.agent_type || a.agent_id) : 'Main thread'
              const typeColor = AGENT_TYPE_COLORS[a.agent_type] ?? '#888'
              return (
                <TableRow key={i} sx={{ '&:hover': { bgcolor: 'action.hover' } }}>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                      {a.agent_type && (
                        <Chip
                          label={a.agent_type}
                          size="small"
                          sx={{ height: 18, fontSize: '0.65rem', bgcolor: typeColor, color: '#fff' }}
                        />
                      )}
                      <Typography variant="caption" noWrap sx={{ maxWidth: 200 }}>{label}</Typography>
                    </Box>
                  </TableCell>
                  <TableCell align="right">{fmtTokens(a.input_tokens)}</TableCell>
                  <TableCell align="right">{fmtTokens(a.output_tokens)}</TableCell>
                  <TableCell align="right">{a.cache_read_input_tokens > 0 ? fmtTokens(a.cache_read_input_tokens) : '-'}</TableCell>
                  <TableCell align="right">{a.cache_creation_input_tokens > 0 ? fmtTokens(a.cache_creation_input_tokens) : '-'}</TableCell>
                  <TableCell align="right">{a.event_count}</TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  )
}

export function SessionDetail({
  slug,
  sessionId,
  onBack,
}: {
  slug: string
  sessionId: number
  onBack?: () => void
}) {
  const sentinelRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const [liveEvents, setLiveEvents] = useState<ClaudeEventItem[]>([])

  const { data: session } = useClaudeSession(slug, sessionId)
  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useInfiniteClaudeSessionEvents(slug, sessionId)
  const { data: tokenUsage } = useClaudeSessionTokenUsage(slug, sessionId)
  const { data: agentUsage } = useClaudeSessionTokenUsageByAgent(slug, sessionId)

  const wsChannel = session?.session_id ? `project:${slug}:claude:${session.session_id}` : null

  const onWsMessage = useCallback((msg: { type: string; payload: unknown }) => {
    if (msg.type !== 'claude.event') return
    const p = msg.payload as { seq: number; event_type: string; event_json: Record<string, unknown>; session_pk: number }
    const newEvent: ClaudeEventItem = {
      id: -(p.seq + 1),
      seq: p.seq,
      event_type: p.event_type,
      event_json: p.event_json,
      created_at: new Date().toISOString(),
    }
    setLiveEvents((prev) => {
      if (prev.some((e) => e.seq === p.seq)) return prev
      return [...prev, newEvent]
    })
  }, [])

  useChannel(wsChannel, onWsMessage)

  useEffect(() => {
    if (liveEvents.length > 0) {
      const el = containerRef.current
      if (el) {
        const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 200
        if (isNearBottom) {
          requestAnimationFrame(() => el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' }))
        }
      }
    }
  }, [liveEvents.length])

  const allEvents = useMemo(() => {
    const fetched = data?.pages.flatMap((p) => p.events) ?? []
    const fetchedSeqs = new Set(fetched.map((e) => e.seq))
    const newLive = liveEvents.filter((e) => !fetchedSeqs.has(e.seq))
    return [...fetched, ...newLive]
  }, [data, liveEvents])
  const total = (data?.pages[0]?.total ?? 0) + liveEvents.filter((e) => e.id < 0).length
  const isLive = liveEvents.length > 0
  const toolDurations = useMemo(() => buildToolDurations(allEvents), [allEvents])
  const { resultMap: toolResultMap, toolResultEventIds } = useMemo(() => buildToolResultMap(allEvents), [allEvents])
  const visibleEvents = useMemo(() => allEvents.filter((e) => !toolResultEventIds.has(e.id)), [allEvents, toolResultEventIds])

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
        {onBack && (
          <IconButton size="small" onClick={onBack}>
            <ArrowBackIcon fontSize="small" />
          </IconButton>
        )}
        <Typography variant="subtitle2">
          {session?.title || session?.session_id?.slice(0, 8) || `Session #${sessionId}`}
        </Typography>
        {session?.status && (
          <Chip
            label={session.status}
            size="small"
            sx={{
              height: 20,
              fontSize: '0.7rem',
              bgcolor: session.status === 'ok' ? 'success.main' : session.status === 'churn' ? 'warning.main' : session.status === 'errored' ? 'error.main' : 'grey.500',
              color: '#fff',
            }}
          />
        )}
        {session?.issue_id && (
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
        {isLive && (
          <Chip
            label="LIVE"
            size="small"
            sx={{
              height: 20,
              fontSize: '0.65rem',
              fontWeight: 700,
              bgcolor: '#f44336',
              color: '#fff',
              animation: 'pulse 1.5s ease-in-out infinite',
              '@keyframes pulse': {
                '0%, 100%': { opacity: 1 },
                '50%': { opacity: 0.6 },
              },
            }}
          />
        )}
      </Box>
      {(session?.header || session?.summary) && (
        <Box sx={{ mb: 1, p: 1.5, bgcolor: 'action.hover', borderRadius: 1, border: 1, borderColor: 'divider' }}>
          {session.header && (
            <Typography variant="subtitle2" sx={{ mb: session.summary ? 0.5 : 0 }}>
              {session.header}
            </Typography>
          )}
          {session.summary && (
            <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-wrap', fontSize: '0.85rem' }}>
              {session.summary}
            </Typography>
          )}
        </Box>
      )}
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
      {agentUsage?.agents && <AgentBreakdown agents={agentUsage.agents} />}

      <Box
        ref={containerRef}
        sx={{
          flex: 1,
          overflow: 'auto',
        }}
      >
        {visibleEvents.map((e) => (
          <EventRow key={e.id} event={e} toolDurations={toolDurations} toolResultMap={toolResultMap} />
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

interface SessionCreatedPayload {
  session_id: string
  session_pk: number
  title: string
  alias: string
}

interface SessionClosedPayload {
  session_id: string
  session_pk: number
  event_count: number
}

export function ClaudeSessionsTab({ slug }: { slug: string }) {
  const { data: sessData, isLoading } = useClaudeSessions(slug)
  const fetchedSessions = sessData?.sessions ?? []
  const [liveSessions, setLiveSessions] = useState<ClaudeSessionItem[]>([])
  const [closedUpdates, setClosedUpdates] = useState<Map<number, { event_count: number }>>(new Map())

  const onWsMessage = useCallback((msg: { type: string; payload: unknown }) => {
    if (msg.type === 'claude.session-created') {
      const p = msg.payload as SessionCreatedPayload
      setLiveSessions((prev) => {
        if (prev.some((s) => s.session_id === p.session_id)) return prev
        return [{
          id: p.session_pk,
          session_id: p.session_id,
          title: p.title,
          header: '',
          summary: '',
          status: '',
          issue_id: '',
          event_count: 0,
          created_at: new Date().toISOString(),
        }, ...prev]
      })
    } else if (msg.type === 'claude.session-closed') {
      const p = msg.payload as SessionClosedPayload
      setClosedUpdates((prev) => {
        const next = new Map(prev)
        next.set(p.session_pk, { event_count: p.event_count })
        return next
      })
    }
  }, [])

  useChannel(`project:${slug}:claude`, onWsMessage)

  const sessions = useMemo(() => {
    const fetchedIds = new Set(fetchedSessions.map((s) => s.session_id))
    const newLive = liveSessions.filter((s) => !fetchedIds.has(s.session_id))
    const merged = [...newLive, ...fetchedSessions]
    return merged.map((s) => {
      const update = closedUpdates.get(s.id)
      if (update) return { ...s, event_count: update.event_count }
      return s
    })
  }, [fetchedSessions, liveSessions, closedUpdates])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

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
            <Link
              key={s.id}
              to="/project/$slug/claude/$sessionId"
              params={{ slug, sessionId: String(s.id) }}
              style={{ textDecoration: 'none', color: 'inherit' }}
            >
              <ListItemButton sx={{ borderBottom: 1, borderColor: 'divider' }}>
                <ListItemText
                  primary={
                    <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                      <Typography variant="body2" noWrap sx={{ flex: 1 }}>
                        {s.title || s.session_id.slice(0, 12)}
                      </Typography>
                      {s.status && (
                        <Chip
                          label={s.status}
                          size="small"
                          sx={{
                            height: 18,
                            fontSize: '0.65rem',
                            bgcolor: s.status === 'ok' ? 'success.main' : s.status === 'churn' ? 'warning.main' : s.status === 'errored' ? 'error.main' : 'grey.500',
                            color: '#fff',
                          }}
                        />
                      )}
                    </Box>
                  }
                  secondary={
                    <Box component="span" sx={{ display: 'flex', flexDirection: 'column', gap: 0.25 }}>
                      {s.header && (
                        <Typography component="span" variant="caption" color="text.primary" noWrap>
                          {s.header}
                        </Typography>
                      )}
                      {s.summary && (
                        <Typography component="span" variant="caption" color="text.secondary" noWrap>
                          {s.summary.slice(0, 120)}
                        </Typography>
                      )}
                      <Box component="span" sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                        <Typography component="span" variant="caption" color="text.secondary">
                          {fmtDate(s.created_at)}
                        </Typography>
                        {s.issue_id && <Chip label={s.issue_id} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.7rem' }} />}
                        <Typography component="span" variant="caption" color="text.disabled">
                          {s.event_count} events
                        </Typography>
                      </Box>
                    </Box>
                  }
                />
              </ListItemButton>
            </Link>
          ))}
        </List>
      )}
    </Box>
  )
}
