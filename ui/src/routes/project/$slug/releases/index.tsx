import { createFileRoute } from '@tanstack/react-router'
import { useMemo } from 'react'
import {
  Box,
  Chip,
  CircularProgress,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material'
import { AccountTree as BranchIcon } from '@mui/icons-material'
import { useEnvironments } from '../../../../api'
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

function EnvRow({ env }: { env: EnvironmentItem }) {
  return (
    <TableRow hover data-testid={`env-row-${env.name}`}>
      <TableCell sx={{ fontFamily: 'monospace', fontWeight: 600 }}>
        {env.name}
      </TableCell>
      <TableCell>
        {env.url ? (
          <Typography
            variant="body2"
            component="a"
            href={env.url}
            target="_blank"
            rel="noopener noreferrer"
            data-testid={`env-url-${env.name}`}
            sx={{ color: 'text.secondary', fontSize: '0.8rem', wordBreak: 'break-all' }}
          >
            {env.url}
          </Typography>
        ) : (
          <Typography variant="body2" color="text.disabled" data-testid={`env-url-${env.name}`}>—</Typography>
        )}
      </TableCell>
      <TableCell data-testid={`env-sha-${env.name}`}>
        {env.current_build_sha ? (
          <Chip
            label={shortSha(env.current_build_sha)}
            size="small"
            variant="outlined"
            sx={{ fontFamily: 'monospace', fontSize: '0.75rem', height: 20 }}
          />
        ) : (
          <Typography variant="body2" color="text.disabled">—</Typography>
        )}
      </TableCell>
      <TableCell data-testid={`env-branch-${env.name}`}>
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
          />
        ) : (
          <Typography variant="body2" color="text.disabled">—</Typography>
        )}
      </TableCell>
      <TableCell sx={{ fontSize: '0.8rem', color: 'text.secondary' }} data-testid={`env-deployed-at-${env.name}`}>
        {formatRelativeTime(env.deployed_at)}
      </TableCell>
    </TableRow>
  )
}

function VersionGroup({ branch, envs }: { branch: string; envs: EnvironmentItem[] }) {
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
      <TableContainer component={Paper} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell sx={{ fontWeight: 600 }}>Environment</TableCell>
              <TableCell sx={{ fontWeight: 600 }}>URL</TableCell>
              <TableCell sx={{ fontWeight: 600 }}>SHA</TableCell>
              <TableCell sx={{ fontWeight: 600 }}>Branch</TableCell>
              <TableCell sx={{ fontWeight: 600 }}>Deployed</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {envs.map(env => (
              <EnvRow key={env.id} env={env} />
            ))}
          </TableBody>
        </Table>
      </TableContainer>
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
          <VersionGroup key={branch} branch={branch} envs={branchEnvs} />
        ))
      )}
    </>
  )
}

export const Route = createFileRoute('/project/$slug/releases/')({
  component: ReleasesPage,
})
