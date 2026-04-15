import { useState, useCallback } from 'react'
import {
  Alert,
  IconButton,
  Snackbar,
  Tooltip,
} from '@mui/material'
import {
  Notifications as NotificationsIcon,
  NotificationsOff as NotificationsOffIcon,
  BugReport as BugReportIcon,
  TaskAlt as TaskAltIcon,
  Comment as CommentIcon,
} from '@mui/icons-material'
import { useNavigate } from '@tanstack/react-router'
import { useChannel } from '../hooks/useChannel'

const STORAGE_KEY = 'zdx:activity-muted'

type Toast = {
  id: string
  type: string
  label: string
  url: string | null
}

function eventIcon(type: string) {
  if (type.startsWith('issue.')) return <BugReportIcon fontSize="small" />
  if (type.startsWith('task.')) return <TaskAltIcon fontSize="small" />
  if (type.startsWith('comment.')) return <CommentIcon fontSize="small" />
  return <NotificationsIcon fontSize="small" />
}

function eventLabel(type: string, payload: Record<string, unknown>): string {
  const id = payload?.id as string | undefined
  switch (type) {
    case 'issue.created': return `Issue ${id ?? ''} created`
    case 'issue.closed': return `Issue ${id ?? ''} closed`
    case 'task.done': return `Task ${id ?? ''} done`
    case 'task.reviewed': return `Task ${id ?? ''} reviewed`
    case 'comment.added': return 'Comment added'
    default: return type
  }
}

function eventUrl(slug: string, type: string, payload: Record<string, unknown>): string | null {
  const id = payload?.id as string | undefined
  if (!id) return null
  if (type.startsWith('issue.')) return `/project/${slug}/issues/${id}`
  if (type.startsWith('task.')) return `/project/${slug}/tasks/${id}`
  const targetType = payload?.target_type as string | undefined
  const targetId = payload?.target_id as string | undefined
  if (type === 'comment.added' && targetType && targetId) {
    if (targetType === 'issue') return `/project/${slug}/issues/${targetId}`
    if (targetType === 'task') return `/project/${slug}/tasks/${targetId}`
    if (targetType === 'feature') return `/project/${slug}/features/${targetId}`
  }
  return null
}

export function ActivityToast({ slug }: { slug: string }) {
  const [muted, setMuted] = useState(() => localStorage.getItem(STORAGE_KEY) === '1')
  const [toasts, setToasts] = useState<Toast[]>([])
  const navigate = useNavigate()

  const handleMessage = useCallback((msg: { channel: string; type: string; payload: unknown }) => {
    setMuted(current => {
      if (current) return current
      const payload = (msg.payload ?? {}) as Record<string, unknown>
      const toast: Toast = {
        id: `${msg.type}-${Date.now()}-${Math.random()}`,
        type: msg.type,
        label: eventLabel(msg.type, payload),
        url: eventUrl(slug, msg.type, payload),
      }
      setToasts(prev => [...prev, toast])
      return current
    })
  }, [slug])

  useChannel(`project:${slug}:issues`, handleMessage)
  useChannel(`project:${slug}:tasks`, handleMessage)
  useChannel(`project:${slug}:comments`, handleMessage)

  const handleClose = useCallback((id: string) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  const handleClick = useCallback((toast: Toast) => {
    if (toast.url) {
      navigate({ to: toast.url })
    }
    handleClose(toast.id)
  }, [navigate, handleClose])

  const toggleMute = useCallback(() => {
    setMuted(prev => {
      const next = !prev
      localStorage.setItem(STORAGE_KEY, next ? '1' : '0')
      return next
    })
  }, [])

  return (
    <>
      <Tooltip title={muted ? 'Unmute notifications' : 'Mute notifications'}>
        <IconButton color="inherit" onClick={toggleMute} size="small">
          {muted ? <NotificationsOffIcon fontSize="small" /> : <NotificationsIcon fontSize="small" />}
        </IconButton>
      </Tooltip>
      {toasts.map((toast, i) => (
        <Snackbar
          key={toast.id}
          open
          autoHideDuration={5000}
          onClose={() => handleClose(toast.id)}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
          sx={{ mb: i * 7 }}
        >
          <Alert
            icon={eventIcon(toast.type)}
            severity="info"
            variant="filled"
            onClose={() => handleClose(toast.id)}
            onClick={() => handleClick(toast)}
            sx={{ cursor: toast.url ? 'pointer' : 'default', width: '100%' }}
          >
            {toast.label}
          </Alert>
        </Snackbar>
      ))}
    </>
  )
}
