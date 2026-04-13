import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Typography,
} from '@mui/material'
import { useProjects } from '../api'

function HomePage() {
  const { data } = useProjects()

  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 3 }}>
        Projects
      </Typography>
      {data?.length === 0 && (
        <Typography color="text.secondary">No projects yet.</Typography>
      )}
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {data?.map(p => (
          <Card key={p.slug} variant="outlined">
            <CardActionArea component={Link as any} to="/project/$slug" params={{ slug: p.slug }}>
              <CardContent sx={{ py: 1.5 }}>
                <Box component="span" sx={{ fontWeight: 600 }}>
                  {p.name}
                </Box>{' '}
                <Box component="span" sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                  ({p.slug})
                </Box>
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
      </Box>
    </Box>
  )
}

export const Route = createFileRoute('/')({
  component: HomePage,
})
