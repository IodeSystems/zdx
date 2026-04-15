import { useState, useCallback } from 'react'
import {
  Box,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Typography,
  Badge,
  Fab,
} from '@mui/material'
import {
  Notifications as NotificationsIcon,
  BugReport as BugReportIcon,
  TaskAlt as TaskAltIcon,
  Comment as CommentIcon,
  Close as CloseIcon,
} from '@mui/icons-material'
import { useChannel } from '../hooks/useChannel'

export const ACTIVITY_PANEL_WIDTH = 300
const MAX_EVENTS = 100

type ActivityEvent = {
  id: string
  channel: string
  type: string
  payload: unknown
  timestamp: number
}

function eventIcon(type: string) {
  if (type.startsWith('issue.')) return <BugReportIcon fontSize="small" color="warning" />
  if (type.startsWith('task.')) return <TaskAltIcon fontSize="small" color="success" />
  if (type.startsWith('comment.')) return <CommentIcon fontSize="small" color="info" />
  return <NotificationsIcon fontSize="small" />
}

function eventLabel(type: string, payload: unknown) {
  const p = payload as Record<string, unknown>
  const id = p?.id as string | undefined
  switch (type) {
    case 'issue.created': return `Issue ${id ?? ''} created`
    case 'issue.closed': return `Issue ${id ?? ''} closed`
    case 'task.done': return `Task ${id ?? ''} done`
    case 'task.reviewed': return `Task ${id ?? ''} reviewed`
    case 'comment.added': return 'Comment added'
    default: return type
  }
}

function timeAgo(ts: number): string {
  const diff = Math.floor((Date.now() - ts) / 1000)
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

export function ActivityFeed({ slug, open, onToggle }: { slug: string; open: boolean; onToggle: () => void }) {
  const [events, setEvents] = useState<ActivityEvent[]>([])
  const [unseen, setUnseen] = useState(0)

  const handleMessage = useCallback((msg: { channel: string; type: string; payload: unknown }) => {
    const event: ActivityEvent = {
      id: `${msg.type}-${Date.now()}-${Math.random()}`,
      channel: msg.channel,
      type: msg.type,
      payload: msg.payload,
      timestamp: Date.now(),
    }
    setEvents(prev => [event, ...prev].slice(0, MAX_EVENTS))
    setUnseen(prev => prev + 1)
  }, [])

  useChannel(`project:${slug}:issues`, handleMessage)
  useChannel(`project:${slug}:tasks`, handleMessage)
  useChannel(`project:${slug}:comments`, handleMessage)

  const handleToggle = useCallback(() => {
    if (!open) setUnseen(0)
    onToggle()
  }, [open, onToggle])

  return (
    <>
      {!open && (
        <Fab
          size="small"
          color="primary"
          onClick={handleToggle}
          sx={{ position: 'fixed', bottom: 80, right: 16, zIndex: (t) => t.zIndex.drawer + 1 }}
        >
          <Badge badgeContent={unseen} color="error" max={99}>
            <NotificationsIcon fontSize="small" />
          </Badge>
        </Fab>
      )}
      <Drawer
        variant="persistent"
        anchor="right"
        open={open}
        sx={{
          width: open ? ACTIVITY_PANEL_WIDTH : 0,
          flexShrink: 0,
          '& .MuiDrawer-paper': {
            width: ACTIVITY_PANEL_WIDTH,
            boxSizing: 'border-box',
            borderLeft: '1px solid',
            borderColor: 'divider',
          },
        }}
      >
        <Toolbar variant="dense" />
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 2, py: 1, borderBottom: 1, borderColor: 'divider' }}>
          <Typography variant="subtitle2">Activity</Typography>
          <IconButton size="small" onClick={handleToggle}>
            <CloseIcon fontSize="small" />
          </IconButton>
        </Box>
        <Box sx={{ overflow: 'auto', flexGrow: 1 }}>
          {events.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ p: 2, textAlign: 'center' }}>
              No activity yet
            </Typography>
          ) : (
            <List dense disablePadding>
              {events.map(e => (
                <ListItem key={e.id} sx={{ py: 0.5 }}>
                  <ListItemIcon sx={{ minWidth: 32 }}>{eventIcon(e.type)}</ListItemIcon>
                  <ListItemText
                    primary={eventLabel(e.type, e.payload)}
                    secondary={timeAgo(e.timestamp)}
                    slotProps={{
                      primary: { variant: 'body2' as const },
                      secondary: { variant: 'caption' as const },
                    }}
                  />
                </ListItem>
              ))}
            </List>
          )}
        </Box>
      </Drawer>
    </>
  )
}
