import { render, screen, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../theme'
import type { FeatureItem, TaskItem, SpecItem } from '../api'

jest.mock('../api', () => {
  const actual = jest.requireActual('../api')
  return {
    ...actual,
    useFeature: jest.fn(),
    useTasks: jest.fn(),
    useSpecTests: jest.fn(),
  }
})

jest.mock('@tanstack/react-router', () => ({
  Link: (props: any) => <a href={typeof props.to === 'string' ? props.to : '#'}>{props.children}</a>,
  useRouter: () => ({ history: { go: () => {} } }),
}))

jest.mock('./MarkdownContent', () => ({
  MarkdownContent: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}))

jest.mock('./CommentsAndRevisions', () => ({
  CommentsAndRevisions: () => <div data-testid="comments" />,
}))

jest.mock('./DemoPlayer', () => ({
  DemosSection: () => <div data-testid="demos" />,
}))

import { useFeature, useTasks, useSpecTests } from '../api'
import { FeatureDetail } from './FeatureDetail'

const mockedUseFeature = jest.mocked(useFeature)
const mockedUseTasks = jest.mocked(useTasks)
const mockedUseSpecTests = jest.mocked(useSpecTests)

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

function makeSpec(overrides: Partial<SpecItem> = {}): SpecItem {
  return {
    id: 1,
    importance: 'must',
    description: 'Given X, when Y, then Z',
    ...overrides,
  }
}

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  mockedUseSpecTests.mockReturnValue({ data: [], isLoading: false } as any)
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
      expect(screen.getAllByText(s.importance).length).toBeGreaterThan(0)
    }

    expect(screen.getByText(`Tasks (${tasks.length})`)).toBeInTheDocument()
    for (const t of tasks) {
      expect(screen.getByText(t.text)).toBeInTheDocument()
    }
  })
})
