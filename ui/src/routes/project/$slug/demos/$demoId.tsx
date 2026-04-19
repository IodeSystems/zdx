import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  List,
  ListItem,
  ListItemText,
  Snackbar,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon, Replay as ReplayIcon } from '@mui/icons-material'
import { useState } from 'react'
import {
  useDemo,
  useDemoConsoleLog,
  useTestCodeRefs,
  useCreateIssue,
} from '../../../../api'
import { CLIDemoPlayerByUrl, VideoDemoPlayerByUrl } from '../../../../components/DemoPlayer'

function statusColor(status: string): 'success' | 'error' | 'default' {
  if (status === 'pass') return 'success'
  if (status === 'fail') return 'error'
  return 'default'
}

function DemoDetailRoute() {
  const { slug, demoId } = Route.useParams()
  const { data: demo, isLoading } = useDemo(Number(demoId))
  const { data: consoleLogs } = useDemoConsoleLog(Number(demoId))
  const { data: codeRefs } = useTestCodeRefs(slug, demo?.test_id ?? 0)
  const createIssue = useCreateIssue()
  const [toast, setToast] = useState<string | null>(null)

  if (isLoading) return <CircularProgress size={24} />
  if (!demo) return <Typography color="error">Demo not found.</Typography>

  const label = demo.test_component && demo.test_name
    ? `${demo.test_component}/${demo.test_name}`
    : demo.name

  const handleReRecord = () => {
    const branchInfo = demo.recorded_branch
      ? ` (recorded on ${demo.recorded_branch}${demo.recorded_sha ? `@${demo.recorded_sha.slice(0, 7)}` : ''})`
      : ''
    createIssue.mutate(
      {
        slug,
        title: `Re-record demo: ${label}${branchInfo}`,
        context: `Request to re-record the demo for test "${demo.test_name}" in component "${demo.test_component}".${demo.recorded_branch ? `\n\nOriginally recorded on branch: ${demo.recorded_branch}${demo.recorded_sha ? `\nGit SHA: ${demo.recorded_sha}` : ''}` : ''}`,
        issue_type: 'ops',
        auto_ready: true,
      },
      {
        onSuccess: (issue) => setToast(`Issue ${issue.id} created`),
        onError: () => setToast('Failed to create re-record issue'),
      },
    )
  }

  return (
    <Box>
      <Box sx={{ mb: 2 }}>
        <Link to="/project/$slug/demos" params={{ slug }} style={{ display: 'inline-flex', alignItems: 'center', gap: 4, textDecoration: 'none', color: 'inherit' }}>
          <ArrowBackIcon fontSize="small" />
          <Typography variant="body2">Demos</Typography>
        </Link>
      </Box>

      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: 1, flexWrap: 'wrap' }}>
        <Typography variant="h6">{label}</Typography>
        <Chip label={demo.test_status || 'unknown'} size="small" color={statusColor(demo.test_status)} />
        <Chip label={demo.type} size="small" variant="outlined" color={demo.type === 'cli' ? 'info' : 'secondary'} />
        {demo.test_duration_ms > 0 && (
          <Typography variant="caption" color="text.secondary">
            {demo.test_duration_ms < 1000
              ? `${demo.test_duration_ms}ms`
              : `${(demo.test_duration_ms / 1000).toFixed(1)}s`}
          </Typography>
        )}
        {demo.recorded_branch && (
          <Chip
            label={`${demo.recorded_branch}${demo.recorded_sha ? `@${demo.recorded_sha.slice(0, 7)}` : ''}`}
            size="small"
            variant="outlined"
            color="default"
          />
        )}
      </Box>

      <Box sx={{ mb: 3 }}>
        {demo.type === 'cli'
          ? <CLIDemoPlayerByUrl url={demo.url} />
          : <VideoDemoPlayerByUrl url={demo.url} />}
      </Box>

      {demo.specs.length > 0 && (
        <>
          <Divider sx={{ mb: 2 }} />
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Covered specs ({demo.specs.length})
          </Typography>
          <List dense disablePadding>
            {demo.specs.map((s) => (
              <ListItem key={s.id} disableGutters>
                <ListItemText
                  primary={s.description}
                  secondary={s.kind}
                />
              </ListItem>
            ))}
          </List>
        </>
      )}

      {codeRefs && codeRefs.length > 0 && (
        <>
          <Divider sx={{ mb: 2, mt: 2 }} />
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Code refs ({codeRefs.length})
          </Typography>
          <List dense disablePadding>
            {codeRefs.map((ref) => (
              <ListItem key={ref.id} disableGutters>
                <ListItemText
                  primary={
                    ref.line_start
                      ? `${ref.file_path}:${ref.line_start}${ref.line_end && ref.line_end !== ref.line_start ? `-${ref.line_end}` : ''}`
                      : ref.file_path
                  }
                  secondary={ref.note || undefined}
                  slotProps={{ primary: { sx: { fontFamily: 'monospace', fontSize: '0.8rem' } } }}
                />
              </ListItem>
            ))}
          </List>
        </>
      )}

      {consoleLogs && (
        <>
          <Divider sx={{ mb: 2, mt: 2 }} />
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Console logs
          </Typography>
          <Box
            component="pre"
            sx={{
              fontFamily: 'monospace',
              fontSize: '0.75rem',
              bgcolor: 'background.paper',
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              p: 1.5,
              overflow: 'auto',
              maxHeight: 300,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {consoleLogs}
          </Box>
        </>
      )}

      <Box sx={{ mt: 3 }}>
        <Button
          size="small"
          variant="outlined"
          startIcon={<ReplayIcon />}
          onClick={handleReRecord}
          disabled={createIssue.isPending}
        >
          Request re-record
        </Button>
      </Box>

      <Snackbar
        open={!!toast}
        autoHideDuration={3000}
        onClose={() => setToast(null)}
        message={toast}
      />
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/demos/$demoId')({
  component: DemoDetailRoute,
})
