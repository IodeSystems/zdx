import { useEffect } from 'react'
import { Link, useRouter } from '@tanstack/react-router'
import {
  Box,
  Button,
  Chip,
  Stack,
  Typography,
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { useGetPlan } from '../api'
import { EventStream } from './EventStream'
import { MarkdownContent } from './MarkdownContent'

const STATUS_COLORS: Record<string, 'warning' | 'success' | 'default' | 'info'> = {
  draft: 'default',
  active: 'info',
  done: 'success',
  archived: 'default',
}

const STEP_STATUS_COLORS: Record<string, 'warning' | 'success' | 'default' | 'info'> = {
  pending: 'default',
  in_progress: 'info',
  done: 'success',
  blocked: 'warning',
}

export function PlanDetail({ slug, planId }: { slug: string; planId: number }) {
  const router = useRouter()
  const { data: plan, isLoading } = useGetPlan(slug, planId)

  useEffect(() => {
    if (!plan) return
    document.title = `Plan #${plan.id}: ${plan.title || '(no title)'} | zdx`
    return () => { document.title = 'zdx' }
  }, [plan?.id, plan?.title])

  if (isLoading) return <Typography color="text.secondary">Loading...</Typography>

  if (!plan) {
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
        <Typography color="error">Plan #{planId} not found.</Typography>
      </Box>
    )
  }

  const steps = plan.steps ?? []

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
        Plan #{plan.id}: {plan.title || '(no title)'}
      </Typography>

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <Chip
          label={plan.status}
          size="small"
          color={STATUS_COLORS[plan.status] || 'default'}
          variant="outlined"
        />
        {plan.plan_type && (
          <Chip label={`type: ${plan.plan_type}`} size="small" variant="outlined" />
        )}
        {plan.complexity && (
          <Chip label={`complexity: ${plan.complexity}`} size="small" variant="outlined" />
        )}
        {plan.issue_id && (
          <Chip
            label={plan.issue_id}
            size="small"
            variant="outlined"
            component={Link as any}
            to="/project/$slug/issues/$id"
            params={{ slug, id: plan.issue_id }}
            clickable
            sx={{ textDecoration: 'none' }}
          />
        )}
      </Box>

      {plan.approach && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Approach
          </Typography>
          <MarkdownContent slug={slug}>{plan.approach}</MarkdownContent>
        </Box>
      )}

      {plan.body && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Body
          </Typography>
          <MarkdownContent slug={slug}>{plan.body}</MarkdownContent>
        </Box>
      )}

      {steps.length > 0 && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
            Steps ({steps.length})
          </Typography>
          <Stack spacing={1}>
            {steps.map(step => (
              <Box key={step.id} sx={{ border: 1, borderColor: 'divider', borderRadius: 1, p: 1.5 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5, flexWrap: 'wrap' }}>
                  <Typography variant="body2" sx={{ fontWeight: 500 }}>
                    Step {step.seq}
                  </Typography>
                  <Chip
                    label={step.status}
                    size="small"
                    color={STEP_STATUS_COLORS[step.status] || 'default'}
                    variant="outlined"
                  />
                </Box>
                {step.text && (
                  <MarkdownContent slug={slug}>{step.text}</MarkdownContent>
                )}
                {step.refs && step.refs.length > 0 && (
                  <Box sx={{ mt: 1, display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                    {step.refs.map((ref, i) => (
                      <Chip
                        key={`${ref.target_type}-${ref.target_id}-${i}`}
                        label={`${ref.target_type}: ${ref.target_id}`}
                        size="small"
                        variant="outlined"
                      />
                    ))}
                  </Box>
                )}
              </Box>
            ))}
          </Stack>
        </Box>
      )}

      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 3, mb: 2 }}>
        Created on {new Date(plan.created_at).toLocaleString()}
      </Typography>

      <EventStream slug={slug} targetType="plan" targetId={String(plan.id)} />
    </Box>
  )
}
