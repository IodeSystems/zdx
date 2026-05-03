import { createFileRoute } from '@tanstack/react-router'
import { Box, Typography } from '@mui/material'
import { DemosSection } from '../../../../components/DemoPlayer'
import { DemosDashboard } from '../../../../components/DemosDashboard'
import { useDemos } from '../../../../api'

function DemosIndexRoute() {
  const { slug } = Route.useParams()
  const { data: demos } = useDemos(slug)
  const list = demos ?? []
  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 2 }}>Demos</Typography>
      <DemosDashboard demos={list} />
      <DemosSection slug={slug} demos={list} />
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/demos/')({
  component: DemosIndexRoute,
})
