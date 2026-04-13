import { Box, Typography } from '@mui/material'
import { useIssues } from '../api'

export function WorkLogTab({ slug, componentSlug = 'all' }: { slug: string; componentSlug?: string }) {
  const { isLoading } = useIssues(slug)

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  // Work entries are no longer embedded in the issue list response.
  // TODO: fetch per-issue work via show endpoint and aggregate here.
  return (
    <Box>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 2 }}>
        0 work entries
      </Typography>
      <Typography variant="body2" color="text.secondary">
        Work log not available — requires per-issue fetch (not yet implemented).
        {componentSlug !== 'all' && ` (component=${componentSlug})`}
      </Typography>
    </Box>
  )
}
