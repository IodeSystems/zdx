import { createFileRoute } from '@tanstack/react-router'
import { Alert, Box, Button, Card, CardContent, Chip, CircularProgress, Dialog, DialogActions, DialogContent, DialogTitle, Divider, FormControl, IconButton, InputLabel, MenuItem, Select, Skeleton, Snackbar, TextField, Tooltip, Typography } from '@mui/material'
import { Add as AddIcon, ArrowUpward as UpgradeIcon, Delete as DeleteIcon, Edit as EditIcon, PowerSettingsNew as EolIcon, Snooze as SnoozeIcon } from '@mui/icons-material'
import { useState } from 'react'
import { useCreateBranch, useBranches, useBranchDoctorRung, useDeferDoctorCheck, useDeleteBranch, useMarkBranchEOL, useSeedBranches, useUpdateBranch, useUpdateBranchSettings, type CreateVersionBranchResult, type VersionBranchItem } from '../../../../api'

const ROLE_ORDER = ['rolling-release', 'dev', 'pr-target', 'named-release']

const ROLE_CHIP_COLOR: Record<string, 'default' | 'primary' | 'warning' | 'success'> = {
  'rolling-release': 'default',
  'dev': 'primary',
  'pr-target': 'warning',
  'named-release': 'success',
}

function formatRelativeTime(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '—'
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const mins = Math.floor(diffMs / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  return `${days}d ago`
}

function SourceChain({ branches }: { branches: VersionBranchItem[] }) {
  if (branches.length === 0) return null

  const root = branches.find(b => !b.source_branch_name)
  if (!root) return null

  const chain: VersionBranchItem[] = []
  const visited = new Set<string>()
  let current: VersionBranchItem | undefined = root
  while (current && !visited.has(current.name)) {
    chain.push(current)
    visited.add(current.name)
    current = branches.find(b => b.source_branch_name === current!.name && !visited.has(b.name))
  }

  if (chain.length < 2) return null

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 0.5, mb: 2, p: 1.5, bgcolor: 'action.hover', borderRadius: 1 }}>
      {chain.map((b, i) => {
        const role = b.role ?? b.type
        return (
          <Box key={b.id} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Box
              component="a"
              href={`#branch-${b.name}`}
              sx={{
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
                px: 1,
                py: 0.5,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                minWidth: 80,
                textDecoration: 'none',
                '&:hover': { bgcolor: 'action.selected' },
              }}
            >
              <Typography sx={{ fontFamily: 'monospace', fontSize: '0.75rem', fontWeight: 600, color: 'text.primary' }}>
                {b.name}
              </Typography>
              <Chip
                label={role}
                size="small"
                color={ROLE_CHIP_COLOR[role] ?? 'default'}
                sx={{ fontSize: '0.65rem', height: 16, mt: 0.25 }}
              />
            </Box>
            {i < chain.length - 1 && (
              <Typography color="text.secondary" sx={{ fontSize: '1rem' }}>→</Typography>
            )}
          </Box>
        )
      })}
    </Box>
  )
}

