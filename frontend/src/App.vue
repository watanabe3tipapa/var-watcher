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

const keyword = ref('')
const since = ref('')
const until = ref('')
const src = ref('')
const limit = ref(200)
const searchEnabled = ref(true)
const searching = ref(false)
const searchResults = ref<LogLine[]>([])

function toISO(v: string): string {
  if (!v) return ''
  const d = new Date(v)
  return Number.isNaN(d.getTime()) ? '' : d.toISOString()
}

async function doSearch() {
  searching.value = true
  const p = new URLSearchParams()
  p.set('limit', String(limit.value || 200))
  if (keyword.value) p.set('q', keyword.value)
  if (src.value) p.set('source', src.value)
  const sinceIso = toISO(since.value)
  const untilIso = toISO(until.value)
  if (sinceIso) p.set('since', sinceIso)
  if (untilIso) p.set('until', untilIso)
  try {
    const res = await fetch(`/api/logs?${p.toString()}`)
    const body = await res.json()
    searchEnabled.value = body.enabled ?? true
    searchResults.value = body.logs ?? []
  } catch {
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

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

    <div class="col">
      <section class="search">
        <h2>Log Search (persisted)</h2>
      <div class="row">
        <input v-model="keyword" type="text" placeholder="キーワード (path / message)" @keyup.enter="doSearch" />
        <select v-model="src">
          <option value="">全 source</option>
          <option v-for="w in watchers" :key="w.name" :value="w.name">{{ w.name }}</option>
        </select>
      </div>
      <div class="row">
        <label>from <input v-model="since" type="datetime-local" /></label>
        <label>to <input v-model="until" type="datetime-local" /></label>
        <label>limit <input v-model.number="limit" type="number" min="1" max="5000" /></label>
      </div>
      <div class="row">
        <button class="on" :disabled="searching" @click="doSearch">検索</button>
        <span v-if="!searchEnabled" class="err">永続化が無効 (store 未設定)</span>
        <span v-else class="count">{{ searchResults.length }} 件</span>
      </div>
      <pre id="search-view">
        <span v-if="searchResults.length === 0 && !searching">検索結果はここに表示されます。</span>
        <span v-for="(l, i) in searchResults" :key="'s' + i">
[{{ formatTs(l.ts) }}] [{{ l.source }}] {{ l.message }}
</span>
      </pre>
      </section>
      <section class="logs">
        <h2>Logs</h2>
        <pre id="log-view">
          <span v-for="(l, i) in logs" :key="i">
[{{ formatTs(l.ts) }}] [{{ l.source }}] {{ l.message }}
</span>
        </pre>
      </section>
    </div>
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
main { padding: 1rem 1.5rem; display: grid; grid-template-columns: 360px 1fr; gap: 1.5rem; align-items: start; }
@media (max-width: 800px) { main { grid-template-columns: 1fr; } }
.col { display: flex; flex-direction: column; gap: 1.5rem; min-width: 0; }
section { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 1rem; }
section h2 { margin: 0 0 0.75rem; font-size: 0.9rem; text-transform: uppercase; color: #94a3b8; }
.row { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; margin-bottom: 0.5rem; }
.row label { display: flex; align-items: center; gap: 0.3rem; color: #94a3b8; font-size: 0.75rem; }
.row input[type="text"], .row select { flex: 1; min-width: 12rem; }
input, select { background: #0b1220; color: #e2e8f0; border: 1px solid #334155; border-radius: 6px; padding: 0.35rem 0.5rem; font-size: 0.8rem; }
.count { color: #58a6ff; font-size: 0.8rem; }
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
#search-view { max-height: 30vh; overflow-y: auto; font-size: 0.8rem; line-height: 1.4; margin: 0; white-space: pre-wrap; word-break: break-all; color: #7dd3a8; }
#log-view { max-height: 70vh; overflow-y: auto; font-size: 0.8rem; line-height: 1.4; margin: 0; white-space: pre-wrap; word-break: break-all; }
</style>
