import { Box, Chip } from '@mui/material'

export function LoadMore({ loaded, total, onLoadMore }: { loaded: number; total: number; onLoadMore: () => void }) {
  if (loaded >= total) return null
  return (
    <Box sx={{ textAlign: 'center', py: 1 }}>
      <Chip
        label={`Load more (${loaded} of ${total})`}
        size="small"
        onClick={onLoadMore}
        sx={{ cursor: 'pointer' }}
      />
    </Box>
  )
}
