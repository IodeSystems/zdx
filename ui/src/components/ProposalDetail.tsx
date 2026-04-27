import { useState, useEffect } from 'react'
import { Link, useRouter } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  ArrowBack as ArrowBackIcon,
  Edit as EditIcon,
  ExpandLess as ExpandLessIcon,
  ExpandMore as ExpandMoreIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material'
import {
  useProposal,
  useUpdateProposal,
  useApproveProposal,
  useRejectProposal,
  useSnoozeProposal,
  useReevaluateProposal,
  type ReevaluateProposalResponse,
} from '../api'
import { CommentsAndRevisions } from './CommentsAndRevisions'
import { MarkdownContent } from './MarkdownContent'

const STATUS_COLORS: Record<string, 'warning' | 'success' | 'default' | 'info'> = {
  proposed: 'warning',
  approved: 'success',
  rejected: 'default',
  snoozed: 'info',
}

function isoPlusDays(days: number): string {
  return new Date(Date.now() + days * 24 * 60 * 60 * 1000).toISOString()
}

function isoToLocalInput(iso: string): string {
  // YYYY-MM-DDTHH:mm in local time for <input type="datetime-local">
  const d = new Date(iso)
  const tz = d.getTimezoneOffset()
  const local = new Date(d.getTime() - tz * 60000)
  return local.toISOString().slice(0, 16)
}

function localInputToIso(v: string): string {
  return new Date(v).toISOString()
}

