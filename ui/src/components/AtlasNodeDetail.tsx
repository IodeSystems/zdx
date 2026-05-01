import { useEffect } from 'react'
import { Link, useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Tooltip,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useAtlasNode, useAtlasCoderefs } from '../api'
import { MarkdownContent } from './MarkdownContent'
import type { AtlasEdgeFromItem, AtlasCoderefItem } from '../api'

// status/verified_at/verified_by are in the API but pending api.gen.ts regen
type AtlasChunk = {
  id: number
  node_id: number
  coderef_id?: number
  title?: string
  body: string
  sha_at_write?: string
  status: string
  verifier_kind: string
  verified_at?: string
  verified_by?: string
  created_at: string
  updated_at: string
}

function staleFractionColor(fraction: number): 'success' | 'warning' | 'error' {
  if (fraction === 0) return 'success'
  if (fraction <= 0.25) return 'warning'
  return 'error'
}

function CoderefDisplay({ coderef }: { coderef: AtlasCoderefItem }) {
  const lineRange = [
    coderef.start_line != null ? String(coderef.start_line) : null,
    coderef.end_line != null ? String(coderef.end_line) : null,
  ]
    .filter(Boolean)
    .join('-')
  const display = lineRange ? `${coderef.file}:${lineRange}` : coderef.file
  return (
    <Typography
      component="span"
      variant="caption"
      sx={{ fontFamily: 'monospace', color: 'text.secondary', display: 'block', mt: 0.5 }}
    >
      {display}
    </Typography>
  )
}

export function AtlasNodeDetail({
  slug,
  kind,
  nodeSlug,
}: {
  slug: string
  kind: string
  nodeSlug: string
}) {
  const { data: node, isLoading } = useAtlasNode(slug, kind, nodeSlug)
  const { data: coderefs = [] } = useAtlasCoderefs(slug)
  const router = useRouter()

  useEffect(() => {
    if (!node) return
    const title = node.title || `${kind}:${nodeSlug}`
    document.title = `${title} | zdx`
    return () => { document.title = 'zdx' }
  }, [node?.title, kind, nodeSlug])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  if (!node) {
    return (
      <Box>
        <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
          Back
        </Button>
        <Typography color="error">Node {kind}/{nodeSlug} not found.</Typography>
      </Box>
    )
  }

  const chunks = (node.chunks ?? []) as AtlasChunk[]
  const edges: AtlasEdgeFromItem[] = node.edges ?? []

  const staleCount = chunks.filter(c => c.status === 'stale').length
  const staleFraction = chunks.length > 0 ? staleCount / chunks.length : 0
  const stalePct = Math.round(staleFraction * 100)

  const verifiedByCount = edges.filter(e => e.edge_type === 'verified_by').length

  const coderefMap = new Map<number, AtlasCoderefItem>(coderefs.map(c => [c.id, c]))

  const edgesByType = edges.reduce((acc, e) => {
    ;(acc[e.edge_type] ||= []).push(e)
    return acc
  }, {} as Record<string, AtlasEdgeFromItem[]>)

  return (
    <Box>
      <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
        Back
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        {node.title || `${kind}:${nodeSlug}`}
      </Typography>

      <Box sx={{ mb: 2, display: 'flex', gap: 1, flexWrap: 'wrap', alignItems: 'center' }}>
        <Chip label={kind} size="small" variant="outlined" />
        <Chip
          label={`${stalePct}% stale`}
          size="small"
          color={staleFractionColor(staleFraction)}
        />
        <Chip label={`${verifiedByCount} demos`} size="small" variant="outlined" />
      </Box>

      {node.description && (
        <Box sx={{ mb: 2 }}>
          <MarkdownContent slug={slug}>{node.description}</MarkdownContent>
        </Box>
      )}

      {chunks.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Documentation ({chunks.length})
          </Typography>
          {chunks.map(chunk => {
            const coderef = chunk.coderef_id != null ? coderefMap.get(chunk.coderef_id) : undefined
            const isDrift = chunk.status === 'stale'
            return (
              <Box
                key={chunk.id}
                sx={{ mb: 2, p: 2, border: 1, borderColor: isDrift ? 'error.light' : 'divider', borderRadius: 1 }}
              >
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: chunk.title ? 1 : 0 }}>
                  {chunk.title && (
                    <Typography variant="h6" component="h3" sx={{ flex: 1 }}>
                      {chunk.title}
                    </Typography>
                  )}
                  {isDrift && <Chip label="drift" size="small" color="error" />}
                </Box>
                <MarkdownContent slug={slug}>{chunk.body}</MarkdownContent>
                {coderef && <CoderefDisplay coderef={coderef} />}
              </Box>
            )
          })}
        </Box>
      )}

      {Object.keys(edgesByType).length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Related Nodes
          </Typography>
          {Object.entries(edgesByType).map(([edgeType, typeEdges]) => (
            <Box key={edgeType} sx={{ mb: 2 }}>
              <Typography
                variant="caption"
                sx={{ textTransform: 'uppercase', fontWeight: 700, display: 'block', mb: 0.5, color: 'text.secondary' }}
              >
                {edgeType} ({typeEdges.length})
              </Typography>
              {typeEdges.map(e => (
                <Box
                  key={e.id}
                  sx={{ py: 0.5, borderBottom: 1, borderColor: 'divider' }}
                >
                  <Link
                    to="/project/$slug/docs/node/$kind/$nodeSlug"
                    params={{ slug, kind: e.to_kind, nodeSlug: e.to_slug }}
                    style={{ textDecoration: 'none', color: 'inherit' }}
                  >
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Chip
                        label={e.to_kind}
                        size="small"
                        variant="outlined"
                        sx={{ height: 18, fontSize: '0.7rem' }}
                      />
                      <Typography
                        variant="body2"
                        sx={{ '&:hover': { textDecoration: 'underline' } }}
                      >
                        {e.to_slug}
                      </Typography>
                    </Box>
                  </Link>
                </Box>
              ))}
            </Box>
          ))}
        </Box>
      )}

      <Tooltip title="requires TK-1523" placement="top">
        <span>
          <Button disabled variant="outlined" size="small">
            File Issue
          </Button>
        </span>
      </Tooltip>
    </Box>
  )
}
