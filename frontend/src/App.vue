<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

interface WatcherInfo {
  name: string
  command: string
  args: string[]
  enabled: boolean
  state: string
  error?: string
  installed: boolean
  pid: number
  builtin: boolean
}

interface LogLine {
  ts: string
  source: string
  level: string
  message: string
}

const watchers = ref<WatcherInfo[]>([])
const logs = ref<LogLine[]>([])
const connected = ref(false)
const maxLogs = 500

async function refresh() {
  const res = await fetch('/api/watchers')
  if (res.ok) watchers.value = await res.json()
}

async function toggle(w: WatcherInfo) {
  const action = w.enabled ? 'stop' : 'start'
  const res = await fetch(`/api/watchers/${encodeURIComponent(w.name)}/${action}`, { method: 'POST' })
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    logs.value.push({ ts: new Date().toISOString(), source: 'api', level: 'error', message: body?.error ?? 'failed' })
    return
  }
  await refresh()
}

function formatTs(ts: string): string {
  const d = new Date(ts)
  return Number.isNaN(d.getTime()) ? ts : d.toLocaleTimeString()
}

let ws: WebSocket | null = null

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws/logs`)
  ws.onopen = () => (connected.value = true)
  ws.onclose = () => {
    connected.value = false
    setTimeout(connect, 2000)
  }
  ws.onmessage = (ev) => {
    try {
      const line = JSON.parse(ev.data) as LogLine
      logs.value.push(line)
      if (logs.value.length > maxLogs) logs.value.splice(0, logs.value.length - maxLogs)
    } catch {
      /* ignore */
    }
  }
}

onMounted(() => {
  refresh()
  connect()
  setInterval(refresh, 5000)
})

onBeforeUnmount(() => ws?.close())
</script>

<template>
  <header>
    <h1>var-watcher</h1>
    <p class="sub">macOS /var 監視 — TUI / Web UI 統合</p>
    <span class="conn" :class="connected ? 'ok' : 'bad'">
      {{ connected ? '● WebSocket 接続中' : '○ 接続待機' }}
    </span>
  </header>

  <main>
    <section class="watchers">
      <h2>Watchers</h2>
      <ul>
        <li v-for="w in watchers" :key="w.name">
          <div class="w-info">
            <strong>{{ w.name }}</strong>
            <code>{{ w.command }} {{ w.args.join(' ') }}</code>
            <em v-if="w.state === 'running'">(pid {{ w.pid }})</em>
            <em v-else-if="w.state === 'error'" class="err">error: {{ w.error }}</em>
          </div>
          <button
            :class="w.enabled ? 'off' : 'on'"
            :disabled="!w.installed"
            @click="toggle(w)"
          >
            {{ w.enabled ? 'Stop' : 'Start' }}
          </button>
        </li>
      </ul>
    </section>

    <section class="logs">
      <h2>Logs</h2>
      <pre id="log-view">
        <span v-for="(l, i) in logs" :key="i">
[{{ formatTs(l.ts) }}] [{{ l.source }}] {{ l.message }}
</span>
      </pre>
    </section>
  </main>
</template>

<style>
* { box-sizing: border-box; }
body { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
header { padding: 1rem 1.5rem; border-bottom: 1px solid #334155; display: flex; align-items: center; gap: 1rem; }
header h1 { margin: 0; font-size: 1.25rem; }
.sub { margin: 0; color: #94a3b8; flex: 1; }
.conn.ok { color: #4ade80; }
.conn.bad { color: #f87171; }
main { padding: 1rem 1.5rem; display: grid; grid-template-columns: 360px 1fr; gap: 1.5rem; }
@media (max-width: 800px) { main { grid-template-columns: 1fr; } }
section { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 1rem; }
section h2 { margin: 0 0 0.75rem; font-size: 0.9rem; text-transform: uppercase; color: #94a3b8; }
ul { list-style: none; margin: 0; padding: 0; }
li { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; padding: 0.5rem 0; border-bottom: 1px dashed #1e293b; }
li:last-child { border-bottom: none; }
.w-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.w-info code { color: #94a3b8; font-size: 0.75rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.err { color: #f87171; }
button { border: none; border-radius: 6px; padding: 0.4rem 0.9rem; cursor: pointer; font-weight: 600; }
button.on { background: #16a34a; color: #fff; }
button.off { background: #dc2626; color: #fff; }
button:disabled { background: #334155; color: #64748b; cursor: not-allowed; }
#log-view { max-height: 70vh; overflow-y: auto; font-size: 0.8rem; line-height: 1.4; margin: 0; white-space: pre-wrap; word-break: break-all; }
</style>
