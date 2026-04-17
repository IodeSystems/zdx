import { useState } from 'react'
import { Box, Chip, CircularProgress, IconButton, Typography } from '@mui/material'
import { Code as CodeIcon, ExpandLess, ExpandMore } from '@mui/icons-material'
import type { CodeRefItem } from '../api'
import { useCodeSnippet } from '../api'

function formatRef(r: CodeRefItem): string {
  let loc = r.file_path
  if (r.line_start > 0) {
    if (r.line_end > 0 && r.line_end !== r.line_start) {
      loc += `:${r.line_start}-${r.line_end}`
    } else {
      loc += `:${r.line_start}`
    }
  }
  return loc
}

function shortHash(h: string): string {
  return h ? h.slice(0, 8) : ''
}

function RefRow({ slug, r, canExpand }: { slug: string; r: CodeRefItem; canExpand: boolean }) {
  const [open, setOpen] = useState(false)
  const { data, isFetching, error } = useCodeSnippet(
    slug,
    r.id,
    {
      path: r.file_path,
      hash: r.git_hash || undefined,
      start: r.line_start || undefined,
      end: r.line_end || r.line_start || undefined,
      pad: 3,
    },
    open && canExpand,
  )

  const toggle = () => {
    if (canExpand) setOpen(o => !o)
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column' }}>
      <Box
        onClick={toggle}
        sx={{
          display: 'flex',
          gap: 1,
          alignItems: 'flex-start',
          flexWrap: 'wrap',
          cursor: canExpand ? 'pointer' : 'default',
          '&:hover': canExpand ? { backgroundColor: 'action.hover' } : undefined,
          borderRadius: 0.5,
          px: 0.5,
          py: 0.25,
        }}
      >
        <CodeIcon sx={{ fontSize: 16, mt: 0.4, color: 'text.secondary', flexShrink: 0 }} />
        <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center', flexWrap: 'wrap', flex: 1 }}>
          <Typography
            variant="body2"
            component="code"
            sx={{ fontFamily: 'monospace', fontSize: '0.8rem', wordBreak: 'break-all' }}
          >
            {formatRef(r)}
          </Typography>
          {r.git_hash && (
            <Chip label={`@${shortHash(r.git_hash)}`} size="small" variant="outlined" sx={{ fontFamily: 'monospace', fontSize: '0.7rem' }} />
          )}
          {r.note && (
            <Typography variant="caption" color="text.secondary">
              — {r.note}
            </Typography>
          )}
        </Box>
        {canExpand && (
          <IconButton
            size="small"
            aria-label={open ? 'collapse' : 'expand'}
            onClick={e => { e.stopPropagation(); toggle() }}
            sx={{ p: 0.25 }}
          >
            {open ? <ExpandLess fontSize="small" /> : <ExpandMore fontSize="small" />}
          </IconButton>
        )}
      </Box>
      {open && (
        <Box sx={{ ml: 3, mt: 0.5, mb: 1 }}>
          {isFetching && <CircularProgress size={14} />}
          {!isFetching && error && (
            <Typography variant="caption" color="error">
              {(error as Error).message || 'failed to load snippet'}
            </Typography>
          )}
          {!isFetching && !error && data && (
            <Snippet
              content={data.content}
              startLine={data.start_line}
              highlightStart={r.line_start}
              highlightEnd={r.line_end || r.line_start}
              truncated={data.truncated}
            />
          )}
        </Box>
      )}
    </Box>
  )
}

function Snippet({
  content,
  startLine,
  highlightStart,
  highlightEnd,
  truncated,
}: {
  content: string
  startLine: number
  highlightStart: number
  highlightEnd: number
  truncated: boolean
}) {
  const lines = content.split('\n')
  const gutterWidth = String(startLine + lines.length - 1).length
  return (
    <Box
      component="pre"
      sx={{
        m: 0,
        p: 1,
        bgcolor: 'action.hover',
        fontFamily: 'monospace',
        fontSize: '0.75rem',
        overflowX: 'auto',
        borderRadius: 1,
      }}
    >
      {lines.map((line, i) => {
        const ln = startLine + i
        const highlighted = highlightStart > 0 && ln >= highlightStart && ln <= highlightEnd
        return (
          <Box
            key={i}
            sx={{
              display: 'flex',
              bgcolor: highlighted ? 'action.selected' : 'transparent',
              whiteSpace: 'pre',
            }}
          >
            <Box
              component="span"
              sx={{
                display: 'inline-block',
                width: `${gutterWidth}ch`,
                pr: 1,
                textAlign: 'right',
                color: 'text.secondary',
                userSelect: 'none',
                flexShrink: 0,
              }}
            >
              {ln}
            </Box>
            <Box component="span">{line || ' '}</Box>
          </Box>
        )
      })}
      {truncated && (
        <Box sx={{ mt: 0.5, color: 'text.secondary', fontStyle: 'italic' }}>… truncated</Box>
      )}
    </Box>
  )
}

export function CodeRefs({ refs, slug }: { refs: CodeRefItem[]; slug?: string }) {
  if (!refs || refs.length === 0) return null
  const canExpand = !!slug

  return (
    <Box sx={{ mb: 3 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        Code References ({refs.length})
      </Typography>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25 }}>
        {refs.map(r => (
          <RefRow key={r.id} slug={slug ?? ''} r={r} canExpand={canExpand} />
        ))}
      </Box>
    </Box>
  )
}
