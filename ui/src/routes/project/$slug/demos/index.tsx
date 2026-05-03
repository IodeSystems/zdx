import { createFileRoute } from '@tanstack/react-router'
import { useState, useDeferredValue } from 'react'
import {
  Box,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { Clear as ClearIcon, Search as SearchIcon } from '@mui/icons-material'
import { DemosSection } from '../../../../components/DemoPlayer'
import { DemosDashboard } from '../../../../components/DemosDashboard'
import { useDemos } from '../../../../api'

function DemosIndexRoute() {
  const { slug } = Route.useParams()
  const { data: demos } = useDemos(slug)
  const list = demos ?? []
  const [search, setSearch] = useState('')
  const deferredSearch = useDeferredValue(search)
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
      <DemosDashboard demos={list} />
      <DemosSection slug={slug} demos={list} query={deferredSearch} />
    </Stack>
  )
}

export const Route = createFileRoute('/project/$slug/demos/')({
  component: DemosIndexRoute,
})