function BranchSettingsPanel({ branch, slug }: { branch: VersionBranchItem; slug: string }) {
  const [mergeStyle, setMergeStyle] = useState(branch.merge_style ?? '')
  const [requiredChecks, setRequiredChecks] = useState(branch.required_checks ?? '')
  const update = useUpdateBranchSettings()

  const handleSave = () => {
    update.mutate({
      slug,
      name: branch.name,
      merge_style: mergeStyle || null,
      required_checks: requiredChecks || null,
    })
  }

  return (
    <Box sx={{ mt: 1.5 }}>
      <Divider sx={{ mb: 1.5 }} />
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1, fontWeight: 600, textTransform: 'uppercase', letterSpacing: 0.5 }}>
        PR Settings
      </Typography>
      <Box sx={{ display: 'flex', gap: 1.5, flexWrap: 'wrap', alignItems: 'flex-start' }}>
        <FormControl size="small" sx={{ minWidth: 140 }}>
          <InputLabel>Merge style</InputLabel>
          <Select
            label="Merge style"
            value={mergeStyle}
            onChange={e => setMergeStyle(e.target.value)}
          >
            <MenuItem value=""><em>None</em></MenuItem>
            <MenuItem value="squash">Squash</MenuItem>
            <MenuItem value="merge">Merge</MenuItem>
            <MenuItem value="rebase">Rebase</MenuItem>
          </Select>
        </FormControl>
        <TextField
          size="small"
          label="Required checks"
          value={requiredChecks}
          onChange={e => setRequiredChecks(e.target.value)}
          placeholder="ci,lint"
          sx={{ minWidth: 200 }}
        />
        <Button
          size="small"
          variant="contained"
          onClick={handleSave}
          disabled={update.isPending}
        >
          Save
        </Button>
        {update.isError && (
          <Typography variant="caption" color="error" sx={{ alignSelf: 'center' }}>
            {update.error?.message ?? 'Save failed'}
          </Typography>
        )}
      </Box>
    </Box>
  )
}

const ROLES = ['rolling-release', 'dev', 'pr-target', 'named-release'] as const

function EditBranchDialog({ slug, branch, open, onClose, branches }: {
  slug: string
  branch: VersionBranchItem
  open: boolean
  onClose: () => void
  branches: VersionBranchItem[]
}) {
  const currentRole = branch.role ?? branch.type
  const [role, setRole] = useState(currentRole)
  const [source, setSource] = useState(branch.source_branch_name ?? '')
  const update = useUpdateBranch()

  const handleSubmit = () => {
    const body: { slug: string; name: string; role?: string; source_branch_name?: string | null } = {
      slug,
      name: branch.name,
    }
    if (role !== currentRole) body.role = role
    if (source !== (branch.source_branch_name ?? '')) {
      body.source_branch_name = source === '' ? null : source
    }
    update.mutate(body, { onSuccess: onClose })
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Edit {branch.name}</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '16px !important' }}>
        <FormControl size="small" fullWidth>
          <InputLabel>Role</InputLabel>
          <Select label="Role" value={role} onChange={e => setRole(e.target.value)} data-testid="edit-branch-role">
            {ROLES.map(r => <MenuItem key={r} value={r}>{r}</MenuItem>)}
          </Select>
        </FormControl>
        <FormControl size="small" fullWidth>
          <InputLabel>Source branch</InputLabel>
          <Select label="Source branch" value={source} onChange={e => setSource(e.target.value)} data-testid="edit-branch-source">
            <MenuItem value=""><em>None</em></MenuItem>
            {branches.filter(b => b.name !== branch.name).map(b => (
              <MenuItem key={b.name} value={b.name}>{b.name}</MenuItem>
            ))}
          </Select>
        </FormControl>
        {update.isError && (
          <Typography variant="caption" color="error" data-testid="edit-branch-error">
            {update.error?.message ?? 'Update failed'}
          </Typography>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={update.isPending}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit} disabled={update.isPending} data-testid="edit-branch-submit">
          Save
        </Button>
      </DialogActions>
    </Dialog>
  )
}

