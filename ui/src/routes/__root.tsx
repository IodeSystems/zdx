import { createRootRoute, Outlet, Link, useNavigate, useMatches } from '@tanstack/react-router'
import {
  AppBar,
  Avatar,
  Box,
  CssBaseline,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  TextField,
  ThemeProvider,
  Toolbar,
  Typography,
  useMediaQuery,
  useTheme,
  Collapse,
  ListSubheader,
} from '@mui/material'
import {
  AutoStories as AutoStoriesIcon,
  Bookmark as BookmarkIcon,
  Extension as ExtensionIcon,
  Flag as FlagIcon,
  HelpOutlined as HelpOutlineIcon,
  TaskAlt as TaskAltIcon,
  BugReport as BugReportIcon,
  History as HistoryIcon,
  Home as HomeIcon,
  Menu as MenuIcon,
  PlaylistPlay as PlaylistPlayIcon,
  QuestionAnswer as QuestionAnswerIcon,
  Settings as SettingsIcon,
  SmartToy as SmartToyIcon,
  PlusOne as PlusOneIcon,
  TextSnippet as TextSnippetIcon,
  Timer as TimerIcon,
  Tune as TuneIcon,
  Layers as LayersIcon,
  Science as ScienceIcon,
  WarningAmber as WarningAmberIcon,
  ExpandLess as ExpandLessIcon,
  ExpandMore as ExpandMoreIcon,
  AccountTree as AccountTreeIcon,
  ChatBubbleOutlined as ChatBubbleOutlineIcon,
  Lightbulb as LightbulbIcon,
  ShieldOutlined as ShieldOutlinedIcon,
} from '@mui/icons-material'
import { theme } from '../theme'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useProjects, useMe, useLogout, useZdxConfig, useBlockerQuestions, useProposals, useOmniboxSearch, type SearchFeatureItem, type SearchFocusItem } from '../api'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { AuthPage } from '../components/AuthPage'
import { IssueReportFab } from '../components/IssueReportFab'
import { ActivityToast } from '../components/ActivityToast'
import { SystemHealthBadge } from '../components/SystemHealthBadge'
import { useComponentFilter } from '../components/ComponentContext'
import { useState, useCallback, useEffect, type FormEvent } from 'react'
import { Paper, Popper, ClickAwayListener, CircularProgress } from '@mui/material'

const queryClient = new QueryClient()

const DRAWER_WIDTH = 220

type NavItem = { label: string; icon: React.ReactElement; path: string }
type NavGroup = { group?: string; items: NavItem[] }

const NAV_GROUPS: NavGroup[] = [
  {
    items: [
      { label: 'Blockers', icon: <HelpOutlineIcon fontSize="small" />, path: 'blocker-questions' },
      { label: 'Proposals', icon: <LightbulbIcon fontSize="small" />, path: 'proposals' },
      { label: 'Discussions', icon: <ChatBubbleOutlineIcon fontSize="small" />, path: 'discussions' },
      { label: 'Tasks', icon: <TaskAltIcon fontSize="small" />, path: 'tasks' },
      { label: 'Issues', icon: <BugReportIcon fontSize="small" />, path: 'issues' },
      { label: 'Worklog', icon: <HistoryIcon fontSize="small" />, path: 'worklog' },
    ],
  },
  {
    group: 'Project',
    items: [
      { label: 'Goals', icon: <FlagIcon fontSize="small" />, path: 'goals' },
      { label: 'Focuses', icon: <BookmarkIcon fontSize="small" />, path: 'focuses' },
      { label: 'Features', icon: <ExtensionIcon fontSize="small" />, path: 'features' },
      { label: 'Patterns', icon: <AccountTreeIcon fontSize="small" />, path: 'patterns' },
      { label: 'Concerns', icon: <ShieldOutlinedIcon fontSize="small" />, path: 'concerns' },
      { label: 'Standup', icon: <AutoStoriesIcon fontSize="small" />, path: 'journal' },
      { label: 'Demos', icon: <PlaylistPlayIcon fontSize="small" />, path: 'demos' },
      { label: 'Tests', icon: <ScienceIcon fontSize="small" />, path: 'tests' },
      { label: 'Environments', icon: <LayersIcon fontSize="small" />, path: 'environments' },
    ],
  },
  {
    group: 'Observability',
    items: [
      { label: 'Timings', icon: <TimerIcon fontSize="small" />, path: 'timings' },
      { label: 'Errors', icon: <WarningAmberIcon fontSize="small" />, path: 'errors' },
      { label: 'Counters', icon: <PlusOneIcon fontSize="small" />, path: 'counters' },
      { label: 'Logs', icon: <TextSnippetIcon fontSize="small" />, path: 'logs' },
    ],
  },
  {
    group: 'Support',
    items: [
      { label: 'Questions', icon: <QuestionAnswerIcon fontSize="small" />, path: 'questions' },
      { label: 'Agents', icon: <SmartToyIcon fontSize="small" />, path: 'agents' },
    ],
  },
]

