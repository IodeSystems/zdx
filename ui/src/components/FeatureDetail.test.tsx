import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../theme'
import type { FeatureItem, TaskItem, SpecItem, SpecCoverageItem } from '../api'

jest.mock('../api', () => {
  const actual = jest.requireActual('../api')
  return {
    ...actual,
    useFeature: jest.fn(),
    useTasks: jest.fn(),
    useSpecTests: jest.fn(),
    useFeatureCoverage: jest.fn(),
  }
})

jest.mock('@tanstack/react-router', () => ({
  Link: (props: any) => <a href={typeof props.to === 'string' ? props.to : '#'}>{props.children}</a>,
  useRouter: () => ({ history: { go: () => {} } }),
}))

jest.mock('./MarkdownContent', () => ({
  MarkdownContent: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}))

jest.mock('./DemoPlayer', () => ({
  DemosSection: () => <div data-testid="demos" />,
}))

import { useFeature, useTasks, useSpecTests, useFeatureCoverage } from '../api'
import { FeatureDetail } from './FeatureDetail'

const mockedUseFeature = jest.mocked(useFeature)
const mockedUseTasks = jest.mocked(useTasks)
const mockedUseSpecTests = jest.mocked(useSpecTests)
const mockedUseFeatureCoverage = jest.mocked(useFeatureCoverage)

