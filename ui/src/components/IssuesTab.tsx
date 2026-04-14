import { useState, useEffect, useRef } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  Autocomplete,
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { useIssues, useSearchIssues, useSimilarIssues, type IssueItem, type SimilarIssueItem } from '../api'

type Issue = IssueItem

type SearchOption =
  | { group: 'Text match'; item: IssueItem }
  | { group: 'Similar'; item: SimilarIssueItem }

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

function IssueSearch({ slug, componentSlug }: { slug: string; componentSlug: string }) {
  const navigate = useNavigate()
  const [inputValue, setInputValue] = useState('')
  const [ftsQuery, setFtsQuery] = useState('')
  const [vecQuery, setVecQuery] = useState('')
  const [vecResults, setVecResults] = useState<SimilarIssueItem[]>([])
  const ftsTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const vecTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
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
    findSimilar.mutate({ slug, text: vecQuery, n: 5 }, {
      onSuccess: (items) => setVecResults(items),
      onError: () => setVecResults([]),
    })
  }, [vecQuery, slug]) // eslint-disable-line react-hooks/exhaustive-deps

  const { data: ftsData } = useSearchIssues(slug, ftsQuery, ftsQuery.length > 1)

  const ftsOptions: SearchOption[] = (ftsData ?? []).map(item => ({ group: 'Text match' as const, item }))
  const vecOptions: SearchOption[] = vecResults
    .filter(v => !(ftsData ?? []).some(f => `IS-${f.id}` === v.id))
    .map(item => ({ group: 'Similar' as const, item }))

  const options: SearchOption[] = [...ftsOptions, ...vecOptions]

  return (
    <Autocomplete<SearchOption, false, false, true>
      freeSolo
      inputValue={inputValue}
      onInputChange={(_, v) => setInputValue(v)}
      options={options}
      groupBy={o => (typeof o === 'string' ? '' : o.group)}
      getOptionLabel={o => {
        if (typeof o === 'string') return o
        if (o.group === 'Text match') return issueDisplayTitle(o.item.title, o.item.context)
        return o.item.title || o.item.id
      }}
      filterOptions={x => x}
      onChange={(_, value) => {
        if (!value || typeof value === 'string') return
        const id = value.group === 'Text match' ? `IS-${value.item.id}` : value.item.id
        navigate({ to: '/project/$slug/$component/issues/$id', params: { slug, component: componentSlug, id } })
        setInputValue('')
      }}
      renderInput={params => (
        <TextField
          {...params}
          size="small"
          placeholder="Search issues…"
          sx={{ minWidth: 240 }}
        />
      )}
      renderOption={(props, option) => {
        if (typeof option === 'string') return null
        const { key, ...rest } = props as { key: React.Key } & React.HTMLAttributes<HTMLLIElement>
        if (option.group === 'Text match') {
          const pLabel = priorityLabel(option.item.priority)
          return (
            <li key={key} {...rest}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%' }}>
                <Typography variant="caption" sx={{ color: 'text.secondary', minWidth: 40 }}>IS-{option.item.id}</Typography>
                <Chip label={pLabel} size="small" color={PRIORITY_COLORS[pLabel] || 'default'} sx={{ minWidth: 60 }} />
                <Typography variant="body2" sx={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {issueDisplayTitle(option.item.title, option.item.context)}
                </Typography>
              </Box>
            </li>
          )
        }
        return (
          <li key={key} {...rest}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%' }}>
              <Typography variant="caption" sx={{ color: 'text.secondary', minWidth: 40 }}>{option.item.id}</Typography>
              <Typography variant="body2" sx={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {option.item.title || option.item.id}
              </Typography>
              <Typography variant="caption" sx={{ color: 'text.secondary' }}>{(option.item.score * 100).toFixed(0)}%</Typography>
            </Box>
          </li>
        )
      }}
      sx={{ display: 'inline-flex' }}
    />
  )
}

export function IssuesTab({ slug, componentSlug = 'all' }: { slug: string; componentSlug?: string }) {
  const [statusFilter, setStatusFilter] = useState<string | null>(null)
  const { data, isLoading } = useIssues(slug)

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const allItems: Issue[] = data ?? []

  const component = componentSlug === 'all' ? '' : componentSlug
  const componentFiltered = component ? allItems.filter(i => i.component === component) : allItems

  const items = statusFilter
    ? componentFiltered.filter(i => i.status === statusFilter)
    : componentFiltered

  const statusCounts = componentFiltered.reduce((acc, i) => {
    acc[i.status] = (acc[i.status] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  function toggleFilter(status: string) {
    setStatusFilter(prev => prev === status ? null : status)
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {statusFilter ? `${items.length} of ${componentFiltered.length}` : `${componentFiltered.length}`} issues
          {componentSlug !== 'all' && ` (component=${componentSlug})`}
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
              onClick={() => setStatusFilter(null)}
              sx={{ cursor: 'pointer' }}
            />
          )}
        </Stack>
        <Box sx={{ ml: 'auto' }}>
          <IssueSearch slug={slug} componentSlug={componentSlug} />
        </Box>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map(i => {
          const pLabel = priorityLabel(i.priority)
          return (
            <Card key={i.id} variant="outlined">
              <CardActionArea
                component={Link as any}
                to="/project/$slug/$component/issues/$id"
                params={{ slug, component: componentSlug, id: `IS-${i.id}` }}
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
                    <Chip label={i.issue_type} size="small" color="secondary" variant="outlined" />
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
      </Box>
    </Box>
  )
}
