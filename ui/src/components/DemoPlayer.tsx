import { useEffect, useMemo, useState } from 'react'
import {
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Box,
  Chip,
  Divider,
  Stack,
  Typography,
} from '@mui/material'
import { ExpandMore as ExpandMoreIcon } from '@mui/icons-material'
import { Link } from '@tanstack/react-router'
import {
  useDemos,
  useSpecDemos,
  type DemoListItem,
  type CLIDemoData,
  type SpecDemoItem,
} from '../api'

function CLIDemoBody({ data }: { data: CLIDemoData }) {
  return (
    <Box sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>
      {data.recorded_at && (
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
          Recorded {new Date(data.recorded_at).toLocaleDateString()}
        </Typography>
      )}
      {data.steps.map((step, i) => (
        <Box key={i} sx={{ mb: 1.5, borderLeft: 2, borderColor: step.exit_code === 0 ? 'success.main' : 'error.main', pl: 1.5 }}>
          <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: 0.5 }}>
            <Typography component="span" sx={{ color: 'info.main', fontWeight: 600, fontFamily: 'monospace', fontSize: '0.85rem' }}>
              $ {step.cmd}
            </Typography>
            <Chip label={`${step.duration_ms}ms`} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.7rem' }} />
          </Box>
          {step.stdout && (
            <Box component="pre" sx={{
              m: 0, p: 1, bgcolor: 'background.default', borderRadius: 1,
              overflow: 'auto', fontSize: '0.8rem', lineHeight: 1.4, whiteSpace: 'pre-wrap',
            }}>
              {step.stdout}
            </Box>
          )}
          {step.stderr && (
            <Box component="pre" sx={{
              m: 0, p: 1, mt: 0.5, bgcolor: 'error.dark', borderRadius: 1, color: 'error.contrastText',
              overflow: 'auto', fontSize: '0.8rem', lineHeight: 1.4, whiteSpace: 'pre-wrap',
            }}>
              {step.stderr}
            </Box>
          )}
        </Box>
      ))}
    </Box>
  )
}

export function CLIDemoPlayerByUrl({ url }: { url: string }) {
  const [data, setData] = useState<CLIDemoData | null>(null)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => {
    let cancelled = false
    fetch(url)
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then((j: CLIDemoData) => { if (!cancelled) setData(j) })
      .catch((e: Error) => { if (!cancelled) setError(e.message) })
    return () => { cancelled = true }
  }, [url])
  if (error) return <Typography variant="body2" color="error">Failed to load: {error}</Typography>
  if (!data) return <Typography variant="body2" color="text.secondary">Loading...</Typography>
  return <CLIDemoBody data={data} />
}

export function VideoDemoPlayerByUrl({ url }: { url: string }) {
  const [error, setError] = useState<string | null>(null)
  return (
    <Box sx={{ maxWidth: 800 }}>
      {error ? (
        <Box sx={{ p: 2, bgcolor: 'background.default', borderRadius: 1, border: 1, borderColor: 'divider' }}>
          <Typography variant="body2" color="error" sx={{ mb: 1 }}>
            Video failed to load ({error}). WebM is not supported in Safari — use Chrome or Firefox, or download the file.
          </Typography>
          <Typography variant="body2">
            <a href={url} download style={{ color: 'inherit' }}>Download video</a>
          </Typography>
        </Box>
      ) : (
        <video
          controls
          preload="metadata"
          style={{ width: '100%', borderRadius: 4 }}
          src={url}
          onError={(e) => {
            const v = e.currentTarget
            const code = v.error?.code ?? 0
            const msg: Record<number, string> = {
              1: 'aborted', 2: 'network error', 3: 'decode error', 4: 'format not supported',
            }
            setError(msg[code] ?? `error ${code}`)
          }}
        />
      )}
    </Box>
  )
}

function statusColor(status: string): 'success' | 'error' | 'default' {
  if (status === 'pass') return 'success'
  if (status === 'fail') return 'error'
  return 'default'
}

