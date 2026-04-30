import { Box } from '@mui/material'

interface SparklineProps {
  values: number[]
  highlightIndex?: number
  danger?: boolean
  width?: number
  height?: number
}

export function Sparkline({ values, highlightIndex, danger = false, width = 80, height = 20 }: SparklineProps) {
  if (values.length === 0) {
    return <Box sx={{ width, height, bgcolor: 'action.hover', borderRadius: 0.5 }} />
  }
  if (values.length === 1) {
    return (
      <svg width={width} height={height} role="img" aria-label="single sample">
        <circle cx={width / 2} cy={height / 2} r={2} fill={danger ? '#d32f2f' : '#1976d2'} />
      </svg>
    )
  }

  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  const stepX = width / (values.length - 1)
  const points = values.map((v, i) => {
    const x = i * stepX
    const y = height - ((v - min) / range) * height
    return [x, y] as const
  })
  const path = points.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`).join(' ')
  const stroke = danger ? '#d32f2f' : '#1976d2'

  return (
    <svg width={width} height={height} role="img" aria-label="sparkline">
      <path d={path} fill="none" stroke={stroke} strokeWidth={1.25} />
      {highlightIndex !== undefined && highlightIndex >= 0 && highlightIndex < points.length && (
        <circle
          cx={points[highlightIndex][0]}
          cy={points[highlightIndex][1]}
          r={2.5}
          fill={stroke}
          stroke="#fff"
          strokeWidth={0.75}
        />
      )}
    </svg>
  )
}
