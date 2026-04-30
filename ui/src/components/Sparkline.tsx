import { useTheme } from '@mui/material'

interface SparklineProps {
  values: number[]
  width?: number
  height?: number
  highlightIndex?: number
  danger?: boolean
}

export function Sparkline({ values, width = 80, height = 28, highlightIndex, danger = false }: SparklineProps) {
  const theme = useTheme()
  if (values.length < 2) return null

  const stroke = danger ? theme.palette.error.main : theme.palette.primary.main
  const dotFill = theme.palette.background.paper

  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min
  const stepX = width / (values.length - 1)
  const points = values.map((v, i) => {
    const x = i * stepX
    const y = range === 0 ? height / 2 : height - ((v - min) / range) * height
    return [x, y] as const
  })
  const polyPoints = points.map(([x, y]) => `${x.toFixed(2)},${y.toFixed(2)}`).join(' ')

  return (
    <svg width={width} height={height} role="img" aria-label="sparkline">
      <polyline points={polyPoints} fill="none" stroke={stroke} strokeWidth={1.25} strokeLinejoin="round" />
      {highlightIndex !== undefined && highlightIndex >= 0 && highlightIndex < points.length && (
        <circle
          cx={points[highlightIndex][0]}
          cy={points[highlightIndex][1]}
          r={2.75}
          fill={dotFill}
          stroke={stroke}
          strokeWidth={1.25}
        />
      )}
    </svg>
  )
}
