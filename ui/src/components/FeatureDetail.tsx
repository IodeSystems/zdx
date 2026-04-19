import { useEffect, useState } from 'react'
import { Link, useRouter } from '@tanstack/react-router'
import {
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Box,
  Button,
  Chip,
  Typography,
} from '@mui/material'
import {
  ArrowBack as ArrowBackIcon,
  ExpandMore as ExpandMoreIcon,
} from '@mui/icons-material'
import { useFeature, useTasks, useSpecTests, type TaskItem, type SpecItem as BaseSpecItem } from '../api'
import { CommentsAndRevisions } from './CommentsAndRevisions'
import { DemosSection } from './DemoPlayer'
import { MarkdownContent } from './MarkdownContent'

type FeatureTask = TaskItem
type Spec = BaseSpecItem & { deferred?: boolean }

function TaskIcon({ status }: { status: string }) {
  const icon = status === 'done' ? '\u2713' : status === 'blocked' ? '\u2717' : '\u25CB'
  const color = status === 'done' ? 'success.main' : status === 'blocked' ? 'error.main' : 'warning.main'
  return <Typography component="span" sx={{ color, fontWeight: 600, mr: 1 }}>{icon}</Typography>
}

function TestStatusIcon({ status }: { status: string }) {
  const icon = status === 'pass' ? '\u2713' : status === 'fail' ? '\u2717' : '\u25CB'
  const color = status === 'pass' ? 'success.main' : status === 'fail' ? 'error.main' : 'text.secondary'
  return <Typography component="span" sx={{ color, fontWeight: 600, mr: 0.5, fontSize: '0.85rem' }}>{icon}</Typography>
}

function SpecRow({ spec, slug }: { spec: Spec; slug: string }) {
  const [expanded, setExpanded] = useState(false)
  const { data: tests, isLoading } = useSpecTests(spec.id, expanded)

  return (
    <Accordion
      disableGutters
      variant="outlined"
      expanded={expanded}
      onChange={(_, e) => setExpanded(e)}
      sx={{ '&:before': { display: 'none' } }}
    >
      <AccordionSummary expandIcon={<ExpandMoreIcon />} sx={{ minHeight: 40, '& .MuiAccordionSummary-content': { my: 0.5 } }}>
        <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center', flex: 1 }}>
          <Chip label={spec.kind} size="small" variant="outlined" color="info" />
          {spec.deferred && <Chip label="deferred" size="small" color="warning" />}
          {spec.deferred && spec.deferred_reason && (
            <Chip label={spec.deferred_reason} size="small" variant="outlined" color="warning" sx={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }} />
          )}
          <Link
            to="/project/$slug/specs/$specId"
            params={{ slug, specId: String(spec.id) }}
            style={{ textDecoration: 'none', color: 'inherit', flex: 1 }}
            onClick={e => e.stopPropagation()}
          >
            <Typography variant="body2" sx={{ '&:hover': { textDecoration: 'underline' } }}>{spec.description}</Typography>
          </Link>
        </Box>
      </AccordionSummary>
      <AccordionDetails sx={{ pt: 0 }}>
        {isLoading && <Typography variant="caption" color="text.secondary">Loading tests...</Typography>}
        {tests && tests.length === 0 && (
          <Typography variant="caption" color="text.secondary">No linked tests</Typography>
        )}
        {tests && tests.length > 0 && (
          <Box>
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
              Linked tests ({tests.length})
            </Typography>
            {tests.map(t => (
              <Box key={t.id} sx={{ display: 'flex', alignItems: 'center', gap: 0.5, py: 0.25 }}>
                <TestStatusIcon status={t.status} />
                <Chip label={t.layer} size="small" variant="outlined" sx={{ height: 18, fontSize: '0.7rem' }} />
                <Link
                  to="/project/$slug/tests"
                  params={{ slug }}
                  style={{ textDecoration: 'none', color: 'inherit' }}
                >
                  <Typography variant="body2" sx={{ fontSize: '0.85rem', '&:hover': { textDecoration: 'underline' } }}>
                    {t.component}/{t.name}
                  </Typography>
                </Link>
              </Box>
            ))}
          </Box>
        )}
      </AccordionDetails>
    </Accordion>
  )
}

