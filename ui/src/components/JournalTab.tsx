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
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material'
import { Add as AddIcon } from '@mui/icons-material'
import { useState } from 'react'
import { useJournalEntries, useCreateJournalEntry, type JournalEntryItem } from '../api'

function EntryCard({ entry, prev }: { entry: JournalEntryItem; prev?: JournalEntryItem }) {
  const [expanded, setExpanded] = useState(false)

  const fields: { label: string; key: keyof JournalEntryItem }[] = [
    { label: 'Assessment', key: 'assessment' },
    { label: 'Concerns', key: 'concerns' },
    { label: 'Next Steps', key: 'next' },
  ]

  return (
    <Card variant="outlined" sx={{ mb: 1 }}>
      <CardContent
        sx={{ py: 1.25, '&:last-child': { pb: 1.25 }, cursor: 'pointer' }}
        onClick={() => setExpanded(e => !e)}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>{entry.date}</Typography>
          {entry.baseline && <Chip label="baseline" size="small" color="info" sx={{ fontSize: '0.7rem' }} />}
          <Typography variant="body2" color="text.secondary" sx={{ flex: 1, ml: 1 }}>
            {entry.tldr}
          </Typography>
        </Box>
        {expanded && (
          <Box sx={{ mt: 1.5, display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {fields.map(f => {
              const val = entry[f.key] as string
              if (!val) return null
              const prevVal = prev?.[f.key] as string | undefined
              const changed = prev && prevVal !== val
              return (
                <Box key={f.key}>
                  <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
                    {f.label}
                    {changed && (
                      <Chip label="changed" size="small" color="warning" sx={{ fontSize: '0.65rem', ml: 0.5, height: 18 }} />
                    )}
                  </Typography>
                  <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap', mt: 0.25 }}>{val}</Typography>
                </Box>
              )
            })}
          </Box>
        )}
      </CardContent>
    </Card>
  )
}

interface CheckinForm {
  tldr: string
  assessment: string
  concerns: string
  next: string
}

const emptyForm: CheckinForm = { tldr: '', assessment: '', concerns: '', next: '' }

export function JournalTab({ slug }: { slug: string }) {
  const [role, setRole] = useState<'owner' | 'tech'>('owner')
  const { data: entries, isLoading } = useJournalEntries(slug, role)
  const create = useCreateJournalEntry()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<CheckinForm>(emptyForm)

  const handleSave = async () => {
    const today = new Date().toISOString().slice(0, 10)
    await create.mutateAsync({ slug, role, date: today, ...form })
    setForm(emptyForm)
    setDialogOpen(false)
  }

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const sorted = entries ?? []

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 2, gap: 2 }}>
        <Tabs value={role} onChange={(_, v) => setRole(v)} sx={{ minHeight: 36 }}>
          <Tab label="Owner" value="owner" sx={{ minHeight: 36, py: 0 }} />
          <Tab label="Tech" value="tech" sx={{ minHeight: 36, py: 0 }} />
        </Tabs>
        <Box sx={{ flex: 1 }} />
        <Button size="small" startIcon={<AddIcon />} onClick={() => setDialogOpen(true)}>
          Check-in
        </Button>
      </Box>
      {sorted.length === 0 && (
        <Typography variant="body2" color="text.secondary">No journal entries yet.</Typography>
      )}
      {sorted.map((entry, i) => (
        <EntryCard key={entry.date + i} entry={entry} prev={sorted[i + 1]} />
      ))}
      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>New Check-in ({role})</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '8px !important' }}>
          <TextField
            label="TL;DR"
            value={form.tldr}
            onChange={e => setForm({ ...form, tldr: e.target.value })}
            size="small"
            fullWidth
            autoFocus
          />
          <TextField
            label="Assessment"
            value={form.assessment}
            onChange={e => setForm({ ...form, assessment: e.target.value })}
            size="small"
            fullWidth
            multiline
            minRows={3}
          />
          <TextField
            label="Concerns"
            value={form.concerns}
            onChange={e => setForm({ ...form, concerns: e.target.value })}
            size="small"
            fullWidth
            multiline
            minRows={2}
          />
          <TextField
            label="Next Steps"
            value={form.next}
            onChange={e => setForm({ ...form, next: e.target.value })}
            size="small"
            fullWidth
            multiline
            minRows={2}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleSave} variant="contained" disabled={!form.tldr.trim()}>Save</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