export function ProposalDetail({
  slug,
  proposalId,
}: {
  slug: string
  proposalId: number
}) {
  const { data, isLoading } = useProposal(slug, proposalId)
  const updateProposal = useUpdateProposal()
  const approveProposal = useApproveProposal()
  const rejectProposal = useRejectProposal()
  const snoozeProposal = useSnoozeProposal()
  const reevaluateProposal = useReevaluateProposal()
  const rejectDuplicate = useRejectProposal()
  const router = useRouter()

  const proposal = data?.proposal
  const versions = data?.versions ?? []

  const [editing, setEditing] = useState(false)
  const [editTitle, setEditTitle] = useState('')
  const [editValue, setEditValue] = useState('')
  const [editBody, setEditBody] = useState('')
  const [previewBody, setPreviewBody] = useState(false)

  const [approveOpen, setApproveOpen] = useState(false)
  const [approveType, setApproveType] = useState('impl')
  const [approvePriority, setApprovePriority] = useState('2')
  const [approvedIssueId, setApprovedIssueId] = useState<string | null>(null)

  const [rejectOpen, setRejectOpen] = useState(false)
  const [rejectReason, setRejectReason] = useState('')

  const [snoozeOpen, setSnoozeOpen] = useState(false)
  const [snoozeUntil, setSnoozeUntil] = useState(() => isoToLocalInput(isoPlusDays(7)))

  const [versionsOpen, setVersionsOpen] = useState(false)

  const [reevaluateOpen, setReevaluateOpen] = useState(false)
  const [reevaluateData, setReevaluateData] = useState<ReevaluateProposalResponse | null>(null)

  useEffect(() => {
    if (!proposal) return
    const headline = proposal.title || (proposal.body ? proposal.body.slice(0, 60) : '(no title)')
    document.title = `Proposal #${proposal.id}: ${headline} | zdx`
    return () => { document.title = 'zdx' }
  }, [proposal?.id, proposal?.title, proposal?.body])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  if (!proposal) {
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
        <Typography color="error">Proposal #{proposalId} not found.</Typography>
      </Box>
    )
  }

  const canEdit = proposal.status === 'proposed' || proposal.status === 'snoozed'

  function startEdit() {
    setEditTitle(proposal!.title ?? '')
    setEditValue(proposal!.value ?? '')
    setEditBody(proposal!.body ?? '')
    setPreviewBody(false)
    setEditing(true)
  }

  function saveEdit() {
    if (!editTitle.trim()) return
    updateProposal.mutate(
      { slug, id: proposal!.id, title: editTitle.trim(), value: editValue, body: editBody },
      { onSuccess: () => setEditing(false) },
    )
  }

  function doApprove() {
    approveProposal.mutate(
      { slug, id: proposal!.id, issue_type: approveType, priority: Number(approvePriority) },
      {
        onSuccess: (res) => {
          setApprovedIssueId(res.issue_id)
          setApproveOpen(false)
        },
      },
    )
  }

  function doReject() {
    rejectProposal.mutate(
      { slug, id: proposal!.id, reason: rejectReason.trim() || undefined },
      { onSuccess: () => setRejectOpen(false) },
    )
  }

  function doSnooze() {
    if (!snoozeUntil) return
    snoozeProposal.mutate(
      { slug, id: proposal!.id, snoozed_until: localInputToIso(snoozeUntil) },
      { onSuccess: () => setSnoozeOpen(false) },
    )
  }

  function doReevaluate() {
    setReevaluateData(null)
    setReevaluateOpen(true)
    reevaluateProposal.mutate(
      { slug, id: proposal!.id },
      { onSuccess: (res) => setReevaluateData(res) },
    )
  }

  function rejectAsDuplicate(duplicateId: number) {
    rejectDuplicate.mutate(
      { slug, id: duplicateId, reason: `Duplicate of PR-${proposal!.id}: ${proposal!.title}` },
      {
        onSuccess: () => {
          setReevaluateData(prev => prev ? {
            ...prev,
            duplicates: (prev.duplicates ?? []).filter(d => d.id !== duplicateId),
          } : null)
        },
      },
    )
  }

  function rejectAsAddressed(issueId: string, issueTitle: string) {
    rejectProposal.mutate(
      { slug, id: proposal!.id, reason: `Already addressed by ${issueId}: ${issueTitle}` },
      { onSuccess: () => setReevaluateOpen(false) },
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

      {editing ? (
        <TextField
          fullWidth
          size="small"
          autoFocus
          value={editTitle}
          onChange={e => setEditTitle(e.target.value)}
          placeholder="Title"
          sx={{ mb: 1 }}
          slotProps={{ input: { style: { fontSize: '1.5rem', fontWeight: 400 } } }}
        />
      ) : (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mb: 1 }}>
          <Typography variant="h5">
            Proposal #{proposal.id}: {proposal.title || '(no title)'}
          </Typography>
          {canEdit && (
            <Tooltip title="Edit proposal">
              <IconButton size="small" onClick={startEdit}>
                <EditIcon fontSize="inherit" />
              </IconButton>
            </Tooltip>
          )}
        </Box>
      )}

      <Box sx={{ display: 'flex', gap: 1, mb: 1, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip
          label={proposal.status}
          size="small"
          color={STATUS_COLORS[proposal.status] || 'default'}
          variant="outlined"
        />
        {proposal.source_type && (
          <Chip label={`source: ${proposal.source_type}${proposal.source_ref ? ` (${proposal.source_ref})` : ''}`} size="small" variant="outlined" />
        )}
        {proposal.snoozed_until && (
          <Chip label={`snoozed until ${new Date(proposal.snoozed_until).toLocaleString()}`} size="small" color="info" variant="outlined" />
        )}
        {proposal.approved_issue_id && (
          <Chip
            label={`approved as: ${proposal.approved_issue_id}`}
            size="small"
            color="success"
            variant="outlined"
            component={Link as any}
            to="/project/$slug/issues/$id"
            params={{ slug, id: proposal.approved_issue_id }}
            clickable
            sx={{ textDecoration: 'none' }}
          />
        )}
      </Box>

      {(proposal.status === 'proposed' || proposal.status === 'snoozed') && (
        <Stack direction="row" spacing={1} sx={{ mb: 2, flexWrap: 'wrap' }}>
          <Button size="small" variant="contained" color="success" onClick={() => setApproveOpen(true)}>
            {proposal.status === 'snoozed' ? 'Approve now' : 'Approve'}
          </Button>
          <Button size="small" variant="outlined" color="warning" onClick={() => { setRejectReason(''); setRejectOpen(true) }}>
            Reject
          </Button>
          {proposal.status === 'proposed' && (
            <Button size="small" variant="outlined" onClick={() => setSnoozeOpen(true)}>
              Snooze
            </Button>
          )}
          <Tooltip title="Find duplicates and check if already addressed">
            <Button size="small" variant="outlined" startIcon={<RefreshIcon />} onClick={doReevaluate}>
              Re-evaluate
            </Button>
          </Tooltip>
        </Stack>
      )}

      {approvedIssueId && (
        <Box sx={{ mb: 2 }}>
          <Chip
            label={`Created issue: ${approvedIssueId}`}
            color="success"
            component={Link as any}
            to="/project/$slug/issues/$id"
            params={{ slug, id: approvedIssueId }}
            clickable
            sx={{ textDecoration: 'none' }}
          />
        </Box>
      )}

      {(editing || proposal.value) && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Value
          </Typography>
          {editing ? (
            <TextField
              fullWidth
              multiline
              minRows={3}
              size="small"
              placeholder="Why does this matter? Which goals does it advance?"
              value={editValue}
              onChange={e => setEditValue(e.target.value)}
            />
          ) : (
            <MarkdownContent slug={slug}>{proposal.value}</MarkdownContent>
          )}
        </Box>
      )}

      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
          Body
        </Typography>
        {editing ? (
          <Box>
            <Box sx={{ display: 'flex', gap: 1, mb: 1 }}>
              <Button size="small" variant={previewBody ? 'outlined' : 'contained'} onClick={() => setPreviewBody(false)}>Edit</Button>
              <Button size="small" variant={previewBody ? 'contained' : 'outlined'} onClick={() => setPreviewBody(true)}>Preview</Button>
            </Box>
            {previewBody ? (
              <MarkdownContent slug={slug}>{editBody}</MarkdownContent>
            ) : (
              <TextField
                fullWidth
                multiline
                minRows={10}
                size="small"
                value={editBody}
                onChange={e => setEditBody(e.target.value)}
              />
            )}
            <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
              <Button
                size="small"
                variant="contained"
                disabled={updateProposal.isPending || !editTitle.trim()}
                onClick={saveEdit}
              >
                Save
              </Button>
              <Button size="small" onClick={() => setEditing(false)}>Cancel</Button>
            </Box>
          </Box>
        ) : (
          proposal.body ? (
            <MarkdownContent slug={slug}>{proposal.body}</MarkdownContent>
          ) : (
            <Typography color="text.disabled" sx={{ fontStyle: 'italic' }}>(empty)</Typography>
          )
        )}
      </Box>

      {versions.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Button
            size="small"
            startIcon={versionsOpen ? <ExpandLessIcon /> : <ExpandMoreIcon />}
            onClick={() => setVersionsOpen(v => !v)}
          >
            Version history ({versions.length})
          </Button>
          {versionsOpen && (
            <Stack spacing={1} sx={{ mt: 1 }}>
              {versions.map(v => (
                <Box key={v.id} sx={{ border: 1, borderColor: 'divider', borderRadius: 1, p: 1.5 }}>
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                    {v.edited_by || 'unknown'} — {new Date(v.edited_at).toLocaleString()}
                  </Typography>
                  <MarkdownContent slug={slug}>{v.body}</MarkdownContent>
                </Box>
              ))}
            </Stack>
          )}
        </Box>
      )}

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3, mb: 2 }}>
        Created by {proposal.created_by || 'unknown'} on {new Date(proposal.created_at).toLocaleString()}
      </Typography>

      <CommentsAndRevisions slug={slug} targetType="proposal" targetId={String(proposal.id)} />

      <Dialog open={approveOpen} onClose={() => setApproveOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Approve proposal #{proposal.id}</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Creates a new issue from this proposal. Title and context come from the proposal.
          </Typography>
          <TextField
            fullWidth
            select
            label="Issue type"
            value={approveType}
            onChange={e => setApproveType(e.target.value)}
            sx={{ mt: 1 }}
            slotProps={{ select: { native: true } }}
          >
            <option value="impl">impl</option>
            <option value="ops">ops</option>
            <option value="ask">ask</option>
            <option value="tracker">tracker</option>
          </TextField>
          <TextField
            fullWidth
            select
            label="Priority"
            value={approvePriority}
            onChange={e => setApprovePriority(e.target.value)}
            sx={{ mt: 2 }}
            slotProps={{ select: { native: true } }}
          >
            <option value="1">1 — urgent</option>
            <option value="2">2 — high</option>
            <option value="3">3 — medium</option>
            <option value="4">4 — low</option>
          </TextField>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setApproveOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            color="success"
            disabled={approveProposal.isPending}
            onClick={doApprove}
          >
            Approve
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={rejectOpen} onClose={() => setRejectOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Reject proposal #{proposal.id}</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            multiline
            minRows={3}
            label="Reason (optional)"
            value={rejectReason}
            onChange={e => setRejectReason(e.target.value)}
            sx={{ mt: 1 }}
            autoFocus
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRejectOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            color="warning"
            disabled={rejectProposal.isPending}
            onClick={doReject}
          >
            Reject
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={snoozeOpen} onClose={() => setSnoozeOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Snooze proposal #{proposal.id}</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            type="datetime-local"
            label="Snooze until"
            value={snoozeUntil}
            onChange={e => setSnoozeUntil(e.target.value)}
            sx={{ mt: 1 }}
            slotProps={{ inputLabel: { shrink: true } }}
          />
          <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
            <Button size="small" onClick={() => setSnoozeUntil(isoToLocalInput(isoPlusDays(1)))}>+1d</Button>
            <Button size="small" onClick={() => setSnoozeUntil(isoToLocalInput(isoPlusDays(7)))}>+7d</Button>
            <Button size="small" onClick={() => setSnoozeUntil(isoToLocalInput(isoPlusDays(30)))}>+30d</Button>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSnoozeOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            disabled={snoozeProposal.isPending || !snoozeUntil}
            onClick={doSnooze}
          >
            Snooze
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={reevaluateOpen} onClose={() => setReevaluateOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Re-evaluate proposal #{proposal.id}</DialogTitle>
        <DialogContent>
          {reevaluateProposal.isPending && (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
              <CircularProgress size={32} />
            </Box>
          )}
          {reevaluateData && (() => {
            const duplicates = reevaluateData.duplicates ?? []
            const existingIssues = reevaluateData.existing_issues ?? []
            return (
            <Stack spacing={2} sx={{ mt: 1 }}>
              {duplicates.length === 0 && existingIssues.length === 0 && (
                <Alert severity="success">No duplicates found. Proposal looks unique.</Alert>
              )}

              {duplicates.length > 0 && (
                <Box>
                  <Typography variant="subtitle2" gutterBottom>
                    Similar proposals ({duplicates.length})
                  </Typography>
                  <Stack spacing={1}>
                    {duplicates.map(dup => (
                      <Box key={dup.id} sx={{ border: 1, borderColor: 'divider', borderRadius: 1, p: 1.5 }}>
                        <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                          <Box sx={{ flex: 1, minWidth: 0 }}>
                            <Typography variant="body2" sx={{ fontWeight: 500 }}>
                              PR-{dup.id}: {dup.title}
                            </Typography>
                            {dup.body && (
                              <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>
                                {dup.body.slice(0, 120)}{dup.body.length > 120 ? '…' : ''}
                              </Typography>
                            )}
                          </Box>
                          <Button
                            size="small"
                            variant="outlined"
                            color="warning"
                            disabled={rejectDuplicate.isPending}
                            onClick={() => rejectAsDuplicate(dup.id)}
                          >
                            Reject duplicate
                          </Button>
                        </Box>
                      </Box>
                    ))}
                  </Stack>
                </Box>
              )}

              {existingIssues.length > 0 && (
                <Box>
                  {duplicates.length > 0 && <Divider sx={{ my: 1 }} />}
                  <Typography variant="subtitle2" gutterBottom>
                    Possibly already addressed by existing issues
                  </Typography>
                  <Stack spacing={1}>
                    {existingIssues.map(iss => (
                      <Box key={iss.id} sx={{ border: 1, borderColor: 'divider', borderRadius: 1, p: 1.5 }}>
                        <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                          <Box sx={{ flex: 1, minWidth: 0 }}>
                            <Typography variant="body2" sx={{ fontWeight: 500 }}>
                              <Link
                                to="/project/$slug/issues/$id"
                                params={{ slug, id: iss.id }}
                                style={{ textDecoration: 'none', color: 'inherit' }}
                              >
                                {iss.id}: {iss.title}
                              </Link>
                            </Typography>
                            <Chip label={iss.status} size="small" variant="outlined" sx={{ mt: 0.5 }} />
                          </Box>
                          <Button
                            size="small"
                            variant="outlined"
                            color="warning"
                            disabled={rejectProposal.isPending}
                            onClick={() => rejectAsAddressed(iss.id, iss.title)}
                          >
                            Reject (addressed)
                          </Button>
                        </Box>
                      </Box>
                    ))}
                  </Stack>
                </Box>
              )}
            </Stack>
            )
          })()}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setReevaluateOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
