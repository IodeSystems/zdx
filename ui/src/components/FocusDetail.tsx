import { Link, useRouter } from '@tanstack/react-router'
import {
  Autocomplete,
  Box,
  Button,
  Chip,
  TextField,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useState } from 'react'
import {
  useListFocuses,
  useSetFocusStatus,
  useAddFocusBlocker,
  useRemoveFocusBlocker,
  useSearchIssues,
  type IssueItem,
  type FocusAttribution,
} from '../api'
import { MarkdownContent } from './MarkdownContent'

const PRIORITY_LABELS: Record<number, string> = { 1: 'urgent', 2: 'high', 3: 'medium', 4: 'low' }
const PRIORITY_COLORS: Record<string, 'error' | 'warning' | 'info' | 'default'> = {
  urgent: 'error',
  high: 'warning',
  medium: 'info',
  low: 'default',
}
const STATUS_COLORS: Record<string, 'success' | 'info' | 'default'> = {
  active: 'info',
  done: 'success',
  archived: 'default',
}

function isConcern(a: FocusAttribution) {
  return a.status === 'open'
}

export function FocusDetail({ slug, name }: { slug: string; name: string }) {
  const router = useRouter()
  const { data: allFocuses, isLoading } = useListFocuses(slug)
  const setFocusStatus = useSetFocusStatus()
  const addBlocker = useAddFocusBlocker()
  const removeBlocker = useRemoveFocusBlocker()
  const [issueSearch, setIssueSearch] = useState('')
  const [justification, setJustification] = useState('')
  const { data: searchResults } = useSearchIssues(slug, issueSearch, issueSearch.length > 1)

  const focus = allFocuses?.find(t => t.name === name)

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (!focus) {
    return (
      <Box>
        <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
          Back
        </Button>
        <Typography color="error">Focus "{name}" not found.</Typography>
      </Box>
    )
  }

  const attributions = focus.attributions ?? []
  const attributionIds = attributions.map(a => a.id)
  const concernCount = attributions.filter(isConcern).length
  const pLabel = PRIORITY_LABELS[focus.priority] ?? 'medium'
  const availableStatuses = ['active', 'done', 'archived'].filter(s => s !== focus.status)

  return (
    <Box>
      <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
        Back
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        FO-{focus.id}: {focus.name}
      </Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip label={pLabel} size="small" color={PRIORITY_COLORS[pLabel] || 'default'} />
        <Chip label={focus.status} size="small" color={STATUS_COLORS[focus.status] || 'default'} variant="outlined" />
        {availableStatuses.map(s => (
          <Button
            key={s}
            size="small"
            variant="outlined"
            onClick={() => setFocusStatus.mutate({ slug, focus: `FO-${focus.id}`, status: s })}
            disabled={setFocusStatus.isPending}
          >
            Mark {s}
          </Button>
        ))}
      </Box>

      {focus.description && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Description
          </Typography>
          <MarkdownContent slug={slug}>{focus.description}</MarkdownContent>
        </Box>
      )}

      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
          Attributions ({attributions.length}){concernCount > 0 && ` — ${concernCount} concern${concernCount !== 1 ? 's' : ''}`}
        </Typography>
        {attributions.length === 0 && (
          <Typography variant="body2" color="text.secondary">No issues linked.</Typography>
        )}
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
          {attributions.map(a => (
            <Box key={a.id} sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
              <Chip
                label={a.id}
                size="small"
                variant={isConcern(a) ? 'filled' : 'outlined'}
                color={isConcern(a) ? 'warning' : 'default'}
                component={Link as any}
                to="/project/$slug/issues/$id"
                params={{ slug, id: a.id }}
                clickable
                sx={{ textDecoration: 'none', mt: a.justification ? 0.25 : 0 }}
                onDelete={() => removeBlocker.mutate({ slug, focus: `FO-${focus.id}`, issue: a.id })}
              />
              <Box>
                <Chip
                  label={a.status}
                  size="small"
                  variant="outlined"
                  color={isConcern(a) ? 'warning' : 'success'}
                  sx={{ mr: 0.5 }}
                />
                {a.justification && (
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.25 }}>
                    {a.justification}
                  </Typography>
                )}
              </Box>
            </Box>
          ))}
        </Box>

        <Box sx={{ mt: 1, display: 'flex', gap: 1, alignItems: 'flex-start', flexWrap: 'wrap' }}>
          <Autocomplete<IssueItem>
            size="small"
            options={(searchResults ?? []).filter(i => !attributionIds.includes(`IS-${i.id}`))}
            getOptionLabel={(o) => `IS-${o.id}: ${o.title || o.context?.slice(0, 60) || '(no title)'}`}
            inputValue={issueSearch}
            onInputChange={(_, v) => setIssueSearch(v)}
            onChange={(_, v) => {
              if (v) {
                addBlocker.mutate({ slug, focus: `FO-${focus.id}`, issue: `IS-${v.id}`, justification })
                setIssueSearch('')
                setJustification('')
              }
            }}
            value={null}
            renderInput={(params) => <TextField {...params} label="Add attribution" placeholder="Search issues..." />}
            sx={{ minWidth: 280 }}
            noOptionsText={issueSearch.length < 2 ? 'Type to search...' : 'No issues found'}
            isOptionEqualToValue={(o, v) => o.id === v.id}
          />
          <TextField
            size="small"
            label="Justification (optional)"
            value={justification}
            onChange={e => setJustification(e.target.value)}
            sx={{ minWidth: 220 }}
            placeholder="Why is this attributed?"
          />
        </Box>
      </Box>

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3 }}>
        Created: {new Date(focus.created_at).toLocaleString()}
      </Typography>
    </Box>
  )
}
