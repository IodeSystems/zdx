import { createFileRoute } from '@tanstack/react-router'
import { Box, Typography } from '@mui/material'
import { DemosSection } from '../../../../components/DemoPlayer'

function DemosIndexRoute() {
  const { slug } = Route.useParams()
  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 2 }}>Demos</Typography>
      <DemosSection slug={slug} />
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/demos/')({
  component: DemosIndexRoute,
})
