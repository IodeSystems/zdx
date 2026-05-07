import { Box, Chip, Typography } from '@mui/material'
import { useTaskReviews } from '../api'
import { EventStream } from './EventStream'
import { MarkdownContent } from './MarkdownContent'

const VERDICT_COLOR: Record<string, 'success' | 'error' | 'warning' | 'default'> = {
  approve: 'success',
  reject: 'error',
}

export function TaskReviewSection({
  slug,
  taskId,
}: {
  slug: string
  taskId: string
}) {
  const { data: reviews } = useTaskReviews(slug, taskId)
  if (!reviews || reviews.length === 0) return null

  return (
    <Box sx={{ mb: 3 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Reviews
      </Typography>
      {reviews.map(r => (
        <Box
          key={r.id}
          sx={{
            border: 1,
            borderColor: 'divider',
            borderRadius: 1,
            p: 2,
            mb: 2,
            backgroundColor: 'background.default',
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1, flexWrap: 'wrap' }}>
            <Chip
              label={r.verdict || 'pending'}
              size="small"
              color={VERDICT_COLOR[r.verdict] || 'default'}
            />
            <Chip label={r.reviewer_role} size="small" variant="outlined" />
            {r.reviewer_email && (
              <Typography variant="caption" color="text.secondary">
                by {r.reviewer_email}
              </Typography>
            )}
            <Typography variant="caption" color="text.disabled" sx={{ ml: 'auto' }}>
              {new Date(r.created_at).toLocaleString()}
            </Typography>
          </Box>
          {r.body && (
            <Box sx={{ mb: 1 }}>
              <MarkdownContent slug={slug}>{r.body}</MarkdownContent>
            </Box>
          )}
          <Box sx={{ mt: 1, pl: 1, borderLeft: 2, borderColor: 'divider' }}>
            <EventStream slug={slug} targetType="review" targetId={String(r.id)} />
          </Box>
        </Box>
      ))}
    </Box>
  )
}
