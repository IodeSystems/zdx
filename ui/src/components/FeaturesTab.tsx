import { Link } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Typography,
  InputAdornment,
  TextField,
} from '@mui/material'
import { Search as SearchIcon } from '@mui/icons-material'
import { useState } from 'react'
import { useFeatures, type FeatureItem } from '../api'
import { useComponentFilter } from './ComponentContext'

type Feature = FeatureItem

export function FeaturesTab({
  slug,
}: {
  slug: string
  categoryFilter?: string
}) {
  const { component } = useComponentFilter()
  const { data, isLoading } = useFeatures(slug)
  const [search, setSearch] = useState('')

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  const allFeatures: Feature[] = data || []

  const componentFiltered = component ? allFeatures.filter(f => f.component === component) : allFeatures

  const features = search
    ? componentFiltered.filter(f =>
        f.name.toLowerCase().includes(search.toLowerCase()) ||
        f.category.toLowerCase().includes(search.toLowerCase())
      )
    : componentFiltered

  const grouped = features.reduce((acc, f) => {
    const cat = f.category || ''
    ;(acc[cat] ||= []).push(f)
    return acc
  }, {} as Record<string, Feature[]>)

  const cats = Object.keys(grouped).sort((a, b) => {
    if (a === '') return 1
    if (b === '') return -1
    return a.localeCompare(b)
  })

  return (
    <Box>
      <TextField
        size="small"
        placeholder="Search features or categories…"
        value={search}
        onChange={e => setSearch(e.target.value)}
        sx={{ mb: 2, width: '100%', maxWidth: 320 }}
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

      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 2 }}>
        {search
          ? `${features.length} feature${features.length !== 1 ? 's' : ''} matching "${search}"`
          : `${features.length} feature${features.length !== 1 ? 's' : ''}`}
      </Typography>

      {cats.map(cat => (
        <Box key={cat} sx={{ mb: 3 }}>
          <Typography
            variant="overline"
            color="text.secondary"
            sx={{ display: 'block', mb: 1, letterSpacing: 1 }}
          >
            {cat || 'Uncategorized'}
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {grouped[cat].map(f => (
              <FeatureCard key={f.id} feature={f} slug={slug} />
            ))}
          </Box>
        </Box>
      ))}

      {features.length === 0 && (
        <Typography variant="body2" color="text.secondary">No features.</Typography>
      )}
    </Box>
  )
}

function FeatureCard({ feature: f, slug }: { feature: Feature; slug: string }) {
  return (
    <Card variant="outlined">
      <CardActionArea
        component={Link as any}
        to="/project/$slug/features/$name"
        params={{ slug, name: f.name }}
      >
        <CardContent sx={{ py: 1.25 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography variant="body2" sx={{ fontWeight: 600, flex: 1 }}>{f.name}</Typography>
            {f.component && (
              <Chip label={f.component} size="small" variant="outlined" sx={{ fontSize: '0.7rem' }} />
            )}
          </Box>
          {f.description && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.25 }}>
              {f.description}
            </Typography>
          )}
        </CardContent>
      </CardActionArea>
    </Card>
  )
}
