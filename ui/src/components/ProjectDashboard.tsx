import { Link } from '@tanstack/react-router'
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Typography,
} from '@mui/material'
import { Extension as ExtensionIcon, TaskAlt as TaskAltIcon, BugReport as BugReportIcon } from '@mui/icons-material'
import { useFeatures, useTasks, useIssues, type IssueItem } from '../api'

type Issue = IssueItem

function priorityLabel(p: string): string {
  if (!p) return 'untriaged'
  return { '1': 'urgent', '2': 'high', '3': 'medium', '4': 'low' }[p] ?? p
}

const PRIORITY_COLORS: Record<string, 'error' | 'warning' | 'default' | 'info'> = {
  urgent: 'error',
  high: 'warning',
  medium: 'info',
  low: 'default',
}

function SummaryCard({
  slug,
  componentSlug,
  section,
  title,
  icon,
  primary,
  secondary,
}: {
  slug: string
  componentSlug: string
  section: 'features' | 'tasks' | 'issues'
  title: string
  icon: React.ReactNode
  primary: string
  secondary?: React.ReactNode
}) {
  return (
    <Card variant="outlined" sx={{ flex: '1 1 220px', minWidth: 220 }}>
      <CardActionArea
        component={Link as any}
        to={`/project/$slug/$component/${section}` as any}
        params={{ slug, component: componentSlug }}
      >
        <CardContent>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1, color: 'text.secondary' }}>
            {icon}
            <Typography variant="overline">{title}</Typography>
          </Box>
          <Typography variant="h4" sx={{ fontWeight: 600 }}>
            {primary}
          </Typography>
          {secondary && (
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              {secondary}
            </Typography>
          )}
        </CardContent>
      </CardActionArea>
    </Card>
  )
}

export function ProjectDashboard({ slug, componentSlug = 'all' }: { slug: string; componentSlug?: string }) {
  const { data: featuresData } = useFeatures(slug)
  const { data: tasksData } = useTasks(slug)
  const { data: issuesData } = useIssues(slug)

  const features = featuresData || []
  const tasks = tasksData || []
  const allIssues: Issue[] = issuesData || []

  const component = componentSlug === 'all' ? '' : componentSlug
  const issues = component ? allIssues.filter(i => i.component === component) : allIssues

  const openIssues = issues.filter(i => i.status !== 'done' && i.status !== 'closed')
  const issuesByPriority = openIssues.reduce((acc, i) => {
    const k = priorityLabel(i.priority)
    acc[k] = (acc[k] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  const tasksDone = tasks.filter(t => t.status === 'done').length
  const tasksTotal = tasks.length

  const recent = [...openIssues]
    .sort((a, b) => (b.created_at > a.created_at ? 1 : -1))
    .slice(0, 5)

  const isComponentScope = componentSlug !== 'all'
  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 1 }}>
        {slug}
        {isComponentScope && (
          <Typography component="span" variant="h5" color="text.secondary" sx={{ ml: 1 }}>
            / {componentSlug}
          </Typography>
        )}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {isComponentScope
          ? `Component dashboard — issues filtered to ${componentSlug}; features and tasks remain project-wide.`
          : 'Project dashboard — overview, then drill into a section.'}
      </Typography>

      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 2, mb: 4 }}>
        <SummaryCard
          slug={slug}
          componentSlug={componentSlug}
          section="features"
          title="Features"
          icon={<ExtensionIcon fontSize="small" />}
          primary={String(features.length)}
        />
        <SummaryCard
          slug={slug}
          componentSlug={componentSlug}
          section="issues"
          title="Open issues"
          icon={<BugReportIcon fontSize="small" />}
          primary={String(openIssues.length)}
          secondary={
            <>
              {Object.entries(issuesByPriority).map(([k, v]) => (
                <Chip
                  key={k}
                  label={`${k} ${v}`}
                  size="small"
                  color={PRIORITY_COLORS[k] || 'default'}
                  sx={{ mr: 0.5, mt: 0.5 }}
                />
              ))}
            </>
          }
        />
        <SummaryCard
          slug={slug}
          componentSlug={componentSlug}
          section="tasks"
          title="Tasks"
          icon={<TaskAltIcon fontSize="small" />}
          primary={`${tasksDone}/${tasksTotal}`}
          secondary={tasksTotal === 0 ? 'no tasks' : `${Math.round((tasksDone / tasksTotal) * 100)}% done`}
        />
      </Box>

      <Typography variant="subtitle1" sx={{ mb: 1 }}>
        Recent open issues
      </Typography>
      {recent.length === 0 ? (
        <Typography variant="body2" color="text.secondary">No open issues.</Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {recent.map((i: Issue) => {
            const pLabel = priorityLabel(i.priority)
            return (
              <Card key={i.id} variant="outlined">
                <CardActionArea
                  component={Link as any}
                  to="/project/$slug/$component/issues/$id"
                  params={{ slug, component: componentSlug, id: `IS-${i.id}` }}
                >
                  <CardContent sx={{ py: 1.25, display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Chip
                      label={pLabel}
                      size="small"
                      color={PRIORITY_COLORS[pLabel] || 'default'}
                      sx={{ minWidth: 70 }}
                    />
                    <Typography variant="body2" sx={{ flex: 1 }}>
                      IS-{i.id}: {i.title || (i.context ? i.context.slice(0, 60) + (i.context.length > 60 ? '…' : '') : '(no title)')}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {i.created_at}
                    </Typography>
                  </CardContent>
                </CardActionArea>
              </Card>
            )
          })}
        </Box>
      )}
    </Box>
  )
}
