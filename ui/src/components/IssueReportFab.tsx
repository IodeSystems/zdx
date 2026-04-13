import { useRef, useState } from 'react'
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Fab,
  IconButton,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import { Add as AddIcon, AttachFile as AttachFileIcon, Close as CloseIcon } from '@mui/icons-material'
import { useCreateIssue, useUploadFile } from '../api'

export function IssueReportFab({ slug, component }: { slug: string; component?: string }) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [context, setContext] = useState('')
  const [screenshot, setScreenshot] = useState<File | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const createIssue = useCreateIssue()
  const uploadFile = useUploadFile()

  const handleClose = () => {
    setOpen(false)
    setTitle('')
    setContext('')
    setScreenshot(null)
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (f) setScreenshot(f)
    e.target.value = ''
  }

  const handleSubmit = async () => {
    let screenshotIds: number[] | undefined
    if (screenshot) {
      const uploaded = await uploadFile.mutateAsync(screenshot)
      screenshotIds = [uploaded.id]
    }
    createIssue.mutate(
      {
        slug,
        title: title.trim() || undefined,
        context: context.trim() || undefined,
        component: component || undefined,
        screenshot_ids: screenshotIds,
      },
      { onSuccess: handleClose },
    )
  }

  const isPending = createIssue.isPending || uploadFile.isPending

  return (
    <>
      <Fab
        color="primary"
        size="medium"
        onClick={() => setOpen(true)}
        sx={{ position: 'fixed', bottom: 24, right: 24, zIndex: 1200 }}
        aria-label="Report issue"
      >
        <AddIcon />
      </Fab>

      <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
        <DialogTitle>Report Issue</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            label="Title (optional — filled by owner)"
            value={title}
            onChange={e => setTitle(e.target.value)}
            sx={{ mt: 1, mb: 2 }}
          />
          <TextField
            fullWidth
            label="Context"
            multiline
            rows={4}
            value={context}
            onChange={e => setContext(e.target.value)}
            placeholder="Steps to reproduce, observed vs expected, links…"
          />
          <Box sx={{ mt: 1.5, display: 'flex', alignItems: 'center', gap: 1 }}>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/png,image/jpeg,image/gif,image/webp"
              style={{ display: 'none' }}
              onChange={handleFileChange}
            />
            <Tooltip title="Attach screenshot">
              <IconButton size="small" onClick={() => fileInputRef.current?.click()}>
                <AttachFileIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            {screenshot ? (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                <Typography variant="caption" noWrap sx={{ maxWidth: 200 }}>
                  {screenshot.name}
                </Typography>
                <IconButton size="small" onClick={() => setScreenshot(null)}>
                  <CloseIcon fontSize="small" />
                </IconButton>
              </Box>
            ) : (
              <Typography variant="caption" color="text.secondary">
                Attach screenshot (optional)
              </Typography>
            )}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose}>Cancel</Button>
          <Button
            variant="contained"
            disabled={isPending}
            onClick={handleSubmit}
          >
            Submit
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
