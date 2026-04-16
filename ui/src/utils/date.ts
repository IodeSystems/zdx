export function fmtDate(ts: string): string {
  if (!ts) return ''
  return new Date(ts).toLocaleDateString()
}

export function fmtRelative(ts: string): string {
  if (!ts) return ''
  const then = new Date(ts)
  const now = new Date()
  const diffMs = now.getTime() - then.getTime()
  if (diffMs < 0) return then.toLocaleDateString()
  const sameDay =
    then.getFullYear() === now.getFullYear() &&
    then.getMonth() === now.getMonth() &&
    then.getDate() === now.getDate()
  if (!sameDay) return then.toLocaleDateString()
  const minutes = Math.floor(diffMs / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ago`
}

