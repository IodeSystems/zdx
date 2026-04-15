import { useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Collapse,
  IconButton,
  Typography,
} from '@mui/material'
import { ExpandMore as ExpandMoreIcon } from '@mui/icons-material'
import { useErrors, useClearErrors, useReportError, type ErrorReportItem } from '../api'
import { fmtDate } from '../utils/date'

function ErrorRow({ e }: { e: ErrorReportItem }) {
  const [open, setOpen] = useState(false)
  return (
    <Card variant="outlined" sx={{ mb: 0.5 }}>
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
          {e.stack_trace && (
            <IconButton
              size="small"
              onClick={() => setOpen(o => !o)}
              sx={{ transform: open ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }}
            >
              <ExpandMoreIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
        {e.stack_trace && (
          <Collapse in={open}>
            <Box
              component="pre"
              sx={{
                mt: 1,
                p: 1,
                bgcolor: 'action.hover',
                borderRadius: 1,
                fontSize: '0.72rem',
                overflowX: 'auto',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
              }}
            >
              {e.stack_trace}
            </Box>
          </Collapse>
        )}
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
      {errors.map(e => <ErrorRow key={e.id} e={e} />)}
    </Box>
  )
}
