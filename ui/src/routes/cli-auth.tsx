import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { useState } from 'react'
import { Box, Button, CircularProgress, Typography } from '@mui/material'
import { useCliToken, useMe } from '../api'

const searchSchema = z.object({
  callback: z.string().optional(),
})

function CliAuthPage() {
  const { callback } = Route.useSearch()
  const { data: me } = useMe()
  const { mutate, isPending, error, data } = useCliToken()
  const [done, setDone] = useState(false)

  if (!me) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '60vh' }}>
        <Typography color="text.secondary">Please log in to authorize the CLI.</Typography>
      </Box>
    )
  }

  if (done && data) {
    if (callback) {
      return (
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '60vh' }}>
          <Typography>Redirecting back to CLI…</Typography>
        </Box>
      )
    }
    return (
      <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2, mt: 8 }}>
        <Typography variant="h6">CLI token generated</Typography>
        <Typography variant="body2" color="text.secondary">Copy this token and paste it into the CLI prompt:</Typography>
        <Box
          component="pre"
          sx={{ bgcolor: 'action.hover', p: 2, borderRadius: 1, fontSize: '0.8rem', wordBreak: 'break-all', maxWidth: 480 }}
        >
          {data.token}
        </Box>
      </Box>
    )
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2, mt: 8 }}>
      <Typography variant="h6">Authorize CLI</Typography>
      <Typography variant="body2" color="text.secondary">
        Logged in as <strong>{me.email}</strong>. Click below to grant CLI access.
      </Typography>
      {error && <Typography color="error" variant="body2">{error.message}</Typography>}
      <Button
        variant="contained"
        disabled={isPending}
        onClick={() => {
          mutate(undefined, {
            onSuccess: ({ token }) => {
              setDone(true)
              if (callback) {
                const dest = new URL(callback)
                dest.searchParams.set('token', token)
                window.location.href = dest.toString()
              }
            },
          })
        }}
      >
        {isPending ? <CircularProgress size={20} /> : 'Authorize CLI'}
      </Button>
    </Box>
  )
}

export const Route = createFileRoute('/cli-auth')({
  validateSearch: searchSchema,
  component: CliAuthPage,
})
