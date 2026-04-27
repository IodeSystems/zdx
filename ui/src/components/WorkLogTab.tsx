import { Box, Card, CardContent, Chip, Typography } from '@mui/material'
import { Link } from '@tanstack/react-router'
import { useWorklog, type WorklogEntry } from '../api'
import { fmtDate } from '../utils/date'

function WorklogRow({ entry, slug }: { entry: WorklogEntry; slug: string }) {
  return (
    <Card variant="outlined">
      <CardContent sx={{ py: 1, display: 'flex', flexDirection: 'column', gap: 0.5, '&:last-child': { pb: 1 } }}>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1 }}>
          <Typography variant="caption" color="text.secondary">
            {fmtDate(entry.created_at)}
          </Typography>
          <Link
            to="/project/$slug/issues/$id"
            params={{ slug, id: entry.issue_id }}
            style={{ textDecoration: 'none' }}
          >
            <Chip label={entry.issue_id} size="small" variant="outlined" sx={{ cursor: 'pointer' }} />
          </Link>
          {entry.agent && (
            <Typography variant="caption" color="text.disabled">
              [{entry.agent}]
            </Typography>
          )}
        </Box>
        {entry.issue_title && (
          <Link
            to="/project/$slug/issues/$id"
            params={{ slug, id: entry.issue_id }}
            style={{ textDecoration: 'none', color: 'inherit' }}
          >
            <Typography variant="body2" sx={{ fontWeight: 500, wordBreak: 'break-word' }}>
              {entry.issue_title}
            </Typography>
          </Link>
        )}
        {entry.note && (
          <Typography variant="body2" color="text.secondary" sx={{ wordBreak: 'break-word' }}>
            {entry.note}
          </Typography>
        )}
      </CardContent>
    </Card>
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
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {entries.map((e, i) => (
            <WorklogRow key={i} entry={e} slug={slug} />
          ))}
        </Box>
      )}
    </Box>
  )
}
