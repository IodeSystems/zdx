import { createFileRoute } from '@tanstack/react-router'
import { Alert, Box, Button, Card, CardContent, Chip, CircularProgress, Divider, FormControl, IconButton, InputLabel, MenuItem, Select, Skeleton, Snackbar, TextField, Tooltip, Typography } from '@mui/material'
import { Delete as DeleteIcon, PowerSettingsNew as EolIcon, Snooze as SnoozeIcon } from '@mui/icons-material'
import { useState } from 'react'
import { useBranches, useBranchDoctorRung, useDeferDoctorCheck, useDeleteBranch, useMarkBranchEOL, useUpdateBranchSettings, type VersionBranchItem } from '../../../../api'

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

function BranchCard({ branch, slug }: { branch: VersionBranchItem; slug: string }) {
  const role = branch.role ?? branch.type
  const isEol = branch.status === 'eol'
  const eol = useMarkBranchEOL()
  const del = useDeleteBranch()
  const [toast, setToast] = useState<string | null>(null)
  const [confirmingDelete, setConfirmingDelete] = useState(false)

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
      <Card variant="outlined" id={`branch-${branch.name}`}>
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
    </>
  )
}

function RungBanner({ slug }: { slug: string }) {
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

function BranchesPage() {
  const { slug } = Route.useParams()
  const { data: branches, isLoading } = useBranches(slug)

  const sorted = [...(branches ?? [])].sort((a, b) => {
    const ai = ROLE_ORDER.indexOf(a.role ?? a.type)
    const bi = ROLE_ORDER.indexOf(b.role ?? b.type)
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi)
  })

  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
        <Typography variant="h6" sx={{ fontWeight: 600, flexGrow: 1 }}>
          Branches
        </Typography>
        {sorted.length > 0 && (
          <Chip label={sorted.length} size="small" sx={{ fontSize: '0.7rem', height: 20 }} />
        )}
      </Box>

      <RungBanner slug={slug} />

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : sorted.length === 0 ? (
        <Box data-testid="empty-state">
          <Typography color="text.secondary" sx={{ mb: 1 }}>No branches configured.</Typography>
          <Typography variant="body2" color="text.secondary">
            Add one via{' '}
            <Typography component="span" variant="body2" sx={{ fontFamily: 'monospace' }}>
              dx branch add
            </Typography>
            {' '}or{' '}
            <Typography component="span" variant="body2" sx={{ fontFamily: 'monospace' }}>
              POST /api/dx/projects/{'{'}slug{'}'}/branches
            </Typography>
          </Typography>
        </Box>
      ) : (
        <>
          <SourceChain branches={sorted} />
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {sorted.map(b => (
              <BranchCard key={b.id} branch={b} slug={slug} />
            ))}
          </Box>
        </>
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/branches/')({
  component: BranchesPage,
})
