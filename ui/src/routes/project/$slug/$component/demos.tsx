import { createFileRoute } from '@tanstack/react-router'
import { Box, Typography } from '@mui/material'
import { DemosSection } from '../../../../components/DemoPlayer'

function DemosRoute() {
  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 2 }}>Demos</Typography>
      <DemosSection />
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/$component/demos')({
  component: DemosRoute,
})
