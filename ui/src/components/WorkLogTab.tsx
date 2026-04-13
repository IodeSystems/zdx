import { Link } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Typography,
} from '@mui/material'
import { useIssues } from '../api'

const STATUS_COLORS: Record<string, 'success' | 'warning' | 'error' | 'info' | 'secondary' | 'default'> = {
  open: 'warning',
  triaged: 'info',
  'in-progress': 'secondary',
  done: 'success',
  closed: 'success',
}

export function WorkLogTab({ slug, componentSlug = 'all' }: { slug: string; componentSlug?: string }) {
  const { data, isLoading } = useIssues(slug)

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const allIssues = data ?? []
  const component = componentSlug === 'all' ? '' : componentSlug
  const issues = component ? allIssues.filter(i => i.component === component) : allIssues

  // Collect all work entries across issues, sorted newest first
  const entries = issues
    .flatMap(i => (i.work ?? []).map(w => ({ ...w, issueId: i.id, issueTitle: i.title || i.context })))
    .sort((a, b) => (b.createdAt > a.createdAt ? 1 : -1))

  return (
    <Box>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 2 }}>
        {entries.length} work entries
      </Typography>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
        {entries.map((e, i) => (
          <Card key={i} variant="outlined">
            <CardActionArea
              component={Link as any}
              to="/project/$slug/$component/issues/$id"
              params={{ slug, component: componentSlug, id: e.issueId }}
            >
              <CardContent sx={{ py: 1, display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                <Chip
                  label={e.agent || 'agent'}
                  size="small"
                  color="info"
                  sx={{ mt: 0.25, flexShrink: 0 }}
                />
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography variant="body2" sx={{ fontWeight: 600 }}>
                    {e.issueId}: {e.issueTitle?.slice(0, 60) || '(no title)'}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {e.createdAt}
                  </Typography>
                  {e.note && (
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 0.25 }}>
                      {e.note}
                    </Typography>
                  )}
                </Box>
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
        {entries.length === 0 && !isLoading && (
          <Typography variant="body2" color="text.secondary">No worklog entries.</Typography>
        )}
      </Box>
    </Box>
  )
}
