import { createFileRoute } from '@tanstack/react-router'
import { useState, useDeferredValue } from 'react'
import {
  Box,
  Chip,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { Clear as ClearIcon, Search as SearchIcon } from '@mui/icons-material'
import {
  DemosSection,
  type DemoStatusFilter,
  type DemoTypeFilter,
} from '../../../../components/DemoPlayer'
import { DemosDashboard } from '../../../../components/DemosDashboard'
import { useDemos } from '../../../../api'

function DemosIndexRoute() {
  const { slug } = Route.useParams()
  const { data: demos } = useDemos(slug)
  const list = demos ?? []
  const [search, setSearch] = useState('')
  const deferredSearch = useDeferredValue(search)
  const [statusFilter, setStatusFilter] = useState<DemoStatusFilter[]>([])
  const [typeFilter, setTypeFilter] = useState<DemoTypeFilter[]>([])
  const hasFilters = statusFilter.length > 0 || typeFilter.length > 0

  function toggleStatus(s: DemoStatusFilter) {
    setStatusFilter(prev => (prev.includes(s) ? prev.filter(x => x !== s) : [...prev, s]))
  }
  function toggleType(t: DemoTypeFilter) {
    setTypeFilter(prev => (prev.includes(t) ? prev.filter(x => x !== t) : [...prev, t]))
  }
  function clearFilters() {
    setStatusFilter([])
    setTypeFilter([])
  }

  return (
    <Stack spacing={2}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <Typography variant="h5" sx={{ fontWeight: 600, mr: 1 }}>Demos</Typography>
        <Box sx={{ flex: 1, minWidth: 200, maxWidth: { xs: '100%', sm: 360 } }}>
          <TextField
            fullWidth
            size="small"
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search demos…"
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" />
                  </InputAdornment>
                ),
                endAdornment: search ? (
                  <InputAdornment position="end">
                    <IconButton size="small" onClick={() => setSearch('')} edge="end">
                      <ClearIcon fontSize="small" />
                    </IconButton>
                  </InputAdornment>
                ) : null,
              },
            }}
          />
        </Box>
      </Box>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1 }}>
        <Typography variant="caption" color="text.secondary">Status:</Typography>
        <Chip
          label="pass"
          size="small"
          color={statusFilter.includes('pass') ? 'success' : 'default'}
          variant={statusFilter.includes('pass') ? 'filled' : 'outlined'}
          onClick={() => toggleStatus('pass')}
          sx={{ cursor: 'pointer' }}
        />
        <Chip
          label="fail"
          size="small"
          color={statusFilter.includes('fail') ? 'error' : 'default'}
          variant={statusFilter.includes('fail') ? 'filled' : 'outlined'}
          onClick={() => toggleStatus('fail')}
          sx={{ cursor: 'pointer' }}
        />
        <Box sx={{ width: 8 }} />
        <Typography variant="caption" color="text.secondary">Type:</Typography>
        <Chip
          label="cli"
          size="small"
          color={typeFilter.includes('cli') ? 'info' : 'default'}
          variant={typeFilter.includes('cli') ? 'filled' : 'outlined'}
          onClick={() => toggleType('cli')}
          sx={{ cursor: 'pointer' }}
        />
        <Chip
          label="video"
          size="small"
          color={typeFilter.includes('video') ? 'secondary' : 'default'}
          variant={typeFilter.includes('video') ? 'filled' : 'outlined'}
          onClick={() => toggleType('video')}
          sx={{ cursor: 'pointer' }}
        />
        {hasFilters && (
          <Chip
            label="clear"
            size="small"
            variant="outlined"
            onClick={clearFilters}
            sx={{ cursor: 'pointer' }}
          />
        )}
      </Box>
      <DemosDashboard demos={list} />
      <DemosSection
        slug={slug}
        demos={list}
        query={deferredSearch}
        statusFilter={statusFilter}
        typeFilter={typeFilter}
      />
    </Stack>
  )
}

export const Route = createFileRoute('/project/$slug/demos/')({
  component: DemosIndexRoute,
})