function AddBranchDialog({ slug, open, onClose, branches }: {
  slug: string
  open: boolean
  onClose: () => void
  branches: VersionBranchItem[]
}) {
  const [name, setName] = useState('')
  const [role, setRole] = useState('dev')
  const [source, setSource] = useState('')
  const create = useCreateBranch()

  const handleSubmit = () => {
    create.mutate(
      { slug, name, role, source_branch_name: source || undefined },
      {
        onSuccess: () => {
          onClose()
          setName('')
          setRole('dev')
          setSource('')
        },
      },
    )
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Add Branch</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '16px !important' }}>
        <TextField
          size="small"
          label="Name"
          value={name}
          onChange={e => setName(e.target.value)}
          required
          data-testid="add-branch-name"
        />
        <FormControl size="small" fullWidth>
          <InputLabel>Role</InputLabel>
          <Select label="Role" value={role} onChange={e => setRole(e.target.value)} data-testid="add-branch-role">
            {ROLES.map(r => <MenuItem key={r} value={r}>{r}</MenuItem>)}
          </Select>
        </FormControl>
        <FormControl size="small" fullWidth>
          <InputLabel>Source branch</InputLabel>
          <Select label="Source branch" value={source} onChange={e => setSource(e.target.value)} data-testid="add-branch-source">
            <MenuItem value=""><em>None</em></MenuItem>
            {branches.map(b => (
              <MenuItem key={b.name} value={b.name}>{b.name}</MenuItem>
            ))}
          </Select>
        </FormControl>
        {create.isError && (
          <Typography variant="caption" color="error" data-testid="add-branch-error">
            {create.error?.message ?? 'Create failed'}
          </Typography>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={create.isPending}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit} disabled={create.isPending || !name} data-testid="add-branch-submit">
          Add
        </Button>
      </DialogActions>
    </Dialog>
  )
}

function CutReleaseDialog({ slug, open, onClose, branches, onSuccess }: {
  slug: string
  open: boolean
  onClose: () => void
  branches: VersionBranchItem[]
  onSuccess: (result: CreateVersionBranchResult) => void
}) {
  const [name, setName] = useState('')
  const [semver, setSemver] = useState('')
  const [source, setSource] = useState('')
  const create = useCreateBranch()

  const handleSubmit = () => {
    create.mutate(
      { slug, name, semver, source_branch_name: source, role: 'named-release' },
      {
        onSuccess: (result) => {
          onClose()
          setName('')
          setSemver('')
          setSource('')
          onSuccess(result)
        },
      },
    )
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Cut Named Release</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '16px !important' }}>
        <TextField
          size="small"
          label="Name"
          value={name}
          onChange={e => setName(e.target.value)}
          required
          placeholder="v1.2.x"
          data-testid="cut-release-name"
        />
        <TextField
          size="small"
          label="Semver"
          value={semver}
          onChange={e => setSemver(e.target.value)}
          required
          placeholder="1.2.0"
          data-testid="cut-release-semver"
        />
        <FormControl size="small" fullWidth>
          <InputLabel>Source branch</InputLabel>
          <Select label="Source branch" value={source} onChange={e => setSource(e.target.value)} data-testid="cut-release-source">
            <MenuItem value=""><em>Select source</em></MenuItem>
            {branches.map(b => (
              <MenuItem key={b.name} value={b.name}>{b.name}</MenuItem>
            ))}
          </Select>
        </FormControl>
        {create.isError && (
          <Typography variant="caption" color="error" data-testid="cut-release-error">
            {create.error?.message ?? 'Cut failed'}
          </Typography>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={create.isPending}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit} disabled={create.isPending || !name || !semver || !source} data-testid="cut-release-submit">
          Cut Release
        </Button>
      </DialogActions>
    </Dialog>
  )
}

type UpgradeStep =
  | { type: 'create'; role: string; name: string; source: string }
  | { type: 'update'; target: string; source_branch_name: string }

function getUpgradeSteps(currentRung: number, branches: VersionBranchItem[]): UpgradeStep[] {
  switch (currentRung) {
    case 0:
      return [{ type: 'create', role: 'rolling-release', name: '', source: '' }]
    case 1:
      return [
        { type: 'create', role: 'dev', name: 'dev', source: '' },
        { type: 'update', target: branches.find(b => (b.role ?? b.type) === 'rolling-release')?.name ?? '', source_branch_name: 'dev' },
      ]
    case 2:
      return [
        { type: 'create', role: 'pr-target', name: 'pr-target', source: branches.find(b => (b.role ?? b.type) === 'dev')?.name ?? '' },
      ]
    default:
      return []
  }
}

