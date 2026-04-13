import { Box, Chip, Typography } from '@mui/material'
import { Code as CodeIcon } from '@mui/icons-material'
import type { CodeRefItem } from '../api'

function formatRef(r: CodeRefItem): string {
  let loc = r.file_path
  if (r.line_start > 0) {
    if (r.line_end > 0 && r.line_end !== r.line_start) {
      loc += `:${r.line_start}-${r.line_end}`
    } else {
      loc += `:${r.line_start}`
    }
  }
  return loc
}

function shortHash(h: string): string {
  return h ? h.slice(0, 8) : ''
}

export function CodeRefs({ refs }: { refs: CodeRefItem[] }) {
  if (!refs || refs.length === 0) return null

  return (
    <Box sx={{ mb: 3 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        Code References ({refs.length})
      </Typography>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
        {refs.map(r => (
          <Box key={r.id} sx={{ display: 'flex', gap: 1, alignItems: 'flex-start', flexWrap: 'wrap' }}>
            <CodeIcon sx={{ fontSize: 16, mt: 0.4, color: 'text.secondary', flexShrink: 0 }} />
            <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center', flexWrap: 'wrap', flex: 1 }}>
              <Typography
                variant="body2"
                component="code"
                sx={{ fontFamily: 'monospace', fontSize: '0.8rem', wordBreak: 'break-all' }}
              >
                {formatRef(r)}
              </Typography>
              {r.git_hash && (
                <Chip label={`@${shortHash(r.git_hash)}`} size="small" variant="outlined" sx={{ fontFamily: 'monospace', fontSize: '0.7rem' }} />
              )}
              {r.note && (
                <Typography variant="caption" color="text.secondary">
                  — {r.note}
                </Typography>
              )}
            </Box>
          </Box>
        ))}
      </Box>
    </Box>
  )
}
