import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Typography,
} from '@mui/material'
import { useNavigate } from '@tanstack/react-router'
import { useErrors, useClearErrors, useReportError, type ErrorReportItem } from '../api'
import { fmtDate } from '../utils/date'

function ErrorRow({ e, slug }: { e: ErrorReportItem; slug: string }) {
  const navigate = useNavigate()
  return (
    <Card
      variant="outlined"
      sx={{ mb: 0.5, cursor: 'pointer', '&:hover': { bgcolor: 'action.hover' } }}
      onClick={() => navigate({ to: '/project/$slug/errors/$id', params: { slug, id: String(e.id) } })}
    >
      <CardContent sx={{ py: 1, '&:last-child': { pb: 1 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="caption" color="text.secondary" sx={{ minWidth: 72 }}>
            {fmtDate(e.created_at)}
          </Typography>
          <Typography variant="body2" sx={{ fontWeight: 500, flex: 1, fontFamily: 'monospace' }}>
            {e.error_name || '(unnamed)'}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {e.source}
          </Typography>
          {e.endpoint && (
            <Chip label={e.endpoint} size="small" variant="outlined" sx={{ maxWidth: 180 }} />
          )}
        </Box>
      </CardContent>
    </Card>
  )
}

export function ErrorsTab({ slug }: { slug: string }) {
  const { data: errData, isLoading: errLoading } = useErrors(slug)
  const errors = errData?.errors ?? []
  const clearErrors = useClearErrors(slug)
  const reportError = useReportError(slug)

  return (
    <Box>
      <Box sx={{ display: 'flex', gap: 1, mb: 1 }}>
        <Button size="small" variant="outlined" color="warning"
          onClick={() => reportError.mutate({ source: 'server', errorName: 'Test server error (simulated)' })}>
          Simulate server error
        </Button>
        <Button size="small" variant="outlined" color="warning"
          onClick={() => reportError.mutate({ source: 'client', errorName: 'Test client error (simulated)' })}>
          Simulate client error
        </Button>
        {errors.length > 0 && (
          <Button size="small" variant="outlined" color="error"
            onClick={() => clearErrors.mutate()}>
            Clear errors
          </Button>
        )}
      </Box>
      {errLoading && !errors.length && (
        <Typography color="text.secondary">Loading...</Typography>
      )}
      {!errLoading && errors.length === 0 && (
        <Typography color="text.secondary">No error reports.</Typography>
      )}
      {errors.map(e => <ErrorRow key={e.id} e={e} slug={slug} />)}
    </Box>
  )
}
