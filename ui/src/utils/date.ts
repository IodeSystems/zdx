export function fmtDate(ts: string): string {
  if (!ts) return ''
  return new Date(ts).toLocaleDateString()
}

