import { useState } from 'react'
import {
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Box,
  Chip,
  Typography,
} from '@mui/material'
import { ExpandMore as ExpandMoreIcon } from '@mui/icons-material'
import { useDemos, useDemoContent, type DemoListItem } from '../api'

function CLIDemoPlayer({ name }: { name: string }) {
  const { data, isLoading } = useDemoContent('cli', name)

  if (isLoading) return <Typography variant="body2" color="text.secondary">Loading...</Typography>
  if (!data) return null

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