const PROJECT_NAV_EXTRAS: NavItem[] = [
  { label: 'Settings', icon: <TuneIcon fontSize="small" />, path: 'settings' },
]

function ProjectLabel() {
  const { data } = useProjects()
  const matches = useMatches()
  const currentSlug = (matches.find(m => (m.params as Record<string, string>).slug)?.params as { slug?: string })?.slug
  const projects = data || []
  const current = projects.find(p => p.slug === currentSlug)
  if (!current) return null
  return (
    <Typography variant="body2" sx={{ ml: 1, opacity: 0.85, fontWeight: 500, whiteSpace: 'nowrap' }}>
      {current.name}
    </Typography>
  )
}

type OmniboxHit = {
  category: 'Issues' | 'Tasks' | 'Patterns' | 'Q&A' | 'Features' | 'Focuses'
  key: string
  ref: string
  primary: string
  secondary?: string
  score: number
  href: string
}

function flattenHits(slug: string, data: ReturnType<typeof useOmniboxSearch>['data']): OmniboxHit[] {
  if (!data) return []
  const hits: OmniboxHit[] = []
  for (const i of data.issues) {
    hits.push({
      category: 'Issues',
      key: `issue:${i.id}`,
      ref: i.id,
      primary: i.title || i.context.slice(0, 80) || '(no title)',
      secondary: i.status,
      score: i.score,
      href: `/project/${slug}/issues/${i.id}`,
    })
  }
  for (const t of data.tasks) {
    hits.push({
      category: 'Tasks',
      key: `task:${t.id}`,
      ref: t.id,
      primary: t.title || t.text.slice(0, 80) || '(no title)',
      secondary: t.issue ? `${t.status} · ${t.issue}` : t.status,
      score: t.score,
      href: `/project/${slug}/tasks/${t.id}`,
    })
  }
  for (const p of data.patterns) {
    hits.push({
      category: 'Patterns',
      key: `pattern:${p.pattern.id}`,
      ref: `PT-${p.pattern.id}`,
      primary: p.pattern.name,
      secondary: p.pattern.description?.slice(0, 80),
      score: p.score,
      href: `/project/${slug}/patterns/${p.pattern.id}`,
    })
  }
  for (const q of data.questions) {
    hits.push({
      category: 'Q&A',
      key: `qa:${q.id}`,
      ref: `BQ-${q.id}`,
      primary: q.question,
      secondary: q.answer ? 'answered' : 'unanswered',
      score: q.score,
      href: `/project/${slug}/questions/${q.id}`,
    })
  }
  for (const f of (data.features ?? []) as SearchFeatureItem[]) {
    hits.push({
      category: 'Features',
      key: `feature:${f.name}`,
      ref: f.category || f.kind || 'feature',
      primary: f.name,
      secondary: f.description?.slice(0, 80) || undefined,
      score: 1,
      href: `/project/${slug}/features/${encodeURIComponent(f.name)}`,
    })
  }
  for (const f of (data.focuses ?? []) as SearchFocusItem[]) {
    hits.push({
      category: 'Focuses',
      key: `focus:${f.id}`,
      ref: f.status,
      primary: f.name,
      secondary: f.description?.slice(0, 80) || undefined,
      score: 1,
      href: `/project/${slug}/focuses/${encodeURIComponent(f.name)}`,
    })
  }
  return hits
}

