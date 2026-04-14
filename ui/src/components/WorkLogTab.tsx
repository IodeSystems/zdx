import { Box, Chip, Typography } from '@mui/material'
import { Link } from '@tanstack/react-router'
import { useWorklog, type WorklogEntry } from '../api'
import { fmtDate } from '../utils/date'

function WorklogRow({ entry, slug }: { entry: WorklogEntry; slug: string }) {
  return (
    <Box sx={{ py: 1, borderBottom: 1, borderColor: 'divider', display: 'flex', gap: 1, alignItems: 'flex-start' }}>
      <Typography variant="caption" color="text.secondary" sx={{ minWidth: 72, pt: 0.25 }}>
        {fmtDate(entry.created_at)}
      </Typography>
      <Link
        to="/project/$slug/issues/$id"
        params={{ slug, id: entry.issue_id }}
        style={{ textDecoration: 'none' }}
      >
        <Chip label={entry.issue_id} size="small" variant="outlined" sx={{ cursor: 'pointer' }} />
      </Link>
      <Box sx={{ flex: 1 }}>
        {entry.issue_title && (
          <Typography component="span" variant="body2" sx={{ fontWeight: 500, mr: 1 }}>
            {entry.issue_title}
          </Typography>
        )}
        {entry.agent && (
          <Typography component="span" variant="caption" color="text.disabled" sx={{ mr: 1 }}>
            [{entry.agent}]
          </Typography>
        )}
        <Typography component="span" variant="body2" color="text.secondary">{entry.note}</Typography>
      </Box>
    </Box>
  )
}

export function WorkLogTab({ slug }: { slug: string }) {
  const { data: wlData, isLoading } = useWorklog(slug)
  const entries = wlData?.entries ?? []

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  return (
    <Box>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 2 }}>
        {entries.length} work {entries.length === 1 ? 'entry' : 'entries'}
      </Typography>
      {entries.length === 0 ? (
        <Typography variant="body2" color="text.secondary">No work log entries.</Typography>
      ) : (
        entries.map((e, i) => (
          <WorklogRow key={i} entry={e} slug={slug} />
        ))
      )}
    </Box>
  )
}