function UpgradeRungDialogContent({ slug, onClose, branches }: {
  slug: string
  onClose: () => void
  branches: VersionBranchItem[]
}) {
  const { data: rung } = useBranchDoctorRung(slug)
  const create = useCreateBranch()
  const update = useUpdateBranch()
  const [stepIndex, setStepIndex] = useState(0)
  const [overrides, setOverrides] = useState<Record<number, { name?: string; source?: string }>>({})
  const [completed, setCompleted] = useState<string[]>([])

  const steps = rung ? getUpgradeSteps(rung.current_rung, branches) : []
  const currentStep = steps[stepIndex]
  const isDone = steps.length > 0 && stepIndex >= steps.length
  const isPending = create.isPending || update.isPending
  const error = create.error ?? update.error

  const defaultName = currentStep?.type === 'create' ? currentStep.name : ''
  const defaultSource = currentStep?.type === 'create' ? currentStep.source : ''
  const name = overrides[stepIndex]?.name ?? defaultName
  const source = overrides[stepIndex]?.source ?? defaultSource
  const setName = (v: string) => setOverrides(prev => ({ ...prev, [stepIndex]: { ...prev[stepIndex], name: v } }))
  const setSource = (v: string) => setOverrides(prev => ({ ...prev, [stepIndex]: { ...prev[stepIndex], source: v } }))

  const advance = (msg: string) => {
    setCompleted(prev => [...prev, msg])
    create.reset()
    update.reset()
    setStepIndex(i => i + 1)
  }

  const handleSubmit = () => {
    if (!currentStep) return
    if (currentStep.type === 'create') {
      create.mutate(
        { slug, name, role: currentStep.role, source_branch_name: source || undefined },
        { onSuccess: () => advance(`Created "${name}" (${currentStep.role})`) },
      )
    } else {
      update.mutate(
        { slug, name: currentStep.target, source_branch_name: currentStep.source_branch_name },
        { onSuccess: () => advance(`Updated "${currentStep.target}" source → "${currentStep.source_branch_name}"`) },
      )
    }
  }

  return (
    <>
      <DialogTitle>Upgrade Branch Rung</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: '16px !important' }}>
        {isDone ? (
          <>
            <Typography variant="body2" color="success.main" sx={{ fontWeight: 600 }}>All steps completed.</Typography>
            {completed.map((msg, i) => (
              <Typography key={i} variant="body2">✓ {msg}</Typography>
            ))}
          </>
        ) : currentStep?.type === 'create' ? (
          <>
            <Typography variant="caption" color="text.secondary">
              Step {stepIndex + 1} of {steps.length} — Create <strong>{currentStep.role}</strong> branch
            </Typography>
            <TextField
              size="small"
              label="Name"
              value={name}
              onChange={e => setName(e.target.value)}
              required
              data-testid="upgrade-rung-name"
            />
            <FormControl size="small" fullWidth>
              <InputLabel>Source branch</InputLabel>
              <Select
                label="Source branch"
                value={source}
                onChange={e => setSource(e.target.value)}
                data-testid="upgrade-rung-source"
              >
                <MenuItem value=""><em>None</em></MenuItem>
                {branches.map(b => (
                  <MenuItem key={b.name} value={b.name}>{b.name}</MenuItem>
                ))}
              </Select>
            </FormControl>
            {error && (
              <Typography variant="caption" color="error" data-testid="upgrade-rung-error">
                {error.message ?? 'Failed'}
              </Typography>
            )}
          </>
        ) : currentStep?.type === 'update' ? (
          <>
            <Typography variant="caption" color="text.secondary">
              Step {stepIndex + 1} of {steps.length}
            </Typography>
            <Typography variant="body2">
              Set <strong>{currentStep.target}</strong> source branch to <strong>{currentStep.source_branch_name}</strong>.
            </Typography>
            {error && (
              <Typography variant="caption" color="error" data-testid="upgrade-rung-error">
                {error.message ?? 'Failed'}
              </Typography>
            )}
          </>
        ) : (
          <CircularProgress size={24} />
        )}
      </DialogContent>
      <DialogActions>
        {isDone ? (
          <Button variant="contained" onClick={onClose} data-testid="upgrade-rung-done">Done</Button>
        ) : (
          <>
            <Button onClick={onClose} disabled={isPending}>Cancel</Button>
            <Button
              variant="contained"
              onClick={handleSubmit}
              disabled={isPending || (currentStep?.type === 'create' && !name)}
              data-testid="upgrade-rung-submit"
            >
              {currentStep?.type === 'update' ? 'Update' : 'Create'}
            </Button>
          </>
        )}
      </DialogActions>
    </>
  )
}

