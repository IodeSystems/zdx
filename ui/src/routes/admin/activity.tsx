import { createFileRoute, Link } from '@tanstack/react-router'
import {
  Box,
  Chip,
  CircularProgress,
  Divider,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material'
import { useActivityWorklog, useActivitySessions } from '../../api'

function lifecycleColor(lifecycle: string): 'default' | 'success' | 'warning' {
  if (lifecycle === 'done') return 'success'
  if (lifecycle === 'active') return 'warning'
  return 'default'
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return `${Math.floor(hrs / 24)}d ago`
}

function ActivityDashboard() {
  const { data: worklogData, isLoading: loadingWorklog } = useActivityWorklog(50)
  const { data: sessionsData, isLoading: loadingSessions } = useActivitySessions(50)

  return (
    <Box sx={{ maxWidth: 1100 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 3 }}>
        Activity
      </Typography>

      <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>
        Agent Sessions
      </Typography>
      {loadingSessions ? (
        <CircularProgress sx={{ mb: 3 }} />
      ) : (
        <TableContainer component={Paper} variant="outlined" sx={{ mb: 4 }}>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Project</TableCell>
                <TableCell>Session</TableCell>
                <TableCell>Issue</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Events</TableCell>
                <TableCell align="right">Updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(sessionsData?.sessions ?? []).length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ color: 'text.secondary', py: 3 }}>
                    No sessions yet.
                  </TableCell>
                </TableRow>
              )}
              {(sessionsData?.sessions ?? []).map(s => (
                <TableRow key={s.id} hover>
                  <TableCell>
                    <Link to="/project/$slug/agents" params={{ slug: s.project_slug }} style={{ textDecoration: 'none', color: 'inherit' }}>
                      {s.project_name}
                    </Link>
                  </TableCell>
                  <TableCell sx={{ maxWidth: 300 }}>
                    <Link to="/project/$slug/agents/$sessionId" params={{ slug: s.project_slug, sessionId: String(s.id) }} style={{ textDecoration: 'none', color: 'inherit' }}>
                      <Typography variant="body2" noWrap title={s.title || s.session_id}>
                        {s.title || s.session_id}
                      </Typography>
                      {s.header && (
                        <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }} title={s.header}>
                          {s.header}
                        </Typography>
                      )}
                    </Link>
                  </TableCell>
                  <TableCell>
                    {s.issue_id && (
                      <Link to="/project/$slug/issues" params={{ slug: s.project_slug }} style={{ textDecoration: 'none', color: 'inherit' }}>
                        <Typography variant="body2">{s.issue_id}</Typography>
                      </Link>
                    )}
                  </TableCell>
                  <TableCell>
                    <Chip label={s.lifecycle} size="small" color={lifecycleColor(s.lifecycle)} variant="outlined" />
                  </TableCell>
                  <TableCell align="right">
                    <Typography variant="body2">{s.event_count}</Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Typography variant="body2" color="text.secondary" noWrap>
                      {relativeTime(s.updated_at)}
                    </Typography>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      <Divider sx={{ mb: 3 }} />

      <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>
        Work Log
      </Typography>
      {loadingWorklog ? (
        <CircularProgress />
      ) : (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Project</TableCell>
                <TableCell>Issue</TableCell>
                <TableCell>Agent</TableCell>
                <TableCell>Note</TableCell>
                <TableCell align="right">Time</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(worklogData?.entries ?? []).length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ color: 'text.secondary', py: 3 }}>
                    No work log entries yet.
                  </TableCell>
                </TableRow>
              )}
              {(worklogData?.entries ?? []).map((e, idx) => (
                <TableRow key={idx} hover>
                  <TableCell>
                    <Link to="/project/$slug/worklog" params={{ slug: e.project_slug }} style={{ textDecoration: 'none', color: 'inherit' }}>
                      {e.project_name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Link to="/project/$slug/issues" params={{ slug: e.project_slug }} style={{ textDecoration: 'none', color: 'inherit' }}>
                      <Typography variant="body2">{e.issue_id}</Typography>
                      <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>
                        {e.issue_title}
                      </Typography>
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2">{e.agent}</Typography>
                  </TableCell>
                  <TableCell sx={{ maxWidth: 400 }}>
                    <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                      {e.note}
                    </Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Typography variant="body2" color="text.secondary" noWrap>
                      {relativeTime(e.created_at)}
                    </Typography>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Box>
  )
}

export const Route = createFileRoute('/admin/activity')({
  component: ActivityDashboard,
})