function DemoItem({ demo, slug }: { demo: DemoListItem; slug: string }) {
  const label = demo.test_component && demo.test_name
    ? `${demo.test_component}/${demo.test_name}`
    : demo.name
  return (
    <Accordion disableGutters variant="outlined" sx={{ '&:before': { display: 'none' } }}>
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap', flex: 1 }}>
          <Chip
            label={demo.test_status || 'unknown'}
            size="small"
            color={statusColor(demo.test_status)}
          />
          <Chip label={demo.type} size="small" variant="outlined" color={demo.type === 'cli' ? 'info' : 'secondary'} />
          <Typography variant="body2" sx={{ flex: 1 }}>{label}</Typography>
          {demo.test_duration_ms > 0 && (
            <Typography variant="caption" color="text.secondary">
              {demo.test_duration_ms < 1000
                ? `${demo.test_duration_ms}ms`
                : `${(demo.test_duration_ms / 1000).toFixed(1)}s`}
            </Typography>
          )}
          <Link
            to="/project/$slug/demos/$demoId"
            params={{ slug, demoId: String(demo.id) }}
            onClick={(e) => e.stopPropagation()}
            style={{ fontSize: '0.75rem' }}
          >
            view
          </Link>
        </Box>
      </AccordionSummary>
      <AccordionDetails>
        {demo.type === 'cli' ? <CLIDemoPlayerByUrl url={demo.url} /> : <VideoDemoPlayerByUrl url={demo.url} />}
      </AccordionDetails>
    </Accordion>
  )
}

export function DemosSection({ slug, demos: demosProp }: { slug: string; demos?: DemoListItem[] }) {
  const { data: fetched, isLoading } = useDemos(demosProp ? '' : slug)
  const list = demosProp ?? fetched ?? []

  const grouped = useMemo(() => {
    const m = new Map<string, DemoListItem[]>()
    for (const d of list) {
      const key = d.test_component || 'Uncategorized'
      const group = m.get(key) ?? []
      group.push(d)
      m.set(key, group)
    }
    return [...m.entries()].sort((a, b) => a[0].localeCompare(b[0]))
  }, [list])

  if (isLoading) {
    return <Typography variant="body2" color="text.secondary">Loading...</Typography>
  }
  if (list.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No demos recorded yet. Run <code>dx test --layer demo</code> to record one; uploaded demos appear here automatically.
      </Typography>
    )
  }

  return (
    <Stack spacing={3}>
      {grouped.map(([component, items]) => (
        <Box key={component}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 0.75 }}>
            {component}
          </Typography>
          <Divider sx={{ mb: 1 }} />
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {items.map((d) => <DemoItem key={d.id} demo={d} slug={slug} />)}
          </Box>
        </Box>
      ))}
    </Stack>
  )
}

function SpecDemoItemRow({ demo, slug }: { demo: SpecDemoItem; slug?: string }) {
  return (
    <Accordion disableGutters variant="outlined" sx={{ '&:before': { display: 'none' } }}>
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flex: 1 }}>
          <Chip label={demo.type} size="small" variant="outlined" color={demo.type === 'cli' ? 'info' : 'secondary'} />
          <Typography variant="body2">{demo.test_component}/{demo.test_name}</Typography>
          {slug && (
            <Link
              to="/project/$slug/demos/$demoId"
              params={{ slug, demoId: String(demo.id) }}
              style={{ marginLeft: 'auto', fontSize: '0.75rem' }}
              onClick={(e) => e.stopPropagation()}
            >
              details
            </Link>
          )}
        </Box>
      </AccordionSummary>
      <AccordionDetails>
        {demo.type === 'cli' ? <CLIDemoPlayerByUrl url={demo.url} /> : <VideoDemoPlayerByUrl url={demo.url} />}
      </AccordionDetails>
    </Accordion>
  )
}

export function SpecDemos({ specId, slug }: { specId: number; slug?: string }) {
  const { data: demos, isLoading } = useSpecDemos(specId)
  const list = demos ?? []
  return (
    <Box sx={{ mb: 2, mt: 3 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
        Demo coverage ({list.length})
      </Typography>
      {isLoading ? (
        <Typography variant="body2" color="text.secondary">Loading...</Typography>
      ) : list.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No demo coverage for this spec. Run <code>dx test --layer demo</code> to record one, or link an existing recording to a covering test.
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {list.map((d) => <SpecDemoItemRow key={d.id} demo={d} slug={slug} />)}
        </Box>
      )}
    </Box>
  )
}
