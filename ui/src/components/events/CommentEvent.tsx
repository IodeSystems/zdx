import { useState } from 'react'
import { Box, Chip, Popover, Stack, Typography } from '@mui/material'
import type { EventItem } from '../../api'
import { MarkdownContent } from '../MarkdownContent'

type Props = { event: EventItem; slug: string }

type AgentProcessResult = {
  status?: string
  reasoning?: string
  addressing_event_id?: number | null
}

const VERDICT_COLORS: Record<string, 'success' | 'info' | 'warning'> = {
  addressed: 'success',
  already_addressed: 'info',
  for_someone_else: 'warning',
}

function parseResult(event: EventItem): AgentProcessResult | null {
  const r = event.agent_process_result
  if (r && typeof r === 'object') return r as AgentProcessResult
  return null
}

export function CommentEvent({ event, slug }: Props) {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null)
  const d = event.detail_json
  const body = d && typeof d === 'object' ? String((d as Record<string, unknown>).body ?? '') : ''
  const result = parseResult(event)
  const showPending = !result && event.author_kind === 'user'

  return (
    <Box>
      <MarkdownContent slug={slug}>{body}</MarkdownContent>
      <Stack direction="row" spacing={0.5} sx={{ mt: 0.5, flexWrap: 'wrap' }}>
        {result?.status && (
          <>
            <Chip
              label={result.status.replace(/_/g, ' ')}
              size="small"
              color={VERDICT_COLORS[result.status] ?? 'default'}
              onClick={e => setAnchorEl(e.currentTarget)}
              sx={{ cursor: 'pointer' }}
            />
            <Popover
              open={Boolean(anchorEl)}
              anchorEl={anchorEl}
              onClose={() => setAnchorEl(null)}
              anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
            >
              <Box sx={{ p: 1.5, maxWidth: 320 }}>
                {result.reasoning && (
                  <Typography variant="body2">{result.reasoning}</Typography>
                )}
                {result.addressing_event_id != null && (
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>
                    See:{' '}
                    <a href={`#E-${result.addressing_event_id}`} style={{ color: 'inherit' }}>
                      event #{result.addressing_event_id}
                    </a>
                  </Typography>
                )}
              </Box>
            </Popover>
          </>
        )}
        {showPending && (
          <Chip label="pending review" size="small" variant="outlined" color="default" />
        )}
      </Stack>
    </Box>
  )
}
