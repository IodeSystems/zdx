import { Link } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Typography,
  InputAdornment,
  TextField,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import { useState } from 'react'
import { useFeatures, type FeatureResp } from '../api'

type Feature = FeatureResp

export function FeaturesTab({
  slug,
  componentSlug = 'all',
}: {
  slug: string
  componentSlug?: string
  categoryFilter?: string
}) {
  const { data, isLoading } = useFeatures(slug)
  const [search, setSearch] = useState('')

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>
  const allFeatures: Feature[] = data || []

  const component = componentSlug === 'all' ? '' : componentSlug
  const componentFiltered = component ? allFeatures.filter(f => f.component === component) : allFeatures

  const features = search
    ? componentFiltered.filter(f => f.name.toLowerCase().includes(search.toLowerCase()))
    : componentFiltered

  return (
    <Box>
      <TextField
        size="small"
        placeholder="Search features…"
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

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {features.map(f => (
          <FeatureCard key={f.id} feature={f} slug={slug} componentSlug={componentSlug} />
        ))}

        {features.length === 0 && (
          <Typography variant="body2" color="text.secondary">No features.</Typography>
        )}
      </Box>
    </Box>
  )
}

function FeatureCard({ feature: f, slug, componentSlug }: { feature: Feature; slug: string; componentSlug: string }) {
  return (
    <Card variant="outlined">
      <CardActionArea
        component={Link as any}
        to="/project/$slug/$component/features/$name"
        params={{ slug, component: componentSlug, name: f.name }}
      >
        <CardContent sx={{ py: 1.25 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography variant="body2" sx={{ fontWeight: 600, flex: 1 }}>{f.name}</Typography>
          </Box>
          {f.description && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.25 }}>
              {f.description}
            </Typography>
          )}
          {f.component && (
            <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 0.25 }}>
              {f.component}
            </Typography>
          )}
        </CardContent>
      </CardActionArea>
    </Card>
  )
}
