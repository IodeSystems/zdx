import { render, screen, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../theme'

jest.mock('../api', () => {
  const actual = jest.requireActual('../api')
  return {
    ...actual,
    useAtlasNode: jest.fn(),
    useAtlasCoderefs: jest.fn(),
  }
})

jest.mock('@tanstack/react-router', () => ({
  Link: (props: any) => <a href={typeof props.to === 'string' ? props.to : '#'}>{props.children}</a>,
  useRouter: () => ({ history: { go: () => {} } }),
}))

jest.mock('./MarkdownContent', () => ({
  MarkdownContent: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}))

import { useAtlasNode, useAtlasCoderefs } from '../api'
import { AtlasNodeDetail } from './AtlasNodeDetail'

const mockedUseAtlasNode = jest.mocked(useAtlasNode)
const mockedUseAtlasCoderefs = jest.mocked(useAtlasCoderefs)

function makeChunk(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    node_id: 10,
    body: 'Chunk body text',
    verifier_kind: 'agent',
    status: 'verified',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function makeEdge(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    from_node_id: 10,
    to_node_id: 20,
    to_kind: 'flow',
    to_slug: 'dx-deploy',
    edge_type: 'calls',
    weight: 1,
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function makeNode(overrides: Record<string, unknown> = {}) {
  return {
    id: 10,
    kind: 'flow',
    slug: 'dx-ship',
    title: 'dx ship flow',
    description: 'The ship flow description.',
    chunks: [],
    edges: [],
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  mockedUseAtlasCoderefs.mockReturnValue({ data: [], isLoading: false } as any)
})

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

function renderDetail(slug = 'zdx', kind = 'flow', nodeSlug = 'dx-ship') {
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <AtlasNodeDetail slug={slug} kind={kind} nodeSlug={nodeSlug} />
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

describe('AtlasNodeDetail — empty state', () => {
  test('shows loading state', () => {
    mockedUseAtlasNode.mockReturnValue({ data: undefined, isLoading: true } as any)
    renderDetail()
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  test('shows not-found when node is null', () => {
    mockedUseAtlasNode.mockReturnValue({ data: null, isLoading: false } as any)
    renderDetail()
    expect(screen.getByText(/not found/i)).toBeInTheDocument()
  })
})

describe('AtlasNodeDetail — populated', () => {
  test('renders title, badges, description, chunks, subgraph, and file-issue button', () => {
    const node = makeNode({
      chunks: [
        makeChunk({ id: 1, title: 'Overview', body: 'overview body', status: 'verified' }),
        makeChunk({ id: 2, title: 'Drift chunk', body: 'drifted body', status: 'stale' }),
      ],
      edges: [
        makeEdge({ id: 1, edge_type: 'calls', to_kind: 'flow', to_slug: 'dx-deploy' }),
        makeEdge({ id: 2, edge_type: 'verified_by', to_kind: 'demo', to_slug: 'demo-1' }),
      ],
    })
    mockedUseAtlasNode.mockReturnValue({ data: node, isLoading: false } as any)

    renderDetail()

    // Title
    expect(screen.getByText('dx ship flow')).toBeInTheDocument()

    // Kind badge (also appears in subgraph as to_kind)
    expect(screen.getAllByText('flow').length).toBeGreaterThanOrEqual(1)

    // Stale fraction: 1/2 = 50% → error badge
    expect(screen.getByText('50% stale')).toBeInTheDocument()

    // Demo coverage badge
    expect(screen.getByText('1 demos')).toBeInTheDocument()

    // Description
    expect(screen.getByText('The ship flow description.')).toBeInTheDocument()

    // Chunk titles
    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByText('Drift chunk')).toBeInTheDocument()

    // Drift pill on stale chunk
    expect(screen.getByText('drift')).toBeInTheDocument()

    // Subgraph edge type heading
    expect(screen.getByText(/calls \(1\)/i)).toBeInTheDocument()
    expect(screen.getByText(/verified_by \(1\)/i)).toBeInTheDocument()

    // Linked node slug
    expect(screen.getByText('dx-deploy')).toBeInTheDocument()

    // File-issue disabled button
    const fileIssueBtn = screen.getByRole('button', { name: /file issue/i })
    expect(fileIssueBtn).toBeDisabled()
  })

  test('0% stale shows success badge when all chunks are verified', () => {
    const node = makeNode({
      chunks: [
        makeChunk({ id: 1, status: 'verified' }),
        makeChunk({ id: 2, status: 'verified' }),
      ],
    })
    mockedUseAtlasNode.mockReturnValue({ data: node, isLoading: false } as any)

    renderDetail()

    expect(screen.getByText('0% stale')).toBeInTheDocument()
  })
})
