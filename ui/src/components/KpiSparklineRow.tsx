import { Box, Skeleton, Typography } from '@mui/material'
import { KPI_TREND_DEFAULT_N, useKpiTrend } from '../api'
import { Sparkline } from './Sparkline'

interface Props {
  slug: string
  scope: string
  checkName: string
  curr: number
  unit: string
  pct: number
  regression: boolean
}

export function KpiSparklineRow({ slug, scope, checkName, curr, unit, pct, regression }: Props) {
  const { data, isLoading } = useKpiTrend(slug, scope, checkName, KPI_TREND_DEFAULT_N)
  const samples = data ?? []
  const values = samples.map(s => s.value)
  const highlightIndex = values.length > 0 ? values.length - 1 : -1
  const sign = pct >= 0 ? '+' : ''

  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: 'minmax(0, 1fr) 80px auto',
        alignItems: 'center',
        gap: 1,
        px: 1,
        py: 0.5,
        borderRadius: 1,
        bgcolor: 'action.hover',
      }}
    >
      <Typography variant="caption" sx={{ fontSize: '0.7rem', fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {checkName}
      </Typography>
      {isLoading ? (
        <Skeleton variant="rectangular" width={80} height={28} sx={{ borderRadius: 0.5 }} />
      ) : (
        <Sparkline values={values} highlightIndex={highlightIndex} danger={regression} />
      )}
      <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 0.5, minWidth: 0 }}>
        <Typography variant="caption" sx={{ fontSize: '0.75rem', fontWeight: 600, color: regression ? 'error.main' : 'text.primary' }}>
          {curr}{unit}
        </Typography>
        <Typography variant="caption" sx={{ fontSize: '0.65rem', color: 'text.secondary' }}>
          ({sign}{pct.toFixed(1)}%)
        </Typography>
      </Box>
    </Box>
  )
}
