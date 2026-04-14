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
  Timer as TimerIcon,
  Tune as TuneIcon,
  Lightbulb as LightbulbIcon,
  WarningAmber as WarningAmberIcon,
} from '@mui/icons-material'
import { theme } from '../theme'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useProjects, useMe, useLogout, useUnreadCount, useZdxConfig } from '../api'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { AuthPage } from '../components/AuthPage'
import { IssueReportFab } from '../components/IssueReportFab'
import { useComponentFilter } from '../components/ComponentContext'
import { useState, type FormEvent } from 'react'

const queryClient = new QueryClient()

const DRAWER_WIDTH = 220

const SECTIONS = [
  { label: 'Goals', icon: <FlagIcon fontSize="small" />, path: 'goals' },
  { label: 'Features', icon: <ExtensionIcon fontSize="small" />, path: 'features' },
  { label: 'Themes', icon: <BookmarkIcon fontSize="small" />, path: 'themes' },
  { label: 'Tasks', icon: <TaskAltIcon fontSize="small" />, path: 'tasks' },
  { label: 'Issues', icon: <BugReportIcon fontSize="small" />, path: 'issues' },
  { label: 'Questions', icon: <QuestionAnswerIcon fontSize="small" />, path: 'questions' },
  { label: 'Blockers', icon: <HelpOutlineIcon fontSize="small" />, path: 'blocker-questions' },
  { label: 'Demos', icon: <PlaylistPlayIcon fontSize="small" />, path: 'demos' },
  { label: 'Worklog', icon: <HistoryIcon fontSize="small" />, path: 'worklog' },
  { label: 'Journal', icon: <AutoStoriesIcon fontSize="small" />, path: 'journal' },
  { label: 'Proposals', icon: <LightbulbIcon fontSize="small" />, path: 'proposals' },
  { label: 'Claude', icon: <SmartToyIcon fontSize="small" />, path: 'claude' },
  { label: 'Errors', icon: <WarningAmberIcon fontSize="small" />, path: 'errors' },
  { label: 'Timings', icon: <TimerIcon fontSize="small" />, path: 'timings' },
  { label: 'Counters', icon: <PlusOneIcon fontSize="small" />, path: 'counters' },
] as const

const PROJECT_NAV_EXTRAS = [
  { label: 'Settings', icon: <TuneIcon fontSize="small" />, path: 'settings' },
] as const

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

function SectionNav({ onNavigate }: { onNavigate?: () => void }) {
  const matches = useMatches()
  const projectMatch = matches.find(m => (m.params as Record<string, string>).slug)
  const currentSlug = (projectMatch?.params as { slug?: string })?.slug
  const { component, setComponent } = useComponentFilter()
  const lastRouteId = (matches[matches.length - 1]?.routeId as string) || ''
  const isQueueActive = lastRouteId === '/project/$slug/queue'
  const activePath = (() => {
    const prefix = '/project/$slug/'
    if (!lastRouteId.startsWith(prefix)) return ''
    return lastRouteId.slice(prefix.length)
  })()

  if (!currentSlug) return null

  return (
    <>
      <Box sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: 'divider' }}>
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
        {SECTIONS.map(s => (
          <ListItemButton
            key={s.path}
            selected={activePath === s.path}
            component={Link as any}
            to={`/project/$slug/${s.path}` as any}
            params={{ slug: currentSlug }}
            onClick={onNavigate}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              {s.icon}
            </ListItemIcon>
            <ListItemText primary={s.label} />
          </ListItemButton>
        ))}
        {PROJECT_NAV_EXTRAS.map(s => (
          <ListItemButton
            key={s.path}
            selected={activePath === s.path || lastRouteId === `/project/$slug/${s.path}`}
            component={Link as any}
            to={`/project/$slug/${s.path}` as any}
            params={{ slug: currentSlug }}
            onClick={onNavigate}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              {s.icon}
            </ListItemIcon>
            <ListItemText primary={s.label} />
          </ListItemButton>
        ))}
      </List>
    </>
  )
}

function HomeNav({ onNavigate }: { onNavigate?: () => void }) {
  const matches = useMatches()
  const lastPath = matches[matches.length - 1]?.pathname ?? ''
  const isHomeActive = lastPath === '/'
  const isAdminActive = lastPath.startsWith('/admin')

  return (
    <>
      <Box sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: 'divider' }}>
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', textTransform: 'uppercase', letterSpacing: 0.5 }}>
          Home
        </Typography>
      </Box>
      <List dense>
        <ListItemButton
          selected={isHomeActive}
          component={Link as any}
          to="/"
          onClick={onNavigate}
        >
          <ListItemIcon sx={{ minWidth: 36 }}>
            <HomeIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText primary="Projects" />
        </ListItemButton>
        <ListItemButton
          selected={lastPath === '/admin'}
          component={Link as any}
          to={'/admin' as any}
          onClick={onNavigate}
        >
          <ListItemIcon sx={{ minWidth: 36 }}>
            <SettingsIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText primary="Admin" />
        </ListItemButton>
        {isAdminActive && (
          <>
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
              selected={lastPath === '/admin/llm'}
              component={Link as any}
              to="/admin/llm"
              onClick={onNavigate}
              sx={{ pl: 5 }}
            >
              <ListItemText primary="LLM" />
            </ListItemButton>
          </>
        )}
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

  const handleNavigate = isMobile ? () => setDrawerOpen(false) : undefined
  const drawerContent = (
    <>
      {!isMobile && <Toolbar variant="dense" />}
      <SectionNav onNavigate={handleNavigate} />
      <HomeNav onNavigate={handleNavigate} />
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

      <Box component="main" sx={{ flexGrow: 1, minWidth: 0, p: 3, overflowX: 'hidden' }}>
        <Toolbar variant="dense" />
        <ErrorBoundary>
          <Outlet />
        </ErrorBoundary>
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
  const { data: unread } = useUnreadCount(slug ?? '')
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
