import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../theme'
import type { FeatureItem } from '../api'

jest.mock('../api', () => {
  const actual = jest.requireActual('../api')
  return {
    ...actual,
    useFeatures: jest.fn(),
  }
})

jest.mock('@tanstack/react-router', () => ({
  Link: (props: any) => <a href={typeof props.to === 'string' ? props.to : '#'}>{props.children}</a>,
}))

import { useFeatures } from '../api'
import { FeaturesTab } from './FeaturesTab'
import { ComponentProvider } from './ComponentContext'

const mockedUseFeatures = jest.mocked(useFeatures)

function makeFeature(overrides: Partial<FeatureItem> = {}): FeatureItem {
  return {
    id: 1,
    name: 'example-feature',
    category: 'Auth',
    component: 'server',
    description: 'Example description',
    done_when: '',
    has_test_refs: true,
    plan_status: '',
    plan_type: '',
    specs: [],
    what: '',
    why: '',
    kind: 'direct',
    baseline_value: '',
    target_value: '',
    metric_name: '',
    metric_unit: '',
    graph_url: '',
    goal_id: 0,
    parent_feature_id: 0,
    ...overrides,
  }
}

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

function renderTab(slug = 'test-project') {
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <ComponentProvider slug={slug}>
          <FeaturesTab slug={slug} />
        </ComponentProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

describe('FeaturesTab', () => {
  test('spec 39: features are grouped by category with name, component, and description', () => {
    const features: FeatureItem[] = [
      makeFeature({ id: 1, name: 'login', category: 'Auth', component: 'server', description: 'Handles sign-in' }),
      makeFeature({ id: 2, name: 'signup', category: 'Auth', component: 'ui', description: 'Account creation' }),
      makeFeature({ id: 3, name: 'dashboard', category: 'DeveloperPortal', component: 'ui', description: 'Main page' }),
    ]
    mockedUseFeatures.mockReturnValue({ data: features, isLoading: false } as any)

    renderTab()

    expect(screen.getByText('Auth')).toBeInTheDocument()
    expect(screen.getByText('DeveloperPortal')).toBeInTheDocument()

    for (const f of features) {
      expect(screen.getByText(f.name)).toBeInTheDocument()
      expect(screen.getByText(f.description)).toBeInTheDocument()
    }

    const serverChips = screen.getAllByText('server')
    expect(serverChips.length).toBe(1)
    const uiChips = screen.getAllByText('ui')
    expect(uiChips.length).toBe(2)
  })

  test('spec 41: features are filtered by name or description when searching', () => {
    const features: FeatureItem[] = [
      makeFeature({ id: 1, name: 'login', category: 'Auth', description: 'Handles sign-in flow' }),
      makeFeature({ id: 2, name: 'signup', category: 'Auth', description: 'Account creation page' }),
      makeFeature({ id: 3, name: 'dashboard', category: 'Portal', description: 'Overview of sign-in history' }),
    ]
    mockedUseFeatures.mockReturnValue({ data: features, isLoading: false } as any)

    renderTab()

    for (const f of features) {
      expect(screen.getByText(f.name)).toBeInTheDocument()
    }

    const input = screen.getByPlaceholderText('Search features…')

    fireEvent.change(input, { target: { value: 'login' } })
    expect(screen.getByText('login')).toBeInTheDocument()
    expect(screen.queryByText('signup')).not.toBeInTheDocument()
    expect(screen.queryByText('dashboard')).not.toBeInTheDocument()

    fireEvent.change(input, { target: { value: 'sign-in' } })
    expect(screen.getByText('login')).toBeInTheDocument()
    expect(screen.queryByText('signup')).not.toBeInTheDocument()
    expect(screen.getByText('dashboard')).toBeInTheDocument()

    fireEvent.change(input, { target: { value: 'creation' } })
    expect(screen.queryByText('login')).not.toBeInTheDocument()
    expect(screen.getByText('signup')).toBeInTheDocument()
    expect(screen.queryByText('dashboard')).not.toBeInTheDocument()

    fireEvent.change(input, { target: { value: 'nope-zzz' } })
    expect(screen.getByText('No features.')).toBeInTheDocument()
  })
})
