import { useState } from 'react'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Link,
  TextField,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon, BugReport as BugReportIcon } from '@mui/icons-material'
import { useError, useCreateIssue, useSimilarIssues, type SimilarIssueItem, type IssueItem } from '../../../../api'
import { fmtDate } from '../../../../utils/date'

function ErrorDetailRoute() {
  const { slug, id } = Route.useParams()
  const navigate = useNavigate()
  const { data: error, isLoading, refetch } = useError(id, slug)
  const createIssue = useCreateIssue()
  const findSimilar = useSimilarIssues()

  const [dialogOpen, setDialogOpen] = useState(false)
  const [extraContext, setExtraContext] = useState('')
  const [similarIssues, setSimilarIssues] = useState<SimilarIssueItem[] | null>(null)
  const [preflight, setPreflight] = useState(false)
  const [createdIssue, setCreatedIssue] = useState<IssueItem | null>(null)

  const buildContext = () => {
    if (!error) return ''
    const parts = [
      error.error_name && `**Error:** ${error.error_name}`,
      error.source && `**Source:** ${error.source}`,
      error.endpoint && `**Endpoint:** ${error.endpoint}`,
      error.stack_trace && `**Stack trace:**\n\`\`\`\n${error.stack_trace}\n\`\`\``,
    ].filter(Boolean).join('\n\n')
    const extra = extraContext.trim()
    return extra ? `${parts}\n\n**Additional context:**\n${extra}` : parts
  }

  const handleOpen = () => {
    setSimilarIssues(null)
    setPreflight(false)
    setCreatedIssue(null)
    setExtraContext('')
    setDialogOpen(true)
  }

  const handleClose = () => {
    setDialogOpen(false)
    setExtraContext('')
    setSimilarIssues(null)
    setPreflight(false)
    setCreatedIssue(null)
  }

  const handlePreflight = () => {
    const text = buildContext()
    if (!text) { doSubmit(); return }
    setPreflight(true)
    findSimilar.mutate(
      { slug, text, n: 5 },
      {
        onSuccess: (issues) => {
          setPreflight(false)
          if (issues.length === 0) { doSubmit() } else { setSimilarIssues(issues) }
        },
        onError: () => { setPreflight(false); doSubmit() },
      },
    )
  }

  const doSubmit = () => {
    createIssue.mutate(
      {
        slug,
        context: buildContext() || undefined,
        source: 'error-report',
        auto_ready: true,
        url: window.location.href,
        source_error_id: error ? Number(error.id) : undefined,
      },
      {
        onSuccess: (issue) => {
          setCreatedIssue(issue)
          setSimilarIssues(null)
          refetch()
        },
      },
    )
  }

  const showSimilar = similarIssues !== null && !preflight
  const isPending = createIssue.isPending || preflight
  const linkedIssueId = (error as any)?.linked_issue_id

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (!error) return <Typography color="text.secondary">Error not found.</Typography>

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
        <Button size="small" startIcon={<ArrowBackIcon />} onClick={() => navigate({ to: '/project/$slug/errors', params: { slug } })}>
          Errors
        </Button>
        {linkedIssueId ? (
          <Button
            size="small"
            variant="outlined"
            startIcon={<BugReportIcon />}
            onClick={() => navigate({ to: '/project/$slug/issues/$id', params: { slug, id: linkedIssueId } })}
          >
            Linked: {linkedIssueId}
          </Button>
        ) : (
          <Button
            size="small"
            variant="contained"
            startIcon={<BugReportIcon />}
            onClick={handleOpen}
          >
            Create Issue
          </Button>
        )}
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

      <Dialog open={dialogOpen} onClose={handleClose} maxWidth="sm" fullWidth>
        {createdIssue ? (
          <>
            <DialogTitle>Issue Created</DialogTitle>
            <DialogContent>
              <Alert severity="success" sx={{ mb: 2 }}>Issue created from error report.</Alert>
              <Link href={`/project/${slug}/issues/IS-${createdIssue.id}`} variant="body1">
                IS-{createdIssue.id}: {createdIssue.title || '(untitled)'}
              </Link>
            </DialogContent>
            <DialogActions>
              <Button variant="contained" onClick={handleClose}>Done</Button>
            </DialogActions>
          </>
        ) : (
          <>
            <DialogTitle>Create Issue from Error</DialogTitle>
            <DialogContent>
              <Box sx={{ mb: 2, mt: 1, p: 1.5, bgcolor: 'action.hover', borderRadius: 1 }}>
                <Typography variant="caption" color="text.secondary">Pre-filled from error:</Typography>
                <Typography variant="body2" sx={{ fontFamily: 'monospace', mt: 0.5 }}>
                  {error.error_name || '(unnamed)'}
                  {error.endpoint && ` — ${error.endpoint}`}
                </Typography>
              </Box>
              <TextField
                autoFocus
                fullWidth
                label="Additional context"
                multiline
                rows={3}
                value={extraContext}
                onChange={e => setExtraContext(e.target.value)}
                placeholder="Reproduction steps, expected vs actual behavior, related context..."
                disabled={showSimilar}
              />

              {preflight && (
                <Box sx={{ mt: 2, display: 'flex', alignItems: 'center', gap: 1 }}>
                  <CircularProgress size={16} />
                  <Typography variant="caption" color="text.secondary">Checking for similar issues...</Typography>
                </Box>
              )}

              {showSimilar && (
                <Box sx={{ mt: 2 }}>
                  <Divider sx={{ mb: 1.5 }} />
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
                    Similar existing issues — is yours already reported?
                  </Typography>
                  <Box sx={{ maxHeight: 250, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 1 }}>
                    {similarIssues!.map(iss => (
                      <Card key={iss.id} variant="outlined" sx={{ flexShrink: 0 }}>
                        <CardContent sx={{ py: 1, px: 1.5, '&:last-child': { pb: 1 } }}>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                            <Link href={`/project/${slug}/issues/${iss.id}`} target="_blank" rel="noopener" variant="body2" sx={{ fontWeight: 600, whiteSpace: 'nowrap' }}>
                              {iss.id}
                            </Link>
                            <Typography variant="body2" sx={{ fontWeight: 500, flex: 1, minWidth: 0 }} noWrap>
                              {iss.title || '(untitled)'}
                            </Typography>
                            <Chip label={iss.status || 'open'} size="small" variant="outlined" sx={{ height: 20, fontSize: '0.7rem' }} />
                            <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: 'nowrap' }}>
                              {(iss.score * 100).toFixed(0)}%
                            </Typography>
                          </Box>
                          {iss.context && (
                            <Typography variant="body2" color="text.secondary" sx={{
                              overflow: 'hidden', display: '-webkit-box',
                              WebkitLineClamp: 3, WebkitBoxOrient: 'vertical',
                            }}>
                              {iss.context}
                            </Typography>
                          )}
                        </CardContent>
                      </Card>
                    ))}
                  </Box>
                </Box>
              )}
            </DialogContent>
            <DialogActions>
              <Button onClick={handleClose}>Cancel</Button>
              {!showSimilar ? (
                <Button variant="contained" disabled={isPending} onClick={handlePreflight}>
                  {preflight ? <CircularProgress size={16} color="inherit" /> : 'Submit'}
                </Button>
              ) : (
                <>
                  <Button onClick={() => setSimilarIssues(null)}>Edit</Button>
                  <Button variant="contained" disabled={createIssue.isPending} onClick={doSubmit}>
                    Submit anyway
                  </Button>
                </>
              )}
            </DialogActions>
          </>
        )}
      </Dialog>
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/errors/$id')({
  component: ErrorDetailRoute,
})
