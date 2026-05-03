import { Box } from '@mui/material'
import type { DemoListItem } from '../api'
import { MetricTile } from './MetricTile'

export function DemosDashboard({ demos }: { demos: DemoListItem[] }) {
  const total = demos.length
  const cli = demos.filter(d => d.type === 'cli').length
  const video = demos.filter(d => d.type === 'video').length
  const passing = demos.filter(d => d.test_status === 'pass').length
  const failing = total - passing
  const passRate = total > 0 ? Math.round((passing / total) * 100) : 0

  const branches = demos.map(d => d.recorded_branch).filter(Boolean) as string[]
  const branchCounts = branches.reduce<Record<string, number>>((acc, b) => { acc[b] = (acc[b] ?? 0) + 1; return acc }, {})
  const deployLine = branches.length > 0
    ? Object.entries(branchCounts).sort((a, b) => b[1] - a[1])[0][0]
    : 'none'

  const totalDuration = demos.reduce((acc, d) => acc + (d.test_duration_ms || 0), 0)
  const totalDurationSec = (totalDuration / 1000).toFixed(1) + 's'

  return (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5, mb: 3 }}>
      <MetricTile label="Total" value={total} />
      <MetricTile label="CLI" value={cli} />
      <MetricTile label="Video" value={video} />
      <MetricTile
        label="Pass rate"
        value={`${passRate}%`}
        sub={`${passing} pass / ${failing} fail`}
        color={passRate === 100 ? 'success.main' : failing > 0 ? 'error.main' : undefined}
      />
      <MetricTile label="Deploy line" value={deployLine} />
      <MetricTile label="Total time" value={totalDurationSec} />
    </Box>
  )
}