function groupHits(hits: OmniboxHit[]): { category: OmniboxHit['category']; items: OmniboxHit[] }[] {
  const order: OmniboxHit['category'][] = ['Features', 'Focuses', 'Issues', 'Tasks', 'Patterns', 'Q&A']
  return order
    .map(category => ({ category, items: hits.filter(h => h.category === category) }))
    .filter(g => g.items.length > 0)
}

function Omnibox() {
  const [value, setValue] = useState('')
  const [debounced, setDebounced] = useState('')
  const [open, setOpen] = useState(false)
  const [activeIdx, setActiveIdx] = useState(0)
  const navigate = useNavigate()
  const matches = useMatches()
  const currentSlug = (matches.find(m => (m.params as Record<string, string>).slug)?.params as { slug?: string })?.slug
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null)

  useEffect(() => {
    const t = setTimeout(() => {
      setDebounced(value.trim())
      setActiveIdx(0)
    }, 200)
    return () => clearTimeout(t)
  }, [value])

  const enabled = open && !!currentSlug && debounced.length >= 2
  const { data, isFetching } = useOmniboxSearch(currentSlug ?? '', debounced, enabled)

  const hits = currentSlug ? flattenHits(currentSlug, data) : []
  const groups = groupHits(hits)

  const close = () => setOpen(false)

  const go = (href: string) => {
    navigate({ to: href })
    setValue('')
    setDebounced('')
    setOpen(false)
  }

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    const input = value.trim()
    if (!input || !currentSlug) return

    const issueMatch = input.match(/^IS-(\d+)$/i)
    if (issueMatch) {
      go(`/project/${currentSlug}/issues/IS-${issueMatch[1]}`)
      return
    }
    const taskMatch = input.match(/^TK-(\d+)$/i)
    if (taskMatch) {
      go(`/project/${currentSlug}/tasks/TK-${taskMatch[1]}`)
      return
    }
    if (hits.length > 0) {
      const idx = Math.min(activeIdx, hits.length - 1)
      go(hits[idx].href)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!open || hits.length === 0) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActiveIdx(i => Math.min(hits.length - 1, i + 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActiveIdx(i => Math.max(0, i - 1))
    } else if (e.key === 'Escape') {
      e.preventDefault()
      setOpen(false)
    }
  }

  if (!currentSlug) return null

  const showDropdown = open && debounced.length >= 2

  let runningIdx = 0

  return (
    <ClickAwayListener onClickAway={close}>
      <Box
        component="form"
        onSubmit={handleSubmit}
        ref={setAnchorEl}
        sx={{ ml: 2, display: 'flex', alignItems: 'center', position: 'relative' }}
      >
        <TextField
          size="small"
          value={value}
          onChange={e => { setValue(e.target.value); setOpen(true) }}
          onFocus={() => { if (debounced.length >= 2) setOpen(true) }}
          onKeyDown={handleKeyDown}
          placeholder="Search by text or IS-N / TK-N…"
          variant="outlined"
          autoComplete="off"
          sx={{
            width: 260,
            '& .MuiOutlinedInput-root': {
              color: 'inherit',
              '& fieldset': { borderColor: 'rgba(255,255,255,0.2)' },
              '&:hover fieldset': { borderColor: 'rgba(255,255,255,0.4)' },
            },
            '& .MuiInputBase-input': { py: 0.5 },
            '& .MuiInputBase-input::placeholder': { color: 'rgba(255,255,255,0.5)', opacity: 1 },
          }}
        />
        <Popper
          open={showDropdown}
          anchorEl={anchorEl}
          placement="bottom-start"
          modifiers={[{ name: 'offset', options: { offset: [0, 4] } }]}
          sx={{ zIndex: (t) => t.zIndex.modal + 1 }}
        >
          <Paper sx={{ width: 360, maxHeight: 480, overflowY: 'auto' }} elevation={6}>
            {isFetching && hits.length === 0 && (
              <Box sx={{ display: 'flex', justifyContent: 'center', py: 1.5 }}>
                <CircularProgress size={18} />
              </Box>
            )}
            {!isFetching && hits.length === 0 && (
              <Box sx={{ px: 1.5, py: 1 }}>
                <Typography variant="body2" color="text.secondary">No matches</Typography>
              </Box>
            )}
            {groups.map(g => (
              <Box key={g.category}>
                <ListSubheader
                  component="div"
                  sx={{
                    lineHeight: '24px', fontSize: '0.7rem', fontWeight: 700,
                    textTransform: 'uppercase', letterSpacing: 0.5, bgcolor: 'background.paper',
                  }}
                >
                  {g.category}
                </ListSubheader>
                <List dense disablePadding>
                  {g.items.map(h => {
                    const idx = runningIdx++
                    const selected = idx === activeIdx
                    return (
                      <ListItemButton
                        key={h.key}
                        selected={selected}
                        onMouseEnter={() => setActiveIdx(idx)}
                        onClick={() => go(h.href)}
                        sx={{ alignItems: 'flex-start', py: 0.5 }}
                      >
                        <Box sx={{ flex: 1, minWidth: 0 }}>
                          <Typography variant="body2" noWrap>
                            <Box component="span" sx={{ fontFamily: 'monospace', mr: 1, color: 'text.secondary' }}>
                              {h.ref}
                            </Box>
                            {h.primary}
                          </Typography>
                          {h.secondary && (
                            <Typography variant="caption" color="text.secondary" noWrap component="div">
                              {h.secondary}
                            </Typography>
                          )}
                        </Box>
                        <Typography variant="caption" color="text.secondary" sx={{ ml: 1, flexShrink: 0 }}>
                          {Math.round(h.score * 100)}%
                        </Typography>
                      </ListItemButton>
                    )
                  })}
                </List>
              </Box>
            ))}
          </Paper>
        </Popper>
      </Box>
    </ClickAwayListener>
  )
}

