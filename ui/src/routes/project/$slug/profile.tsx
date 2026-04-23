import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material'
import { Delete as DeleteIcon } from '@mui/icons-material'
import { useMe, useMyComments, useMyApiKeys, useDeleteApiKey } from '../../../api'
import { MarkdownContent } from '../../../components/MarkdownContent'

function ProfilePage() {
  const { slug } = Route.useParams()
  const { data: me } = useMe()
  const { data: mcData, isLoading } = useMyComments(slug)
  const { data: apiKeys, isLoading: keysLoading } = useMyApiKeys()
  const { mutate: deleteKey, isPending: deleting } = useDeleteApiKey()
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

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>Security</Typography>
      <Paper variant="outlined" sx={{ p: 2, mb: 3 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
          <Typography variant="body2" color="text.secondary">API Tokens</Typography>
          <Button
            variant="outlined"
            size="small"
            component={Link}
            to="/code"
            target="_blank"
          >
            Generate Token
          </Button>
        </Box>
        {keysLoading ? (
          <CircularProgress size={20} />
        ) : !apiKeys || apiKeys.length === 0 ? (
          <Typography variant="body2" color="text.secondary">No tokens yet. Generate one to use with the CLI.</Typography>
        ) : (
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Created</TableCell>
                <TableCell>Last Used</TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {apiKeys.map((k) => (
                <TableRow key={k.id}>
                  <TableCell>{k.name}</TableCell>
                  <TableCell>{new Date(k.created_at).toLocaleDateString()}</TableCell>
                  <TableCell>{k.last_used_at ? new Date(k.last_used_at).toLocaleDateString() : '—'}</TableCell>
                  <TableCell align="right">
                    <Tooltip title="Revoke">
                      <IconButton
                        size="small"
                        disabled={deleting}
                        onClick={() => deleteKey(k.id)}
                      >
                        <DeleteIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
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
                href={`/project/${slug}/${c.target_type}s/${c.target_id}#C-${c.id}`}
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
            <MarkdownContent slug={slug}>{c.body}</MarkdownContent>
          </Paper>
        ))
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/profile')({
  component: ProfilePage,
})
