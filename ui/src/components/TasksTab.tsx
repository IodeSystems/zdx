import { useEffect, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Box, Chip, IconButton, InputAdornment, Stack, TextField, Typography } from '@mui/material'
import { Search as SearchIcon, NavigateBefore, NavigateNext } from '@mui/icons-material'
import { useTasks, type TaskItem } from '../api'

type Task = TaskItem

const TASKS_PAGE_SIZE = 50

function TaskIcon({ status }: { status: string }) {
  const icon = status === 'done' ? '\u2713' : status === 'blocked' ? '\u2717' : '\u25CB'
  const color = status === 'done' ? 'success.main' : status === 'blocked' ? 'error.main' : 'warning.main'
  return <Typography component="span" sx={{ color, fontWeight: 600, mr: 1 }}>{icon}</Typography>
}

const STATUS_COLORS: Record<string, 'success' | 'error' | 'warning' | 'default'> = {
  done: 'success',
  blocked: 'error',
}

const STATUSES = ['ready', 'active', 'blocked', 'done']

export function TasksTab({
  slug,
  statusFilter,
  page,
  search,
  onStatusFilter,
  onPageChange,
  onSearch,
}: {
  slug: string
  statusFilter: string | null
  page: number
  search: string
  onStatusFilter: (status: string | null) => void
  onPageChange: (page: number) => void
  onSearch: (search: string) => void
}) {
  const [localSearch, setLocalSearch] = useState(search)
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => { setLocalSearch(search) }, [search])

  function handleSearchChange(value: string) {
    setLocalSearch(value)
    clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => onSearch(value), 300)
  }

  const offset = (page - 1) * TASKS_PAGE_SIZE
  const { data, isLoading } = useTasks(
    slug,
    { status: statusFilter ?? undefined, search: search || undefined },
    TASKS_PAGE_SIZE,
    offset,
  )

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const tasks: Task[] = data?.tasks ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / TASKS_PAGE_SIZE))

  function toggleFilter(status: string) {
    onStatusFilter(statusFilter === status ? null : status)
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {total} tasks
          {statusFilter && ` — filtered: ${statusFilter}`}
          {search && ` — search: "${search}"`}
        </Typography>
        <Stack direction="row" spacing={0.5} sx={{ flexWrap: 'wrap' }}>
          {STATUSES.map((status) => (
            <Chip
              key={status}
              label={status}
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
      </Box>
      <TextField
        size="small"
        placeholder="Search tasks…"
        value={localSearch}
        onChange={(e) => handleSearchChange(e.target.value)}
        sx={{ mb: 2, width: 280 }}
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
      {tasks.map(t => (
        <Box
          key={t.id}
          sx={{ display: 'flex', alignItems: 'flex-start', gap: 1, py: 0.75, borderBottom: 1, borderColor: 'divider', '&:hover': { bgcolor: 'action.hover' } }}
        >
          <TaskIcon status={t.status} />
          <Typography variant="caption" color="text.disabled" sx={{ minWidth: 60, pt: 0.1 }}>
            TK-{t.id}
          </Typography>
          <Box sx={{ flex: 1 }}>
            <Link
              to="/project/$slug/tasks/$id"
              params={{ slug, id: `TK-${t.id}` }}
              style={{ textDecoration: 'none', color: 'inherit' }}
            >
              <Typography variant="body2">{t.text}</Typography>
            </Link>
            <Typography variant="caption" color="text.secondary">{t.feature}</Typography>
            {t.task_group && (
              <Typography variant="caption" color="text.disabled" sx={{ display: 'inline', ml: 1 }}>[{t.task_group}]</Typography>
            )}
            {t.reason && (
              <Typography variant="caption" color="warning.main" sx={{ display: 'block' }}>{t.reason}</Typography>
            )}
          </Box>
        </Box>
      ))}
      {tasks.length === 0 && !isLoading && (
        <Typography variant="body2" color="text.secondary">No tasks.</Typography>
      )}
      {totalPages > 1 && (
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 1, py: 1 }}>
          <IconButton size="small" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
            <NavigateBefore />
          </IconButton>
          <Typography variant="body2" color="text.secondary">
            Page {page} of {totalPages}
          </Typography>
          <IconButton size="small" disabled={page >= totalPages} onClick={() => onPageChange(page + 1)}>
            <NavigateNext />
          </IconButton>
        </Box>
      )}
    </Box>
  )
}
