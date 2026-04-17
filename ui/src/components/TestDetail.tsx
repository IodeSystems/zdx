import { useEffect } from 'react'
import { useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
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
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useTest, useTestHistory, useTestCodeRefs } from '../api'
import { CodeRefs } from './CodeRefs'

const STATUS_COLOR: Record<string, 'success' | 'error' | 'default' | 'warning'> = {
  pass: 'success',
  fail: 'error',
  skip: 'warning',
}

function formatDuration(ms: number): string {
  if (ms === 0) return '—'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function formatRelativeTime(iso: string): string {
  const d = new Date(iso)
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

export function TestDetail({ slug, testId }: { slug: string; testId: number }) {
  const { data: test, isLoading } = useTest(slug, testId)
  const { data: historyData, isLoading: historyLoading } = useTestHistory(slug, test?.name ?? '', !!test)
  const { data: codeRefs } = useTestCodeRefs(slug, testId)
  const router = useRouter()

  useEffect(() => {
    if (!test) return
    document.title = `${test.name} | Tests | zdx`
    return () => { document.title = 'zdx' }
  }, [test?.name])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  if (!test) {
    return (
      <Box>
        <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
          Back
        </Button>
        <Typography color="error">Test not found.</Typography>
      </Box>
    )
  }

  const history = historyData?.history ?? []

  return (
    <Box>
      <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
        Back to tests
      </Button>

      <Typography variant="h5" sx={{ mb: 1, fontFamily: 'monospace' }}>{test.name}</Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip label={test.component} size="small" variant="outlined" />
        <Chip label={test.layer} size="small" variant="outlined" />
        <Chip label={test.status} size="small" color={STATUS_COLOR[test.status] ?? 'default'} />
        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>{formatDuration(test.duration_ms)}</Typography>
        {test.last_run_at && (
          <Typography variant="caption" color="text.secondary">
            Last run: {formatRelativeTime(test.last_run_at)}
          </Typography>
        )}
      </Box>

      <CodeRefs refs={codeRefs ?? []} slug={slug} />

      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: 600 }}>Run History</Typography>
        {historyLoading ? (
          <CircularProgress size={16} />
        ) : history.length === 0 ? (
          <Typography variant="body2" color="text.secondary">No history recorded.</Typography>
        ) : (
          <TableContainer component={Paper} variant="outlined">
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Status</TableCell>
                  <TableCell>Duration</TableCell>
                  <TableCell>Run At</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {history.map((h) => (
                  <TableRow key={h.id}>
                    <TableCell>
                      <Chip label={h.status} size="small" color={STATUS_COLOR[h.status] ?? 'default'} />
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                      {formatDuration(h.duration_ms)}
                    </TableCell>
                    <TableCell sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>
                      {formatRelativeTime(h.run_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Box>
    </Box>
  )
}
