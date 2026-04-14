import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import {
  Box,
  CircularProgress,
  MenuItem,
  Paper,
  Select,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { useAdminUsers, useUpdateUserRole } from '../../api'

const ROLES = ['admin', 'user', 'viewer'] as const

function UsersPage() {
  const [search, setSearch] = useState('')
  const { data: users, isLoading } = useAdminUsers(search || undefined)
  const updateRole = useUpdateUserRole()

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />

  return (
    <Box sx={{ maxWidth: 720 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 3 }}>
        Users
      </Typography>
      <TextField
        label="Search"
        value={search}
        onChange={e => setSearch(e.target.value)}
        placeholder="Search by email or name"
        size="small"
        fullWidth
        sx={{ mb: 2 }}
      />
      <TableContainer component={Paper} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Email</TableCell>
              <TableCell>Name</TableCell>
              <TableCell>Role</TableCell>
              <TableCell>Created</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(!users || users.length === 0) && (
              <TableRow>
                <TableCell colSpan={4} sx={{ textAlign: 'center', color: 'text.secondary' }}>
                  No users found
                </TableCell>
              </TableRow>
            )}
            {users?.map(u => (
              <TableRow key={u.id}>
                <TableCell>{u.email}</TableCell>
                <TableCell>{u.name}</TableCell>
                <TableCell>
                  <Select
                    value={u.role}
                    size="small"
                    variant="standard"
                    onChange={e => updateRole.mutate({ id: u.id, role: e.target.value })}
                    disabled={updateRole.isPending}
                  >
                    {ROLES.map(r => (
                      <MenuItem key={r} value={r}>{r}</MenuItem>
                    ))}
                  </Select>
                </TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>
                  {new Date(u.created_at).toLocaleDateString()}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  )
}

export const Route = createFileRoute('/admin/users')({
  component: UsersPage,
})
