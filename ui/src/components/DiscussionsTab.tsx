import { useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { useDiscussions, useCreateDiscussion, type DiscussionItem } from '../api'

const STATUS_COLORS: Record<string, 'success' | 'default' | 'warning'> = {
  active: 'success',
  archived: 'default',
  closed: 'warning',
}

const FILTERS = ['active', 'archived', 'closed', 'all'] as const

function ageLabel(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  if (ms < 0) return 'just now'
  const m = Math.floor(ms / 60000)
  if (m < 1) return 'just now'
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h`
  const d = Math.floor(h / 24)
  if (d < 30) return `${d}d`
  const mo = Math.floor(d / 30)
  if (mo < 12) return `${mo}mo`
  return `${Math.floor(mo / 12)}y`
}

export function DiscussionsTab({
  slug,
  statusFilter,
  onStatusFilter,
}: {
  slug: string
  statusFilter: string | null
  onStatusFilter: (status: string | null) => void
}) {
  const { data, isLoading } = useDiscussions(slug, statusFilter ?? undefined)
  const createDiscussion = useCreateDiscussion()
  const navigate = useNavigate()
  const [createOpen, setCreateOpen] = useState(false)
  const [newMessage, setNewMessage] = useState('')
  const [newProvider, setNewProvider] = useState('claude')

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const items: DiscussionItem[] = data ?? []

  const handleCreate = () => {
    if (!newMessage.trim()) return
    createDiscussion.mutate(
      { slug, title: '', provider: newProvider },
      {
        onSuccess: d => {
          setCreateOpen(false)
          setNewMessage('')
          setNewProvider('claude')
          navigate({
            to: '/project/$slug/discussions/$id',
            params: { slug, id: String(d.id) },
            search: { initialMessage: newMessage.trim() },
          })
        },
      },
    )
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {items.length} discussions{statusFilter ? ` — filtered: ${statusFilter}` : ''}
        </Typography>
        <Stack direction="row" spacing={0.5} sx={{ flexWrap: 'wrap', flex: 1 }}>
          {FILTERS.map(status => {
            const isActive = (statusFilter ?? 'active') === status
            return (
              <Chip
                key={status}
                label={status}
                size="small"
                color={isActive ? STATUS_COLORS[status] ?? 'default' : 'default'}
                variant={isActive ? 'filled' : 'outlined'}
                onClick={() => onStatusFilter(status === 'active' ? null : status)}
                sx={{ cursor: 'pointer' }}
              />
            )
          })}
        </Stack>
        <Button size="small" variant="outlined" onClick={() => setCreateOpen(true)}>
          New
        </Button>
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map(d => (
          <Card key={d.id} variant="outlined">
            <CardActionArea
              component={Link as React.ElementType}
              to="/project/$slug/discussions/$id"
              params={{ slug, id: String(d.id) }}
            >
              <CardContent sx={{ py: 1.25, display: 'flex', alignItems: 'center', gap: 1 }}>
                <Typography variant="caption" color="text.secondary" sx={{ minWidth: 56 }}>
                  #{d.id}
                </Typography>
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {d.title || '(untitled)'}
                </Typography>
                <Chip
                  label={d.provider}
                  size="small"
                  variant="outlined"
                  sx={{ fontSize: '0.65rem' }}
                />
                <Chip
                  label={d.status}
                  size="small"
                  color={STATUS_COLORS[d.status] ?? 'default'}
                  variant="outlined"
                  sx={{ fontSize: '0.65rem' }}
                />
                <Typography variant="caption" color="text.secondary" sx={{ minWidth: 40, textAlign: 'right' }}>
                  {ageLabel(d.updated_at)}
                </Typography>
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
        {items.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            No discussions.
          </Typography>
        )}
      </Box>

      <Dialog open={createOpen} onClose={() => setCreateOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>New discussion</DialogTitle>
        <DialogContent>
          <TextField
            label="What do you want to discuss?"
            fullWidth
            multiline
            minRows={2}
            margin="dense"
            value={newMessage}
            onChange={e => setNewMessage(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && !e.shiftKey && handleCreate()}
            autoFocus
          />
          <TextField
            label="Provider"
            fullWidth
            margin="dense"
            select
            slotProps={{ select: { native: true } }}
            value={newProvider}
            onChange={e => setNewProvider(e.target.value)}
          >
            <option value="claude">claude</option>
            <option value="openai">openai</option>
            <option value="local">local</option>
          </TextField>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            onClick={handleCreate}
            disabled={!newMessage.trim() || createDiscussion.isPending}
          >
            Start
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
