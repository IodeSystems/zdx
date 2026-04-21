import { createRootRoute, Outlet, Link, useNavigate, useMatches } from '@tanstack/react-router'
import {
  AppBar,
  Avatar,
  Badge,
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
  Science as ScienceIcon,
  WarningAmber as WarningAmberIcon,
  ExpandLess as ExpandLessIcon,
  ExpandMore as ExpandMoreIcon,
  AccountTree as AccountTreeIcon,
  ChatBubbleOutlined as ChatBubbleOutlineIcon,
  Lightbulb as LightbulbIcon,
} from '@mui/icons-material'
import { theme } from '../theme'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useProjects, useMe, useLogout, useUnreadCount, useUnreadThreads, useDismissAllNotifications, useZdxConfig, type UnreadThread } from '../api'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { AuthPage } from '../components/AuthPage'
import { IssueReportFab } from '../components/IssueReportFab'
import { ActivityToast } from '../components/ActivityToast'
import { useComponentFilter } from '../components/ComponentContext'
import { useState, useCallback, type FormEvent } from 'react'

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
      { label: 'Standup', icon: <AutoStoriesIcon fontSize="small" />, path: 'journal' },
      { label: 'Demos', icon: <PlaylistPlayIcon fontSize="small" />, path: 'demos' },
      { label: 'Tests', icon: <ScienceIcon fontSize="small" />, path: 'tests' },
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

function Omnibox() {
  const [value, setValue] = useState('')
  const navigate = useNavigate()
  const matches = useMatches()
  const currentSlug = (matches.find(m => (m.params as Record<string, string>).slug)?.params as { slug?: string })?.slug

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    const input = value.trim()
    if (!input || !currentSlug) return

    const issueMatch = input.match(/^IS-(\d+)$/i)
    if (issueMatch) {
      navigate({ to: '/project/$slug/issues/$id', params: { slug: currentSlug, id: `IS-${issueMatch[1]}` } })
      setValue('')
      return
    }

    const taskMatch = input.match(/^TK-(\d+)$/i)
    if (taskMatch) {
      navigate({ to: '/project/$slug/tasks/$id', params: { slug: currentSlug, id: `TK-${taskMatch[1]}` } })
      setValue('')
      return
    }

    // Treat anything else as a feature name
    navigate({ to: '/project/$slug/features/$name', params: { slug: currentSlug, name: input } })
    setValue('')
  }

  if (!currentSlug) return null

  return (
    <Box component="form" onSubmit={handleSubmit} sx={{ ml: 2, display: 'flex', alignItems: 'center' }}>
      <TextField
        size="small"
        value={value}
        onChange={e => setValue(e.target.value)}
        placeholder="IS-N / TK-N / Feature"
        variant="outlined"
        sx={{
          width: 180,
          '& .MuiOutlinedInput-root': {
            color: 'inherit',
            '& fieldset': { borderColor: 'rgba(255,255,255,0.2)' },
            '&:hover fieldset': { borderColor: 'rgba(255,255,255,0.4)' },
          },
          '& .MuiInputBase-input': { py: 0.5 },
          '& .MuiInputBase-input::placeholder': { color: 'rgba(255,255,255,0.5)', opacity: 1 },
        }}
      />
    </Box>
  )
}

function NavGroupList({ currentSlug, activePath, isQueueActive, onNavigate }: {
  currentSlug: string; activePath: string; isQueueActive: boolean; onNavigate?: () => void
}) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const toggle = useCallback((group: string) => {
    setCollapsed(prev => ({ ...prev, [group]: !prev[group] }))
  }, [])

  const renderItem = (s: NavItem) => (
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
  )

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
  const isAdminActive = lastPath.startsWith('/admin')
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
          selected={isAdminActive}
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
          p: 3,
          overflowX: 'hidden',
          transition: 'margin 225ms cubic-bezier(0, 0, 0.2, 1)',
        }}
      >
        <Toolbar variant="dense" />
        <ErrorBoundary>
          <Outlet />
        </ErrorBoundary>
      </Box>
      <ReportFab />
    </Box>
  )
}

function threadPath(slug: string, t: UnreadThread): string {
  if (t.target_type === 'issue') return `/project/${slug}/issues/${t.target_id}`
  if (t.target_type === 'task') return `/project/${slug}/tasks/${t.target_id}`
  if (t.target_type === 'feature') return `/project/${slug}/features/${t.target_id}`
  return `/project/${slug}`
}

function AvatarMenu() {
  const { data: me } = useMe()
  const logout = useLogout()
  const matches = useMatches()
  const projectMatch = matches.find(m => (m.params as Record<string, string>).slug)
  const slug = (projectMatch?.params as { slug?: string })?.slug
  const { data: unread } = useUnreadCount(slug ?? '')
  const { data: threads } = useUnreadThreads(slug ?? '')
  const dismissAll = useDismissAllNotifications()
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null)
  const [notifOpen, setNotifOpen] = useState(false)

  if (!me) return null

  const initials = me.name
    .split(/\s+/)
    .map(w => w[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  const hasUnread = (unread ?? 0) > 0

  return (
    <>
      <IconButton onClick={e => setAnchorEl(e.currentTarget)} sx={{ ml: 1 }}>
        <Badge badgeContent={unread ?? 0} color="error" overlap="circular">
          <Avatar sx={{ width: 28, height: 28, fontSize: '0.8rem', bgcolor: 'primary.light' }}>
            {initials}
          </Avatar>
        </Badge>
      </IconButton>
      <Menu anchorEl={anchorEl} open={!!anchorEl} onClose={() => setAnchorEl(null)}>
        <MenuItem disabled sx={{ opacity: '1 !important' }}>
          <Box>
            <Typography variant="subtitle2">{me.name}</Typography>
            <Typography variant="caption" color="text.secondary">{me.email}</Typography>
          </Box>
        </MenuItem>
        <Divider />
        {slug && hasUnread && (
          <>
            <MenuItem onClick={() => setNotifOpen(o => !o)} sx={{ justifyContent: 'space-between' }}>
              <Typography variant="body2">Notifications ({unread})</Typography>
              {notifOpen ? <ExpandLessIcon fontSize="small" /> : <ExpandMoreIcon fontSize="small" />}
            </MenuItem>
            <Collapse in={notifOpen} timeout="auto">
              <Box sx={{ maxHeight: 240, overflowY: 'auto', bgcolor: 'action.hover' }}>
                {(threads ?? []).map(t => (
                  <MenuItem
                    key={`${t.target_type}:${t.target_id}`}
                    component="a"
                    href={threadPath(slug, t)}
                    onClick={() => setAnchorEl(null)}
                    sx={{ pl: 3, display: 'flex', justifyContent: 'space-between', gap: 1 }}
                  >
                    <Typography variant="body2" noWrap sx={{ flex: 1 }}>
                      {t.target_type.charAt(0).toUpperCase() + t.target_type.slice(1)} {t.target_id}
                    </Typography>
                    <Badge badgeContent={t.unread_count} color="error" sx={{ mr: 1 }} />
                  </MenuItem>
                ))}
                <MenuItem
                  onClick={() => { if (slug) dismissAll.mutate(slug) }}
                  sx={{ pl: 3, color: 'text.secondary', fontSize: '0.8rem' }}
                >
                  Dismiss all
                </MenuItem>
              </Box>
            </Collapse>
            <Divider />
          </>
        )}
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
