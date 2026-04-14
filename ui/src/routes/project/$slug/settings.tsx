import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import {
  Box,
  Button,
  CircularProgress,
  Paper,
  TextField,
  Typography,
} from '@mui/material'
import {
  useProjectGitConfig,
  useSetProjectGitConfig,
  useTestProjectGitConfig,
} from '../../../api'

function ProjectSettingsPage() {
  const { slug } = Route.useParams()
  const { data, isLoading } = useProjectGitConfig(slug)
  const setConfig = useSetProjectGitConfig()
  const testConfig = useTestProjectGitConfig()

  const [gitUrl, setGitUrl] = useState('')
  const [gitBranch, setGitBranch] = useState('main')
  const [gitToken, setGitToken] = useState('')
  const [saved, setSaved] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  useEffect(() => {
    if (data) {
      setGitUrl(data.git_url || '')
      setGitBranch(data.git_branch || 'main')
    }
  }, [data])

  const handleSave = () => {
    setSaved(false)
    setConfig.mutate(
      { slug, git_url: gitUrl, git_branch: gitBranch, git_token: gitToken || undefined },
      { onSuccess: () => setSaved(true) },
    )
  }

  const handleTest = () => {
    setTestResult(null)
    testConfig.mutate(
      { slug, git_url: gitUrl, git_branch: gitBranch, git_token: gitToken || undefined },
      {
        onSuccess: (res) => setTestResult(res),
        onError: (err) => setTestResult({ ok: false, message: err.message }),
      },
    )
  }

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />

  return (
    <Box sx={{ maxWidth: 480 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 3 }}>
        Project Settings
      </Typography>

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
        Git Configuration
      </Typography>
      <Paper variant="outlined" sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <TextField
          label="Repository URL"
          value={gitUrl}
          onChange={e => setGitUrl(e.target.value)}
          placeholder="https://github.com/org/repo.git"
          size="small"
          fullWidth
        />
        <TextField
          label="Default Branch"
          value={gitBranch}
          onChange={e => setGitBranch(e.target.value)}
          placeholder="main"
          size="small"
          fullWidth
        />
        <TextField
          label="Access Token"
          type="password"
          value={gitToken}
          onChange={e => setGitToken(e.target.value)}
          placeholder="Leave blank to keep existing"
          helperText="Used for HTTPS clone authentication"
          size="small"
          fullWidth
        />
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <Button
            variant="contained"
            onClick={handleSave}
            disabled={setConfig.isPending}
          >
            Save
          </Button>
          <Button
            variant="outlined"
            onClick={handleTest}
            disabled={testConfig.isPending || !gitUrl}
          >
            {testConfig.isPending ? 'Testing…' : 'Test'}
          </Button>
          {saved && (
            <Typography variant="caption" color="success.main">Saved</Typography>
          )}
          {setConfig.isError && (
            <Typography variant="caption" color="error">{setConfig.error?.message}</Typography>
          )}
          {testResult && (
            <Typography
              variant="caption"
              color={testResult.ok ? 'success.main' : 'error'}
            >
              {testResult.ok ? 'Connection OK' : `Failed: ${testResult.message}`}
            </Typography>
          )}
        </Box>
      </Paper>
    </Box>
  )
}

export const Route = createFileRoute('/project/$slug/settings')({
  component: ProjectSettingsPage,
})
