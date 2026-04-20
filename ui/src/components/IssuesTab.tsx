import { useState, useEffect, useRef } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { Search as SearchIcon, Close as CloseIcon } from '@mui/icons-material'
import { useIssues, useSearchIssues, useSimilarIssues, type IssueItem, type SimilarIssueItem } from '../api'
import { useLoadMore } from '../api/pagination'
import { LoadMore } from './LoadMore'
import { useComponentFilter } from './ComponentContext'

type Issue = IssueItem

function priorityLabel(p: string): string {
  if (!p) return 'untriaged'
  return { '1': 'urgent', '2': 'high', '3': 'medium', '4': 'low' }[p] ?? p
}

function issueDisplayTitle(title: string, context: string): string {
  if (title) return title
  if (context) return context.slice(0, 60) + (context.length > 60 ? '…' : '')
  return '(no title)'
}

const PRIORITY_COLORS: Record<string, 'error' | 'warning' | 'default' | 'info'> = {
  urgent: 'error',
  high: 'warning',
  medium: 'info',
  low: 'default',
}

const STATUS_COLORS: Record<string, 'warning' | 'info' | 'secondary' | 'success' | 'default'> = {
  open: 'warning',
  triaged: 'info',
  'in-progress': 'secondary',
  done: 'success',
  closed: 'success',
}

const STATUS_RANK: Record<string, number> = { open: 0, triaged: 1, 'in-progress': 2, done: 3, closed: 4 }