function UpgradeRungDialog({ slug, open, onClose, branches }: {
  slug: string
  open: boolean
  onClose: () => void
  branches: VersionBranchItem[]
}) {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      {open && <UpgradeRungDialogContent slug={slug} onClose={onClose} branches={branches} />}
    </Dialog>
  )
}

function BranchCard({ branch, slug, branches }: { branch: VersionBranchItem; slug: string; branches: VersionBranchItem[] }) {
  const role = branch.role ?? branch.type
  const isEol = branch.status === 'eol'
  const eol = useMarkBranchEOL()
  const del = useDeleteBranch()
  const [toast, setToast] = useState<string | null>(null)
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const [editOpen, setEditOpen] = useState(false)

  const handleConfirmDelete = () => {
    del.mutate({ slug, name: branch.name }, {
      onError: (e) => {
        const msg = e.message
        if (msg.includes('blocked by environments:')) {
          setToast(`Cannot delete: ${msg}`)
        } else {
          setToast(`Delete failed: ${msg}`)
        }
      },
      onSettled: () => setConfirmingDelete(false),
    })
  }

  return (
    <>
      <Card variant="outlined" id={`branch-${branch.name}`} data-eol={isEol || undefined} sx={isEol ? { opacity: 0.6 } : undefined}>
        <CardContent sx={{ pb: '12px !important' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
            <Typography sx={{ fontFamily: 'monospace', fontWeight: 700, flex: 1 }}>{branch.name}</Typography>
            <Typography variant="caption" color="text.secondary">{formatRelativeTime(branch.created_at)}</Typography>
            <Tooltip title={isEol ? 'Mark active' : 'Mark EOL'}>
              <IconButton
                size="small"
                aria-label={isEol ? 'Mark active' : 'Mark EOL'}
                onClick={() => eol.mutate({ slug, name: branch.name })}
                disabled={eol.isPending}
                color={isEol ? 'error' : 'default'}
                data-testid={`branch-eol-${branch.name}`}
              >
                <EolIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title="Edit branch">
              <IconButton
                size="small"
                aria-label="Edit branch"
                onClick={() => setEditOpen(true)}
                data-testid={`branch-edit-${branch.name}`}
              >
                <EditIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            {confirmingDelete ? (
              <>
                <Button
                  size="small"
                  variant="contained"
                  color="error"
                  onClick={handleConfirmDelete}
                  disabled={del.isPending}
                  sx={{ fontSize: '0.75rem', py: 0.25 }}
                  data-testid={`branch-confirm-delete-${branch.name}`}
                >
                  Confirm delete
                </Button>
                <Button
                  size="small"
                  onClick={() => setConfirmingDelete(false)}
                  disabled={del.isPending}
                  sx={{ fontSize: '0.75rem', py: 0.25 }}
                  data-testid={`branch-cancel-delete-${branch.name}`}
                >
                  Cancel
                </Button>
              </>
            ) : (
              <Tooltip title="Delete branch">
                <IconButton
                  size="small"
                  aria-label="Delete branch"
                  onClick={() => setConfirmingDelete(true)}
                  disabled={del.isPending}
                  data-testid={`branch-delete-${branch.name}`}
                >
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            )}
          </Box>
          <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', alignItems: 'center' }}>
            <Chip
              label={role}
              size="small"
              color={ROLE_CHIP_COLOR[role] ?? 'default'}
              sx={{ fontSize: '0.7rem', height: 20 }}
            />
            {branch.semver && (
              <Chip
                label={branch.semver}
                size="small"
                variant="outlined"
                sx={{ fontFamily: 'monospace', fontSize: '0.7rem', height: 20 }}
              />
            )}
            {isEol && (
              <Chip label="EOL" size="small" color="error" sx={{ fontSize: '0.7rem', height: 20 }} />
            )}
            {branch.source_branch_name && (
              <Typography
                component="a"
                href={`#branch-${branch.source_branch_name}`}
                variant="caption"
                color="text.secondary"
                sx={{ textDecoration: 'none', '&:hover': { textDecoration: 'underline' } }}
              >
                → {branch.source_branch_name}
              </Typography>
            )}
          </Box>
          {role === 'pr-target' && <BranchSettingsPanel branch={branch} slug={slug} />}
        </CardContent>
      </Card>
      <Snackbar
        open={!!toast}
        autoHideDuration={5000}
        onClose={() => setToast(null)}
        message={toast}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      />
      <EditBranchDialog
        slug={slug}
        branch={branch}
        open={editOpen}
        onClose={() => setEditOpen(false)}
        branches={branches}
      />
    </>
  )
}

function RungBanner({ slug, onUpgrade }: { slug: string; onUpgrade?: () => void }) {
  const { data: rung, isLoading } = useBranchDoctorRung(slug)
  const defer = useDeferDoctorCheck()

  if (isLoading) {
    return <Skeleton variant="rounded" height={48} sx={{ mb: 2 }} />
  }
  if (!rung) return null

  if (rung.status === 'pass') {
    return (
      <Alert severity="success" sx={{ mb: 2 }}>
        {rung.message}
      </Alert>
    )
  }

  return (
    <Alert
      severity="warning"
      sx={{ mb: 2 }}
      action={
        <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
          {rung.current_rung < 3 && (
            <Tooltip title="Upgrade rung">
              <IconButton
                size="small"
                aria-label="Upgrade Rung"
                onClick={() => onUpgrade?.()}
                data-testid="upgrade-rung-button"
              >
                <UpgradeIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          )}
          <Tooltip title="Defer this check">
            <IconButton
              size="small"
              aria-label="Defer"
              onClick={() => defer.mutate({ slug, check_name: 'branching_strategy_appropriate', rung: String(rung.current_rung) })}
              disabled={defer.isPending}
            >
              <SnoozeIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
      }
    >
      <Typography variant="body2">{rung.message}</Typography>
      {rung.proposal && (
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>
          {rung.proposal}
        </Typography>
      )}
    </Alert>
  )
}

type SeedPreview = { name: string; role: string; source?: string }

function classificationSeedPreview(classification: string | undefined): SeedPreview[] {
  switch (classification) {
    case 'library':
    case 'tool':
    case 'site':
      return [{ name: 'main', role: 'rolling-release' }]
    case 'service':
    case 'saas':
      return [
        { name: 'dev', role: 'dev' },
        { name: 'main', role: 'rolling-release', source: 'dev' },
      ]
    default:
      return []
  }
}

function EmptyState({ slug }: { slug: string }) {
  const { data: rung } = useBranchDoctorRung(slug)
  const seed = useSeedBranches()
  const classification = rung?.classification
  const preview = classificationSeedPreview(classification)

  if (preview.length === 0) {
    return (
      <Box data-testid="empty-state">
        <Typography color="text.secondary" sx={{ mb: 1 }}>No branches configured.</Typography>
        <Typography variant="body2" color="text.secondary">
          {classification
            ? `No seed defined for classification "${classification}".`
            : 'Project classification not set yet — add one via dx classify, then return here to seed.'}
          {' '}Add a branch manually via{' '}
          <Typography component="span" variant="body2" sx={{ fontFamily: 'monospace' }}>
            dx branch add
          </Typography>
          {' '}or the Add Branch button above.
        </Typography>
      </Box>
    )
  }

  return (
    <Card variant="outlined" data-testid="empty-state">
      <CardContent>
        <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 0.5 }}>
          No branches yet
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          Based on this project's classification (<strong>{classification}</strong>),
          {preview.length === 1 ? ' one branch will be seeded:' : ` ${preview.length} branches will be seeded:`}
        </Typography>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75, mb: 2, p: 1.5, bgcolor: 'action.hover', borderRadius: 1 }}>
          {preview.map(s => (
            <Box key={s.name} sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }} data-testid={`seed-preview-${s.name}`}>
              <Typography sx={{ fontFamily: 'monospace', fontWeight: 600 }}>{s.name}</Typography>
              <Chip
                label={s.role}
                size="small"
                color={ROLE_CHIP_COLOR[s.role] ?? 'default'}
                sx={{ fontSize: '0.7rem', height: 20 }}
              />
              {s.source && (
                <Typography variant="caption" color="text.secondary">→ {s.source}</Typography>
              )}
            </Box>
          ))}
        </Box>
        <Button
          variant="contained"
          size="small"
          onClick={() => seed.mutate({ slug })}
          disabled={seed.isPending}
          data-testid="seed-branches-button"
        >
          {seed.isPending ? 'Seeding…' : 'Seed Branches'}
        </Button>
        {seed.isError && (
          <Typography variant="caption" color="error" sx={{ display: 'block', mt: 1 }} data-testid="seed-branches-error">
            {seed.error?.message ?? 'Seed failed'}
          </Typography>
        )}
      </CardContent>
    </Card>
  )
}

