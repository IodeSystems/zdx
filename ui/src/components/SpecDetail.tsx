import { useState } from 'react'
import { Link, useRouter } from '@tanstack/react-router'
import {
  Autocomplete,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon, Edit as EditIcon } from '@mui/icons-material'
import { useQueryClient } from '@tanstack/react-query'
import {
  useSpecDetail,
  useSpecTests,
  useSpecCodeRefs,
  useCreateIssue,
  linkSpecIssue,
  useListConcernsForSpec,
  useListConcerns,
  useLinkConcern,
  useUnlinkConcern,
  type ConcernItem,
  type SpecIssueItem,
  type SpecTestItem,
} from '../api'
import { EventStream } from './EventStream'
import { CodeRefs } from './CodeRefs'
import { SpecDemos } from './DemoPlayer'

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
  const { data: codeRefs } = useSpecCodeRefs(slug, specId)
  const { data: linkedConcerns = [] } = useListConcernsForSpec(specId)
  const { data: allConcerns = [] } = useListConcerns(slug)
  const linkConcern = useLinkConcern()
  const unlinkConcern = useUnlinkConcern()
  const [issueOpen, setIssueOpen] = useState(false)
  const [issueContext, setIssueContext] = useState('')
  const [concernsOpen, setConcernsOpen] = useState(false)
  const [pendingConcerns, setPendingConcerns] = useState<ConcernItem[]>([])
  const createIssue = useCreateIssue()

  const handleOpenConcerns = () => {
    setPendingConcerns(linkedConcerns)
    setConcernsOpen(true)
  }

  const handleSaveConcerns = async () => {
    const linked = new Set(linkedConcerns.map(c => c.name))
    const pending = new Set(pendingConcerns.map(c => c.name))
    const toAdd = pendingConcerns.filter(c => !linked.has(c.name))
    const toRemove = linkedConcerns.filter(c => !pending.has(c.name))
    await Promise.all([
      ...toAdd.map(c => linkConcern.mutateAsync({ slug, concern_name: c.name, spec_id: specId })),
      ...toRemove.map(c => unlinkConcern.mutateAsync({ slug, concern_name: c.name, spec_id: specId })),
    ])
    setConcernsOpen(false)
  }

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (!data) return <Typography color="error">Spec not found.</Typography>

  const { spec, issues } = data

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
        <Chip label={spec.importance} size="small" variant="outlined" color="info" />
      </Box>

      <Typography variant="body1" sx={{ mb: 2 }}>{spec.description}</Typography>

      <CodeRefs refs={codeRefs ?? []} slug={slug} />

      <Box sx={{ mb: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mb: 0.5 }}>
          <Typography variant="subtitle2" color="text.secondary">
            Concerns ({linkedConcerns.length})
          </Typography>
          <IconButton size="small" onClick={handleOpenConcerns} aria-label="edit concerns">
            <EditIcon fontSize="small" />
          </IconButton>
        </Box>
        {linkedConcerns.length === 0 ? (
          <Typography variant="body2" color="text.secondary">No linked concerns.</Typography>
        ) : (
          <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
            {linkedConcerns.map(c => (
              <Chip key={c.id} label={c.name} size="small" variant="outlined" />
            ))}
          </Box>
        )}
      </Box>

      <Box sx={{ mb: 2, display: 'flex', gap: 1 }}>
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
            <Box key={issue.issue_id} sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, py: 0.75, borderBottom: 1, borderColor: 'divider' }}>
              <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 0.75 }}>
                <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                  {issue.issue_id}
                </Typography>
                <Chip
                  label={issue.status}
                  size="small"
                  color={issue.status === 'closed' ? 'success' : issue.status === 'wip' ? 'warning' : 'default'}
                  variant="outlined"
                />
              </Box>
              <Link
                to="/project/$slug/issues/$id"
                params={{ slug, id: issue.issue_id }}
                style={{ textDecoration: 'none', color: 'inherit' }}
              >
                <Typography variant="body2" sx={{ fontWeight: 500, wordBreak: 'break-word', '&:hover': { textDecoration: 'underline' } }}>
                  {issue.title}
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

      <SpecDemos specId={specId} slug={slug} />

      <EventStream slug={slug} targetType="spec" targetId={String(specId)} />

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

      <Dialog open={concernsOpen} onClose={() => setConcernsOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Manage Concerns</DialogTitle>
        <DialogContent>
          <Autocomplete
            multiple
            options={allConcerns}
            getOptionLabel={c => c.name}
            value={pendingConcerns}
            onChange={(_, val) => setPendingConcerns(val)}
            isOptionEqualToValue={(a, b) => a.name === b.name}
            renderInput={params => <TextField {...params} label="Concerns" sx={{ mt: 1 }} />}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConcernsOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            disabled={linkConcern.isPending || unlinkConcern.isPending}
            onClick={handleSaveConcerns}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
