import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Stack,
  Typography,
} from '@mui/material'
import { Refresh as RefreshIcon } from '@mui/icons-material'
import {
  useProposals,
  useRejectProposal,
  useDeduplicateProposals,
  type ProposalItem,
  type ProposalDedupGroup,
} from '../api'

const STATUS_COLORS: Record<string, 'warning' | 'success' | 'default' | 'info' | 'secondary'> = {
  proposed: 'warning',
  approved: 'success',
  rejected: 'default',
  snoozed: 'info',
}

const STATUS_RANK: Record<string, number> = { proposed: 0, snoozed: 1, approved: 2, rejected: 3 }

const FILTERS = ['proposed', 'snoozed', 'approved', 'rejected'] as const

function ageLabel(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  if (ms < 0) return 'just now'
  const m = Math.floor(ms / 60000)
  if (m < 1) return 'just now'
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h`
  const d = Math.floor(h / 24)
  if (d < 30) return `${d}d`
  const mo = Math.floor(d / 30)
  if (mo < 12) return `${mo}mo`
  return `${Math.floor(mo / 12)}y`
}

function proposalDisplayTitle(title: string, body: string): string {
  if (title) return title
  if (body) return body.slice(0, 60) + (body.length > 60 ? '…' : '')
  return '(no title)'
}

export function ProposalsTab({
  slug,
  statusFilter,
  onStatusFilter,
}: {
  slug: string
  statusFilter: string | null
  onStatusFilter: (status: string | null) => void
}) {
  const { data, isLoading, refetch } = useProposals(slug, statusFilter ?? undefined)
  const deduplicateProposals = useDeduplicateProposals()
  const rejectProposal = useRejectProposal()

  const [dedupeOpen, setDedupeOpen] = useState(false)
  const [dedupeGroups, setDedupeGroups] = useState<ProposalDedupGroup[] | null>(null)
  const [rejectedIDs, setRejectedIDs] = useState<Set<number>>(new Set())

  function doDeduplicate() {
    setDedupeGroups(null)
    setRejectedIDs(new Set())
    setDedupeOpen(true)
    deduplicateProposals.mutate({ slug }, {
      onSuccess: (res) => setDedupeGroups(res.groups),
    })
  }

  function rejectAsDuplicate(duplicateId: number, canonicalId: number, canonicalTitle: string) {
    rejectProposal.mutate(
      { slug, id: duplicateId, reason: `Duplicate of PR-${canonicalId}: ${canonicalTitle}` },
      {
        onSuccess: () => {
          setRejectedIDs(prev => new Set([...prev, duplicateId]))
        },
      },
    )
  }

  function rejectAsAddressed(proposalId: number, issueId: string, issueTitle: string) {
    rejectProposal.mutate(
      { slug, id: proposalId, reason: `Already addressed by ${issueId}: ${issueTitle}` },
      {
        onSuccess: () => {
          setRejectedIDs(prev => new Set([...prev, proposalId]))
        },
      },
    )
  }

  function closeDedupeDialog() {
    setDedupeOpen(false)
    if (rejectedIDs.size > 0) refetch()
  }

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const items: ProposalItem[] = data ?? []

  const allCountsByStatus = items.reduce((acc, p) => {
    acc[p.status] = (acc[p.status] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  const activeGroups = (dedupeGroups ?? []).filter(g =>
    !rejectedIDs.has(g.proposal.id) ||
    (g.duplicates ?? []).some(d => !rejectedIDs.has(d.id)) ||
    (g.existing_issues ?? []).length > 0
  )

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {items.length} proposals{statusFilter ? ` — filtered: ${statusFilter}` : ''}
        </Typography>
        <Stack direction="row" spacing={0.5} sx={{ flexWrap: 'wrap' }}>
          {FILTERS
            .slice()
            .sort((a, b) => (STATUS_RANK[a] ?? 99) - (STATUS_RANK[b] ?? 99))
            .map(status => {
              const count = statusFilter === status ? items.length : (allCountsByStatus[status] ?? 0)
              const isActive = statusFilter === status
              return (
                <Chip
                  key={status}
                  label={isActive ? `${status} (${count})` : status}
                  size="small"
                  color={STATUS_COLORS[status] || 'default'}
                  variant={isActive ? 'filled' : 'outlined'}
                  onClick={() => onStatusFilter(isActive ? null : status)}
                  sx={{ cursor: 'pointer' }}
                />
              )
            })}
          {statusFilter && (
            <Chip
              label="clear"
              size="small"
              variant="outlined"
              onClick={() => onStatusFilter(null)}
              sx={{ cursor: 'pointer' }}
            />
          )}
        </Stack>
        <Box sx={{ flex: 1 }} />
        <Button size="small" variant="outlined" startIcon={<RefreshIcon />} onClick={doDeduplicate}>
          Deduplicate
        </Button>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map(p => (
          <Card key={p.id} variant="outlined">
            <CardActionArea
              component={Link as any}
              to="/project/$slug/proposals/$id"
              params={{ slug, id: String(p.id) }}
            >
              <CardContent sx={{ py: 1.25 }}>
                <Typography variant="body2" sx={{ mb: 0.5 }}>
                  {proposalDisplayTitle(p.title, p.body)}
                </Typography>
                {p.value && (
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5, fontStyle: 'italic' }}>
                    {p.value.slice(0, 100)}{p.value.length > 100 ? '…' : ''}
                  </Typography>
                )}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
                  <Typography variant="caption" color="text.secondary">
                    #{p.id}
                  </Typography>
                  {p.source_type && (
                    <Chip
                      label={p.source_type}
                      size="small"
                      variant="outlined"
                      sx={{ maxWidth: 160 }}
                    />
                  )}
                  <Box sx={{ flex: 1 }} />
                  <Typography variant="caption" color="text.secondary">
                    {ageLabel(p.created_at)}
                  </Typography>
                  <Chip
                    label={p.status}
                    size="small"
                    color={STATUS_COLORS[p.status] || 'default'}
                    variant="outlined"
                  />
                </Box>
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
        {items.length === 0 && !isLoading && (
          <Typography variant="body2" color="text.secondary">No proposals.</Typography>
        )}
      </Box>

      <Dialog open={dedupeOpen} onClose={closeDedupeDialog} maxWidth="sm" fullWidth>
        <DialogTitle>Deduplicate proposals</DialogTitle>
        <DialogContent>
          {deduplicateProposals.isPending && (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
              <CircularProgress size={32} />
            </Box>
          )}
          {dedupeGroups !== null && activeGroups.length === 0 && (
            <Alert severity="success">No duplicates found. All proposals look unique.</Alert>
          )}
          {activeGroups.length > 0 && (
            <Stack spacing={3} sx={{ mt: 1 }}>
              {activeGroups.map((group, i) => {
                const duplicates = (group.duplicates ?? []).filter(d => !rejectedIDs.has(d.id))
                const issues = group.existing_issues ?? []
                const canonicalRejected = rejectedIDs.has(group.proposal.id)
                return (
                  <Box key={group.proposal.id}>
                    {i > 0 && <Divider sx={{ mb: 2 }} />}
                    <Box sx={{ mb: 1 }}>
                      <Typography variant="subtitle2">
                        PR-{group.proposal.id}:{' '}
                        <Link
                          to="/project/$slug/proposals/$id"
                          params={{ slug, id: String(group.proposal.id) }}
                          style={{ textDecoration: 'none', color: 'inherit' }}
                        >
                          {proposalDisplayTitle(group.proposal.title, group.proposal.body)}
                        </Link>
                      </Typography>
                    </Box>

                    {duplicates.length > 0 && (
                      <Box sx={{ mb: 1.5 }}>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                          Similar proposals
                        </Typography>
                        <Stack spacing={0.75}>
                          {duplicates.map(dup => (
                            <Box key={dup.id} sx={{ display: 'flex', alignItems: 'flex-start', gap: 1, border: 1, borderColor: 'divider', borderRadius: 1, p: 1 }}>
                              <Box sx={{ flex: 1, minWidth: 0 }}>
                                <Typography variant="body2" sx={{ fontWeight: 500 }}>
                                  PR-{dup.id}: {dup.title || '(no title)'}
                                </Typography>
                                {dup.body && (
                                  <Typography variant="caption" color="text.secondary">
                                    {dup.body.slice(0, 100)}{dup.body.length > 100 ? '…' : ''}
                                  </Typography>
                                )}
                              </Box>
                              <Button
                                size="small"
                                variant="outlined"
                                color="warning"
                                disabled={rejectProposal.isPending}
                                onClick={() => rejectAsDuplicate(dup.id, group.proposal.id, group.proposal.title)}
                              >
                                Reject duplicate
                              </Button>
                            </Box>
                          ))}
                        </Stack>
                      </Box>
                    )}

                    {issues.length > 0 && !canonicalRejected && (
                      <Box>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                          Possibly already addressed
                        </Typography>
                        <Stack spacing={0.75}>
                          {issues.map(iss => (
                            <Box key={iss.id} sx={{ display: 'flex', alignItems: 'flex-start', gap: 1, border: 1, borderColor: 'divider', borderRadius: 1, p: 1 }}>
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
                                onClick={() => rejectAsAddressed(group.proposal.id, iss.id, iss.title)}
                              >
                                Reject (addressed)
                              </Button>
                            </Box>
                          ))}
                        </Stack>
                      </Box>
                    )}
                  </Box>
                )
              })}
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDedupeDialog}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
