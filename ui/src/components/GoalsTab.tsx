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
  IconButton,
  TextField,
  Typography,
} from '@mui/material'
import { Add as AddIcon, Delete as DeleteIcon, Edit as EditIcon } from '@mui/icons-material'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import {
  useGoals,
  useCreateGoal,
  useUpdateGoal,
  useDeleteGoal,
  type GoalItem,
} from '../api'

type Entity = GoalItem

interface FormState {
  id?: number
  title: string
  description: string
  priority: number
  status: string
}

const emptyForm: FormState = { title: '', description: '', priority: 1, status: 'active' }

const STATUS_COLORS: Record<string, 'success' | 'warning' | 'default'> = {
  active: 'success',
  paused: 'warning',
  archived: 'default',
}

function EntityCard({
  item,
  slug,
  onEdit,
  onDelete,
}: {
  item: Entity
  slug?: string
  onEdit: () => void
  onDelete: () => void
}) {
  const titleContent = (
    <Typography
      variant="body2"
      sx={{ fontWeight: 600, flex: 1 }}
    >
      {item.title}
    </Typography>
  )

  return (
    <Card variant="outlined">
      <CardContent sx={{ py: 1.25, '&:last-child': { pb: 1.25 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          {slug ? (
            <Link
              to="/project/$slug/goals/$id"
              params={{ slug, id: String(item.id) }}
              onClick={(e) => e.stopPropagation()}
              style={{ flex: 1, textDecoration: 'none', color: 'inherit', cursor: 'pointer' }}
            >
              {titleContent}
            </Link>
          ) : (
            titleContent
          )}
          <Chip
            label={item.status}
            size="small"
            color={STATUS_COLORS[item.status] ?? 'default'}
            sx={{ fontSize: '0.7rem' }}
          />
          <Chip label={`P${item.priority}`} size="small" variant="outlined" sx={{ fontSize: '0.7rem' }} />
          <IconButton size="small" onClick={onEdit}><EditIcon fontSize="small" /></IconButton>
          <IconButton size="small" onClick={onDelete}><DeleteIcon fontSize="small" /></IconButton>
        </Box>
        {item.description && (
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.25 }}>
            {item.description}
          </Typography>
        )}
      </CardContent>
    </Card>
  )
}

function EntityForm({
  open,
  title,
  form,
  onChange,
  onSave,
  onCancel,
}: {
  open: boolean
  title: string
  form: FormState
  onChange: (f: FormState) => void
  onSave: () => void
  onCancel: () => void
}) {
  return (
    <Dialog open={open} onClose={onCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '8px !important' }}>
        <TextField
          label="Title"
          value={form.title}
          onChange={e => onChange({ ...form, title: e.target.value })}
          size="small"
          fullWidth
          autoFocus
        />
        <TextField
          label="Description"
          value={form.description}
          onChange={e => onChange({ ...form, description: e.target.value })}
          size="small"
          fullWidth
          multiline
          minRows={2}
        />
        <TextField
          label="Priority"
          type="number"
          value={form.priority}
          onChange={e => onChange({ ...form, priority: Number(e.target.value) || 1 })}
          size="small"
          sx={{ width: 120 }}
          slotProps={{ htmlInput: { min: 1, max: 4 } }}
        />
        <TextField
          label="Status"
          select
          value={form.status}
          onChange={e => onChange({ ...form, status: e.target.value })}
          size="small"
          sx={{ width: 160 }}
          slotProps={{ select: { native: true } }}
        >
          <option value="active">Active</option>
          <option value="paused">Paused</option>
          <option value="archived">Archived</option>
        </TextField>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel}>Cancel</Button>
        <Button onClick={onSave} variant="contained" disabled={!form.title.trim()}>Save</Button>
      </DialogActions>
    </Dialog>
  )
}

function Section({
  label,
  items,
  slug,
  useCreate,
  useUpdate,
  useDelete,
}: {
  label: string
  items: Entity[]
  slug: string
  useCreate: () => { mutateAsync: (body: any) => Promise<any> }
  useUpdate: (slug: string) => { mutateAsync: (body: any) => Promise<any> }
  useDelete: (slug: string) => { mutateAsync: (id: number) => Promise<any> }
}) {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<FormState>(emptyForm)
  const create = useCreate()
  const update = useUpdate(slug)
  const del = useDelete(slug)

  const openNew = () => {
    setForm(emptyForm)
    setDialogOpen(true)
  }

  const openEdit = (item: Entity) => {
    setForm({ id: item.id, title: item.title, description: item.description, priority: item.priority, status: item.status })
    setDialogOpen(true)
  }

  const save = async () => {
    if (form.id) {
      await update.mutateAsync({ id: form.id, title: form.title, description: form.description, priority: form.priority, status: form.status })
    } else {
      await create.mutateAsync({ slug, title: form.title, description: form.description, priority: form.priority, status: form.status })
    }
    setDialogOpen(false)
  }

  return (
    <Box sx={{ mb: 4 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 1.5 }}>
        <Typography variant="overline" color="text.secondary" sx={{ flex: 1, letterSpacing: 1 }}>
          {label} ({items.length})
        </Typography>
        <Button size="small" startIcon={<AddIcon />} onClick={openNew}>Add</Button>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map(item => (
          <EntityCard
            key={item.id}
            item={item}
            slug={label === 'Goals' ? slug : undefined}
            onEdit={() => openEdit(item)}
            onDelete={() => del.mutateAsync(item.id)}
          />
        ))}
        {items.length === 0 && (
          <Typography variant="body2" color="text.secondary">No {label.toLowerCase()} defined.</Typography>
        )}
      </Box>
      <EntityForm
        open={dialogOpen}
        title={form.id ? `Edit ${label.slice(0, -1)}` : `Add ${label.slice(0, -1)}`}
        form={form}
        onChange={setForm}
        onSave={save}
        onCancel={() => setDialogOpen(false)}
      />
    </Box>
  )
}

export function GoalsTab({ slug }: { slug: string }) {
  const { data: goals, isLoading: goalsLoading } = useGoals(slug)

  if (goalsLoading) {
    return <Typography color="text.secondary">Loading...</Typography>
  }

  return (
    <Box>
      <Section
        label="Goals"
        items={goals ?? []}
        slug={slug}
        useCreate={useCreateGoal}
        useUpdate={useUpdateGoal}
        useDelete={useDeleteGoal}
      />
    </Box>
  )
}