function NavGroupList({ currentSlug, activePath, isQueueActive, onNavigate }: {
  currentSlug: string; activePath: string; isQueueActive: boolean; onNavigate?: () => void
}) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const toggle = useCallback((group: string) => {
    setCollapsed(prev => ({ ...prev, [group]: !prev[group] }))
  }, [])

  const { data: pendingBlockers } = useBlockerQuestions(currentSlug, 'pending')
  const { data: proposedProposals } = useProposals(currentSlug, 'proposed')
  const counts: Record<string, number> = {
    'blocker-questions': pendingBlockers?.total ?? 0,
    'proposals': proposedProposals?.length ?? 0,
  }

  const renderItem = (s: NavItem) => {
    const count = counts[s.path] ?? 0
    return (
      <ListItemButton
        key={s.path}
        selected={activePath === s.path}
        component={Link as any}
        to={`/project/$slug/${s.path}` as any}
        params={{ slug: currentSlug }}
        onClick={onNavigate}
      >
        <ListItemIcon sx={{ minWidth: 36 }}>{s.icon}</ListItemIcon>
        <ListItemText primary={s.label} />
        {count > 0 && (
          <Box
            component="span"
            sx={{
              ml: 1,
              minWidth: 20,
              height: 18,
              px: 0.75,
              borderRadius: 9,
              bgcolor: 'warning.main',
              color: 'warning.contrastText',
              fontSize: '0.7rem',
              fontWeight: 600,
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              lineHeight: 1,
            }}
          >
            {count}
          </Box>
        )}
      </ListItemButton>
    )
  }

  return (
    <List dense>
      <ListItemButton
        selected={isQueueActive}
        component={Link as any}
        to="/project/$slug/queue"
        params={{ slug: currentSlug }}
        onClick={onNavigate}
      >
        <ListItemIcon sx={{ minWidth: 36 }}>
          <PlaylistPlayIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText primary="Queue" />
      </ListItemButton>
      {NAV_GROUPS.map((g, i) => {
        if (!g.group) return g.items.map(renderItem)
        const isCollapsed = !!collapsed[g.group]
        return (
          <Box key={g.group || i}>
            <ListSubheader
              component="div"
              sx={{
                lineHeight: '32px', fontSize: '0.7rem', fontWeight: 700,
                textTransform: 'uppercase', letterSpacing: 0.5,
                cursor: 'pointer', userSelect: 'none',
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                bgcolor: 'transparent',
              }}
              onClick={() => toggle(g.group!)}
            >
              {g.group}
              {isCollapsed ? <ExpandMoreIcon sx={{ fontSize: 16 }} /> : <ExpandLessIcon sx={{ fontSize: 16 }} />}
            </ListSubheader>
            <Collapse in={!isCollapsed}>
              {g.items.map(renderItem)}
            </Collapse>
          </Box>
        )
      })}
      {PROJECT_NAV_EXTRAS.map(s => (
        <ListItemButton
          key={s.path}
          selected={activePath === s.path}
          component={Link as any}
          to={`/project/$slug/${s.path}` as any}
          params={{ slug: currentSlug }}
          onClick={onNavigate}
        >
          <ListItemIcon sx={{ minWidth: 36 }}>{s.icon}</ListItemIcon>
          <ListItemText primary={s.label} />
        </ListItemButton>
      ))}
    </List>
  )
}

