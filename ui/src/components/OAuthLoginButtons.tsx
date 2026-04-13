import { useEffect, useState } from 'react'
import { Box, Button, Stack } from '@mui/material'

interface OAuthLoginButtonsProps {
  /** Optional path to return to after successful login. Defaults to '/'. */
  redirectTo?: string
  /** Override the providers fetcher (used by tests). */
  fetchProviders?: () => Promise<string[]>
}

const PROVIDER_LABELS: Record<string, string> = {
  google: 'Continue with Google',
  github: 'Continue with GitHub',
  microsoft: 'Continue with Microsoft',
}

function defaultFetchProviders(): Promise<string[]> {
  return fetch('/api/auth/oauth/providers')
    .then(r => r.ok ? r.json() : { providers: [] })
    .then((d: { providers: string[] }) => d.providers || [])
    .catch(() => [])
}

/**
 * Renders one button per OAuth2 provider configured on the server.
 * Each button navigates the browser to /api/auth/oauth/authorize?provider=X
 * which 302s to the provider; the provider then redirects to our callback.
 *
 * Renders nothing when no providers are configured — OAuth is opt-in.
 */
export function OAuthLoginButtons({
  redirectTo = '/',
  fetchProviders = defaultFetchProviders,
}: OAuthLoginButtonsProps) {
  const [providers, setProviders] = useState<string[]>([])

  useEffect(() => {
    fetchProviders().then(setProviders)
  }, [fetchProviders])

  if (providers.length === 0) return null

  return (
    <Box>
      <Stack spacing={1}>
        {providers.map(name => {
          const label = PROVIDER_LABELS[name] || `Continue with ${name}`
          const href = `/api/auth/oauth/authorize?provider=${encodeURIComponent(name)}&redirect_to=${encodeURIComponent(redirectTo)}`
          return (
            <Button
              key={name}
              variant="outlined"
              fullWidth
              href={href}
              data-provider={name}
            >
              {label}
            </Button>
          )
        })}
      </Stack>
    </Box>
  )
}
