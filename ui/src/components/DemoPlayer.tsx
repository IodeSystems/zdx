import { useEffect, useState } from 'react'
import {
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Box,
  Chip,
  Typography,
} from '@mui/material'
import { ExpandMore as ExpandMoreIcon } from '@mui/icons-material'
import {
  useDemos,
  useDemoContent,
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

function CLIDemoPlayer({ name }: { name: string }) {
  const { data, isLoading } = useDemoContent('cli', name)
  if (isLoading) return <Typography variant="body2" color="text.secondary">Loading...</Typography>
  if (!data) return null
  return <CLIDemoBody data={data} />
}

function CLIDemoPlayerByUrl({ url }: { url: string }) {
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

function VideoDemoPlayer({ name }: { name: string }) {
  return (
    <Box sx={{ maxWidth: 800 }}>
      <video
        controls
        preload="metadata"
        style={{ width: '100%', borderRadius: 4 }}
        src={`/api/dx/demos/video/${encodeURIComponent(name)}`}
      />
    </Box>
  )
}

function VideoDemoPlayerByUrl({ url }: { url: string }) {
  return (
    <Box sx={{ maxWidth: 800 }}>
      <video controls preload="metadata" style={{ width: '100%', borderRadius: 4 }} src={url} />
    </Box>
  )
}

function DemoItem({ demo }: { demo: DemoListItem }) {
  return (
    <Accordion disableGutters variant="outlined" sx={{ '&:before': { display: 'none' } }}>
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
          <Chip label={demo.type} size="small" variant="outlined" color={demo.type === 'cli' ? 'info' : 'secondary'} />
          <Typography variant="body2">{demo.name}</Typography>
        </Box>
      </AccordionSummary>
      <AccordionDetails>
        {demo.type === 'cli' ? <CLIDemoPlayer name={demo.name} /> : <VideoDemoPlayer name={demo.name} />}
      </AccordionDetails>
    </Accordion>
  )
}

export function DemosSection() {
  const { data: demos, isLoading } = useDemos()
  const [expanded, setExpanded] = useState(true)

  if (isLoading) return null
  if (!demos || demos.length === 0) return null

  return (
    <Box sx={{ mb: 2, mt: 2 }}>
      <Typography
        variant="subtitle2"
        color="text.secondary"
        sx={{ mb: 1, cursor: 'pointer' }}
        onClick={() => setExpanded(!expanded)}
      >
        Demos ({demos.length}) {expanded ? '▾' : '▸'}
      </Typography>
      {expanded && (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {demos.map((d) => <DemoItem key={`${d.type}-${d.name}`} demo={d} />)}
        </Box>
      )}
    </Box>
  )
}

function SpecDemoItemRow({ demo }: { demo: SpecDemoItem }) {
  return (
    <Accordion disableGutters variant="outlined" sx={{ '&:before': { display: 'none' } }}>
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
          <Chip label={demo.type} size="small" variant="outlined" color={demo.type === 'cli' ? 'info' : 'secondary'} />
          <Typography variant="body2">{demo.test_component}/{demo.test_name}</Typography>
        </Box>
      </AccordionSummary>
      <AccordionDetails>
        {demo.type === 'cli' ? <CLIDemoPlayerByUrl url={demo.url} /> : <VideoDemoPlayerByUrl url={demo.url} />}
      </AccordionDetails>
    </Accordion>
  )
}

export function SpecDemos({ specId }: { specId: number }) {
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
          {list.map((d) => <SpecDemoItemRow key={d.id} demo={d} />)}
        </Box>
      )}
    </Box>
  )
}