function makeFeature(overrides: Partial<FeatureItem> = {}): FeatureItem {
  return {
    id: 1,
    name: 'example-feature',
    category: 'DeveloperPortal',
    component: 'ui',
    description: 'Example description',
    done_when: '',
    has_test_refs: false,
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

function makeTask(overrides: Partial<TaskItem> = {}): TaskItem {
  return {
    id: 1,
    title: '',
    text: 'example task',
    status: 'ready',
    feature: 'example-feature',
    depends: '',
    reason: '',
    task_group: '',
    test_plan: '',
    test_refs: '',
    completed_at: '',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function makeCoverage(overrides: Partial<SpecCoverageItem> = {}): SpecCoverageItem {
  return {
    spec_id: 1,
    has_unit: false,
    has_integration: false,
    has_demo: false,
    ...overrides,
  }
}

function makeSpec(overrides: Partial<SpecItem> = {}): SpecItem {
  return {
    id: 1,
    importance: 'must',
    description: 'Given X, when Y, then Z',
    green_demos: 0,
    total_demos: 0,
    ...overrides,
  }
}

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  mockedUseSpecTests.mockReturnValue({ data: [], isLoading: false } as any)
  mockedUseFeatureCoverage.mockReturnValue({ data: [], isLoading: false } as any)
})

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

function renderDetail(slug = 'test-project', name = 'example-feature') {
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <FeatureDetail slug={slug} name={name} />
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

describe('FeatureDetail — spec tier grouping', () => {
  test('spec 41: tier headings, implemented/planned badges, and ship-readiness render correctly', () => {
    const specs: SpecItem[] = [
      makeSpec({ id: 10, importance: 'must', description: 'Must spec with demo', green_demos: 1, total_demos: 1 }),
      makeSpec({ id: 11, importance: 'must', description: 'Must spec no demo', green_demos: 0, total_demos: 0 }),
      makeSpec({ id: 20, importance: 'should', description: 'Should spec with demo', green_demos: 2, total_demos: 2 }),
      makeSpec({ id: 30, importance: 'nice-to-have', description: 'Nice spec planned', green_demos: 0, total_demos: 0 }),
    ]
    const feature = makeFeature({ specs })
    mockedUseFeature.mockReturnValue({ data: feature, isLoading: false } as any)
    mockedUseTasks.mockReturnValue({ data: { tasks: [], total: 0 }, isLoading: false } as any)

    renderDetail()

    // Tier headings visible
    expect(screen.getByText(/MUST \(deal-breaker\)/)).toBeInTheDocument()
    expect(screen.getByText(/SHOULD \(friction\)/)).toBeInTheDocument()
    expect(screen.getByText(/NICE-TO-HAVE \(polish\)/)).toBeInTheDocument()

    // Implemented badge for green_demos > 0 spec
    const implementedBadges = screen.getAllByText('Implemented')
    expect(implementedBadges.length).toBeGreaterThanOrEqual(1)

    // Planned badge for zero green_demos spec
    const plannedBadges = screen.getAllByText('Planned')
    expect(plannedBadges.length).toBeGreaterThanOrEqual(1)

    // Ship-readiness: one must is blocking
    expect(screen.getByText(/must\(s\) blocking ship/)).toBeInTheDocument()
    expect(screen.getByText(/SP-11/)).toBeInTheDocument()
  })

  test('spec 42: Ship-ready shown when all must specs are implemented', () => {
    const specs: SpecItem[] = [
      makeSpec({ id: 1, importance: 'must', description: 'Must A', green_demos: 1, total_demos: 1 }),
      makeSpec({ id: 2, importance: 'should', description: 'Should B', green_demos: 0, total_demos: 0 }),
    ]
    const feature = makeFeature({ specs })
    mockedUseFeature.mockReturnValue({ data: feature, isLoading: false } as any)
    mockedUseTasks.mockReturnValue({ data: { tasks: [], total: 0 }, isLoading: false } as any)

    renderDetail()

    expect(screen.getByText('Ship-ready')).toBeInTheDocument()
  })
})

describe('FeatureDetail', () => {
  test('spec 40: specs, tasks, what/why/done-when criteria are displayed', () => {
    const specs: SpecItem[] = [
      makeSpec({ id: 10, description: 'Given a feature, when viewed, then name is shown', importance: 'must' }),
      makeSpec({ id: 11, description: 'Given specs, when listed, then importance chip appears', importance: 'should' }),
    ]
    const feature: FeatureItem = makeFeature({
      what: 'WHAT: users can view feature detail',
      why: 'WHY: to understand the feature scope',
      done_when: 'DONE-WHEN: detail page renders all fields',
      specs,
    })
    const tasks: TaskItem[] = [
      makeTask({ id: 101, text: 'Implement detail page', status: 'done' }),
      makeTask({ id: 102, text: 'Wire up tasks list', status: 'ready' }),
    ]

    mockedUseFeature.mockReturnValue({ data: feature, isLoading: false } as any)
    mockedUseTasks.mockReturnValue({ data: { tasks, total: tasks.length }, isLoading: false } as any)

    renderDetail()

    expect(screen.getByText(feature.name)).toBeInTheDocument()

    const markdownBlocks = screen.getAllByTestId('markdown').map(n => n.textContent)
    expect(markdownBlocks).toContain(feature.what)
    expect(markdownBlocks).toContain(feature.why)
    expect(markdownBlocks).toContain(feature.done_when)

    expect(screen.getByText(`Specs (${specs.length})`)).toBeInTheDocument()
    for (const s of specs) {
      expect(screen.getByText(s.description)).toBeInTheDocument()
    }
    // Importance is now shown via tier headings, not per-spec chips
    expect(screen.getByText(/MUST \(deal-breaker\)/)).toBeInTheDocument()
    expect(screen.getByText(/SHOULD \(friction\)/)).toBeInTheDocument()

    expect(screen.getByText(`Tasks (${tasks.length})`)).toBeInTheDocument()
    for (const t of tasks) {
      expect(screen.getByText(t.text)).toBeInTheDocument()
    }
  })
})

describe('FeatureDetail — coverage badges and layer filter (spec 31)', () => {
  test('spec 31: per-spec layer badges render; layer filter narrows and deactivates', () => {
    const specs: SpecItem[] = [
      makeSpec({ id: 10, importance: 'must', description: 'Spec with unit coverage', green_demos: 1, total_demos: 1 }),
      makeSpec({ id: 11, importance: 'must', description: 'Spec without unit coverage', green_demos: 0, total_demos: 0 }),
      makeSpec({ id: 20, importance: 'should', description: 'Spec with all layers', green_demos: 2, total_demos: 2 }),
    ]
    const coverage: SpecCoverageItem[] = [
      makeCoverage({ spec_id: 10, has_unit: true, has_integration: false, has_demo: true }),
      makeCoverage({ spec_id: 11, has_unit: false, has_integration: false, has_demo: false }),
      makeCoverage({ spec_id: 20, has_unit: true, has_integration: true, has_demo: true }),
    ]
    const feature = makeFeature({ id: 5, specs })
    mockedUseFeature.mockReturnValue({ data: feature, isLoading: false } as any)
    mockedUseTasks.mockReturnValue({ data: { tasks: [], total: 0 }, isLoading: false } as any)
    mockedUseFeatureCoverage.mockReturnValue({ data: coverage, isLoading: false } as any)

    renderDetail()

    // Three layer badges appear in the spec accordion headers
    const unitBadges = screen.getAllByTestId('layer-badge-unit')
    expect(unitBadges.length).toBe(3)
    const demoBadges = screen.getAllByTestId('layer-badge-demo')
    expect(demoBadges.length).toBe(3)

    // Badge for spec 10 unit is active; spec 11 unit is not
    expect(unitBadges[0]).toHaveAttribute('data-active', 'true')
    expect(unitBadges[1]).toHaveAttribute('data-active', 'false')

    // All specs visible before filter
    expect(screen.getByText('Spec with unit coverage')).toBeInTheDocument()
    expect(screen.getByText('Spec without unit coverage')).toBeInTheDocument()

    // Click unit layer filter chip — should narrow to specs missing unit
    const unitFilter = screen.getByTestId('layer-filter-unit')
    fireEvent.click(unitFilter)

    // Only specs without unit coverage remain
    expect(screen.queryByText('Spec with unit coverage')).not.toBeInTheDocument()
    expect(screen.getByText('Spec without unit coverage')).toBeInTheDocument()
    expect(screen.queryByText('Spec with all layers')).not.toBeInTheDocument()

    // Click unit filter again to deactivate — all specs return
    fireEvent.click(unitFilter)
    expect(screen.getByText('Spec with unit coverage')).toBeInTheDocument()
    expect(screen.getByText('Spec without unit coverage')).toBeInTheDocument()
    expect(screen.getByText('Spec with all layers')).toBeInTheDocument()
  })

  test('spec 31: filter chips show gap counts and disable when no gaps', () => {
    const specs: SpecItem[] = [
      makeSpec({ id: 10, importance: 'must', description: 'A', green_demos: 1, total_demos: 1 }),
      makeSpec({ id: 11, importance: 'must', description: 'B', green_demos: 0, total_demos: 0 }),
      makeSpec({ id: 20, importance: 'should', description: 'C', green_demos: 2, total_demos: 2 }),
    ]
    const coverage: SpecCoverageItem[] = [
      makeCoverage({ spec_id: 10, has_unit: true, has_integration: true, has_demo: true }),
      makeCoverage({ spec_id: 11, has_unit: false, has_integration: false, has_demo: true }),
      makeCoverage({ spec_id: 20, has_unit: true, has_integration: true, has_demo: true }),
    ]
    const feature = makeFeature({ id: 5, specs })
    mockedUseFeature.mockReturnValue({ data: feature, isLoading: false } as any)
    mockedUseTasks.mockReturnValue({ data: { tasks: [], total: 0 }, isLoading: false } as any)
    mockedUseFeatureCoverage.mockReturnValue({ data: coverage, isLoading: false } as any)

    renderDetail()

    // Gap counts: unit=1 (spec 11), integration=1 (spec 11), demo=0
    expect(screen.getByTestId('layer-filter-unit')).toHaveAttribute('data-gap-count', '1')
    expect(screen.getByTestId('layer-filter-integration')).toHaveAttribute('data-gap-count', '1')
    expect(screen.getByTestId('layer-filter-demo')).toHaveAttribute('data-gap-count', '0')

    // Demo chip with zero gaps should be disabled (no clickable filter)
    const demoFilter = screen.getByTestId('layer-filter-demo')
    expect(demoFilter).toHaveClass('Mui-disabled')

    // Clicking disabled demo filter should not narrow the list
    fireEvent.click(demoFilter)
    expect(screen.getByText('A')).toBeInTheDocument()
    expect(screen.getByText('B')).toBeInTheDocument()
    expect(screen.getByText('C')).toBeInTheDocument()
  })
})
