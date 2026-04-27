import { useState, useRef, useEffect } from 'react'
import { Link } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  ArrowBack as ArrowBackIcon,
  ContentCopy as ContentCopyIcon,
  Lightbulb as LightbulbIcon,
  Send as SendIcon,
} from '@mui/icons-material'
import {
  useDiscussion,
  useDiscussionMessages,
  useAddDiscussionMessage,
  useCreateProposal,
  type DiscussionMessageItem,
} from '../api'

function MessageBubble({
  msg,
  onPromote,
}: {
  msg: DiscussionMessageItem
  onPromote: (msg: DiscussionMessageItem) => void
}) {
  const isAssistant = msg.role === 'assistant'
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: isAssistant ? 'flex-start' : 'flex-end',
        gap: 0.25,
      }}
    >
      <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
        <Typography variant="caption" color="text.secondary">
          {msg.role}
        </Typography>
        <Typography variant="caption" color="text.disabled" sx={{ fontSize: '0.6rem' }}>
          {new Date(msg.created_at).toLocaleTimeString()}
        </Typography>
      </Stack>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 0.5, maxWidth: '80%' }}>
        {isAssistant && (
          <Tooltip title="Promote to proposal">
            <IconButton size="small" onClick={() => onPromote(msg)} sx={{ mt: 0.25 }}>
              <LightbulbIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        )}
        <Paper
          variant="outlined"
          sx={{
            p: 1.25,
            bgcolor: isAssistant ? 'background.paper' : 'primary.dark',
            borderColor: isAssistant ? 'divider' : 'primary.main',
          }}
        >
          <Typography
            variant="body2"
            sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}
          >
            {msg.content}
          </Typography>
        </Paper>
        {!isAssistant && (
          <Tooltip title="Copy">
            <IconButton
              size="small"
              onClick={() => navigator.clipboard.writeText(msg.content)}
              sx={{ mt: 0.25 }}
            >
              <ContentCopyIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        )}
      </Box>
    </Box>
  )
}

export function DiscussionDetail({
  slug,
  discussionId,
  initialMessage,
}: {
  slug: string
  discussionId: number
  initialMessage?: string
}) {
  const { data: discussion, isLoading: loadingDiscussion } = useDiscussion(slug, discussionId)
  const { data: messages, isLoading: loadingMessages } = useDiscussionMessages(slug, discussionId)
  const addMessage = useAddDiscussionMessage()
  const createProposal = useCreateProposal()

  const [input, setInput] = useState(initialMessage ?? '')
  const [promoteMsg, setPromoteMsg] = useState<DiscussionMessageItem | null>(null)
  const [propTitle, setPropTitle] = useState('')
  const [propValue, setPropValue] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages?.length])

  if (loadingDiscussion && !discussion)
    return <Typography color="text.secondary">Loading...</Typography>
  if (!discussion)
    return <Typography color="error">Discussion not found.</Typography>

  const handleSend = () => {
    const trimmed = input.trim()
    if (!trimmed) return
    addMessage.mutate({ slug, id: discussionId, role: 'user', content: trimmed })
    setInput('')
  }

  const handlePromote = (msg: DiscussionMessageItem) => {
    setPromoteMsg(msg)
    setPropTitle(msg.content.slice(0, 80).replace(/\n/g, ' '))
    setPropValue('')
  }

  const handleConfirmPromote = () => {
    if (!promoteMsg) return
    createProposal.mutate(
      {
        slug,
        title: propTitle,
        value: propValue,
        body: promoteMsg.content,
        source_type: 'discussion',
        source_ref: String(discussionId),
      },
      {
        onSuccess: () => {
          setPromoteMsg(null)
          setPropTitle('')
          setPropValue('')
        },
      },
    )
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', gap: 1.5 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <IconButton
          size="small"
          component={Link as React.ElementType}
          to="/project/$slug/discussions/"
          params={{ slug }}
        >
          <ArrowBackIcon fontSize="small" />
        </IconButton>
        <Typography variant="h6" sx={{ flex: 1 }}>
          {discussion.title || `Discussion #${discussion.id}`}
        </Typography>
        <Chip label={discussion.provider} size="small" variant="outlined" />
        <Chip
          label={discussion.status}
          size="small"
          color={discussion.status === 'active' ? 'success' : 'default'}
          variant="outlined"
        />
      </Box>

      {/* Message thread */}
      <Box
        sx={{
          flex: 1,
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: 1.5,
          p: 1,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
          minHeight: 200,
        }}
      >
        {loadingMessages && !messages ? (
          <CircularProgress size={20} sx={{ m: 'auto' }} />
        ) : (messages ?? []).length === 0 ? (
          <Typography color="text.secondary" variant="body2" sx={{ m: 'auto' }}>
            No messages yet.
          </Typography>
        ) : (
          (messages ?? []).map(m => (
            <MessageBubble key={m.id} msg={m} onPromote={handlePromote} />
          ))
        )}
        <div ref={messagesEndRef} />
      </Box>

      {/* Input */}
      <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-end' }}>
        <TextField
          multiline
          maxRows={6}
          fullWidth
          size="small"
          placeholder="Type a message…"
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={e => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              handleSend()
            }
          }}
        />
        <IconButton
          color="primary"
          onClick={handleSend}
          disabled={!input.trim() || addMessage.isPending}
        >
          {addMessage.isPending ? <CircularProgress size={20} /> : <SendIcon />}
        </IconButton>
      </Stack>

      {/* Promote to proposal dialog */}
      <Dialog open={!!promoteMsg} onClose={() => setPromoteMsg(null)} maxWidth="sm" fullWidth>
        <DialogTitle>Promote to proposal</DialogTitle>
        <DialogContent>
          <TextField
            label="Title"
            fullWidth
            margin="dense"
            value={propTitle}
            onChange={e => setPropTitle(e.target.value)}
            autoFocus
          />
          <TextField
            label="Value (why does this matter?)"
            fullWidth
            margin="dense"
            multiline
            minRows={2}
            value={propValue}
            onChange={e => setPropValue(e.target.value)}
            placeholder="Which goals does this advance? What problem does it solve?"
          />
          <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
            Body will be set to the full message content.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPromoteMsg(null)}>Cancel</Button>
          <Button
            variant="contained"
            onClick={handleConfirmPromote}
            disabled={!propTitle.trim() || createProposal.isPending}
          >
            {createProposal.isPending ? <CircularProgress size={16} /> : 'Promote'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