function BranchesPage() {
  const { slug } = Route.useParams()
  const { data: branches, isLoading } = useBranches(slug)
  const [addOpen, setAddOpen] = useState(false)
  const [cutOpen, setCutOpen] = useState(false)
  const [upgradeOpen, setUpgradeOpen] = useState(false)
  const [backportToast, setBackportToast] = useState<string | null>(null)

  const sorted = [...(branches ?? [])].sort((a, b) => {
    const ai = ROLE_ORDER.indexOf(a.role ?? a.type)
    const bi = ROLE_ORDER.indexOf(b.role ?? b.type)
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi)
  })

  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 2, gap: 1 }}>
        <Typography variant="h6" sx={{ fontWeight: 600, flexGrow: 1 }}>
          Branches
        </Typography>
        {sorted.length > 0 && (
          <Chip label={sorted.length} size="small" sx={{ fontSize: '0.7rem', height: 20 }} />
        )}
        <Button variant="outlined" size="small" startIcon={<AddIcon />} onClick={() => setAddOpen(true)} data-testid="add-branch-button">
          Add Branch
        </Button>
        <Button variant="contained" size="small" color="primary" onClick={() => setCutOpen(true)} data-testid="cut-release-button">
          Cut Release
        </Button>
      </Box>

      <RungBanner slug={slug} onUpgrade={() => setUpgradeOpen(true)} />

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : sorted.length === 0 ? (
        <EmptyState slug={slug} />
      ) : (
        <>
          <SourceChain branches={sorted} />
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {sorted.map(b => (
              <BranchCard key={b.id} branch={b} slug={slug} branches={sorted} />
            ))}
          </Box>
        </>
      )}

      <AddBranchDialog slug={slug} open={addOpen} onClose={() => setAddOpen(false)} branches={sorted} />
      <UpgradeRungDialog slug={slug} open={upgradeOpen} onClose={() => setUpgradeOpen(false)} branches={sorted} />
      <CutReleaseDialog
        slug={slug}
        open={cutOpen}
        onClose={() => setCutOpen(false)}
        branches={sorted}
        onSuccess={(result) => setBackportToast(`Release cut. Backport tasks created: ${result.backport_tasks_created}`)}
      />
      <Snackbar
        open={!!backportToast}
        autoHideDuration={5000}
        onClose={() => setBackportToast(null)}
        message={backportToast}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
        data-testid="backport-toast"
      />
    </>
  )
}

export const Route = createFileRoute('/project/$slug/branches/')({
  component: BranchesPage,
})
