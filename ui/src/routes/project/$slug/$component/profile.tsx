import { createFileRoute } from '@tanstack/react-router'
import {
  Box,
  Chip,
  CircularProgress,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import { useMe, useMyComments } from '../../../../api'

function ProfilePage() {
  const { slug, component } = Route.useParams()
  const { data: me } = useMe()
  const { data: mcData, isLoading } = useMyComments(slug)
  const comments = mcData?.comments

  if (!me) return null

  return (
    <>
      <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>Profile</Typography>

      <Paper variant="outlined" sx={{ p: 2, mb: 3 }}>
        <Stack spacing={0.5}>
          <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>{me.name}</Typography>
          <Typography variant="body2" color="text.secondary">{me.email}</Typography>
          <Box><Chip label={me.role} size="small" /></Box>
        </Stack>
      </Paper>

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>My Comments</Typography>

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : !comments || comments.length === 0 ? (
        <Typography color="text.secondary">No comments yet.</Typography>
      ) : (
        comments.map(c => (
          <Paper key={c.id} variant="outlined" sx={{ p: 2, mb: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
              <Chip label={c.target_type} size="small" variant="outlined" />
              <Typography
                component="a"
                href={`/project/${slug}/${component}/${c.target_type}s/${c.target_id}`}
                variant="body2"
                color="primary"
                sx={{ fontWeight: 500, textDecoration: 'none', '&:hover': { textDecoration: 'underline' } }}
              >
                {c.target_id}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {new Date(c.created_at).toLocaleString()}
              </Typography>
            </Box>
            <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>{c.body}</Typography>
          </Paper>
        ))
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/$component/profile')({
  component: ProfilePage,
})
