import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Box,
  Chip,
  CircularProgress,
  Divider,
  List,
  ListItem,
  ListItemText,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useDemo } from '../../../../api'
import { CLIDemoPlayerByUrl, VideoDemoPlayerByUrl } from '../../../../components/DemoPlayer'

function statusColor(status: string): 'success' | 'error' | 'default' {
  if (status === 'pass') return 'success'
  if (status === 'fail') return 'error'
  return 'default'
}

function DemoDetailRoute() {
  const { slug, demoId } = Route.useParams()
  const { data: demo, isLoading } = useDemo(Number(demoId))

  if (isLoading) return <CircularProgress size={24} />
  if (!demo) return <Typography color="error">Demo not found.</Typography>

  const label = demo.test_component && demo.test_name
    ? `${demo.test_component}/${demo.test_name}`
    : demo.name

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
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/demos/$demoId')({
  component: DemoDetailRoute,
})
