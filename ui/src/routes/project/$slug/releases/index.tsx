import { createFileRoute } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import {
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Snackbar,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  AccountTree as BranchIcon,
  RocketLaunch as RocketLaunchIcon,
} from '@mui/icons-material'
import { useEnvironments, useRequestEnvironmentTodo } from '../../../../api'
import type { EnvironmentItem } from '../../../../api'

function shortSha(sha: string): string {
  if (!sha) return '—'
  return sha.length > 8 ? sha.slice(0, 8) : sha
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

function EnvCard({ slug, branch, env }: { slug: string; branch: string; env: EnvironmentItem }) {
  const request = useRequestEnvironmentTodo()
  const [toast, setToast] = useState<string | null>(null)
  const drifted = (env.drift_count ?? 0) > 0

  const handleShip = () => {
    request.mutate({ slug, name: env.name, kind: 'ship' }, {
      onSuccess: () => setToast(`Ship todo created for ${env.name}`),
      onError: (e) => setToast(`Error: ${e.message}`),
    })
  }

  return (
    <>
      <Card variant="outlined" data-testid={`env-row-${env.name}`}>
        <Box
          sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 1.5, pt: 1, pb: 0.5 }}
          data-testid={`env-card-header-${env.name}`}
        >
          <Chip
            label={branch}
            size="small"
            variant="outlined"
            sx={{ fontFamily: 'monospace', fontSize: '0.75rem', height: 22 }}
            data-testid={`env-version-${env.name}`}
          />
          {drifted ? (
            <Tooltip title={`release branch is ${env.drift_count} commit${env.drift_count === 1 ? '' : 's'} behind dev`}>
              <Chip
                label={`${env.drift_count} behind`}
                size="small"
                color="warning"
                variant="filled"
                sx={{ fontSize: '0.7rem', height: 18 }}
                data-testid={`env-drift-${env.name}`}
              />
            </Tooltip>
          ) : null}
        </Box>

        <Box sx={{ px: 1.5, pb: 1 }} data-testid={`env-card-content-${env.name}`}>
          <Typography sx={{ fontFamily: 'monospace', fontWeight: 700 }}>{env.name}</Typography>
          {env.url ? (
            <Typography
              variant="body2"
              component="a"
              href={env.url}
              target="_blank"
              rel="noopener noreferrer"
              data-testid={`env-url-${env.name}`}
              sx={{ color: 'text.secondary', fontSize: '0.8rem', wordBreak: 'break-all', display: 'block' }}
            >
              {env.url}
            </Typography>
          ) : (
            <Typography variant="body2" color="text.disabled" data-testid={`env-url-${env.name}`}>—</Typography>
          )}
          <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', alignItems: 'center', mt: 0.5 }}>
            {env.current_build_sha ? (
              <Chip
                label={shortSha(env.current_build_sha)}
                size="small"
                variant="outlined"
                sx={{ fontFamily: 'monospace', fontSize: '0.75rem', height: 20 }}
                data-testid={`env-sha-${env.name}`}
              />
            ) : (
              <Typography variant="body2" color="text.disabled" data-testid={`env-sha-${env.name}`}>—</Typography>
            )}
            {env.current_build_branch ? (
              <Chip
                label={env.current_build_branch}
                size="small"
                color={
                  env.current_build_branch === 'main' || env.current_build_branch === 'master'
                    ? 'primary'
                    : 'default'
                }
                variant="outlined"
                sx={{ fontSize: '0.75rem', height: 20 }}
                data-testid={`env-branch-${env.name}`}
              />
            ) : (
              <Typography variant="body2" color="text.disabled" data-testid={`env-branch-${env.name}`}>—</Typography>
            )}
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ fontSize: '0.75rem' }}
              data-testid={`env-deployed-at-${env.name}`}
            >
              {formatRelativeTime(env.deployed_at)}
            </Typography>
          </Box>
        </Box>

        <Box
          sx={{ display: 'flex', gap: 0.5, px: 1.5, pb: 1.5, flexWrap: 'wrap' }}
          data-testid={`env-card-footer-${env.name}`}
        >
          <Tooltip title={env.release_branch ? `Deploy from ${env.release_branch}` : 'Create a ship todo for this environment'}>
            <Button
              size="small"
              variant="outlined"
              startIcon={<RocketLaunchIcon fontSize="small" />}
              onClick={handleShip}
              disabled={request.isPending}
              sx={{ fontSize: '0.75rem', py: 0.25 }}
            >
              Ship
            </Button>
          </Tooltip>
        </Box>
      </Card>
      <Snackbar
        open={!!toast}
        autoHideDuration={3000}
        onClose={() => setToast(null)}
        message={toast}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      />
    </>
  )
}

function VersionGroup({ slug, branch, envs }: { slug: string; branch: string; envs: EnvironmentItem[] }) {
  return (
    <Box sx={{ mb: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
        <BranchIcon fontSize="small" sx={{ color: 'text.secondary' }} />
        <Typography
          variant="subtitle2"
          sx={{ fontFamily: 'monospace', fontWeight: 600 }}
          data-testid={`version-group-${branch}`}
        >
          {branch}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {envs.length} environment{envs.length !== 1 ? 's' : ''}
        </Typography>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {envs.map(env => (
          <EnvCard key={env.id} slug={slug} branch={branch} env={env} />
        ))}
      </Box>
    </Box>
  )
}

function ReleasesPage() {
  const { slug } = Route.useParams()
  const { data: environments, isLoading } = useEnvironments(slug)
  const envs = environments ?? []

  const grouped = useMemo(() => {
    const groups: Map<string, EnvironmentItem[]> = new Map()
    for (const env of envs) {
      const key = env.release_branch || '(no version branch)'
      if (!groups.has(key)) groups.set(key, [])
      groups.get(key)!.push(env)
    }
    return groups
  }, [envs])

  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
        <Typography variant="h6" sx={{ fontWeight: 600 }}>Releases</Typography>
      </Box>

      {isLoading ? (
        <CircularProgress sx={{ m: 4 }} />
      ) : envs.length === 0 ? (
        <Typography color="text.secondary">No environments registered.</Typography>
      ) : (
        Array.from(grouped.entries()).map(([branch, branchEnvs]) => (
          <VersionGroup key={branch} slug={slug} branch={branch} envs={branchEnvs} />
        ))
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/releases/')({
  component: ReleasesPage,
})
