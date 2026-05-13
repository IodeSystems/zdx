import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import {
  Box,
  Button,
  CircularProgress,
  Divider,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { ArrowUpward as ArrowUpwardIcon, ArrowDownward as ArrowDownwardIcon, Delete as DeleteIcon } from '@mui/icons-material'
import {
  useLLMConfigs,
  useCreateLLMConfig,
  useUpdateLLMConfig,
  useDeleteLLMConfig,
  useReorderLLMConfigs,
  useTestLLMConfig,
  useLLMModels,
  type LLMConfigBody,
} from '../../api'

const AGENT_TYPES = ['openai', 'local', 'claude'] as const
const SLOTS = [
  { key: 'low', label: 'Low' },
  { key: 'medium', label: 'Medium' },
  { key: 'high', label: 'High' },
  { key: 'xhigh', label: 'XHigh' },
  { key: 'max', label: 'Max' },
] as const
type SlotKey = (typeof SLOTS)[number]['key']

const NEW_CONFIG: LLMConfigBody = {
  name: 'default',
  priority: 0,
  type: 'openai',
  url: '',
  embedding_model: '',
  agent_type: 'openai',
  model_low: '',
  model_medium: '',
  model_high: '',
  model_xhigh: '',
  model_max: '',
  timeout_seconds: 600,
}

function ConfigRow({
  cfg,
  index,
  total,
  onMove,
}: {
  cfg: LLMConfigBody
  index: number
  total: number
  onMove: (dir: -1 | 1) => void
}) {
  const [draft, setDraft] = useState<LLMConfigBody>(cfg)
  const [apiKey, setApiKey] = useState('')
  const [saved, setSaved] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  const update = useUpdateLLMConfig()
  const del = useDeleteLLMConfig()
  const test = useTestLLMConfig()
  const models = useLLMModels(cfg.id ?? 0, draft.url, !!draft.url && !!cfg.id)

  const patch = (p: Partial<LLMConfigBody>) => {
    setDraft({ ...draft, ...p })
    setSaved(false)
  }

  const isClaude = draft.agent_type === 'claude'

  const buildBody = (): LLMConfigBody => ({
    ...draft,
    embedding_model: isClaude ? '' : draft.embedding_model,
    api_key: apiKey || undefined,
  })

  const onSave = () => {
    setSaved(false)
    update.mutate(
      { id: cfg.id!, body: buildBody() },
      {
        onSuccess: () => {
          setSaved(true)
          setApiKey('')
        },
      },
    )
  }

  const onDelete = () => {
    if (!cfg.id) return
    if (!confirm(`Delete LLM config "${cfg.name}"?`)) return
    del.mutate(cfg.id)
  }

  const onTest = () => {
    setTestResult(null)
    test.mutate(
      { id: cfg.id!, body: buildBody() },
      {
        onSuccess: (res) => setTestResult(res),
        onError: (err) => setTestResult({ ok: false, message: err.message }),
      },
    )
  }

  const slotValue = (slot: SlotKey): string => {
    switch (slot) {
      case 'low':
        return draft.model_low
      case 'medium':
        return draft.model_medium
      case 'high':
        return draft.model_high
      case 'xhigh':
        return draft.model_xhigh ?? ''
      case 'max':
        return draft.model_max ?? ''
    }
  }
  const setSlot = (slot: SlotKey, v: string) => {
    switch (slot) {
      case 'low':
        patch({ model_low: v })
        break
      case 'medium':
        patch({ model_medium: v })
        break
      case 'high':
        patch({ model_high: v })
        break
      case 'xhigh':
        patch({ model_xhigh: v })
        break
      case 'max':
        patch({ model_max: v })
        break
    }
  }

  const modelOptions = models.data ?? []

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} sx={{ mb: 2, alignItems: 'center' }}>
        <Stack direction="column">
          <IconButton size="small" disabled={index === 0} onClick={() => onMove(-1)}>
            <ArrowUpwardIcon fontSize="small" />
          </IconButton>
          <IconButton size="small" disabled={index === total - 1} onClick={() => onMove(1)}>
            <ArrowDownwardIcon fontSize="small" />
          </IconButton>
        </Stack>
        <Typography variant="subtitle1" sx={{ fontWeight: 600, flex: 1 }}>
          #{cfg.priority} · {cfg.name}
          <Typography component="span" variant="caption" color="text.secondary" sx={{ ml: 1 }}>
            {cfg.agent_type}
          </Typography>
        </Typography>
        <IconButton size="small" color="error" onClick={onDelete} disabled={del.isPending}>
          <DeleteIcon fontSize="small" />
        </IconButton>
      </Stack>

      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
        <TextField
          label="Name"
          value={draft.name}
          onChange={(e) => patch({ name: e.target.value })}
          size="small"
          sx={{ flex: 1 }}
        />
        <FormControl size="small" sx={{ minWidth: 140 }}>
          <InputLabel>Agent Type</InputLabel>
          <Select
            label="Agent Type"
            value={draft.agent_type}
            onChange={(e) =>
              patch({
                agent_type: e.target.value,
                embedding_model: e.target.value === 'claude' ? '' : draft.embedding_model,
              })
            }
          >
            {AGENT_TYPES.map((t) => (
              <MenuItem key={t} value={t}>
                {t}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>

      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mt: 2 }}>
        <TextField
          label="URL"
          value={draft.url}
          onChange={(e) => patch({ url: e.target.value })}
          size="small"
          placeholder="http://host:port"
          sx={{ flex: 1 }}
        />
        <TextField
          label="API Key"
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          size="small"
          placeholder={cfg.has_api_key ? '(set — leave blank to keep)' : ''}
          sx={{ flex: 1 }}
        />
      </Stack>

      <TextField
        label="Embedding Model"
        value={isClaude ? '' : draft.embedding_model}
        onChange={(e) => patch({ embedding_model: e.target.value })}
        size="small"
        fullWidth
        disabled={isClaude}
        placeholder={isClaude ? 'Not used for Claude' : 'nomic-embed-text'}
        sx={{ mt: 2 }}
      />

      <TextField
        label="Request Timeout (seconds)"
        type="number"
        value={draft.timeout_seconds}
        onChange={(e) => patch({ timeout_seconds: parseInt(e.target.value, 10) || 600 })}
        size="small"
        fullWidth
        sx={{ mt: 2 }}
        slotProps={{ htmlInput: { min: 10, max: 3600 } }}
      />

      <Divider sx={{ my: 2 }}>
        <Typography variant="caption" color="text.secondary">
          Level slots — leave blank for low/medium/high to fall through to the next config; leave XHigh or Max empty to fall through to High in this row.
        </Typography>
      </Divider>

      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
        {SLOTS.map(({ key, label }) => {
          const v = slotValue(key)
          return (
            <FormControl size="small" fullWidth key={key}>
              <InputLabel shrink>{label}</InputLabel>
              <Select
                label={label}
                value={v && modelOptions.includes(v) ? v : v ? '__custom__' : ''}
                onChange={(e) => {
                  const sel = e.target.value
                  if (sel === '__custom__') return
                  setSlot(key, sel)
                }}
                displayEmpty
                disabled={!draft.url || models.isLoading}
              >
                <MenuItem value="">
                  <em>none</em>
                </MenuItem>
                {modelOptions.map((m) => (
                  <MenuItem key={m} value={m}>
                    {m}
                  </MenuItem>
                ))}
                {v && !modelOptions.includes(v) && (
                  <MenuItem value="__custom__">{v} (saved)</MenuItem>
                )}
              </Select>
            </FormControl>
          )
        })}
      </Stack>

      <Stack direction="row" spacing={2} sx={{ mt: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <Button variant="contained" onClick={onSave} disabled={update.isPending}>
          Save
        </Button>
        <Button
          variant="outlined"
          onClick={onTest}
          disabled={test.isPending || !draft.url || isClaude}
        >
          {test.isPending ? 'Testing…' : 'Test'}
        </Button>
        {saved && (
          <Typography variant="caption" color="success.main">
            Saved
          </Typography>
        )}
        {update.isError && (
          <Typography variant="caption" color="error">
            {update.error?.message}
          </Typography>
        )}
        {testResult && (
          <Typography variant="caption" color={testResult.ok ? 'success.main' : 'error'}>
            {testResult.ok ? testResult.message : `Failed: ${testResult.message}`}
          </Typography>
        )}
        {isClaude && (
          <Typography variant="caption" color="text.secondary">
            Test is disabled for Claude (no embedding path).
          </Typography>
        )}
      </Stack>
    </Paper>
  )
}

function NewConfigForm({ onCancel }: { onCancel: () => void }) {
  const [draft, setDraft] = useState<LLMConfigBody>(NEW_CONFIG)
  const [apiKey, setApiKey] = useState('')
  const create = useCreateLLMConfig()
  const isClaude = draft.agent_type === 'claude'

  const submit = () => {
    create.mutate(
      {
        ...draft,
        embedding_model: isClaude ? '' : draft.embedding_model,
        api_key: apiKey || undefined,
      },
      { onSuccess: () => onCancel() },
    )
  }

  return (
    <Paper variant="outlined" sx={{ p: 2, borderStyle: 'dashed' }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Add LLM Configuration
      </Typography>
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
        <TextField
          label="Name"
          value={draft.name}
          onChange={(e) => setDraft({ ...draft, name: e.target.value })}
          size="small"
          sx={{ flex: 1 }}
        />
        <FormControl size="small" sx={{ minWidth: 140 }}>
          <InputLabel>Agent Type</InputLabel>
          <Select
            label="Agent Type"
            value={draft.agent_type}
            onChange={(e) =>
              setDraft({
                ...draft,
                agent_type: e.target.value,
                embedding_model: e.target.value === 'claude' ? '' : draft.embedding_model,
              })
            }
          >
            {AGENT_TYPES.map((t) => (
              <MenuItem key={t} value={t}>
                {t}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>
      <TextField
        label="URL"
        value={draft.url}
        onChange={(e) => setDraft({ ...draft, url: e.target.value })}
        size="small"
        fullWidth
        sx={{ mt: 2 }}
      />
      <TextField
        label="API Key"
        type="password"
        value={apiKey}
        onChange={(e) => setApiKey(e.target.value)}
        size="small"
        fullWidth
        sx={{ mt: 2 }}
      />
      <TextField
        label="Embedding Model"
        value={isClaude ? '' : draft.embedding_model}
        onChange={(e) => setDraft({ ...draft, embedding_model: e.target.value })}
        size="small"
        fullWidth
        disabled={isClaude}
        placeholder={isClaude ? 'Not used for Claude' : 'nomic-embed-text'}
        sx={{ mt: 2 }}
      />
      <TextField
        label="Request Timeout (seconds)"
        type="number"
        value={draft.timeout_seconds}
        onChange={(e) => setDraft({ ...draft, timeout_seconds: parseInt(e.target.value, 10) || 600 })}
        size="small"
        fullWidth
        sx={{ mt: 2 }}
        slotProps={{ htmlInput: { min: 10, max: 3600 } }}
      />
      <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
        <Button variant="contained" onClick={submit} disabled={create.isPending}>
          Create
        </Button>
        <Button onClick={onCancel}>Cancel</Button>
        {create.isError && (
          <Typography variant="caption" color="error">
            {create.error?.message}
          </Typography>
        )}
      </Stack>
    </Paper>
  )
}

function LLMConfigPage() {
  const { data, isLoading } = useLLMConfigs()
  const reorder = useReorderLLMConfigs()
  const [adding, setAdding] = useState(false)

  if (isLoading) return <CircularProgress sx={{ m: 4 }} />

  const configs = data ?? []

  const move = (idx: number, dir: -1 | 1) => {
    const next = [...configs]
    const target = idx + dir
    if (target < 0 || target >= next.length) return
    ;[next[idx], next[target]] = [next[target], next[idx]]
    reorder.mutate(next.map((c) => c.id!).filter(Boolean))
  }

  return (
    <Box sx={{ maxWidth: 880 }}>
      <Stack direction="row" sx={{ mb: 3, alignItems: 'center' }}>
        <Typography variant="h5" sx={{ fontWeight: 600, flex: 1 }}>
          LLM Configurations
        </Typography>
        {!adding && (
          <Button variant="contained" onClick={() => setAdding(true)}>
            Add Config
          </Button>
        )}
      </Stack>

      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Configs are evaluated in priority order. For any requested level
        (low/medium/high/xhigh/max), the first non-empty slot wins — empty
        slots fall through to the next config, then to defaults. XHigh and
        Max additionally fall through to High within the same row when their
        dedicated slot is empty. Claude configs don't use the embedding model.
      </Typography>

      <Stack spacing={2}>
        {configs.map((c, i) => (
          <ConfigRow
            key={c.id}
            cfg={c}
            index={i}
            total={configs.length}
            onMove={(dir) => move(i, dir)}
          />
        ))}
        {adding && <NewConfigForm onCancel={() => setAdding(false)} />}
        {configs.length === 0 && !adding && (
          <Paper variant="outlined" sx={{ p: 4, textAlign: 'center' }}>
            <Typography color="text.secondary" sx={{ mb: 2 }}>
              No LLM configurations yet.
            </Typography>
            <Button variant="contained" onClick={() => setAdding(true)}>
              Add your first config
            </Button>
          </Paper>
        )}
      </Stack>
    </Box>
  )
}

export const Route = createFileRoute('/admin/llm')({
  component: LLMConfigPage,
})
