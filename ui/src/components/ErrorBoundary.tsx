import { Component, type ReactNode } from 'react'
import { Box, Button, Typography } from '@mui/material'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

/** POST a client-side error to the server for admin visibility. Fire-and-forget. */
function reportError(error: Error, context: string): void {
  fetch('/api/admin/errors/report', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      source: 'client',
      endpoint: window.location.pathname,
      error_name: `${error.name}: ${error.message}`.slice(0, 500),
      stack_trace: (error.stack ?? '') + (context ? `\n\nComponent stack:\n${context}` : ''),
    }),
  }).catch(() => {/* ignore */})
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: { componentStack: string }): void {
    reportError(error, info.componentStack)
  }

  render(): ReactNode {
    if (this.state.error) {
      return (
        <Box sx={{ p: 4, textAlign: 'center' }}>
          <Typography variant="h6" color="error" sx={{ mb: 1 }}>
            Something went wrong
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2, fontFamily: 'monospace' }}>
            {this.state.error.message}
          </Typography>
          <Button variant="outlined" onClick={() => this.setState({ error: null })}>
            Retry
          </Button>
        </Box>
      )
    }
    return this.props.children
  }
}