function DrawerNav({ onNavigate }: { onNavigate?: () => void }) {
  const { data: projects } = useProjects()
  const { data: me } = useMe()
  const matches = useMatches()
  const projectMatch = matches.find(m => (m.params as Record<string, string>).slug)
  const currentSlug = (projectMatch?.params as { slug?: string })?.slug
  const lastPath = matches[matches.length - 1]?.pathname ?? ''
  const lastRouteId = (matches[matches.length - 1]?.routeId as string) || ''
  const isAdminActive = lastPath.startsWith('/admin') && lastPath !== '/admin/activity'
  const { component, setComponent } = useComponentFilter()

  // User's manual expand/collapse choices (null = user collapsed explicitly)
  const [manualProject, setManualProject] = useState<string | null | undefined>(undefined)
  const [adminManualExpanded, setAdminManualExpanded] = useState(false)

  const projectList = projects ?? []

  const isProjectExpanded = (slug: string) => {
    // User's explicit choice takes precedence over route
    if (manualProject === null) return false
    if (manualProject !== undefined) return manualProject === slug
    // Route controls expansion when viewing a project
    if (currentSlug) return currentSlug === slug
    // Auto-expand if there's only one project
    if (projectList.length === 1) return projectList[0].slug === slug
    return false
  }

  const adminExpanded = isAdminActive || adminManualExpanded

  const isQueueActive = lastRouteId === '/project/$slug/queue'
  const activePath = (() => {
    const prefix = '/project/$slug/'
    if (!lastRouteId.startsWith(prefix)) return ''
    return lastRouteId.slice(prefix.length)
  })()

  return (
    <>
      <Box sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: 'divider' }}>
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', textTransform: 'uppercase', letterSpacing: 0.5 }}>
          Projects
        </Typography>
      </Box>
      <List dense sx={{ overflowY: 'auto' }}>
        {projectList.length === 0 && me?.role === 'admin' && (
          <ListItemButton
            component={Link as any}
            to="/"
            onClick={onNavigate}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              <HomeIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText primary="New Project" />
          </ListItemButton>
        )}
        {projectList.map(p => {
          const isExpanded = isProjectExpanded(p.slug)
          const isCurrentProject = currentSlug === p.slug
          return (
            <Box key={p.slug}>
              <ListItemButton
                selected={isCurrentProject && !isAdminActive}
                component={Link as any}
                to="/project/$slug"
                params={{ slug: p.slug }}
                onClick={() => {
                  setManualProject(p.slug)
                  setAdminManualExpanded(false)
                  onNavigate?.()
                }}
              >
                <ListItemIcon sx={{ minWidth: 36 }}>
                  <HomeIcon fontSize="small" />
                </ListItemIcon>
                <ListItemText primary={p.name} />
                <IconButton
                  size="small"
                  onClick={e => {
                    e.preventDefault()
                    e.stopPropagation()
                    setManualProject(isExpanded ? null : p.slug)
                  }}
                  sx={{ p: 0.25 }}
                >
                  {isExpanded ? <ExpandLessIcon sx={{ fontSize: 16 }} /> : <ExpandMoreIcon sx={{ fontSize: 16 }} />}
                </IconButton>
              </ListItemButton>
              <Collapse in={isExpanded}>
                {isCurrentProject && (
                  <Box sx={{ px: 2, py: 1, borderBottom: 1, borderColor: 'divider' }}>
                    <Typography variant="caption" color="text.secondary" sx={{ display: 'block', textTransform: 'uppercase', letterSpacing: 0.5 }}>
                      Component
                    </Typography>
                    <TextField
                      select
                      size="small"
                      value={component}
                      onChange={e => setComponent(e.target.value)}
                      slotProps={{ select: { native: true } }}
                      sx={{ mt: 0.5, width: '100%', '& .MuiInputBase-input': { py: 0.5, fontSize: '0.85rem' } }}
                    >
                      <option value="">All</option>
                      <option value="server">server</option>
                    </TextField>
                  </Box>
                )}
                <NavGroupList
                  currentSlug={p.slug}
                  activePath={isCurrentProject ? activePath : ''}
                  isQueueActive={isCurrentProject && isQueueActive}
                  onNavigate={onNavigate}
                />
              </Collapse>
            </Box>
          )
        })}
        <Divider sx={{ my: 0.5 }} />
        <ListItemButton
          selected={lastPath === '/admin/activity'}
          component={Link as any}
          to={'/admin/activity' as any}
          onClick={() => {
            setManualProject(null)
            onNavigate?.()
          }}
        >
          <ListItemIcon sx={{ minWidth: 36 }}>
            <SmartToyIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText primary="Activity" />
        </ListItemButton>
        <ListItemButton
          selected={isAdminActive && lastPath !== '/admin/activity'}
          component={Link as any}
          to={'/admin' as any}
          onClick={() => {
            setAdminManualExpanded(prev => !prev)
            setManualProject(null)
            onNavigate?.()
          }}
        >
          <ListItemIcon sx={{ minWidth: 36 }}>
            <SettingsIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText primary="Admin" />
          {adminExpanded ? <ExpandLessIcon sx={{ fontSize: 16 }} /> : <ExpandMoreIcon sx={{ fontSize: 16 }} />}
        </ListItemButton>
        <Collapse in={adminExpanded}>
          <ListItemButton
            selected={lastPath === '/admin/users'}
            component={Link as any}
            to={'/admin/users' as any}
            onClick={onNavigate}
            sx={{ pl: 5 }}
          >
            <ListItemText primary="Users" />
          </ListItemButton>
          <ListItemButton
            selected={lastPath === '/admin/invites'}
            component={Link as any}
            to={'/admin/invites' as any}
            onClick={onNavigate}
            sx={{ pl: 5 }}
          >
            <ListItemText primary="Invites" />
          </ListItemButton>
          <ListItemButton
            selected={lastPath === '/admin/projects'}
            component={Link as any}
            to={'/admin/projects' as any}
            onClick={onNavigate}
            sx={{ pl: 5 }}
          >
            <ListItemText primary="Projects" />
          </ListItemButton>
          <ListItemButton
            selected={lastPath === '/admin/llm'}
            component={Link as any}
            to="/admin/llm"
            onClick={onNavigate}
            sx={{ pl: 5 }}
          >
            <ListItemText primary="LLM" />
          </ListItemButton>
          <ListItemButton
            selected={lastPath === '/admin/activity'}
            component={Link as any}
            to={'/admin/activity' as any}
            onClick={onNavigate}
            sx={{ pl: 5 }}
          >
            <ListItemText primary="Activity" />
          </ListItemButton>
          <ListItemButton
            selected={lastPath === '/admin/websocket'}
            component={Link as any}
            to={'/admin/websocket' as any}
            onClick={onNavigate}
            sx={{ pl: 5 }}
          >
            <ListItemText primary="WebSocket" />
          </ListItemButton>
        </Collapse>
      </List>
    </>
  )
}