function IssueSearch({ slug }: { slug: string }) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [inputValue, setInputValue] = useState('')
  const [ftsQuery, setFtsQuery] = useState('')
  const [vecQuery, setVecQuery] = useState('')
  const [vecResults, setVecResults] = useState<SimilarIssueItem[]>([])
  const ftsTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const vecTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)
  const findSimilar = useSimilarIssues()

  useEffect(() => {
    if (ftsTimer.current) clearTimeout(ftsTimer.current)
    if (vecTimer.current) clearTimeout(vecTimer.current)
    if (!inputValue.trim()) {
      ftsTimer.current = setTimeout(() => setFtsQuery(''), 0)
      vecTimer.current = setTimeout(() => { setVecQuery(''); setVecResults([]) }, 0)
      return () => {
        if (ftsTimer.current) clearTimeout(ftsTimer.current)
        if (vecTimer.current) clearTimeout(vecTimer.current)
      }
    }
    ftsTimer.current = setTimeout(() => setFtsQuery(inputValue), 150)
    vecTimer.current = setTimeout(() => setVecQuery(inputValue), 500)
    return () => {
      if (ftsTimer.current) clearTimeout(ftsTimer.current)
      if (vecTimer.current) clearTimeout(vecTimer.current)
    }
  }, [inputValue])

  useEffect(() => {
    if (!vecQuery.trim()) return
    findSimilar.mutate({ slug, text: vecQuery, n: 10 }, {
      onSuccess: (items) => setVecResults(items),
      onError: () => setVecResults([]),
    })
  }, [vecQuery, slug]) // eslint-disable-line react-hooks/exhaustive-deps

  const { data: ftsData } = useSearchIssues(slug, ftsQuery, ftsQuery.length > 1)

  const ftsItems = ftsData ?? []
  const vecItems = vecResults.filter(v => !ftsItems.some(f => `IS-${f.id}` === v.id))

  function handleSelect(id: string) {
    navigate({ to: '/project/$slug/issues/$id', params: { slug, id } })
    setOpen(false)
    setInputValue('')
  }

  function handleClose() {
    setOpen(false)
    setInputValue('')
    setFtsQuery('')
    setVecQuery('')
    setVecResults([])
  }

  return (
    <>
      <TextField
        size="small"
        placeholder="Search issues…"
        onClick={() => setOpen(true)}
        slotProps={{
          input: {
            readOnly: true,
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
          },
        }}
        sx={{ minWidth: 200, cursor: 'pointer', '& input': { cursor: 'pointer' } }}
      />
      <Dialog
        open={open}
        onClose={handleClose}
        maxWidth="md"
        fullWidth
        slotProps={{ paper: { sx: { minHeight: '60vh', maxHeight: '80vh' } } }}
      >
        <DialogTitle sx={{ display: 'flex', alignItems: 'center', gap: 1, pb: 1 }}>
          <TextField
            inputRef={searchInputRef}
            autoFocus
            fullWidth
            size="small"
            placeholder="Search issues…"
            value={inputValue}
            onChange={e => setInputValue(e.target.value)}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" />
                  </InputAdornment>
                ),
              },
            }}
          />
          <IconButton onClick={handleClose} size="small">
            <CloseIcon />
          </IconButton>
        </DialogTitle>
        <DialogContent sx={{ pt: 0, overflow: 'auto' }}>
          {!inputValue.trim() && (
            <Typography color="text.secondary" sx={{ mt: 2, textAlign: 'center' }}>
              Type to search issues by text or similarity
            </Typography>
          )}
          {ftsItems.length > 0 && (
            <Box sx={{ mb: 2 }}>
              <Typography variant="overline" color="text.secondary" sx={{ px: 1 }}>
                Text match
              </Typography>
              <Stack spacing={1} sx={{ mt: 0.5 }}>
                {ftsItems.map(item => {
                  const pLabel = priorityLabel(item.priority)
                  return (
                    <Card key={item.id} variant="outlined">
                      <CardActionArea onClick={() => handleSelect(`IS-${item.id}`)}>
                        <CardContent sx={{ py: 1.25, '&:last-child': { pb: 1.25 } }}>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                            <Typography variant="caption" color="text.secondary">IS-{item.id}</Typography>
                            <Chip label={pLabel} size="small" color={PRIORITY_COLORS[pLabel] || 'default'} />
                            <Chip label={item.status} size="small" color={STATUS_COLORS[item.status] || 'default'} variant="outlined" />
                          </Box>
                          <Typography variant="body2" sx={{ fontWeight: 500 }}>
                            {issueDisplayTitle(item.title, item.context)}
                          </Typography>
                          {item.context && item.title && (
                            <Typography variant="body2" color="text.secondary" sx={{
                              mt: 0.5,
                              display: '-webkit-box',
                              WebkitLineClamp: 2,
                              WebkitBoxOrient: 'vertical',
                              overflow: 'hidden',
                            }}>
                              {item.context}
                            </Typography>
                          )}
                        </CardContent>
                      </CardActionArea>
                    </Card>
                  )
                })}
              </Stack>
            </Box>
          )}
          {vecItems.length > 0 && (
            <Box>
              <Typography variant="overline" color="text.secondary" sx={{ px: 1 }}>
                Similar
              </Typography>
              <Stack spacing={1} sx={{ mt: 0.5 }}>
                {vecItems.map(item => (
                  <Card key={item.id} variant="outlined">
                    <CardActionArea onClick={() => handleSelect(item.id)}>
                      <CardContent sx={{ py: 1.25, '&:last-child': { pb: 1.25 } }}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                          <Typography variant="caption" color="text.secondary">{item.id}</Typography>
                          <Typography variant="caption" color="text.secondary" sx={{ ml: 'auto' }}>
                            {(item.score * 100).toFixed(0)}% similar
                          </Typography>
                        </Box>
                        <Typography variant="body2" sx={{ fontWeight: 500 }}>
                          {item.title || item.id}
                        </Typography>
                      </CardContent>
                    </CardActionArea>
                  </Card>
                ))}
              </Stack>
            </Box>
          )}
          {inputValue.trim() && ftsItems.length === 0 && vecItems.length === 0 && (
            <Typography color="text.secondary" sx={{ mt: 2, textAlign: 'center' }}>
              No results
            </Typography>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}

export function IssuesTab({
  slug,
  statusFilter,
  onStatusFilter,
}: {
  slug: string
  statusFilter: string | null
  onStatusFilter: (status: string | null) => void
}) {
  const { component } = useComponentFilter()
  const { offset, loadMore, pageSize } = useLoadMore()
  const { data, isLoading } = useIssues(slug, pageSize, offset)

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const allItems: Issue[] = data?.issues ?? []
  const total = data?.total ?? 0

  const componentFiltered = component ? allItems.filter(i => i.component === component) : allItems

  const items = statusFilter
    ? componentFiltered.filter(i => i.status === statusFilter)
    : componentFiltered

  const statusCounts = componentFiltered.reduce((acc, i) => {
    acc[i.status] = (acc[i.status] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  function toggleFilter(status: string) {
    onStatusFilter(statusFilter === status ? null : status)
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {statusFilter ? `${items.length} of ${componentFiltered.length}` : `${componentFiltered.length}`} issues
          {component && ` (component=${component})`}
          {statusFilter && ` — filtered: ${statusFilter}`}
        </Typography>
        <Stack direction="row" spacing={0.5} sx={{ flexWrap: 'wrap' }}>
          {Object.entries(statusCounts)
            .sort(([a], [b]) => (STATUS_RANK[a] ?? 99) - (STATUS_RANK[b] ?? 99))
            .map(([status, count]) => (
              <Chip
                key={status}
                label={`${status} (${count})`}
                size="small"
                color={STATUS_COLORS[status] || 'default'}
                variant={statusFilter === status ? 'filled' : 'outlined'}
                onClick={() => toggleFilter(status)}
                sx={{ cursor: 'pointer' }}
              />
            ))}
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
        <Box sx={{ ml: 'auto' }}>
          <IssueSearch slug={slug} />
        </Box>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map(i => {
          const pLabel = priorityLabel(i.priority)
          return (
            <Card key={i.id} variant="outlined">
              <CardActionArea
                component={Link as any}
                to="/project/$slug/issues/$id"
                params={{ slug, id: `IS-${i.id}` }}
              >
                <CardContent sx={{ py: 1.25, display: 'flex', alignItems: 'center', gap: 1 }}>
                  <Chip
                    label={pLabel}
                    size="small"
                    color={PRIORITY_COLORS[pLabel] || 'default'}
                    sx={{ minWidth: 70 }}
                  />
                  <Typography variant="body2" sx={{ flex: 1 }}>
                    IS-{i.id}: {issueDisplayTitle(i.title, i.context)}
                  </Typography>
                  {(i.issue_type && i.issue_type !== 'ops') && (
                    <Chip
                      label={i.issue_type}
                      size="small"
                      color={i.issue_type === 'ask' ? 'info' : 'secondary'}
                      variant="outlined"
                    />
                  )}
                  <Chip
                    label={i.status}
                    size="small"
                    color={STATUS_COLORS[i.status] || 'default'}
                    variant="outlined"
                  />
                </CardContent>
              </CardActionArea>
            </Card>
          )
        })}
        {items.length === 0 && !isLoading && (
          <Typography variant="body2" color="text.secondary">No issues.</Typography>
        )}
        <LoadMore loaded={allItems.length} total={total} onLoadMore={loadMore} />
      </Box>
    </Box>
  )
}
