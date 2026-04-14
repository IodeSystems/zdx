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
import { useLLMConfig, useSetLLMConfig, useTestLLMConfig } from '../../api'

function LLMConfigPage() {
  const { data, isLoading } = useLLMConfig()
  const setConfig = useSetLLMConfig()
  const testConfig = useTestLLMConfig()

  const [type, setType] = useState('openai')
  const [url, setUrl] = useState('')
  const [model, setModel] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [saved, setSaved] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  useEffect(() => {
    if (data) {
      setType(data.type || 'openai')
      setUrl(data.url || '')
      setModel(data.model || '')
    }
  }, [data])

  const handleSave = () => {
    setSaved(false)
    setConfig.mutate(
      { type, url, model, api_key: apiKey || undefined },
      { onSuccess: () => setSaved(true) },
    )
  }

  const handleTest = () => {
    setTestResult(null)
    testConfig.mutate(
      { type, url, model, api_key: apiKey || undefined },
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
        LLM Config
      </Typography>
      <Paper variant="outlined" sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <TextField
          label="Type"
          value={type}
          onChange={e => setType(e.target.value)}
          helperText="openai (OpenAI-compatible API)"
          size="small"
          fullWidth
        />
        <TextField
          label="URL"
          value={url}
          onChange={e => setUrl(e.target.value)}
          placeholder="http://192.168.1.76:8111"
          size="small"
          fullWidth
        />
        <TextField
          label="Model"
          value={model}
          onChange={e => setModel(e.target.value)}
          placeholder="nomic-embed-text"
          size="small"
          fullWidth
        />
        <TextField
          label="API Key"
          type="password"
          value={apiKey}
          onChange={e => setApiKey(e.target.value)}
          placeholder="Leave blank to keep existing"
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
            disabled={testConfig.isPending || !url}
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
              {testResult.ok ? testResult.message : `Failed: ${testResult.message}`}
            </Typography>
          )}
        </Box>
      </Paper>
    </Box>
  )
}

export const Route = createFileRoute('/admin/llm')({
  component: LLMConfigPage,
})
