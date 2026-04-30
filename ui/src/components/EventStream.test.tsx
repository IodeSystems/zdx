import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../theme'
import type { EventItem } from '../api'

jest.mock('../api', () => {
  const actual = jest.requireActual('../api')
  return {
    ...actual,
    useEvents: jest.fn(),
    usePostComment: jest.fn(),
    useReplyToEvent: jest.fn(),
  }
})

jest.mock('./MarkdownContent', () => ({
  MarkdownContent: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}))

import { useEvents, usePostComment, useReplyToEvent } from '../api'
import { EventStream } from './EventStream'

const mockedUseEvents = jest.mocked(useEvents)
const mockedUsePostComment = jest.mocked(usePostComment)
const mockedUseReplyToEvent = jest.mocked(useReplyToEvent)

function makeEvent(overrides: Partial<EventItem> = {}): EventItem {
  return {
    id: 1,
    target_type: 'proposal',
    target_id: '1',
    event_type: 'comment',
    author: 'alice@example.com',
    author_kind: 'user',
    summary_json: { body: 'Hello world' },
    detail_json: { body: 'Hello world', format: 'markdown' },
    created_at: '2026-04-30T00:00:00Z',
    ...overrides,
  }
}

function renderWithProviders(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        {ui}
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

describe('EventStream', () => {
  beforeEach(() => {
    mockedUsePostComment.mockReturnValue({
      mutate: jest.fn(),
      isPending: false,
    } as any)
    mockedUseReplyToEvent.mockReturnValue({
      mutate: jest.fn(),
      isPending: false,
    } as any)
  })

  afterEach(() => {
    cleanup()
    jest.clearAllMocks()
  })

  it('renders comment events using the comment renderer', () => {
    mockedUseEvents.mockReturnValue({ data: { events: [makeEvent()] } } as any)
    renderWithProviders(<EventStream slug="test" targetType="proposal" targetId="1" />)
    expect(screen.getAllByTestId('markdown')[0]).toHaveTextContent('Hello world')
    expect(screen.getByText(/Activity \(1\)/)).toBeInTheDocument()
  })

  it('renders revision events with field summary', () => {
    mockedUseEvents.mockReturnValue({
      data: {
        events: [
          makeEvent({
            id: 2,
            event_type: 'revision',
            detail_json: { field: 'body', from_version: 1, to_version: 2 },
          }),
        ],
      },
    } as any)
    renderWithProviders(<EventStream slug="test" targetType="proposal" targetId="1" />)
    expect(screen.getByText(/body updated \(v1 → v2\)/)).toBeInTheDocument()
  })

  it('hides reaction events', () => {
    mockedUseEvents.mockReturnValue({
      data: {
        events: [
          makeEvent({ id: 3, event_type: 'reaction', detail_json: { kind: 'thumbs_up' } }),
          makeEvent({ id: 4 }),
        ],
      },
    } as any)
    renderWithProviders(<EventStream slug="test" targetType="proposal" targetId="1" />)
    expect(screen.getByText(/Activity \(1\)/)).toBeInTheDocument()
  })

  it('shows threads icon and panel when events have thread_id', () => {
    mockedUseEvents.mockReturnValue({
      data: {
        events: [
          makeEvent({ id: 5 }),
          makeEvent({ id: 6, thread_id: 10, detail_json: { body: 'thread reply' } }),
        ],
      },
    } as any)
    renderWithProviders(<EventStream slug="test" targetType="proposal" targetId="1" />)
    const toggle = screen.getByLabelText('Toggle threads')
    expect(toggle).toBeInTheDocument()
    fireEvent.click(toggle)
    expect(screen.getByText('Show all')).toBeInTheDocument()
    expect(screen.getByText(/Threads \(1\)/)).toBeInTheDocument()
  })

  it('posts a comment when Comment button clicked', () => {
    const mutate = jest.fn()
    mockedUsePostComment.mockReturnValue({ mutate, isPending: false } as any)
    mockedUseEvents.mockReturnValue({ data: { events: [] } } as any)
    renderWithProviders(<EventStream slug="test" targetType="proposal" targetId="1" />)
    const textbox = screen.getByPlaceholderText('Add a comment…') as HTMLTextAreaElement
    fireEvent.change(textbox, { target: { value: 'New comment' } })
    fireEvent.click(screen.getByRole('button', { name: 'Comment' }))
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        slug: 'test',
        target_type: 'proposal',
        target_id: '1',
        body: 'New comment',
      }),
      expect.anything(),
    )
  })
})
