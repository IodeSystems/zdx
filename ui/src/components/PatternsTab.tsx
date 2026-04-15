import { Link } from '@tanstack/react-router'
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { useState } from 'react'
import { useListPatterns, useAddPattern } from '../api'

export function PatternsTab({ slug }: { slug: string }) {
  const [search, setSearch] = useState('')
  const { data, isLoading } = useListPatterns(slug, search || undefined)
  const addPattern = useAddPattern()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [codeRefs, setCodeRefs] = useState('')

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const items = data?.patterns ?? []

  const handleCreate = () => {
    const refs = codeRefs
      .split(',')
      .map(s => s.trim())
      .filter(Boolean)
      .map(path => ({ path }))
    addPattern.mutate(
      { slug, name, description, code_refs: refs },
      {
        onSuccess: () => {
          setOpen(false)
          setName('')
          setDescription('')
          setCodeRefs('')
        },
      },
    )
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {data?.total ?? 0} pattern{(data?.total ?? 0) !== 1 ? 's' : ''}
        </Typography>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <TextField
            size="small"
            placeholder="Search patterns..."
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
          <Button size="small" variant="contained" onClick={() => setOpen(true)}>
            New Pattern
          </Button>
        </Box>
      </Box>

      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Description</TableCell>
              <TableCell align="right">Refs</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {items.map(p => (
              <TableRow
                key={p.id}
                component={Link as any}
                to="/project/$slug/patterns/$id"
                params={{ slug, id: String(p.id) }}
                sx={{ textDecoration: 'none', color: 'inherit', '&:hover': { bgcolor: 'action.hover' } }}
              >
                <TableCell>{p.name}</TableCell>
                <TableCell sx={{ maxWidth: 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {p.description}
                </TableCell>
                <TableCell align="right">{Array.isArray(p.code_refs) ? p.code_refs.length : 0}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>New Pattern</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            label="Name"
            value={name}
            onChange={e => setName(e.target.value)}
            sx={{ mt: 1 }}
            autoFocus
          />
          <TextField
            fullWidth
            label="Description"
            value={description}
            onChange={e => setDescription(e.target.value)}
            sx={{ mt: 2 }}
            multiline
            rows={4}
          />
          <TextField
            fullWidth
            label="Code references (comma-separated file:line paths)"
            value={codeRefs}
            onChange={e => setCodeRefs(e.target.value)}
            sx={{ mt: 2 }}
            placeholder="internal/server/handlers.go:42, internal/db/models.go:10"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)}>Cancel</Button>
          <Button variant="contained" disabled={addPattern.isPending || !name.trim()} onClick={handleCreate}>
            Create
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