function ReportFab() {
  const matches = useMatches()
  const projectMatch = matches.find(m => (m.params as Record<string, string>).slug)
  const routeSlug = (projectMatch?.params as { slug?: string })?.slug
  const { data: config } = useZdxConfig()
  const zdxSlug = config?.zdx_project_slug || ''
  const slug = zdxSlug || routeSlug
  if (!slug) return null
  return <IssueReportFab slug={slug} />
}

function AppShell() {
  const muiTheme = useTheme()
  const isMobile = useMediaQuery(muiTheme.breakpoints.down('md'))
  const [drawerOpen, setDrawerOpen] = useState(false)
  const matches = useMatches()
  const projectMatch = matches.find(m => (m.params as Record<string, string>).slug)
  const currentSlug = (projectMatch?.params as { slug?: string })?.slug

  const handleNavigate = isMobile ? () => setDrawerOpen(false) : undefined
  const drawerContent = (
    <>
      {!isMobile && <Toolbar variant="dense" />}
      <DrawerNav onNavigate={handleNavigate} />
    </>
  )

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh', overflow: 'hidden' }}>
      <AppBar position="fixed" sx={{ zIndex: (t) => t.zIndex.drawer + 1 }} elevation={0}>
        <Toolbar variant="dense">
          {isMobile && (
            <IconButton
              color="inherit"
              edge="start"
              onClick={() => setDrawerOpen(o => !o)}
              sx={{ mr: 1 }}
            >
              <MenuIcon />
            </IconButton>
          )}
          <ProjectLabel />
          <Omnibox />
          <Box sx={{ flexGrow: 1 }} />
          <SystemHealthBadge />
          {currentSlug && <ActivityToast slug={currentSlug} />}
          <AvatarMenu />
        </Toolbar>
      </AppBar>

      {/* Desktop: permanent drawer */}
      {!isMobile && (
        <Drawer
          variant="permanent"
          sx={{
            width: DRAWER_WIDTH,
            flexShrink: 0,
            '& .MuiDrawer-paper': {
              width: DRAWER_WIDTH,
              boxSizing: 'border-box',
              borderRight: '1px solid',
              borderColor: 'divider',
            },
          }}
        >
          {drawerContent}
        </Drawer>
      )}

      {/* Mobile: temporary overlay drawer */}
      {isMobile && (
        <Drawer
          variant="temporary"
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          ModalProps={{ keepMounted: true }}
          sx={{
            '& .MuiDrawer-paper': {
              width: DRAWER_WIDTH,
              boxSizing: 'border-box',
            },
          }}
        >
          {drawerContent}
        </Drawer>
      )}

      <Box
        component="main"
        sx={{
          flexGrow: 1,
          minWidth: 0,
          height: '100dvh',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          transition: 'margin 225ms cubic-bezier(0, 0, 0.2, 1)',
        }}
      >
        <Toolbar variant="dense" />
        <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', overflowX: 'hidden', p: 3 }}>
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </Box>
      </Box>
      <ReportFab />
    </Box>
  )
}