export function FeatureDetail({
  slug,
  name,
}: {
  slug: string
  name: string
}) {
  const { data: feature, isLoading } = useFeature(slug, name)
  const { data: tasksData } = useTasks(slug, { feature: name })
  const router = useRouter()

  useEffect(() => {
    if (!feature) return
    document.title = `${name}: ${feature.description || name} | zdx`
    return () => { document.title = 'zdx' }
  }, [name, feature?.description])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  const tasks: FeatureTask[] = tasksData?.tasks ?? []
  const specList: Spec[] = feature?.specs ?? []

  if (!feature) {
    return (
      <Box>
        <Button
          startIcon={<ArrowBackIcon />}
          size="small"
          sx={{ mb: 2 }}
          onClick={() => router.history.go(-1)}
        >
          Back
        </Button>
        <Typography color="error">Feature {name} not found.</Typography>
      </Box>
    )
  }

  const grouped = tasks.reduce((acc, t) => {
    const k = t.status || 'ready'
    ;(acc[k] ||= []).push(t)
    return acc
  }, {} as Record<string, FeatureTask[]>)

  return (
    <Box>
      <Button
        startIcon={<ArrowBackIcon />}
        size="small"
        sx={{ mb: 2 }}
        onClick={() => router.history.go(-1)}
      >
        Back
      </Button>

      <Typography variant="h5" sx={{ mb: 1 }}>
        {feature.name}
      </Typography>

      {(feature.category || feature.component) && (
        <Box sx={{ mb: 1, display: 'flex', gap: 1, flexWrap: 'wrap' }}>
          {feature.category && (
            <Chip label={feature.category} size="small" color="primary" variant="outlined" />
          )}
          {feature.component && (
            <Chip label={feature.component} size="small" variant="outlined" />
          )}
        </Box>
      )}

      {feature.what && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            What
          </Typography>
          <MarkdownContent slug={slug}>{feature.what}</MarkdownContent>
        </Box>
      )}

      {feature.why && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Why
          </Typography>
          <MarkdownContent slug={slug}>{feature.why}</MarkdownContent>
        </Box>
      )}

      {feature.done_when && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Done when
          </Typography>
          <MarkdownContent slug={slug}>{feature.done_when}</MarkdownContent>
        </Box>
      )}

      {feature.description && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Description
          </Typography>
          <MarkdownContent slug={slug}>{feature.description}</MarkdownContent>
        </Box>
      )}

      {specList.length > 0 && (
        <Box sx={{ mb: 2, mt: 2 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Specs ({specList.length})
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {specList.map((s: Spec) => <SpecRow key={s.id} spec={s} slug={slug} />)}
          </Box>
        </Box>
      )}

      <DemosSection slug={slug} />

      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1, mt: 2 }}>
        Tasks ({tasks.length})
      </Typography>
      {tasks.length === 0 ? (
        <Typography variant="body2" color="text.secondary">No tasks.</Typography>
      ) : (
        Object.entries(grouped).map(([status, sTasks]) => (
          <Box key={status} sx={{ mb: 2 }}>
            <Typography variant="caption" color="text.secondary" sx={{ textTransform: 'uppercase', display: 'block', mb: 0.5 }}>
              {status} ({sTasks.length})
            </Typography>
            {sTasks.map(t => (
              <Box key={t.id} sx={{ display: 'flex', alignItems: 'flex-start', py: 0.5, borderBottom: 1, borderColor: 'divider' }}>
                <TaskIcon status={t.status} />
                <Box sx={{ flex: 1 }}>
                  <Link
                    to="/project/$slug/tasks/$id"
                    params={{ slug, id: `TK-${t.id}` }}
                    style={{ textDecoration: 'none', color: 'inherit' }}
                  >
                    <Typography variant="body2">{t.title || t.text}</Typography>
                  </Link>
                  {t.reason && (
                    <Typography variant="caption" color="warning.main">{t.reason}</Typography>
                  )}
                </Box>
              </Box>
            ))}
          </Box>
        ))
      )}
      <CommentsAndRevisions slug={slug} targetType="feature" targetId={name} />
    </Box>
  )
}
