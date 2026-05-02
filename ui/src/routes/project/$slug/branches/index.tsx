import { createFileRoute } from '@tanstack/react-router'
import { Box, Card, CardContent, Chip, CircularProgress, Typography } from '@mui/material'
import { useBranches, type VersionBranchItem } from '../../../../api'

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

function BranchCard({ branch }: { branch: VersionBranchItem }) {
  const role = branch.role ?? branch.type
  const isEol = branch.status === 'eol'

  return (
    <Card variant="outlined" id={`branch-${branch.name}`}>
      <CardContent sx={{ pb: '12px !important' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
          <Typography sx={{ fontFamily: 'monospace', fontWeight: 700, flex: 1 }}>{branch.name}</Typography>
          <Typography variant="caption" color="text.secondary">{formatRelativeTime(branch.created_at)}</Typography>
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
      </CardContent>
    </Card>
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

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : sorted.length === 0 ? (
        <Box>
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
              <BranchCard key={b.id} branch={b} />
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
