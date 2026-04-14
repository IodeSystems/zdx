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
  useListThemes,
  useSetThemeStatus,
  useAddThemeBlocker,
  useRemoveThemeBlocker,
  useSearchIssues,
  type IssueItem,
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

export function ThemeDetail({ slug, name }: { slug: string; name: string }) {
  const router = useRouter()
  const { data: allThemes, isLoading } = useListThemes(slug)
  const setThemeStatus = useSetThemeStatus()
  const addBlocker = useAddThemeBlocker()
  const removeBlocker = useRemoveThemeBlocker()
  const [issueSearch, setIssueSearch] = useState('')
  const { data: searchResults } = useSearchIssues(slug, issueSearch, issueSearch.length > 1)

  const theme = allThemes?.find(t => t.name === name)

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  if (!theme) {
    return (
      <Box>
        <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
          Back
        </Button>
        <Typography color="error">Theme "{name}" not found.</Typography>
      </Box>
    )
  }

  const blockerIds = theme.blockers ? theme.blockers.split(',').filter(Boolean) : []
  const pLabel = PRIORITY_LABELS[theme.priority] ?? 'medium'
  const availableStatuses = ['active', 'done', 'archived'].filter(s => s !== theme.status)

  return (
    <Box>
      <Button startIcon={<ArrowBackIcon />} size="small" sx={{ mb: 2 }} onClick={() => router.history.go(-1)}>
        Back
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        TH-{theme.id}: {theme.name}
      </Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip label={pLabel} size="small" color={PRIORITY_COLORS[pLabel] || 'default'} />
        <Chip label={theme.status} size="small" color={STATUS_COLORS[theme.status] || 'default'} variant="outlined" />
        {availableStatuses.map(s => (
          <Button
            key={s}
            size="small"
            variant="outlined"
            onClick={() => setThemeStatus.mutate({ slug, theme: `TH-${theme.id}`, status: s })}
            disabled={setThemeStatus.isPending}
          >
            Mark {s}
          </Button>
        ))}
      </Box>

      {theme.description && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Description
          </Typography>
          <MarkdownContent slug={slug}>{theme.description}</MarkdownContent>
        </Box>
      )}

      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
          Blocking Issues ({blockerIds.length})
        </Typography>
        {blockerIds.length === 0 && (
          <Typography variant="body2" color="text.secondary">No issues linked.</Typography>
        )}
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
          {blockerIds.map(id => (
            <Box key={id} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Chip
                label={id}
                size="small"
                variant="outlined"
                component={Link as any}
                to="/project/$slug/issues/$id"
                params={{ slug, id }}
                clickable
                sx={{ textDecoration: 'none' }}
                onDelete={() => removeBlocker.mutate({ slug, theme: `TH-${theme.id}`, issue: id })}
              />
            </Box>
          ))}
        </Box>

        <Box sx={{ mt: 1 }}>
          <Autocomplete<IssueItem>
            size="small"
            options={(searchResults ?? []).filter(i => !blockerIds.includes(`IS-${i.id}`))}
            getOptionLabel={(o) => `IS-${o.id}: ${o.title || o.context?.slice(0, 60) || '(no title)'}`}
            inputValue={issueSearch}
            onInputChange={(_, v) => setIssueSearch(v)}
            onChange={(_, v) => {
              if (v) {
                addBlocker.mutate({ slug, theme: `TH-${theme.id}`, issue: `IS-${v.id}` })
                setIssueSearch('')
              }
            }}
            value={null}
            renderInput={(params) => <TextField {...params} label="Add blocker issue" placeholder="Search issues..." />}
            sx={{ maxWidth: 400 }}
            noOptionsText={issueSearch.length < 2 ? 'Type to search...' : 'No issues found'}
            isOptionEqualToValue={(o, v) => o.id === v.id}
          />
        </Box>
      </Box>

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3 }}>
        Created: {new Date(theme.created_at).toLocaleString()}
      </Typography>
    </Box>
  )
}
