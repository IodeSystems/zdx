import { useEffect, useRef, useCallback, useLayoutEffect } from 'react'
import { getToken } from '../api/client'

type MessageHandler = (msg: { channel: string; type: string; payload: unknown }) => void

const RECONNECT_DELAY = 2000

type OutgoingMessage =
  | { type: 'subscribe'; token: string }
  | { type: 'unsubscribe'; channel: string }
  | { type: 'ping' }

type IncomingMessage =
  | { type: 'subscribed'; channel: string }
  | { type: 'unsubscribed'; channel: string }
  | { type: 'message'; channel: string; event: string; payload: unknown }
  | { type: 'error'; message: string }
  | { type: 'pong' }

type ChannelState = {
  handlers: Set<MessageHandler>
  subscribed: boolean
}

const channels = new Map<string, ChannelState>()

let socket: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let pending: OutgoingMessage[] = []

function getWsUrl(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}/ws`
}

async function signChannel(channel: string): Promise<string> {
  const apiToken = getToken()
  if (!apiToken) throw new Error('not authenticated')
  const res = await fetch('/api/ws/sign', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Api-Key': apiToken },
    body: JSON.stringify({ channel }),
  })
  if (!res.ok) throw new Error(`sign failed: ${res.status}`)
  const data = await res.json()
  return data.token as string
}

function sendOrQueue(msg: OutgoingMessage) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(msg))
  } else {
    pending.push(msg)
    ensureConnected()
  }
}

function ensureConnected() {
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) return
  if (channels.size === 0 && pending.length === 0) return

  const ws = new WebSocket(getWsUrl())
  socket = ws

  ws.onopen = () => {
    // Flush queued messages first.
    const queued = pending
    pending = []
    for (const m of queued) ws.send(JSON.stringify(m))
    // Re-subscribe any channels that think they have handlers but are not
    // marked subscribed (fresh connection after a drop).
    for (const [channel, state] of channels) {
      if (state.handlers.size === 0) continue
      if (state.subscribed) continue
      signChannel(channel)
        .then(token => {
          if (socket === ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'subscribe', token }))
          }
        })
        .catch(() => {})
    }
  }

  ws.onmessage = (ev) => {
    let msg: IncomingMessage
    try {
      msg = JSON.parse(ev.data)
    } catch {
      return
    }
    switch (msg.type) {
      case 'message': {
        const state = channels.get(msg.channel)
        if (!state) return
        const relay = { channel: msg.channel, type: msg.event, payload: msg.payload }
        state.handlers.forEach(h => h(relay))
        return
      }
      case 'subscribed': {
        const state = channels.get(msg.channel)
        if (state) state.subscribed = true
        return
      }
      case 'unsubscribed': {
        const state = channels.get(msg.channel)
        if (state) state.subscribed = false
        return
      }
      case 'error':
        console.warn('[ws] server error:', msg.message)
        return
      case 'pong':
        return
    }
  }

  const onClose = () => {
    if (socket !== ws) return
    socket = null
    for (const state of channels.values()) state.subscribed = false
    if (channels.size > 0 && !reconnectTimer) {
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        ensureConnected()
      }, RECONNECT_DELAY)
    }
  }
  ws.onclose = onClose
  ws.onerror = onClose
}

function subscribeChannel(channel: string) {
  signChannel(channel)
    .then(token => {
      const state = channels.get(channel)
      if (!state || state.handlers.size === 0) return
      sendOrQueue({ type: 'subscribe', token })
    })
    .catch(() => {})
}

function unsubscribeChannel(channel: string) {
  sendOrQueue({ type: 'unsubscribe', channel })
}

export function useChannel(channel: string | null, onMessage: MessageHandler) {
  const handlerRef = useRef(onMessage)
  useLayoutEffect(() => {
    handlerRef.current = onMessage
  })

  const stableHandler = useCallback((msg: { channel: string; type: string; payload: unknown }) => {
    handlerRef.current(msg)
  }, [])

  useEffect(() => {
    if (!channel) return

    let state = channels.get(channel)
    const isFirst = !state || state.handlers.size === 0
    if (!state) {
      state = { handlers: new Set(), subscribed: false }
      channels.set(channel, state)
    }
    state.handlers.add(stableHandler)

    if (isFirst) {
      ensureConnected()
      subscribeChannel(channel)
    }

    return () => {
      const s = channels.get(channel)
      if (!s) return
      s.handlers.delete(stableHandler)
      if (s.handlers.size === 0) {
        if (s.subscribed) unsubscribeChannel(channel)
        channels.delete(channel)
        if (channels.size === 0 && socket) {
          socket.close()
          socket = null
          if (reconnectTimer) {
            clearTimeout(reconnectTimer)
            reconnectTimer = null
          }
        }
      }
    }
  }, [channel, stableHandler])
}
