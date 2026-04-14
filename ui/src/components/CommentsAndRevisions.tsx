import { useState } from 'react'
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  TextField,
  Typography,
} from '@mui/material'
import { Close as CloseIcon, CompareArrows as DiffIcon } from '@mui/icons-material'
import { useComments, useAddComment, useRevisions, type RevisionItem } from '../api'
import { MarkdownContent } from './MarkdownContent'

function DiffModal({ revision, onClose }: { revision: RevisionItem; onClose: () => void }) {
  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span>{revision.target_id} — {revision.field}</span>
        <IconButton size="small" onClick={onClose}><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <DialogContent>
        <Box sx={{ mb: 1 }}>
          <Typography variant="caption" color="text.secondary">Before</Typography>
          <Box sx={{ bgcolor: 'error.light', p: 1, borderRadius: 1, mt: 0.5, whiteSpace: 'pre-wrap' }}>
            <Typography variant="body2">{revision.old_val || '(empty)'}</Typography>
          </Box>
        </Box>
        <Box>
          <Typography variant="caption" color="text.secondary">After</Typography>
          <Box sx={{ bgcolor: 'success.light', p: 1, borderRadius: 1, mt: 0.5, whiteSpace: 'pre-wrap' }}>
            <Typography variant="body2">{revision.new_val || '(empty)'}</Typography>
          </Box>
        </Box>
      </DialogContent>
    </Dialog>
  )
}

export function CommentsAndRevisions({
  slug,
  targetType,
  targetId,
}: {
  slug: string
  targetType: string
  targetId: string
}) {
  const { data: commentsData } = useComments(slug, targetType, targetId)
  const { data: revisionsData } = useRevisions(slug, targetType, targetId)
  const comments = commentsData?.comments ?? []
  const revisions = revisionsData?.revisions ?? []
  const addComment = useAddComment()
  const [body, setBody] = useState('')
  const [diffRevision, setDiffRevision] = useState<RevisionItem | null>(null)

  const handleAddComment = () => {
    if (!body.trim()) return
    addComment.mutate(
      { slug, target_type: targetType, target_id: targetId, body: body.trim() },
      { onSuccess: () => setBody('') },
    )
  }

  return (
    <Box>
      {/* Revision history */}
      {revisions.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            History ({revisions.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {revisions.map(r => {
              const date = r.created_at.slice(0, 10)
              const preview = r.old_val.slice(0, 20) || '…'
              const newPreview = r.new_val.slice(0, 20) || '…'
              return (
                <Box
                  key={r.id}
                  sx={{ display: 'flex', alignItems: 'center', gap: 1, borderLeft: 2, borderColor: 'divider', pl: 1.5 }}
                >
                  <Typography variant="caption" color="text.secondary" sx={{ minWidth: 80 }}>
                    {date}
                  </Typography>
                  <Chip label={r.field} size="small" variant="outlined" sx={{ fontSize: '0.65rem' }} />
                  <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
                    {preview}→{newPreview}
                  </Typography>
                  {r.agent && (
                    <Typography variant="caption" color="text.disabled">({r.agent})</Typography>
                  )}
                  <IconButton size="small" onClick={() => setDiffRevision(r)} title="View diff">
                    <DiffIcon fontSize="inherit" />
                  </IconButton>
                </Box>
              )
            })}
          </Box>
        </Box>
      )}

      <Divider sx={{ mb: 2 }} />

      {/* Comments */}
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Comments ({comments.length})
      </Typography>
      {comments.map(c => (
        <Box key={c.id} sx={{ mb: 1.5, borderLeft: 2, borderColor: 'primary.light', pl: 1.5 }}>
          <Typography variant="caption" color="text.secondary">
            {c.created_at.slice(0, 10)} — {c.author || 'anonymous'}
          </Typography>
          <Box sx={{ mt: 0.25 }}>
            <MarkdownContent slug={slug}>{c.body}</MarkdownContent>
          </Box>
        </Box>
      ))}

      <Box sx={{ mt: 2 }}>
        <TextField
          fullWidth
          multiline
          rows={2}
          size="small"
          placeholder="Add a comment…"
          value={body}
          onChange={e => setBody(e.target.value)}
        />
        <Button
          size="small"
          variant="contained"
          disabled={!body.trim() || addComment.isPending}
          onClick={handleAddComment}
          sx={{ mt: 1 }}
        >
          Comment
        </Button>
      </Box>

      {diffRevision && (
        <DiffModal revision={diffRevision} onClose={() => setDiffRevision(null)} />
      )}
    </Box>
  )
}
