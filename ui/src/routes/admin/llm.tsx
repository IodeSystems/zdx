import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import {
  Box,
  Button,
  CircularProgress,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  TextField,
  Typography,
} from '@mui/material'
import {
  useLLMConfig,
  useLLMModels,
  useSetLLMConfig,
  useTestLLMConfig,
} from '../../api'

const AGENT_TYPES = ['openai', 'local', 'claude'] as const
const COMPLEXITIES = [
  { key: 'low', label: 'Low' },
  { key: 'medium', label: 'Medium' },
  { key: 'high', label: 'High' },
] as const

function LLMConfigPage() {
  const { data, isLoading } = useLLMConfig()
  const setConfig = useSetLLMConfig()
  const testConfig = useTestLLMConfig()

  const [initialized, setInitialized] = useState(false)
  const [type, setType] = useState('openai')
  const [agentType, setAgentType] = useState<string>('openai')
  const [url, setUrl] = useState('')
  const [model, setModel] = useState('')
  const [modelLow, setModelLow] = useState('')
  const [modelMedium, setModelMedium] = useState('')
  const [modelHigh, setModelHigh] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [saved, setSaved] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)
  if (data && !initialized) {
    setInitialized(true)
    setType(data.type || 'openai')
    setAgentType(data.agent_type || 'openai')
    setUrl(data.url || '')
    setModel(data.model || '')
    setModelLow(data.model_low || '')
    setModelMedium(data.model_medium || '')
    setModelHigh(data.model_high || '')
  }

  const models = useLLMModels(url, !!url)

  const buildBody = () => ({
    type,
    url,
    model,
    api_key: apiKey || undefined,
    agent_type: agentType,
    model_low: modelLow,
    model_medium: modelMedium,
    model_high: modelHigh,
  })

  const handleSave = () => {
    setSaved(false)
    setConfig.mutate(buildBody() as never, { onSuccess: () => setSaved(true) })
  }

  const handleTest = () => {
    setTestResult(null)
    testConfig.mutate(buildBody() as never, {
      onSuccess: (res) => setTestResult(res),
      onError: (err) => setTestResult({ ok: false, message: err.message }),
    })
  }

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />

  const slotSetter = (slot: 'low' | 'medium' | 'high') =>
    slot === 'low' ? setModelLow : slot === 'medium' ? setModelMedium : setModelHigh
  const slotValue = (slot: 'low' | 'medium' | 'high') =>
    slot === 'low' ? modelLow : slot === 'medium' ? modelMedium : modelHigh

  const modelOptions = models.data ?? []

  return (
    <Box sx={{ maxWidth: 480 }}>
      <Typography variant="h5" sx={{ fontWeight: 600, mb: 3 }}>
        LLM Config
      </Typography>
      <Paper variant="outlined" sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <FormControl size="small" fullWidth>
          <InputLabel id="agent-type-label">Agent Type</InputLabel>
          <Select
            labelId="agent-type-label"
            label="Agent Type"
            value={agentType}
            onChange={e => setAgentType(e.target.value)}
          >
            {AGENT_TYPES.map(t => (
              <MenuItem key={t} value={t}>{t}</MenuItem>
            ))}
          </Select>
        </FormControl>
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
          label="Model (legacy / embeddings)"
          value={model}
          onChange={e => setModel(e.target.value)}
          placeholder="nomic-embed-text"
          size="small"
          fullWidth
        />
        {COMPLEXITIES.map(({ key, label }) => (
          <FormControl size="small" fullWidth key={key}>
            <InputLabel id={`model-${key}-label`}>{label} Model</InputLabel>
            <Select
              labelId={`model-${key}-label`}
              label={`${label} Model`}
              value={modelOptions.includes(slotValue(key)) ? slotValue(key) : ''}
              displayEmpty
              onChange={e => slotSetter(key)(e.target.value)}
              disabled={!url || models.isLoading}
            >
              <MenuItem value="">
                <em>{models.isError ? `error: ${models.error?.message}` : url ? 'select…' : 'enter URL first'}</em>
              </MenuItem>
              {modelOptions.map(m => (
                <MenuItem key={m} value={m}>{m}</MenuItem>
              ))}
              {slotValue(key) && !modelOptions.includes(slotValue(key)) && (
                <MenuItem value={slotValue(key)}>{slotValue(key)} (saved)</MenuItem>
              )}
            </Select>
          </FormControl>
        ))}
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
