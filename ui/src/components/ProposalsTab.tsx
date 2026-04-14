import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from '@mui/material'
import { Add as AddIcon } from '@mui/icons-material'
import { useState } from 'react'
import {
  useProposals,
  useCreateProposal,
  useUpdateProposal,
  useFileProposal,
  type ProposalItem,
} from '../api'

const STATUS_COLORS: Record<string, 'success' | 'warning' | 'default' | 'error' | 'info'> = {
  proposed: 'info',
  filed: 'success',
  backlogged: 'warning',
  dismissed: 'default',
}

interface FormState {
  title: string
  context: string
  priority: number
}

const emptyForm: FormState = { title: '', context: '', priority: 0 }

function ProposalCard({
  proposal,
  onBacklog,
  onDismiss,
  onFile,
}: {
  proposal: ProposalItem
  onBacklog: () => void
  onDismiss: () => void
  onFile: () => void
}) {
  return (
    <Card variant="outlined">
      <CardContent sx={{ py: 1.25, '&:last-child': { pb: 1.25 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="body2" sx={{ fontWeight: 600, flex: 1 }}>{proposal.title}</Typography>
          <Chip
            label={proposal.status}
            size="small"
            color={STATUS_COLORS[proposal.status] ?? 'default'}
            sx={{ fontSize: '0.7rem' }}
          />
          {proposal.priority > 0 && (
            <Chip label={`P${proposal.priority}`} size="small" variant="outlined" sx={{ fontSize: '0.7rem' }} />
          )}
          {proposal.filed_issue_id && (
            <Chip label={proposal.filed_issue_id} size="small" color="success" variant="outlined" sx={{ fontSize: '0.7rem' }} />
          )}
        </Box>
        {proposal.context && (
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.25, whiteSpace: 'pre-wrap' }}>
            {proposal.context}
          </Typography>
        )}
        {proposal.status === 'proposed' && (
          <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
            <Button size="small" variant="contained" color="primary" onClick={onFile}>File as Issue</Button>
            <Button size="small" variant="outlined" onClick={onBacklog}>Backlog</Button>
            <Button size="small" color="inherit" onClick={onDismiss}>Dismiss</Button>
          </Box>
        )}
      </CardContent>
    </Card>
  )
}

export function ProposalsTab({ slug }: { slug: string }) {
  const { data: proposals, isLoading } = useProposals(slug)
  const create = useCreateProposal()
  const update = useUpdateProposal(slug)
  const file = useFileProposal(slug)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<FormState>(emptyForm)

  const handleCreate = async () => {
    await create.mutateAsync({ slug, ...form })
    setForm(emptyForm)
    setDialogOpen(false)
  }

  const handleFile = async (p: ProposalItem) => {
    const resp = await fetch('/api/dx/todo/issue/add', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ slug, title: p.title, context: p.context }),
    })
    if (!resp.ok) return
    const issue = await resp.json()
    await file.mutateAsync({ id: p.id, filed_issue_id: issue.id })
  }

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const proposed = (proposals ?? []).filter(p => p.status === 'proposed')
  const rest = (proposals ?? []).filter(p => p.status !== 'proposed')

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 1.5 }}>
        <Typography variant="overline" color="text.secondary" sx={{ flex: 1, letterSpacing: 1 }}>
          Proposals ({proposed.length} pending)
        </Typography>
        <Button size="small" startIcon={<AddIcon />} onClick={() => { setForm(emptyForm); setDialogOpen(true) }}>
          Add
        </Button>
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {proposed.map(p => (
          <ProposalCard
            key={p.id}
            proposal={p}
            onBacklog={() => update.mutateAsync({ id: p.id, status: 'backlogged' })}
            onDismiss={() => update.mutateAsync({ id: p.id, status: 'dismissed' })}
            onFile={() => handleFile(p)}
          />
        ))}
        {proposed.length === 0 && (
          <Typography variant="body2" color="text.secondary">No pending proposals.</Typography>
        )}
      </Box>

      {rest.length > 0 && (
        <Box sx={{ mt: 3 }}>
          <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: 1, mb: 1 }}>
            Reviewed ({rest.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mt: 1 }}>
            {rest.map(p => (
              <ProposalCard
                key={p.id}
                proposal={p}
                onBacklog={() => update.mutateAsync({ id: p.id, status: 'backlogged' })}
                onDismiss={() => update.mutateAsync({ id: p.id, status: 'dismissed' })}
                onFile={() => handleFile(p)}
              />
            ))}
          </Box>
        </Box>
      )}

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Add Proposal</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '8px !important' }}>
          <TextField
            label="Title"
            value={form.title}
            onChange={e => setForm({ ...form, title: e.target.value })}
            size="small"
            fullWidth
            autoFocus
          />
          <TextField
            label="Context"
            value={form.context}
            onChange={e => setForm({ ...form, context: e.target.value })}
            size="small"
            fullWidth
            multiline
            minRows={3}
          />
          <TextField
            label="Priority"
            type="number"
            value={form.priority}
            onChange={e => setForm({ ...form, priority: Number(e.target.value) || 0 })}
            size="small"
            sx={{ width: 120 }}
            slotProps={{ htmlInput: { min: 0, max: 10 } }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleCreate} variant="contained" disabled={!form.title.trim()}>Add</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
