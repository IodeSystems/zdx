import { render, screen, cleanup, fireEvent, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../theme'
import type {
  CommentItem,
  HistoryEvent,
  IssueWorkItem,
  ReservationItem,
  ResolutionItem,
} from '../api'

jest.mock('../api', () => {
  const actual = jest.requireActual('../api')
  return {
    ...actual,
    useReservationsByIssue: jest.fn(),
    useIssueResolutions: jest.fn(),
    useHistory: jest.fn(),
    useComments: jest.fn(),
  }
})

jest.mock('@tanstack/react-router', () => ({
  Link: (props: any) => <a href={typeof props.to === 'string' ? props.to : '#'}>{props.children}</a>,
}))

jest.mock('./MarkdownContent', () => ({
  MarkdownContent: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}))

import {
  useReservationsByIssue,
  useIssueResolutions,
  useHistory,
  useComments,
} from '../api'
import { UnifiedTimeline } from './UnifiedTimeline'

const mockedUseReservations = jest.mocked(useReservationsByIssue)
const mockedUseResolutions = jest.mocked(useIssueResolutions)
const mockedUseHistory = jest.mocked(useHistory)
const mockedUseComments = jest.mocked(useComments)

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

function renderTimeline(
  workEntries: IssueWorkItem[] = [],
  opts: {
    reservations?: ReservationItem[]
    resolutions?: ResolutionItem[]
    history?: HistoryEvent[]
    comments?: CommentItem[]
  } = {},
) {
  mockedUseReservations.mockReturnValue({ data: { reservations: opts.reservations ?? [] } } as any)
  mockedUseResolutions.mockReturnValue({ data: opts.resolutions ?? [] } as any)
  mockedUseHistory.mockReturnValue({ data: { events: opts.history ?? [] } } as any)
  mockedUseComments.mockReturnValue({ data: { comments: opts.comments ?? [], total: (opts.comments ?? []).length } } as any)
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <UnifiedTimeline slug="zdx" issueId="IS-100" workEntries={workEntries} />
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

describe('UnifiedTimeline', () => {
  test('renders an empty state when no events exist', () => {
    renderTimeline()
    expect(screen.getByText('No activity yet.')).toBeInTheDocument()
  })

  test('renders all kinds of events with header counts', () => {
    renderTimeline(
      [{ agent: 'agent-a', created_at: '2026-04-01T10:00:00Z', note: 'work-note-1' }],
      {
        reservations: [
          { id: 1, target_type: 'issue', target_id: 'IS-100', claimed_by: 'agent-x', claimed_at: '2026-04-02T10:00:00Z', released_at: '', lease_expires_at: '2026-04-02T10:30:00Z' } as any,
        ],
        resolutions: [
          { id: 2, issue_id: 'IS-100', branch_of_origin: 'main', resolved_at: '2026-04-03T10:00:00Z', author: 'me@example.com', source: 'manual', commits: [{ sha: 'abcdef0123' }], reverted: false } as any,
        ],
        history: [
          { kind: 'status', id: 3, created_at: '2026-04-04T10:00:00Z', from_status: 'open', to_status: 'closed', agent_id: '', session_id: '', user_id: 'me' },
        ],
        comments: [
          { id: 4, target_type: 'issue', target_id: 'IS-100', author: 'me', body: 'top-comment', created_at: '2026-04-05T10:00:00Z' } as any,
        ],
      },
    )
    expect(screen.getByText('Timeline (5)')).toBeInTheDocument()
    expect(screen.getByText('work-note-1')).toBeInTheDocument()
    expect(screen.getByText(/agent-x/)).toBeInTheDocument()
    expect(screen.getByText('abcdef0123')).toBeInTheDocument()
    expect(screen.getAllByText('Resolutions (1)').length).toBeGreaterThan(0)
    expect(screen.getByText('top-comment')).toBeInTheDocument()
  })

  test('events sort DESC by timestamp regardless of source', () => {
    renderTimeline(
      [
        { agent: 'a', created_at: '2026-04-10T10:00:00Z', note: 'newest-work' },
        { agent: 'a', created_at: '2026-04-01T10:00:00Z', note: 'oldest-work' },
      ],
      {
        comments: [
          { id: 9, target_type: 'issue', target_id: 'IS-100', author: 'me', body: 'middle-comment', created_at: '2026-04-05T10:00:00Z' } as any,
        ],
      },
    )
    const list = screen.getByText('Timeline (3)').parentElement!.parentElement!
    const text = list.textContent ?? ''
    const iNew = text.indexOf('newest-work')
    const iMid = text.indexOf('middle-comment')
    const iOld = text.indexOf('oldest-work')
    expect(iNew).toBeGreaterThan(-1)
    expect(iNew).toBeLessThan(iMid)
    expect(iMid).toBeLessThan(iOld)
  })

  test('clicking a kind chip filters the timeline; All resets', () => {
    renderTimeline(
      [{ agent: 'a', created_at: '2026-04-01T10:00:00Z', note: 'work-only' }],
      {
        comments: [
          { id: 9, target_type: 'issue', target_id: 'IS-100', author: 'me', body: 'comment-only', created_at: '2026-04-05T10:00:00Z' } as any,
        ],
      },
    )
    expect(screen.getByText('work-only')).toBeInTheDocument()
    expect(screen.getByText('comment-only')).toBeInTheDocument()

    fireEvent.click(screen.getByText('Comments (1)'))
    expect(screen.queryByText('work-only')).not.toBeInTheDocument()
    expect(screen.getByText('comment-only')).toBeInTheDocument()

    fireEvent.click(screen.getByText('All'))
    expect(screen.getByText('work-only')).toBeInTheDocument()
    expect(screen.getByText('comment-only')).toBeInTheDocument()
  })

  test('comment replies render nested under their parent', () => {
    renderTimeline([], {
      comments: [
        { id: 100, target_type: 'issue', target_id: 'IS-100', author: 'a', body: 'parent-body', created_at: '2026-04-05T10:00:00Z' } as any,
        { id: 101, target_type: 'issue', target_id: 'IS-100', author: 'b', body: 'reply-body', created_at: '2026-04-05T10:05:00Z', parent_id: 100 } as any,
      ],
    })
    expect(screen.getByText('Timeline (1)')).toBeInTheDocument()
    const parent = screen.getByText('parent-body').closest('[id^="C-"]')!
    expect(within(parent as HTMLElement).getByText('reply-body')).toBeInTheDocument()
  })

})
