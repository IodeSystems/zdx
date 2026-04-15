import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { Box, Button, Card, CardContent, Chip, Typography } from '@mui/material'
import { ArrowBack as ArrowBackIcon, BugReport as BugReportIcon } from '@mui/icons-material'
import { useError, useCreateIssue } from '../../../../api'
import { fmtDate } from '../../../../utils/date'

function ErrorDetailRoute() {
  const { slug, id } = Route.useParams()
  const navigate = useNavigate()
  const { data: error, isLoading } = useError(id)
  const createIssue = useCreateIssue()

  const handleCreateIssue = () => {
    if (!error) return
    const context = [
      error.error_name && `**Error:** ${error.error_name}`,
      error.source && `**Source:** ${error.source}`,
      error.endpoint && `**Endpoint:** ${error.endpoint}`,
      error.stack_trace && `**Stack trace:**\n\`\`\`\n${error.stack_trace}\n\`\`\``,
    ].filter(Boolean).join('\n\n')
    createIssue.mutate({
      slug,
      context,
      source: 'error-report',
    })
  }

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (!error) return <Typography color="text.secondary">Error not found.</Typography>

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
        <Button size="small" startIcon={<ArrowBackIcon />} onClick={() => navigate({ to: '/project/$slug/errors', params: { slug } })}>
          Errors
        </Button>
        <Button
          size="small"
          variant="contained"
          startIcon={<BugReportIcon />}
          onClick={handleCreateIssue}
          disabled={createIssue.isPending}
        >
          {createIssue.isSuccess ? `Created ${(createIssue.data as any)?.id ?? 'issue'}` : 'Create Issue'}
        </Button>
      </Box>

      <Card variant="outlined">
        <CardContent>
          <Typography variant="h6" sx={{ fontFamily: 'monospace', mb: 2 }}>
            {error.error_name || '(unnamed error)'}
          </Typography>

          <Box sx={{ display: 'flex', gap: 1, mb: 2, flexWrap: 'wrap' }}>
            <Chip label={error.source} size="small" color="default" />
            {error.endpoint && <Chip label={error.endpoint} size="small" variant="outlined" />}
            <Chip label={fmtDate(error.created_at)} size="small" variant="outlined" />
          </Box>

          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>ID</Typography>
          <Typography variant="body2" sx={{ mb: 2, fontFamily: 'monospace' }}>{error.id}</Typography>

          {error.stack_trace && (
            <>
              <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>Stack Trace</Typography>
              <Box
                component="pre"
                sx={{
                  p: 1.5,
                  bgcolor: 'action.hover',
                  borderRadius: 1,
                  fontSize: '0.75rem',
                  overflowX: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {error.stack_trace}
              </Box>
            </>
          )}
        </CardContent>
      </Card>
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/errors/$id')({
  component: ErrorDetailRoute,
})
