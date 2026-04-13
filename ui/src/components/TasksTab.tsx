import { Link } from '@tanstack/react-router'
import { Box, Chip, InputAdornment, Stack, TextField, Typography } from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import { useTasks, type TaskResp } from '../api'

type Task = TaskResp

function TaskIcon({ status }: { status: string }) {
  const icon = status === 'done' ? '\u2713' : status === 'blocked' ? '\u2717' : '\u25CB'
  const color = status === 'done' ? 'success.main' : status === 'blocked' ? 'error.main' : 'warning.main'
  return <Typography component="span" sx={{ color, fontWeight: 600, mr: 1 }}>{icon}</Typography>
}

const STATUS_COLORS: Record<string, 'success' | 'error' | 'warning' | 'default'> = {
  done: 'success',
  blocked: 'error',
}

const STATUS_RANK: Record<string, number> = { blocked: 0, open: 1, 'in-progress': 2, done: 3 }

export function TasksTab({
  slug,
  componentSlug,
  statusFilter,
  search,
  onStatusFilter,
  onSearch,
}: {
  slug: string
  componentSlug: string
  statusFilter: string | null
  page: number
  search: string
  onStatusFilter: (status: string | null) => void
  onPageChange: (page: number) => void
  onSearch: (search: string) => void
}) {
  const { data, isLoading } = useTasks(slug)

  if (isLoading && !data) return <Typography color="text.secondary">Loading...</Typography>

  const allTasks: Task[] = data ?? []

  const filtered = statusFilter ? allTasks.filter(t => t.status === statusFilter) : allTasks
  const tasks = search
    ? filtered.filter(t =>
        t.text.toLowerCase().includes(search.toLowerCase()) ||
        t.feature.toLowerCase().includes(search.toLowerCase()) ||
        t.id.toLowerCase().includes(search.toLowerCase())
      )
    : filtered

  const statusCounts = allTasks.reduce((acc, t) => {
    acc[t.status] = (acc[t.status] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  function toggleFilter(status: string) {
    onStatusFilter(statusFilter === status ? null : status)
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">
          {statusFilter ? `${tasks.length} of ${allTasks.length}` : `${allTasks.length}`} tasks
          {statusFilter && ` — filtered: ${statusFilter}`}
          {search && ` — search: "${search}"`}
        </Typography>
        <Stack direction="row" spacing={0.5} sx={{ flexWrap: 'wrap' }}>
          {Object.entries(statusCounts).sort(([a], [b]) => (STATUS_RANK[a] ?? 99) - (STATUS_RANK[b] ?? 99)).map(([status, count]) => (
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
      </Box>
      <TextField
        size="small"
        placeholder="Search tasks…"
        value={search}
        onChange={(e) => onSearch(e.target.value)}
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
            {t.id}
          </Typography>
          <Box sx={{ flex: 1 }}>
            <Link
              to="/project/$slug/$component/tasks/$id"
              params={{ slug, component: componentSlug, id: t.id }}
              style={{ textDecoration: 'none', color: 'inherit' }}
            >
              <Typography variant="body2">{t.text}</Typography>
            </Link>
            <Typography variant="caption" color="text.secondary">{t.feature}</Typography>
            {t.reason && (
              <Typography variant="caption" color="warning.main" sx={{ display: 'block' }}>{t.reason}</Typography>
            )}
          </Box>
        </Box>
      ))}
      {tasks.length === 0 && !isLoading && (
        <Typography variant="body2" color="text.secondary">No tasks.</Typography>
      )}
    </Box>
  )
}
