import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Paper,
  TextField,
  Typography,
} from '@mui/material'
import {
  useQuestionProposals,
  useAcceptQuestionProposal,
  useDenyQuestionProposal,
} from '../api'

export function QuestionProposalsSection({
  slug,
  questionId,
  questionType,
}: {
  slug: string
  questionId: number
  questionType: string
}) {
  const { data } = useQuestionProposals(slug, questionId, questionType)
  const proposals = data?.proposals ?? []
  const acceptMut = useAcceptQuestionProposal()
  const denyMut = useDenyQuestionProposal()
  const [denyingId, setDenyingId] = useState<number | null>(null)
  const [denyReason, setDenyReason] = useState('')

  if (proposals.length === 0) return null

  const statusColor = (s: string) => {
    if (s === 'accepted') return 'success'
    if (s === 'denied') return 'error'
    return 'warning'
  }

  return (
    <Box sx={{ mb: 3 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        Issue Proposals ({proposals.length})
      </Typography>
      {proposals.map((p) => (
        <Paper key={p.id} variant="outlined" sx={{ p: 2, mb: 1 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
            <Chip label={p.status} size="small" color={statusColor(p.status)} variant="outlined" />
            {p.created_issue_id && (
              <Chip
                label={p.created_issue_id}
                size="small"
                variant="outlined"
                color="info"
                component={Link as any}
                to="/project/$slug/issues/$id"
                params={{ slug, id: p.created_issue_id }}
                clickable
              />
            )}
          </Box>
          <Typography variant="body1" sx={{ fontWeight: 500 }}>{p.title}</Typography>
          {p.context && (
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>{p.context}</Typography>
          )}
          {p.denied_reason && (
            <Typography variant="body2" color="error" sx={{ mt: 0.5 }}>Reason: {p.denied_reason}</Typography>
          )}
          {p.status === 'proposed' && (
            <Box sx={{ mt: 1, display: 'flex', gap: 1, alignItems: 'flex-start' }}>
              <Button
                size="small"
                variant="contained"
                color="success"
                disabled={acceptMut.isPending}
                onClick={() => acceptMut.mutate({ slug, id: p.id })}
              >
                Accept
              </Button>
              {denyingId === p.id ? (
                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexGrow: 1 }}>
                  <TextField
                    size="small"
                    placeholder="Reason (optional)"
                    value={denyReason}
                    onChange={(e) => setDenyReason(e.target.value)}
                    sx={{ flexGrow: 1 }}
                  />
                  <Button
                    size="small"
                    variant="outlined"
                    color="error"
                    disabled={denyMut.isPending}
                    onClick={() => {
                      denyMut.mutate({ slug, id: p.id, reason: denyReason }, {
                        onSuccess: () => { setDenyingId(null); setDenyReason('') },
                      })
                    }}
                  >
                    Confirm
                  </Button>
                  <Button size="small" onClick={() => { setDenyingId(null); setDenyReason('') }}>
                    Cancel
                  </Button>
                </Box>
              ) : (
                <Button
                  size="small"
                  variant="outlined"
                  color="error"
                  onClick={() => setDenyingId(p.id)}
                >
                  Deny
                </Button>
              )}
            </Box>
          )}
        </Paper>
      ))}
    </Box>
  )
}
