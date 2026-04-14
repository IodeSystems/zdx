import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import {
  Box,
  Button,
  CircularProgress,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
  Chip,
} from '@mui/material'
import { Delete as DeleteIcon, ContentCopy as ContentCopyIcon } from '@mui/icons-material'
import { useInvites, useCreateInvite, useDeleteInvite } from '../../api'

function InvitesPage() {
  const { data: invites, isLoading } = useInvites()
  const createInvite = useCreateInvite()
  const deleteInvite = useDeleteInvite()
  const [email, setEmail] = useState('')
  const [copied, setCopied] = useState<number | null>(null)

  const handleCreate = () => {
    if (!email.trim()) return
    createInvite.mutate({ email: email.trim() }, { onSuccess: () => setEmail('') })
  }

  const handleCopy = (token: string, id: number) => {
    navigator.clipboard.writeText(token)
    setCopied(id)
    setTimeout(() => setCopied(null), 2000)
  }

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />

  const now = new Date()

  return (
    <Box sx={{ maxWidth: 720 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 3 }}>
        Invite Codes
      </Typography>
      <Paper variant="outlined" sx={{ p: 3, mb: 3, display: 'flex', gap: 2, alignItems: 'flex-end' }}>
        <TextField
          label="Email"
          value={email}
          onChange={e => setEmail(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleCreate()}
          size="small"
          sx={{ flex: 1 }}
        />
        <Button
          variant="contained"
          onClick={handleCreate}
          disabled={createInvite.isPending || !email.trim()}
        >
          Create
        </Button>
      </Paper>
      {createInvite.isError && (
        <Typography color="error" sx={{ mb: 2 }}>{createInvite.error.message}</Typography>
      )}
      <TableContainer component={Paper} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Email</TableCell>
              <TableCell>Token</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Created</TableCell>
              <TableCell />
            </TableRow>
          </TableHead>
          <TableBody>
            {(!invites || invites.length === 0) && (
              <TableRow>
                <TableCell colSpan={5} sx={{ textAlign: 'center', color: 'text.secondary' }}>
                  No invites yet
                </TableCell>
              </TableRow>
            )}
            {invites?.map(inv => {
              const expired = new Date(inv.expires_at) < now
              const used = !!inv.used_at
              return (
                <TableRow key={inv.id}>
                  <TableCell>{inv.email}</TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, fontFamily: 'monospace', fontSize: '0.8rem' }}>
                      {inv.token.slice(0, 8)}...
                      <Tooltip title={copied === inv.id ? 'Copied!' : 'Copy token'}>
                        <IconButton size="small" onClick={() => handleCopy(inv.token, inv.id)}>
                          <ContentCopyIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  </TableCell>
                  <TableCell>
                    {used ? (
                      <Chip label="Used" size="small" color="default" />
                    ) : expired ? (
                      <Chip label="Expired" size="small" color="warning" />
                    ) : (
                      <Chip label="Active" size="small" color="success" />
                    )}
                  </TableCell>
                  <TableCell sx={{ whiteSpace: 'nowrap' }}>
                    {new Date(inv.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <IconButton
                      size="small"
                      onClick={() => deleteInvite.mutate(inv.id)}
                      disabled={deleteInvite.isPending}
                    >
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  )
}

export const Route = createFileRoute('/admin/invites')({
  component: InvitesPage,
})
