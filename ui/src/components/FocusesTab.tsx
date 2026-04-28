import { Link } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
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
import { useListFocuses, useAddFocus } from '../api'

const PRIORITY_LABELS: Record<number, string> = { 1: 'urgent', 2: 'high', 3: 'medium', 4: 'low' }
const PRIORITY_COLORS: Record<string, 'error' | 'warning' | 'info' | 'default'> = {
  urgent: 'error',
  high: 'warning',
  medium: 'info',
  low: 'default',
}
const STATUS_COLORS: Record<string, 'success' | 'info' | 'default'> = {
  active: 'info',
  done: 'success',
  archived: 'default',
}

export function FocusesTab({ slug }: { slug: string }) {
  const { data: focuses, isLoading } = useListFocuses(slug)
  const addFocus = useAddFocus()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState(2)

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const items = focuses ?? []

  const handleCreate = () => {
    addFocus.mutate(
      { slug, name, description, priority, blockers: '' },
      { onSuccess: () => { setOpen(false); setName(''); setDescription(''); setPriority(2) } },
    )
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {items.length} focus{items.length !== 1 ? 'es' : ''}
        </Typography>
        <Button size="small" variant="contained" onClick={() => setOpen(true)}>
          New Focus
        </Button>
      </Box>

      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Priority</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right">Concerns</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {items.map(t => {
              const pLabel = PRIORITY_LABELS[t.priority] ?? 'medium'
              const concernCount = (t.attributions ?? []).filter(a => a.status === 'open').length
              return (
                <TableRow
                  key={t.id}
                  component={Link as any}
                  to="/project/$slug/focuses/$name"
                  params={{ slug, name: t.name }}
                  sx={{ textDecoration: 'none', color: 'inherit', '&:hover': { bgcolor: 'action.hover' } }}
                >
                  <TableCell>{t.name}</TableCell>
                  <TableCell>
                    <Chip label={pLabel} size="small" color={PRIORITY_COLORS[pLabel] || 'default'} />
                  </TableCell>
                  <TableCell>
                    <Chip label={t.status} size="small" variant="outlined" color={STATUS_COLORS[t.status] || 'default'} />
                  </TableCell>
                  <TableCell align="right">{concernCount || ''}</TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>New Focus</DialogTitle>
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
            rows={3}
          />
          <TextField
            fullWidth
            select
            label="Priority"
            value={priority}
            onChange={e => setPriority(Number(e.target.value))}
            sx={{ mt: 2 }}
            slotProps={{ select: { native: true } }}
          >
            <option value={1}>Urgent</option>
            <option value={2}>High</option>
            <option value={3}>Medium</option>
            <option value={4}>Low</option>
          </TextField>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            disabled={addFocus.isPending || !name.trim()}
            onClick={handleCreate}
          >
            Create
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
