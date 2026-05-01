import { useState, useRef, useMemo, useEffect, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  Box,
  Button,
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
  useExtractPatternFromSession,
  useActiveClaims,
  type ClaudeSessionItem,
  type ClaudeEventItem,
  type AgentTokenUsage,
} from '../api'
import { useChannel } from '../hooks/useChannel'
import { fmtRelative } from '../utils/date'
import { MarkdownContent } from './MarkdownContent'

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

type TurnMetrics = { turnMs: number; tps: number; tgs: number }
type TurnMetricsMap = Map<number, TurnMetrics>

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

function buildTurnMetrics(events: ClaudeEventItem[]): TurnMetricsMap {
  const metrics = new Map<number, TurnMetrics>()
  let lastUserTs: string | null = null

  for (const event of events) {
    const ts = (event.event_json.timestamp as string) || event.created_at
    if (event.event_type === 'user') {
      lastUserTs = ts
    } else if (event.event_type === 'assistant' && lastUserTs) {
      const endMs = new Date(ts).getTime()
      const startMs = new Date(lastUserTs).getTime()
      const turnMs = endMs - startMs
      if (turnMs > 0) {
        const msg = event.event_json.message as Record<string, unknown> | undefined
        const usage = msg?.usage as Record<string, number> | undefined
        const inputTok = usage?.input_tokens ?? 0
        const outputTok = usage?.output_tokens ?? 0
        const secs = turnMs / 1000
        metrics.set(event.id, { turnMs, tps: inputTok / secs, tgs: outputTok / secs })
      }
      lastUserTs = null
    }
  }
  return metrics
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

type DisplayItem =
  | { kind: 'event'; event: ClaudeEventItem }
  | { kind: 'agent-group'; parentEvent: ClaudeEventItem; agentId: string; agentType: string; agentDescription: string; events: ClaudeEventItem[] }

function buildDisplayItems(events: ClaudeEventItem[]): DisplayItem[] {
  const subagentEvents = new Map<string, ClaudeEventItem[]>()
  for (const e of events) {
    if (e.agent_id) {
      const arr = subagentEvents.get(e.agent_id) ?? []
      arr.push(e)
      subagentEvents.set(e.agent_id, arr)
    }
  }
  if (subagentEvents.size === 0) {
    return events.map((e) => ({ kind: 'event', event: e }))
  }

  const agentToolUses: { event: ClaudeEventItem; agentId: string }[] = []
  const orderedAgentIds: string[] = []
  for (const e of events) {
    if (e.agent_id && !orderedAgentIds.includes(e.agent_id)) {
      orderedAgentIds.push(e.agent_id)
    }
  }

  let agentIdx = 0
  for (const e of events) {
    if (!e.agent_id) {
      const { toolName } = getToolInfo(e)
      if (toolName === 'Agent' && agentIdx < orderedAgentIds.length) {
        agentToolUses.push({ event: e, agentId: orderedAgentIds[agentIdx] })
        agentIdx++
      }
    }
  }

  const assignedAgentIds = new Set(agentToolUses.map((a) => a.agentId))
  const items: DisplayItem[] = []
  for (const e of events) {
    if (e.agent_id) continue
    const match = agentToolUses.find((a) => a.event === e)
    if (match) {
      const subEvents = subagentEvents.get(match.agentId) ?? []
      const sample = subEvents[0]
      items.push({
        kind: 'agent-group',
        parentEvent: e,
        agentId: match.agentId,
        agentType: sample?.agent_type ?? '',
        agentDescription: sample?.agent_description ?? '',
        events: subEvents,
      })
    } else {
      items.push({ kind: 'event', event: e })
    }
  }

  for (const [agentId, evts] of subagentEvents) {
    if (!assignedAgentIds.has(agentId)) {
      const sample = evts[0]
      items.push({
        kind: 'agent-group',
        parentEvent: evts[0],
        agentId,
        agentType: sample?.agent_type ?? '',
        agentDescription: sample?.agent_description ?? '',
        events: evts,
      })
    }
  }

  return items
}

function AgentGroupRow({
  item,
  toolDurations,
  toolResultMap,
  turnMetrics,
}: {
  item: Extract<DisplayItem, { kind: 'agent-group' }>
  toolDurations: ToolDurations
  toolResultMap: ToolResultMap
  turnMetrics?: TurnMetricsMap
}) {
  const [expanded, setExpanded] = useState(false)
  const typeColor = AGENT_TYPE_COLORS[item.agentType] ?? '#888'
  const label = item.agentDescription || item.agentType || item.agentId.slice(0, 8)

  return (
    <Box sx={{ mb: 0.5 }}>
      <Box
        sx={{
          py: 0.5,
          px: 1,
          borderLeft: 3,
          borderColor: typeColor,
          cursor: 'pointer',
          '&:hover': { bgcolor: 'action.hover' },
          fontFamily: 'monospace',
          fontSize: '0.8rem',
          display: 'flex',
          gap: 1,
          alignItems: 'center',
        }}
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? <ExpandMoreIcon sx={{ fontSize: 16 }} /> : <ChevronRightIcon sx={{ fontSize: 16 }} />}
        <Chip label="Agent" size="small" sx={{ bgcolor: '#4caf50', color: '#fff', height: 20, fontSize: '0.7rem' }} />
        <Chip label={item.agentType || 'subagent'} size="small" sx={{ bgcolor: typeColor, color: '#fff', height: 18, fontSize: '0.65rem' }} />
        <Typography variant="body2" color="text.secondary" noWrap sx={{ flex: 1, fontSize: '0.8rem' }}>
          {label}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {item.events.length} events
        </Typography>
      </Box>
      <Collapse in={expanded}>
        <Box sx={{ pl: 2, borderLeft: 2, borderColor: typeColor, ml: 1 }}>
          {item.events.map((e) => (
            <EventRow key={e.id} event={e} toolDurations={toolDurations} toolResultMap={toolResultMap} turnMetrics={turnMetrics} />
          ))}
        </Box>
      </Collapse>
    </Box>
  )
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

function TodoWriteBlock({ input }: { input: Record<string, unknown> }) {
  const todos = (input.todos as Record<string, unknown>[] | undefined) ?? []
  if (todos.length === 0) {
    return <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>(empty list)</Typography>
  }
  const statusIcon = (s: string) => s === 'completed' ? '✓' : s === 'in_progress' ? '⚡' : '☐'
  const statusColor = (s: string) => s === 'completed' ? 'success.main' : s === 'in_progress' ? 'info.main' : 'text.secondary'
  const priorityColor = (p: string) => p === 'high' ? 'error.main' : p === 'medium' ? 'warning.main' : 'text.secondary'
  return (
    <Box sx={{ mt: 0.5, display: 'flex', flexDirection: 'column', gap: 0.25 }}>
      {todos.map((t, j) => {
        const status = (t.status as string) || 'pending'
        const priority = t.priority as string | undefined
        const content = (t.content as string) || ''
        const activeForm = t.activeForm as string | undefined
        const text = status === 'in_progress' && activeForm ? activeForm : content
        return (
          <Box key={j} sx={{ display: 'flex', alignItems: 'flex-start', gap: 0.75, fontSize: '0.75rem' }}>
            <Box sx={{ color: statusColor(status), fontFamily: 'monospace', minWidth: 14, textAlign: 'center', fontWeight: 600 }}>{statusIcon(status)}</Box>
            {priority && <Chip label={priority} size="small" sx={{ height: 16, fontSize: '0.6rem', bgcolor: priorityColor(priority), color: '#fff', '& .MuiChip-label': { px: 0.75 } }} />}
            <Typography variant="caption" sx={{ fontSize: '0.75rem', textDecoration: status === 'completed' ? 'line-through' : 'none', color: status === 'completed' ? 'text.secondary' : 'text.primary' }}>{text}</Typography>
          </Box>
        )
      })}
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
              ) : name === 'TodoWrite' ? (
                <TodoWriteBlock input={input} />
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

function EventRow({ event, toolDurations, toolResultMap, turnMetrics }: { event: ClaudeEventItem; toolDurations: ToolDurations; toolResultMap: ToolResultMap; turnMetrics?: TurnMetricsMap }) {
  const [expanded, setExpanded] = useState(false)
  const color = EVENT_COLORS[event.event_type] ?? '#888'

  const summary = getSummary(event)
  const { toolName, toolUseId } = getToolInfo(event)
  const duration = toolUseId ? toolDurations.get(toolUseId) : undefined
  const turn = turnMetrics?.get(event.id)

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
        {turn !== undefined && (
          <Typography variant="caption" color="text.secondary" sx={{ flexShrink: 0 }}>
            {fmtDuration(turn.turnMs)} · {turn.tps.toFixed(0)}tp/s · {turn.tgs.toFixed(0)}tg/s
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
  const queryClient = useQueryClient()
  const sentinelRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const [liveEvents, setLiveEvents] = useState<ClaudeEventItem[]>([])
  const [extractedPatternName, setExtractedPatternName] = useState<string | null>(null)
  const extractPattern = useExtractPatternFromSession()

  const { data: session } = useClaudeSession(slug, sessionId)
  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useInfiniteClaudeSessionEvents(slug, sessionId)
  const { data: tokenUsage } = useClaudeSessionTokenUsage(slug, sessionId)
  const { data: agentUsage } = useClaudeSessionTokenUsageByAgent(slug, sessionId)

  const wsChannel = session?.session_id ? `project:${slug}:claude:${session.session_id}` : null

  const onWsMessage = useCallback((msg: { type: string; payload: unknown }) => {
    if (msg.type === 'claude.session-updated') {
      const p = msg.payload as { header: string; summary: string; status: string }
      queryClient.setQueryData<ClaudeSessionItem>(['claude-session', slug, sessionId], (old) =>
        old ? { ...old, header: p.header, summary: p.summary, status: p.status } : old
      )
      return
    }
    if (msg.type !== 'claude.event') return
    const p = msg.payload as { seq: number; event_type: string; event_json: Record<string, unknown>; session_pk: number }
    const newEvent: ClaudeEventItem = {
      id: -(p.seq + 1),
      seq: p.seq,
      event_type: p.event_type,
      event_json: p.event_json,
      created_at: new Date().toISOString(),
      agent_id: (p as Record<string, unknown>).agent_id as string ?? '',
      agent_type: (p as Record<string, unknown>).agent_type as string ?? '',
      agent_description: (p as Record<string, unknown>).agent_description as string ?? '',
      is_sidechain: !!(p as Record<string, unknown>).is_sidechain,
    }
    setLiveEvents((prev) => {
      if (prev.some((e) => e.seq === p.seq)) return prev
      return [...prev, newEvent]
    })
  }, [queryClient, slug, sessionId])

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
  const turnMetrics = useMemo(() => buildTurnMetrics(allEvents), [allEvents])
  const { resultMap: toolResultMap, toolResultEventIds } = useMemo(() => buildToolResultMap(allEvents), [allEvents])
  const visibleEvents = useMemo(() => allEvents.filter((e) => !toolResultEventIds.has(e.id)), [allEvents, toolResultEventIds])
  const displayItems = useMemo(() => buildDisplayItems(visibleEvents), [visibleEvents])

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
      {(session?.todo_title || session?.todo_description || session?.todo_text) && (
        <Box sx={{ mb: 1, p: 1.5, bgcolor: 'action.hover', borderRadius: 1, border: 1, borderColor: 'divider' }}>
          <Box sx={{ display: 'flex', gap: 0.75, alignItems: 'center', mb: session.todo_description ? 0.5 : 0 }}>
            <Typography variant="caption" sx={{ fontWeight: 600, color: 'primary.main' }}>
              Todo:
            </Typography>
            <Typography variant="subtitle2" sx={{ flex: 1 }}>
              {session.todo_title || session.todo_text}
            </Typography>
            {session.todo_target_type && session.todo_target_id && (
              <Chip
                label={`${session.todo_target_type}:${session.todo_target_id}`}
                size="small"
                variant="outlined"
                sx={{ height: 20, fontSize: '0.7rem' }}
              />
            )}
          </Box>
          {session.todo_description && (
            <MarkdownContent slug={slug} variant="body2">{session.todo_description}</MarkdownContent>
          )}
        </Box>
      )}
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
      {session?.status === 'churn' && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
          {extractedPatternName ? (
            <Chip label={`Pattern: ${extractedPatternName}`} size="small" color="success" sx={{ fontSize: '0.7rem' }} />
          ) : (
            <Button
              size="small"
              variant="outlined"
              color="warning"
              onClick={async () => {
                const pattern = await extractPattern.mutateAsync({ slug, sessionId })
                setExtractedPatternName(pattern.name)
              }}
              disabled={extractPattern.isPending}
            >
              {extractPattern.isPending ? 'Extracting...' : 'Extract Pattern'}
            </Button>
          )}
          {extractPattern.isError && (
            <Typography variant="caption" color="error">
              {extractPattern.error.message}
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
        {displayItems.map((item, idx) =>
          item.kind === 'event' ? (
            <EventRow key={item.event.id} event={item.event} toolDurations={toolDurations} toolResultMap={toolResultMap} turnMetrics={turnMetrics} />
          ) : (
            <AgentGroupRow key={`agent-${item.agentId}-${idx}`} item={item} toolDurations={toolDurations} toolResultMap={toolResultMap} turnMetrics={turnMetrics} />
          )
        )}
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

function ActiveReservations({ slug }: { slug: string }) {
  const { data, isLoading } = useActiveClaims(slug)
  const todos = data?.todos ?? []
  const tasks = data?.tasks ?? []
  if (isLoading || (todos.length === 0 && tasks.length === 0)) return null

  return (
    <Box sx={{ mb: 2 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Active Reservations
      </Typography>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
        {todos.map((t) => (
          <Box key={`todo-${t.id}`} sx={{ display: 'flex', gap: 1, alignItems: 'center', px: 1, py: 0.5, border: 1, borderColor: 'divider', borderRadius: 1 }}>
            <Chip label="todo" size="small" sx={{ height: 18, fontSize: '0.65rem', bgcolor: 'primary.main', color: '#fff' }} />
            {t.issue_ref && <Chip label={t.issue_ref} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.65rem' }} />}
            <Typography variant="caption" noWrap sx={{ flex: 1 }}>{t.text}</Typography>
            <Typography variant="caption" color="text.secondary">claimed-by: {t.claimed_by}</Typography>
            {t.lease_expires_at && (
              <Typography variant="caption" color="warning.main" title={`expires ${t.lease_expires_at}`}>
                expires {fmtRelative(t.lease_expires_at)}
              </Typography>
            )}
          </Box>
        ))}
        {tasks.map((t) => (
          <Box key={`task-${t.id}`} sx={{ display: 'flex', gap: 1, alignItems: 'center', px: 1, py: 0.5, border: 1, borderColor: 'divider', borderRadius: 1 }}>
            <Chip label="task" size="small" sx={{ height: 18, fontSize: '0.65rem', bgcolor: 'secondary.main', color: '#fff' }} />
            {t.issue && <Chip label={t.issue} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.65rem' }} />}
            <Typography variant="caption" noWrap sx={{ flex: 1 }}>{t.title || t.text}</Typography>
            <Typography variant="caption" color="text.secondary">claimed-by: {t.claimed_by}</Typography>
            <Typography variant="caption" color="warning.main" title={`expires ${t.lease_expires_at}`}>
              expires {fmtRelative(t.lease_expires_at)}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  )
}

export function ClaudeSessionsTab({ slug }: { slug: string }) {
  const { data: sessData, isLoading } = useClaudeSessions(slug)
  const fetchedSessions = sessData?.sessions ?? []
  const [liveSessions, setLiveSessions] = useState<ClaudeSessionItem[]>([])
  const [closedUpdates, setClosedUpdates] = useState<Map<number, { event_count: number; updated_at: string }>>(new Map())

  const onWsMessage = useCallback((msg: { type: string; payload: unknown }) => {
    if (msg.type === 'claude.session-created') {
      const p = msg.payload as SessionCreatedPayload
      setLiveSessions((prev) => {
        if (prev.some((s) => s.session_id === p.session_id)) return prev
        const nowISO = new Date().toISOString()
        return [{
          id: p.session_pk,
          session_id: p.session_id,
          title: p.title,
          header: '',
          summary: '',
          status: '',
          lifecycle: 'starting',
          issue_id: '',
          event_count: 0,
          created_at: nowISO,
          updated_at: nowISO,
        }, ...prev]
      })
    } else if (msg.type === 'claude.session-closed') {
      const p = msg.payload as SessionClosedPayload
      setClosedUpdates((prev) => {
        const next = new Map(prev)
        next.set(p.session_pk, { event_count: p.event_count, updated_at: new Date().toISOString() })
        return next
      })
    }
  }, [])

  useChannel(`project:${slug}:claude`, onWsMessage)

  const sessions = useMemo(() => {
    const fetchedIds = new Set(fetchedSessions.map((s) => s.session_id))
    const newLive = liveSessions.filter((s) => !fetchedIds.has(s.session_id))
    const merged = [...newLive, ...fetchedSessions]
    const updated = merged.map((s) => {
      const update = closedUpdates.get(s.id)
      if (update) return { ...s, event_count: update.event_count, updated_at: update.updated_at, lifecycle: 'done' }
      return s
    })
    updated.sort((a, b) => {
      const ta = new Date(a.updated_at || a.created_at).getTime()
      const tb = new Date(b.updated_at || b.created_at).getTime()
      return tb - ta
    })
    return updated
  }, [fetchedSessions, liveSessions, closedUpdates])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  return (
    <Box>
      <ActiveReservations slug={slug} />
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
              to="/project/$slug/agents/$sessionId"
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
                      {s.lifecycle && (
                        <Chip
                          label={s.lifecycle}
                          size="small"
                          variant={s.lifecycle === 'done' ? 'outlined' : 'filled'}
                          sx={{
                            height: 18,
                            fontSize: '0.65rem',
                            ...(s.lifecycle === 'starting' && { bgcolor: 'grey.500', color: '#fff' }),
                            ...(s.lifecycle === 'active' && {
                              bgcolor: '#1976d2',
                              color: '#fff',
                              animation: 'pulse 1.5s ease-in-out infinite',
                              '@keyframes pulse': { '0%, 100%': { opacity: 1 }, '50%': { opacity: 0.6 } },
                            }),
                            ...(s.lifecycle === 'done' && { color: 'text.secondary', borderColor: 'divider' }),
                          }}
                        />
                      )}
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
                      {(s.todo_title || s.todo_text) && (
                        <Box component="span" sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
                          <Typography component="span" variant="caption" sx={{ fontWeight: 600, color: 'primary.main' }}>
                            Todo:
                          </Typography>
                          <Typography component="span" variant="caption" color="text.primary" noWrap sx={{ flex: 1 }}>
                            {(s.todo_title || s.todo_text || '').slice(0, 140)}
                          </Typography>
                          {s.todo_target_type && s.todo_target_id && (
                            <Chip
                              label={`${s.todo_target_type}:${s.todo_target_id}`}
                              size="small"
                              variant="outlined"
                              sx={{ height: 16, fontSize: '0.65rem' }}
                            />
                          )}
                        </Box>
                      )}
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
                        <Typography
                          component="span"
                          variant="caption"
                          color="text.secondary"
                          title={`Created: ${new Date(s.created_at).toLocaleString()} · Updated: ${new Date(s.updated_at || s.created_at).toLocaleString()}`}
                        >
                          {fmtRelative(s.updated_at || s.created_at)}
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
