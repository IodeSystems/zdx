import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  CircularProgress,
  Typography,
} from '@mui/material'
import { useAdminStats } from '../../api'

function AdminDashboard() {
  const { data: stats, isLoading } = useAdminStats()

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />

  const cards = [
    { label: 'Users', value: stats?.user_count ?? 0, to: '/admin/users' as const },
    { label: 'Invites', value: stats?.invite_count ?? 0, to: '/admin/invites' as const },
    { label: 'Projects', value: stats?.project_count ?? 0, to: '/' as const },
    { label: 'WebSocket', value: 'Diag', to: '/admin/websocket' as const },
  ]

  return (
    <Box sx={{ maxWidth: 720 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 3 }}>
        Admin Dashboard
      </Typography>
      <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
        {cards.map(c => (
          <Card key={c.label} variant="outlined" sx={{ minWidth: 160, flex: 1 }}>
            <CardActionArea component={Link as any} to={c.to}>
              <CardContent>
                <Typography variant="h3" sx={{ fontWeight: 700 }}>
                  {c.value}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {c.label}
                </Typography>
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
      </Box>
    </Box>
  )
}

export const Route = createFileRoute('/admin/')({
  component: AdminDashboard,
})
