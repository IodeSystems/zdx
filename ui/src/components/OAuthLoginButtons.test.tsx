import { describe, test, expect, afterEach, vi } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/react'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { theme } from '../theme'
import { OAuthLoginButtons } from './OAuthLoginButtons'

afterEach(() => {
  cleanup()
})

function renderWithTheme(ui: React.ReactElement) {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      {ui}
    </ThemeProvider>,
  )
}

describe('OAuth2: login buttons', () => {
  test('renders one button per configured provider', async () => {
    const fetchProviders = vi.fn().mockResolvedValue(['google', 'github'])
    renderWithTheme(<OAuthLoginButtons fetchProviders={fetchProviders} />)
    await waitFor(() => {
      expect(screen.getByText('Continue with Google')).toBeInTheDocument()
    })
    expect(screen.getByText('Continue with GitHub')).toBeInTheDocument()
  })

  test('renders nothing when no providers are configured', async () => {
    const fetchProviders = vi.fn().mockResolvedValue([])
    const { container } = renderWithTheme(<OAuthLoginButtons fetchProviders={fetchProviders} />)
    await waitFor(() => {
      expect(fetchProviders).toHaveBeenCalled()
    })
    // Component returns null on empty list — container has no buttons.
    expect(container.querySelector('button')).toBeNull()
  })

  test('button hrefs point to authorize endpoint with provider + redirect_to', async () => {
    const fetchProviders = vi.fn().mockResolvedValue(['google'])
    renderWithTheme(<OAuthLoginButtons fetchProviders={fetchProviders} redirectTo="/dashboard" />)
    const btn = await screen.findByRole('link', { name: /continue with google/i })
    expect(btn.getAttribute('href')).toBe(
      '/api/auth/oauth/authorize?provider=google&redirect_to=%2Fdashboard',
    )
  })

  test('falls back to "Continue with <name>" for unknown providers', async () => {
    const fetchProviders = vi.fn().mockResolvedValue(['acme-sso'])
    renderWithTheme(<OAuthLoginButtons fetchProviders={fetchProviders} />)
    expect(await screen.findByText('Continue with acme-sso')).toBeInTheDocument()
  })

  test('encodes provider name in href', async () => {
    const fetchProviders = vi.fn().mockResolvedValue(['weird name'])
    renderWithTheme(<OAuthLoginButtons fetchProviders={fetchProviders} />)
    const btn = await screen.findByRole('link', { name: /continue with weird name/i })
    expect(btn.getAttribute('href')).toContain('provider=weird%20name')
  })
})
