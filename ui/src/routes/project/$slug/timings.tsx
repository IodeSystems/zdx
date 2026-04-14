import { createFileRoute } from '@tanstack/react-router'
import {
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
import { useTimed } from '../../../api'

function TimingsPage() {
  const { slug } = Route.useParams()
  const { data: timedData, isLoading } = useTimed(slug)
  const data = timedData?.items

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />
  if (!data || data.length === 0) return <Typography color="text.secondary">No timed entries.</Typography>

  return (
    <>
      <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>Timings</Typography>
      <TableContainer component={Paper} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell align="right">Max (ms)</TableCell>
              <TableCell align="right">Avg (ms)</TableCell>
              <TableCell align="right">Count</TableCell>
              <TableCell align="right">Total (ms)</TableCell>
              <TableCell>Source</TableCell>
              <TableCell>Last Seen</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {data.map(item => (
              <TableRow key={item.id} hover>
                <TableCell>{item.name}</TableCell>
                <TableCell align="right">{item.duration_ms}</TableCell>
                <TableCell align="right">{item.avg_ms}</TableCell>
                <TableCell align="right">{item.count}</TableCell>
                <TableCell align="right">{item.total_ms}</TableCell>
                <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>{item.source}</TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>{new Date(item.created_at).toLocaleString()}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </>
  )
}

export const Route = createFileRoute('/project/$slug/timings')({
  component: TimingsPage,
})