function AvatarMenu() {
  const { data: me } = useMe()
  const logout = useLogout()
  const matches = useMatches()
  const projectMatch = matches.find(m => (m.params as Record<string, string>).slug)
  const slug = (projectMatch?.params as { slug?: string })?.slug
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null)

  if (!me) return null

  const initials = me.name
    .split(/\s+/)
    .map(w => w[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  return (
    <>
      <IconButton onClick={e => setAnchorEl(e.currentTarget)} sx={{ ml: 1 }}>
        <Avatar sx={{ width: 28, height: 28, fontSize: '0.8rem', bgcolor: 'primary.light' }}>
          {initials}
        </Avatar>
      </IconButton>
      <Menu anchorEl={anchorEl} open={!!anchorEl} onClose={() => setAnchorEl(null)}>
        <MenuItem disabled sx={{ opacity: '1 !important' }}>
          <Box>
            <Typography variant="subtitle2">{me.name}</Typography>
            <Typography variant="caption" color="text.secondary">{me.email}</Typography>
          </Box>
        </MenuItem>
        <Divider />
        {slug && (
          <MenuItem component="a" href={`/project/${slug}/profile`} onClick={() => setAnchorEl(null)}>
            Profile
          </MenuItem>
        )}
        <MenuItem onClick={() => { setAnchorEl(null); logout() }}>
          Sign out
        </MenuItem>
      </Menu>
    </>
  )
}

function AuthGate({ children }: { children: React.ReactNode }) {
  const { data: me, isLoading } = useMe()
  if (isLoading) return null
  if (!me) return <AuthPage />
  return <>{children}</>
}

function RootLayout() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <QueryClientProvider client={queryClient}>
        <ErrorBoundary>
          <AuthGate>
            <AppShell />
          </AuthGate>
        </ErrorBoundary>
      </QueryClientProvider>
    </ThemeProvider>
  )
}

export const Route = createRootRoute({
  component: RootLayout,
})
