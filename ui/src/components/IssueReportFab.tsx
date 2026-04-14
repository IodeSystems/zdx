import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Fab,
  IconButton,
  Link,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import { Add as AddIcon, AttachFile as AttachFileIcon, Close as CloseIcon } from '@mui/icons-material'
import html2canvas from 'html2canvas'
import { useCreateIssue, useUploadFile, useSimilarIssues, type SimilarIssueItem, type IssueItem } from '../api'
import { useMatches } from '@tanstack/react-router'

async function capturePageScreenshot(): Promise<File | null> {
  try {
    const canvas = await html2canvas(document.body, { useCORS: true, logging: false })
    return await new Promise<File | null>((resolve) => {
      canvas.toBlob((blob) => {
        if (!blob) { resolve(null); return }
        resolve(new File([blob], 'screenshot.jpg', { type: 'image/jpeg' }))
      }, 'image/jpeg', 0.8)
    })
  } catch {
    return null
  }
}

export function IssueReportFab({ slug, component }: { slug: string; component?: string }) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [context, setContext] = useState('')
  const [screenshot, setScreenshot] = useState<File | null>(null)
  const [capturing, setCapturing] = useState(false)
  // preflight state
  const [similarIssues, setSimilarIssues] = useState<SimilarIssueItem[] | null>(null)
  const [preflight, setPreflight] = useState(false)
  const [createdIssue, setCreatedIssue] = useState<IssueItem | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const matches = useMatches()

  const screenshotPreview = useMemo(() => screenshot ? URL.createObjectURL(screenshot) : null, [screenshot])
  useEffect(() => {
    return () => { if (screenshotPreview) URL.revokeObjectURL(screenshotPreview) }
  }, [screenshotPreview])

  const createIssue = useCreateIssue()
  const uploadFile = useUploadFile()
  const findSimilar = useSimilarIssues()

  const handleOpen = async () => {
    setSimilarIssues(null)
    setPreflight(false)
    setOpen(true)
    setCapturing(true)
    const file = await capturePageScreenshot()
    setCapturing(false)
    if (file) setScreenshot(file)
  }

  const handleClose = () => {
    setOpen(false)
    setTitle('')
    setContext('')
    setScreenshot(null)
    setSimilarIssues(null)
    setPreflight(false)
    setCreatedIssue(null)
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (f) setScreenshot(f)
    e.target.value = ''
  }

  // Step 1: run preflight similarity check
  const handlePreflight = async () => {
    const text = [title, context].filter(Boolean).join(' ')
    if (!text.trim()) {
      // No text — skip preflight, submit directly
      await doSubmit()
      return
    }
    setPreflight(true)
    findSimilar.mutate(
      { slug, text, n: 5 },
      {
        onSuccess: (issues) => {
          setPreflight(false)
          if (issues.length === 0) {
            doSubmit()
          } else {
            setSimilarIssues(issues)
          }
        },
        onError: () => {
          // If similarity check fails, proceed to submit
          setPreflight(false)
          doSubmit()
        },
      },
    )
  }

  // Step 2: actual submission (after user confirms from similar list, or directly)
  const doSubmit = async () => {
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
      {
        onSuccess: (issue) => {
          setCreatedIssue(issue)
          setSimilarIssues(null)
        },
      },
    )
  }

  // Resolve current slug for issue links
  const projectMatch = matches.find(m => (m.params as Record<string, string>).slug)
  const routeSlug = (projectMatch?.params as { slug?: string })?.slug ?? slug

  const isPending = createIssue.isPending || uploadFile.isPending || capturing || preflight
  const showSimilar = similarIssues !== null && !preflight

  return (
    <>
      <Fab
        color="primary"
        size="medium"
        onClick={handleOpen}
        sx={{ position: 'fixed', bottom: 24, right: 24, zIndex: 1200 }}
        aria-label="Report issue"
      >
        <AddIcon />
      </Fab>

      <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
        {createdIssue ? (
          <>
            <DialogTitle>Issue Reported</DialogTitle>
            <DialogContent>
              <Alert severity="success" sx={{ mb: 2 }}>
                Your issue has been created.
              </Alert>
              <Link
                href={`/project/${routeSlug}/all/issues/${createdIssue.id}`}
                variant="body1"
              >
                IS-{createdIssue.id}: {createdIssue.title || '(untitled)'}
              </Link>
            </DialogContent>
            <DialogActions>
              <Button variant="contained" onClick={handleClose}>Done</Button>
            </DialogActions>
          </>
        ) : (
          <>
            <DialogTitle>Report Issue</DialogTitle>
            <DialogContent>
              <TextField
                autoFocus
                fullWidth
                label="Title (optional — filled by owner)"
                value={title}
                onChange={e => setTitle(e.target.value)}
                sx={{ mt: 1, mb: 2 }}
                disabled={showSimilar}
              />
              <TextField
                fullWidth
                label="Context"
                multiline
                rows={4}
                value={context}
                onChange={e => setContext(e.target.value)}
                placeholder="Steps to reproduce, observed vs expected, links…"
                disabled={showSimilar}
              />
              <Box sx={{ mt: 1.5 }}>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/gif,image/webp"
                  style={{ display: 'none' }}
                  onChange={handleFileChange}
                />
                {capturing ? (
                  <Typography variant="caption" color="text.secondary">Capturing screenshot…</Typography>
                ) : screenshotPreview ? (
                  <Box sx={{ position: 'relative', display: 'inline-block' }}>
                    <Box
                      component="img"
                      src={screenshotPreview}
                      alt="Screenshot preview"
                      onClick={() => !showSimilar && fileInputRef.current?.click()}
                      sx={{
                        display: 'block',
                        maxWidth: '100%',
                        maxHeight: 200,
                        borderRadius: 1,
                        border: '1px solid',
                        borderColor: 'divider',
                        cursor: showSimilar ? 'default' : 'pointer',
                      }}
                    />
                    {!showSimilar && (
                      <Tooltip title="Remove screenshot">
                        <IconButton
                          size="small"
                          onClick={() => setScreenshot(null)}
                          sx={{ position: 'absolute', top: 4, right: 4, bgcolor: 'background.paper' }}
                        >
                          <CloseIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    )}
                  </Box>
                ) : (
                  !showSimilar && (
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Tooltip title="Attach screenshot">
                        <IconButton size="small" onClick={() => fileInputRef.current?.click()}>
                          <AttachFileIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <Typography variant="caption" color="text.secondary">
                        Attach screenshot (optional)
                      </Typography>
                    </Box>
                  )
                )}
              </Box>

              {/* Preflight spinner */}
              {preflight && (
                <Box sx={{ mt: 2, display: 'flex', alignItems: 'center', gap: 1 }}>
                  <CircularProgress size={16} />
                  <Typography variant="caption" color="text.secondary">Checking for similar issues…</Typography>
                </Box>
              )}

              {/* Similar issues list */}
              {showSimilar && (
                <Box sx={{ mt: 2 }}>
                  <Divider sx={{ mb: 1.5 }} />
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
                    Similar existing issues — is yours already reported?
                  </Typography>
                  <Box sx={{ maxHeight: 300, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 1 }}>
                    {similarIssues!.map(iss => (
                      <Card key={iss.id} variant="outlined" sx={{ flexShrink: 0 }}>
                        <CardContent sx={{ py: 1, px: 1.5, '&:last-child': { pb: 1 } }}>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                            <Link
                              href={`/project/${routeSlug}/all/issues/${iss.id}`}
                              target="_blank"
                              rel="noopener"
                              variant="body2"
                              sx={{ fontWeight: 600, whiteSpace: 'nowrap' }}
                            >
                              {iss.id}
                            </Link>
                            <Typography variant="body2" sx={{ fontWeight: 500, flex: 1, minWidth: 0 }} noWrap>
                              {iss.title || '(untitled)'}
                            </Typography>
                            <Chip label={iss.status || 'open'} size="small" variant="outlined" sx={{ height: 20, fontSize: '0.7rem' }} />
                            <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: 'nowrap' }}>
                              {(iss.score * 100).toFixed(0)}%
                            </Typography>
                          </Box>
                          {iss.context && (
                            <Typography variant="body2" color="text.secondary" sx={{
                              overflow: 'hidden',
                              display: '-webkit-box',
                              WebkitLineClamp: 3,
                              WebkitBoxOrient: 'vertical',
                            }}>
                              {iss.context}
                            </Typography>
                          )}
                        </CardContent>
                      </Card>
                    ))}
                  </Box>
                </Box>
              )}
            </DialogContent>

            <DialogActions>
              <Button onClick={handleClose}>Cancel</Button>
              {!showSimilar ? (
                <Button
                  variant="contained"
                  disabled={isPending}
                  onClick={handlePreflight}
                >
                  {preflight ? <CircularProgress size={16} color="inherit" /> : 'Submit'}
                </Button>
              ) : (
                <>
                  <Button onClick={() => setSimilarIssues(null)}>Edit</Button>
                  <Button
                    variant="contained"
                    disabled={createIssue.isPending || uploadFile.isPending}
                    onClick={doSubmit}
                  >
                    Submit anyway
                  </Button>
                </>
              )}
            </DialogActions>
          </>
        )}
      </Dialog>
    </>
  )
}
