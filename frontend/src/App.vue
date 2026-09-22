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

interface FiredAlert {
  id: string
  name: string
  count: number
  fired_at: string
  last?: string
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

const alertsEnabled = ref(false)
const alerts = ref<FiredAlert[]>([])

interface StatsBucket { label: string; count: number }
interface StatsSource { source: string; count: number }
interface StatsTopPath { path: string; count: number }
interface StatsPayload {
  total: number
  hourly: StatsBucket[]
  weekly: StatsBucket[]
  by_source: StatsSource[]
  top_paths: StatsTopPath[]
  window_sec: number
}

const statsEnabled = ref(false)
const stats = ref<StatsPayload | null>(null)

async function refreshStats() {
  try {
    const res = await fetch('/api/stats?hours=24')
    const body = await res.json()
    statsEnabled.value = body.enabled ?? false
    stats.value = body.stats ?? null
  } catch {
    statsEnabled.value = false
  }
}

interface TreeNode {
  name: string
  path: string
  count: number
  children?: TreeNode[]
}

const treeEnabled = ref(false)
const tree = ref<TreeNode | null>(null)
const collapsed = ref<Set<string>>(new Set())

async function refreshTree() {
  try {
    const res = await fetch('/api/tree?hours=24')
    const body = await res.json()
    treeEnabled.value = body.enabled ?? false
    tree.value = body.tree ?? null
    collapsed.value = new Set()
  } catch {
    treeEnabled.value = false
  }
}

function toggleNode(n: TreeNode) {
  const next = new Set(collapsed.value)
  if (next.has(n.path)) next.delete(n.path)
  else next.add(n.path)
  collapsed.value = next
}

function heatColor(count: number, max: number): string {
  if (max <= 0 || count <= 0) return 'rgba(56,189,248,0.08)'
  const r = count / max
  const hue = 200 - r * 100 // 青 → 橙 → 赤
  return `hsl(${hue}, 80%, ${50 - r * 20}%)`
}

function treeMax(n: TreeNode | null): number {
  if (!n) return 0
  let m = n.count
  for (const c of n.children ?? []) m = Math.max(m, treeMax(c))
  return m
}

interface TreeRow {
  path: string
  label: string
  count: number
  depth: number
  hasChildren: boolean
}

function flattenTree(n: TreeNode, depth: number, coll: Set<string>): TreeRow[] {
  const rows: TreeRow[] = []
  const visit = (node: TreeNode, d: number) => {
    rows.push({ path: node.path, label: node.name, count: node.count, depth: d, hasChildren: (node.children?.length ?? 0) > 0 })
    if (coll.has(node.path)) return
    for (const c of node.children ?? []) visit(c, d + 1)
  }
  for (const c of n.children ?? []) visit(c, depth)
  return rows
}

function applyTreeFilter(path: string) {
  keyword.value = path
  doSearch()
}

interface DiffChange {
  kind: 'added' | 'removed' | 'context'
  line: string
  line_no: number
}

interface FileDiff {
  path: string
  ts: string
  added: number
  removed: number
  changes: DiffChange[]
}

const diffsEnabled = ref(false)
const diffs = ref<FileDiff[]>([])

async function refreshDiffs() {
  try {
    const res = await fetch('/api/diffs?limit=20')
    const body = await res.json()
    diffsEnabled.value = body.enabled ?? false
    diffs.value = body.diffs ?? []
  } catch {
    diffsEnabled.value = false
  }
}

function diffKindCls(kind: string): string {
  return kind === 'added' ? 'diff-add' : kind === 'removed' ? 'diff-del' : 'diff-ctx'
}

function maxCount(buckets: StatsBucket[]): number {
  return buckets.reduce((m, b) => (b.count > m ? b.count : m), 0)
}

function barWidth(b: StatsBucket, n: number): string {
  if (n === 0) return '0%'
  return `${Math.max(2, Math.round((b.count / n) * 100))}%`
}

async function refreshAlerts() {
  try {
    const res = await fetch('/api/alerts')
    const body = await res.json()
    alertsEnabled.value = body.enabled ?? false
    alerts.value = body.fired ?? []
  } catch {
    /* ignore */
  }
}

function toISO(v: string): string {
  if (!v) return ''
  const d = new Date(v)
  return Number.isNaN(d.getTime()) ? '' : d.toISOString()
}

const exportFormat = ref('json')

function doExport() {
  const p = new URLSearchParams()
  p.set('format', exportFormat.value)
  if (keyword.value) p.set('q', keyword.value)
  if (src.value) p.set('source', src.value)
  const sinceIso = toISO(since.value)
  const untilIso = toISO(until.value)
  if (sinceIso) p.set('since', sinceIso)
  if (untilIso) p.set('until', untilIso)
  window.location.href = `/api/export?${p.toString()}`
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
  refreshAlerts()
  refreshStats()
  refreshTree()
  refreshDiffs()
  connect()
  setInterval(refresh, 5000)
  setInterval(refreshAlerts, 5000)
  setInterval(refreshStats, 30000)
  setInterval(refreshTree, 30000)
  setInterval(refreshDiffs, 5000)
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
    <span v-if="alertsEnabled && alerts.length" class="alert-badge">⚠ {{ alerts.length }} alerts</span>
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

    <section class="alerts">
      <h2>Alerts</h2>
      <p v-if="alertsEnabled && alerts.length === 0" class="hint">まだアラートはありません。</p>
      <p v-else-if="!alertsEnabled" class="err">アラート未設定 (config の alerts を確認)</p>
      <ul v-else>
        <li v-for="a in alerts" :key="a.id + a.fired_at">
          <div class="w-info">
            <strong>⚠ {{ a.name }}</strong>
            <code>{{ formatTs(a.fired_at) }} — {{ a.count }} events</code>
            <em v-if="a.last">{{ a.last }}</em>
          </div>
        </li>
      </ul>
    </section>

    <section class="dashboard">
      <h2>Dashboard (24h)</h2>
      <p v-if="!statsEnabled" class="err">永続化が無効 (store 未設定)</p>
      <template v-else-if="stats">
        <div class="stat-total">
          合計イベント: <strong>{{ stats.total }}</strong>
          <span class="hint">(過去 24 時間)</span>
        </div>

        <h3 class="stat-h">時間帯別イベント数</h3>
        <div class="bars">
          <div v-for="b in stats.hourly" :key="b.label" class="bar-row">
            <span class="bar-label">{{ b.label }}</span>
            <div class="bar-track">
              <div class="bar-fill" :style="{ width: barWidth(b, stats.total) }" :title="`${b.label}: ${b.count}`"></div>
            </div>
            <span class="bar-count">{{ b.count }}</span>
          </div>
        </div>

        <h3 class="stat-h">エンジン別</h3>
        <div class="bars">
          <div v-for="s in stats.by_source" :key="s.source" class="bar-row">
            <span class="bar-label">{{ s.source }}</span>
            <div class="bar-track">
              <div class="bar-fill src" :style="{ width: barWidth({ label: s.source, count: s.count }, stats.total) }" :title="`${s.source}: ${s.count}`"></div>
            </div>
            <span class="bar-count">{{ s.count }}</span>
          </div>
        </div>

        <h3 class="stat-h">TOP 10 変更パス</h3>
        <ul>
          <li v-for="p in stats.top_paths" :key="p.path">
            <div class="w-info">
              <code>{{ p.path }}</code>
              <em>{{ p.count }} 件</em>
            </div>
          </li>
        </ul>
      </template>
    </section>

    <section class="dashboard">
      <h2>Directory Tree (heatmap 24h)</h2>
      <p v-if="!treeEnabled" class="err">永続化が無効 (store 未設定)</p>
      <template v-else-if="tree">
        <p class="hint">色が濃いほど変更が多い(クリックで展開/折りたたみ、パスで検索)</p>
        <div class="tree">
          <div class="tree-row" :style="{ background: heatColor(tree.count, treeMax(tree)) }">
            <button class="tree-folder" @click="toggleNode(tree)">{{ collapsed.has(tree.path) ? '▸' : '▾' }}</button>
            <span class="tree-name" @click="applyTreeFilter(tree.path)">/</span>
            <span class="tree-count">{{ tree.count }}</span>
          </div>
          <div
            v-for="row in flattenTree(tree, 1, collapsed)"
            :key="row.path"
            class="tree-row"
            :style="{ background: heatColor(row.count, treeMax(tree)), paddingLeft: (row.depth * 14 + 8) + 'px' }"
          >
            <span class="tree-mark" v-if="row.hasChildren">{{ row.depth > 0 ? '└' : '├' }}</span>
            <button class="tree-folder" v-if="row.hasChildren" @click="toggleNode(row)">{{ collapsed.has(row.path) ? '▸' : '▾' }}</button>
            <span class="tree-name" :style="{ color: heatColor(row.count, treeMax(tree)) }" @click="applyTreeFilter(row.path)">{{ row.label }}</span>
            <span class="tree-count">{{ row.count }}</span>
          </div>
        </div>
      </template>
    </section>

    <section class="diffs">
      <h2>Diffs (file changes)</h2>
      <p class="hint">テキストファイルの変更前後をハイライト(緑: 追加 / 赤: 削除)。CTRL で複数選択可。</p>
      <p v-if="!diffsEnabled" class="err">差分管理が無効 (main で未登録)</p>
      <p v-else-if="diffs.length === 0" class="hint">まだ差分はありません。</p>
      <div v-for="d in diffs" :key="d.path + d.ts" class="diff-card">
        <div class="diff-head">
          <strong>{{ d.path }}</strong>
          <span class="diff-meta">
            {{ formatTs(d.ts) }} — <em class="add">+{{ d.added }}</em> / <em class="del">-{{ d.removed }}</em>
          </span>
        </div>
        <pre class="diff-body">
          <span v-for="(c, i) in d.changes" :key="i" :class="[diffKindCls(c.kind), c.kind + '-row']">
<span v-if="c.kind !== 'context'" class="diff-mark">{{ c.kind === 'added' ? '+' : '-' }}</span><span v-else class="diff-mark"> </span>{{ c.line }}
</span>
        </pre>
      </div>
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
        <button class="ex" :disabled="!searchEnabled" @click="doExport">エクスポート</button>
        <select v-model="exportFormat" class="fmt">
          <option value="json">JSON</option>
          <option value="csv">CSV</option>
        </select>
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
button.ex { background: #3b82f6; color: #fff; }
button.off { background: #dc2626; color: #fff; }
.fmt { min-width: 5rem; width: auto !important; }
button:disabled { background: #334155; color: #64748b; cursor: not-allowed; }
#search-view { max-height: 30vh; overflow-y: auto; font-size: 0.8rem; line-height: 1.4; margin: 0; white-space: pre-wrap; word-break: break-all; color: #7dd3a8; }
#log-view { max-height: 70vh; overflow-y: auto; font-size: 0.8rem; line-height: 1.4; margin: 0; white-space: pre-wrap; word-break: break-all; }
.stat-total { margin-bottom: 0.5rem; font-size: 0.9rem; }
.stat-h { margin: 0.75rem 0 0.25rem; font-size: 0.8rem; text-transform: uppercase; color: #94a3b8; }
.bars { display: flex; flex-direction: column; gap: 0.25rem; }
.bar-row { display: flex; align-items: center; gap: 0.5rem; font-size: 0.7rem; }
.bar-label { min-width: 3.5rem; color: #94a3b8; text-align: right; }
.bar-track { flex: 1; background: #1e293b; border-radius: 4px; height: 0.8rem; overflow: hidden; }
.bar-fill { height: 100%; background: #38bdf8; border-radius: 4px 0 0 4px; }
.bar-fill.src { background: #a78bfa; }
.bar-count { min-width: 2.5rem; color: #e2e8f0; }
.hint { color: #94a3b8; font-size: 0.75rem; }
.tree { display: flex; flex-direction: column; gap: 2px; font-size: 0.72rem; }
.tree-row { display: flex; align-items: center; gap: 0.4rem; border-radius: 4px; padding: 2px 6px; cursor: pointer; }
.tree-folder { background: none; border: none; color: #94a3b8; padding: 0 2px; cursor: pointer; font-size: 0.7rem; }
.tree-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #e2e8f0; }
.tree-count { color: #94a3b8; min-width: 3rem; text-align: right; }
.diff-card { border: 1px solid #334155; border-radius: 6px; margin-bottom: 0.75rem; overflow: hidden; }
.diff-head { display: flex; align-items: center; justify-content: space-between; gap: 0.5rem; padding: 0.4rem 0.6rem; background: #0b1220; font-size: 0.75rem; }
.diff-meta { color: #94a3b8; white-space: nowrap; }
.diff-meta .add { color: #4ade80; }
.diff-meta .del { color: #f87171; }
.diff-body { margin: 0; padding: 0.3rem 0.5rem; font-size: 0.72rem; line-height: 1.5; max-height: 16rem; overflow-y: auto; }
.add-row { background: rgba(74, 222, 128, 0.12); color: #a7f3d0; }
.del-row { background: rgba(248, 113, 113, 0.12); color: #fecaca; }
.ctx-row { color: #64748b; }
.diff-mark { display: inline-block; width: 1rem; user-select: none; }
</style>
