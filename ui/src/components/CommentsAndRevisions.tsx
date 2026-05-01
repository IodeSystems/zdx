import { useState, useEffect, useRef } from 'react'
import {
  Box,
  Button,
  TextField,
  Typography,
} from '@mui/material'
import { useComments, useAddComment } from '../api'
import { MarkdownContent } from './MarkdownContent'

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
  const comments = commentsData?.comments ?? []
  const addComment = useAddComment()
  const [body, setBody] = useState('')
  const hasScrolledToHash = useRef(false)

  useEffect(() => {
    if (hasScrolledToHash.current || comments.length === 0) return
    const m = window.location.hash.match(/^#C-(\d+)$/)
    if (!m) return
    const el = document.getElementById(`C-${m[1]}`)
    if (!el) return
    hasScrolledToHash.current = true
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    el.classList.add('comment-hash-highlight')
    const t = setTimeout(() => el.classList.remove('comment-hash-highlight'), 2000)
    return () => clearTimeout(t)
  }, [comments.length])

  const handleAddComment = () => {
    if (!body.trim()) return
    addComment.mutate(
      { slug, target_type: targetType, target_id: targetId, body: body.trim() },
      { onSuccess: () => setBody('') },
    )
  }

  return (
    <Box>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Comments ({comments.length})
      </Typography>
      {(() => {
        const topLevel = comments.filter(c => !c.parent_id)
        const byParent = new Map<number, typeof comments>()
        for (const c of comments) {
          if (c.parent_id) {
            const arr = byParent.get(c.parent_id) ?? []
            arr.push(c)
            byParent.set(c.parent_id, arr)
          }
        }
        const renderComment = (c: typeof comments[0], depth: number) => {
          const displayAuthor = c.author_alias
            ? `${c.author_alias} (${c.author || 'anonymous'})`
            : (c.author || 'anonymous')
          const replies = byParent.get(c.id) ?? []
          return (
            <Box key={c.id} id={`C-${c.id}`} sx={{ ml: depth * 3, borderRadius: 1, transition: 'background-color 0.5s', '&.comment-hash-highlight': { backgroundColor: 'warning.light' } }}>
              <Box sx={{ mb: 1.5, borderLeft: 2, borderColor: c.unread ? 'info.main' : 'success.light', pl: 1.5 }}>
                <Typography variant="caption" sx={{ color: c.unread ? '#1976d2' : '#2e7d32' }}>
                  {new Date(c.created_at).toLocaleDateString()} — {displayAuthor}
                </Typography>
                <Box sx={{ mt: 0.25 }}>
                  <MarkdownContent slug={slug}>{c.body}</MarkdownContent>
                </Box>
              </Box>
              {replies.map(r => renderComment(r, depth + 1))}
            </Box>
          )
        }
        return topLevel.map(c => renderComment(c, 0))
      })()}

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
    </Box>
  )
}
