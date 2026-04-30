import { useState } from 'react'
import { Box, Button, Typography } from '@mui/material'
import type { EventItem } from '../../api'
import { MarkdownContent } from '../MarkdownContent'

type Props = { event: EventItem; slug: string }

type RevisionDetail = {
  field?: string
  from?: string
  to?: string
  from_version?: number
  to_version?: number
}

export function RevisionEvent({ event, slug }: Props) {
  const [open, setOpen] = useState(false)
  const d = event.detail_json as RevisionDetail | null ?? {}
  const field = String(d.field ?? 'field')
  const fromV = Number(d.from_version ?? 0)
  const toV = Number(d.to_version ?? 0)
  const toText = d.to ?? ''

  return (
    <Box>
      <Typography variant="body2" color="text.secondary" component="span">
        {field} updated (v{fromV} → v{toV})
      </Typography>
      {toText && (
        <Button
          size="small"
          variant="text"
          onClick={() => setOpen(o => !o)}
          sx={{ ml: 1, py: 0, minHeight: 0, fontSize: '0.75rem' }}
        >
          {open ? 'hide diff' : 'show diff'}
        </Button>
      )}
      {open && toText && (
        <Box sx={{ mt: 0.5, pl: 1, borderLeft: 2, borderColor: 'divider' }}>
          <MarkdownContent slug={slug}>{toText}</MarkdownContent>
        </Box>
      )}
    </Box>
  )
}
