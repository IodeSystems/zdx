import { useState } from 'react'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Fab,
  TextField,
} from '@mui/material'
import { Add as AddIcon } from '@mui/icons-material'
import { useCreateIssue } from '../api'

export function IssueReportFab({ slug, component }: { slug: string; component?: string }) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [context, setContext] = useState('')
  const createIssue = useCreateIssue()

  const handleClose = () => {
    setOpen(false)
    setTitle('')
    setContext('')
  }

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
            label="Title"
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
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose}>Cancel</Button>
          <Button
            variant="contained"
            disabled={!title.trim() || createIssue.isPending}
            onClick={() =>
              createIssue.mutate(
                { slug, title, context: context || undefined, component: component || undefined },
                { onSuccess: handleClose },
              )
            }
          >
            Submit
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
