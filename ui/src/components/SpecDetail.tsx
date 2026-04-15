import { useState } from 'react'
import { Link, useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useQueryClient } from '@tanstack/react-query'
import {
  useSpecDetail,
  useSpecTests,
  useCreateIssue,
  deferSpec,
  undeferSpec,
  linkSpecIssue,
  type SpecIssueItem,
  type SpecTestItem,
} from '../api'
import { CommentsAndRevisions } from './CommentsAndRevisions'
import { DemosSection } from './DemoPlayer'

function TestStatusIcon({ status }: { status: string }) {
  const icon = status === 'pass' ? '\u2713' : status === 'fail' ? '\u2717' : '\u25CB'
  const color = status === 'pass' ? 'success.main' : status === 'fail' ? 'error.main' : 'text.secondary'
  return <Typography component="span" sx={{ color, fontWeight: 600, mr: 0.5, fontSize: '0.85rem' }}>{icon}</Typography>
}

export function SpecDetail({ slug, specId }: { slug: string; specId: number }) {
  const router = useRouter()
  const qc = useQueryClient()
  const { data, isLoading } = useSpecDetail(specId)
  const { data: tests } = useSpecTests(specId)
  const [deferOpen, setDeferOpen] = useState(false)
  const [deferReason, setDeferReason] = useState('')
  const [issueOpen, setIssueOpen] = useState(false)
  const [issueContext, setIssueContext] = useState('')
  const createIssue = useCreateIssue()

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (!data) return <Typography color="error">Spec not found.</Typography>

  const { spec, issues } = data

  const handleDefer = async () => {
    await deferSpec(specId, deferReason)
    setDeferOpen(false)
    setDeferReason('')
    qc.invalidateQueries({ queryKey: ['spec-detail', specId] })
  }

  const handleUndefer = async () => {
    await undeferSpec(specId)
    qc.invalidateQueries({ queryKey: ['spec-detail', specId] })
  }

  const handleCreateIssue = () => {
    setIssueContext(`Spec #${specId}: ${spec.description}\n\n`)
    setIssueOpen(true)
  }

  const handleSubmitIssue = () => {
    createIssue.mutate(
      { slug, context: issueContext.trim() || undefined, auto_ready: true },
      {
        onSuccess: async (issue) => {
          await linkSpecIssue(specId, `IS-${issue.id}`)
          setIssueOpen(false)
          setIssueContext('')
          qc.invalidateQueries({ queryKey: ['spec-detail', specId] })
        },
      },
    )
  }

  return (
    <Box>
      <Button
        startIcon={<ArrowBackIcon />}
        size="small"
        sx={{ mb: 2 }}
        onClick={() => router.history.go(-1)}
      >
        Back
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>Spec #{specId}</Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center' }}>
        <Chip label={spec.kind} size="small" variant="outlined" color="info" />
        {spec.deferred && <Chip label="deferred" size="small" color="warning" />}
      </Box>

      <Typography variant="body1" sx={{ mb: 2 }}>{spec.description}</Typography>

      {spec.deferred && spec.deferred_reason && (
        <Box sx={{ mb: 2, p: 1.5, bgcolor: 'warning.50', borderRadius: 1, border: 1, borderColor: 'warning.200' }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Deferral reason
          </Typography>
          <Typography variant="body2">{spec.deferred_reason}</Typography>
        </Box>
      )}

      <Box sx={{ mb: 2, display: 'flex', gap: 1 }}>
        {spec.deferred ? (
          <Button size="small" variant="outlined" onClick={handleUndefer}>
            Remove deferral
          </Button>
        ) : (
          <Button size="small" variant="outlined" color="warning" onClick={() => setDeferOpen(true)}>
            Defer spec
          </Button>
        )}
        <Button size="small" variant="outlined" onClick={handleCreateIssue}>
          File issue against spec
        </Button>
      </Box>

      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1, mt: 3 }}>
        Linked issues ({issues.length})
      </Typography>
      {issues.length === 0 ? (
        <Typography variant="body2" color="text.secondary">No linked issues.</Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
          {issues.map((issue: SpecIssueItem) => (
            <Box key={issue.issue_id} sx={{ display: 'flex', alignItems: 'center', gap: 1, py: 0.5, borderBottom: 1, borderColor: 'divider' }}>
              <Chip
                label={issue.status}
                size="small"
                color={issue.status === 'closed' ? 'success' : issue.status === 'wip' ? 'warning' : 'default'}
                variant="outlined"
              />
              <Link
                to="/project/$slug/issues/$id"
                params={{ slug, id: issue.issue_id }}
                style={{ textDecoration: 'none', color: 'inherit' }}
              >
                <Typography variant="body2" sx={{ '&:hover': { textDecoration: 'underline' } }}>
                  {issue.issue_id}: {issue.title}
                </Typography>
              </Link>
            </Box>
          ))}
        </Box>
      )}

      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1, mt: 3 }}>
        Linked tests ({tests?.length ?? 0})
      </Typography>
      {(!tests || tests.length === 0) ? (
        <Typography variant="body2" color="text.secondary">No linked tests.</Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
          {tests.map((t: SpecTestItem) => (
            <Box key={t.id} sx={{ display: 'flex', alignItems: 'center', gap: 0.5, py: 0.25 }}>
              <TestStatusIcon status={t.status} />
              <Chip label={t.layer} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.7rem' }} />
              <Link
                to="/project/$slug/tests"
                params={{ slug }}
                style={{ textDecoration: 'none', color: 'inherit' }}
              >
                <Typography variant="body2" sx={{ fontSize: '0.85rem', '&:hover': { textDecoration: 'underline' } }}>
                  {t.component}/{t.name}
                </Typography>
              </Link>
            </Box>
          ))}
        </Box>
      )}

      <Box sx={{ mt: 3 }}>
        <DemosSection />
      </Box>

      <CommentsAndRevisions slug={slug} targetType="spec" targetId={String(specId)} />

      <Dialog open={deferOpen} onClose={() => setDeferOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Defer spec</DialogTitle>
        <DialogContent>
          <Typography variant="body2" sx={{ mb: 2 }}>Why is this spec being deferred?</Typography>
          <TextField
            autoFocus
            fullWidth
            multiline
            rows={3}
            value={deferReason}
            onChange={e => setDeferReason(e.target.value)}
            placeholder="e.g. Blocked on external API availability"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeferOpen(false)}>Cancel</Button>
          <Button onClick={handleDefer} variant="contained" color="warning">Defer</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={issueOpen} onClose={() => setIssueOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>File issue against spec</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            multiline
            rows={5}
            sx={{ mt: 1 }}
            value={issueContext}
            onChange={e => setIssueContext(e.target.value)}
            placeholder="Describe what needs to be done for this spec..."
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setIssueOpen(false)}>Cancel</Button>
          <Button
            onClick={handleSubmitIssue}
            variant="contained"
            disabled={createIssue.isPending}
          >
            Create issue
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
