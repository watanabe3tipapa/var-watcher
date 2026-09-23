<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LANG_KEY, i18n, normalizeLang } from './i18n'

const { t, locale } = useI18n()

function setLang(lang: string) {
  locale.value = normalizeLang(lang)
  localStorage.setItem(LANG_KEY, locale.value)
}

async function initLang() {
  const saved = localStorage.getItem(LANG_KEY)
  if (saved && normalizeLang(saved) === saved) {
    locale.value = saved
    return
  }
  try {
    const res = await fetch('/api/config')
    const body = await res.json()
    locale.value = normalizeLang(body?.lang)
  } catch {
    locale.value = normalizeLang(navigator.language)
  }
}

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

interface PresetInfo {
  id: string
  name?: string
  target: string
  enabled?: Record<string, boolean>
  filter?: string
}

const presets = ref<PresetInfo[]>([])
const presetTarget = ref('')
const presetId = ref('')

async function refreshPresets() {
  try {
    const res = await fetch('/api/presets')
    const body = await res.json()
    presets.value = body.presets ?? []
    presetTarget.value = body.target ?? ''
    if (!presetId.value) presetId.value = presets.value[0]?.id ?? ''
  } catch {
    presets.value = []
  }
}

async function applyPreset(id: string) {
  const res = await fetch(`/api/presets/${encodeURIComponent(id)}/apply`, { method: 'POST' })
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    logs.value.push({ ts: new Date().toISOString(), source: 'api', level: 'error', message: body?.error ?? t('errors.presetApply') })
    return
  }
  const body = await res.json()
  presetTarget.value = body.target ?? presetTarget.value
  await refresh()
  logs.value.push({ ts: new Date().toISOString(), source: 'api', level: 'info', message: t('errors.presetApplied', { id: body.applied, target: presetTarget.value }) })
}

interface PerfSample {
  ts: string
  events_sec: number
  alloc_mb: number
  sys_mb: number
  goroutines: number
  cpu_sec: number
}

interface PerfPayload {
  enabled: boolean
  warned: boolean
  mem_warn_mb: number
  samples: PerfSample[]
}

const perfEnabled = ref(false)
const perf = ref<PerfPayload | null>(null)

async function refreshPerf() {
  try {
    const res = await fetch('/api/perf')
    const body = await res.json()
    perfEnabled.value = body.enabled ?? false
    perf.value = body
  } catch {
    perfEnabled.value = false
  }
}

function perfMax(field: keyof PerfSample): number {
  return perf.value?.samples.reduce((m, s) => Math.max(m, Number(s[field]) || 0), 0) ?? 0
}

function perfWidth(v: number, max: number): string {
  if (!max) return '0%'
  return `${Math.max(2, Math.round((v / max) * 100))}%`
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
    logs.value.push({ ts: new Date().toISOString(), source: 'api', level: 'error', message: body?.error ?? t('errors.watchAction') })
    return
  }
  await refresh()
}

function formatTs(ts: string): string {
  const d = new Date(ts)
  return Number.isNaN(d.getTime()) ? ts : d.toLocaleTimeString(locale.value)
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
  initLang()
  refresh()
  refreshAlerts()
  refreshStats()
  refreshTree()
  refreshDiffs()
  refreshPresets()
  refreshPerf()
  connect()
  setInterval(refresh, 5000)
  setInterval(refreshAlerts, 5000)
  setInterval(refreshStats, 30000)
  setInterval(refreshTree, 30000)
  setInterval(refreshDiffs, 5000)
  setInterval(refreshPerf, 2000)
})

onBeforeUnmount(() => ws?.close())
</script>

<template>
  <header>
    <h1>var-watcher</h1>
    <p class="sub">{{ t('header.sub') }}</p>
    <button class="lang" @click="setLang(locale === 'ja' ? 'en' : 'ja')">{{ t('header.switchTo') }}</button>
    <span class="conn" :class="connected ? 'ok' : 'bad'">
      {{ connected ? t('header.connOk') : t('header.connWait') }}
    </span>
    <span v-if="alertsEnabled && alerts.length" class="alert-badge">⚠ {{ t('header.alerts', alerts.length) }}</span>
  </header>

  <main>
    <section class="watchers">
      <h2>{{ t('watchers.title') }}</h2>
      <ul>
        <li v-for="w in watchers" :key="w.name">
          <div class="w-info">
            <strong>{{ w.name }}</strong>
            <code>{{ w.command }} {{ w.args.join(' ') }}</code>
            <em v-if="w.state === 'running'">(pid {{ w.pid }})</em>
            <em v-else-if="w.state === 'error'" class="err">{{ t('watchers.error') }}: {{ w.error }}</em>
          </div>
          <button
            :class="w.enabled ? 'off' : 'on'"
            :disabled="!w.installed"
            @click="toggle(w)"
          >
            {{ w.enabled ? t('watchers.stop') : t('watchers.start') }}
          </button>
        </li>
      </ul>
    </section>

    <section class="presets">
      <h2>{{ t('presets.title') }}</h2>
      <div class="row">
        <select v-model="presetId" class="preset-select">
          <option v-for="p in presets" :key="p.id" :value="p.id">{{ p.name || p.id }}</option>
        </select>
        <button class="ex" @click="applyPreset(presetId)">{{ t('presets.apply') }}</button>
        <span class="count">{{ t('presets.target') }} <code>{{ presetTarget }}</code></span>
      </div>
      <p class="hint">{{ t('presets.hint') }}</p>
    </section>

    <section class="dashboard">
      <h2>{{ t('perf.title') }}</h2>
      <p v-if="!perfEnabled" class="err">{{ t('perf.disabled') }}</p>
      <template v-else-if="perf">
        <div v-if="perf.warned" class="perf-warn">{{ t('perf.warn', { n: perf.mem_warn_mb }) }}</div>
        <div class="stat-total">
          {{ t('perf.eventsPerSec') }} <strong>{{ perfMax('events_sec') > 0 ? perf.samples[perf.samples.length - 1]?.events_sec ?? 0 : 0 }}</strong>
          <span class="hint">{{ t('perf.last', { n: perf.samples.length }) }}</span>
        </div>
        <h3 class="stat-h">{{ t('perf.rate') }}</h3>
        <div class="bars">
          <div v-for="(s, i) in perf.samples.slice(-20)" :key="i" class="bar-row">
            <span class="bar-label">{{ new Date(s.ts).toLocaleTimeString(locale, { minute: '2-digit', second: '2-digit' }) }}</span>
            <div class="bar-track">
              <div class="bar-fill" :style="{ width: perfWidth(s.events_sec, perfMax('events_sec')) }" :title="`${s.events_sec} events/s`"></div>
            </div>
            <span class="bar-count">{{ s.events_sec }}</span>
          </div>
        </div>
        <h3 class="stat-h">{{ t('perf.heap') }}</h3>
        <div class="bars">
          <div v-for="(s, i) in perf.samples.slice(-20)" :key="i" class="bar-row">
            <span class="bar-label">{{ new Date(s.ts).toLocaleTimeString(locale, { minute: '2-digit', second: '2-digit' }) }}</span>
            <div class="bar-track">
              <div class="bar-fill mem" :style="{ width: perfWidth(s.alloc_mb, perfMax('alloc_mb')) }" :title="`${s.alloc_mb} MiB`"></div>
            </div>
            <span class="bar-count">{{ s.alloc_mb }}</span>
          </div>
        </div>
        <p class="hint">{{ t('perf.cpu', { g: perf.samples[perf.samples.length - 1]?.goroutines ?? 0, c: (perf.samples[perf.samples.length - 1]?.cpu_sec ?? 0).toFixed(2) }) }}</p>
      </template>
    </section>

    <section class="alerts">
      <h2>{{ t('alerts.title') }}</h2>
      <p v-if="alertsEnabled && alerts.length === 0" class="hint">{{ t('alerts.none') }}</p>
      <p v-else-if="!alertsEnabled" class="err">{{ t('alerts.notConfigured') }}</p>
      <ul v-else>
        <li v-for="a in alerts" :key="a.id + a.fired_at">
          <div class="w-info">
            <strong>⚠ {{ a.name }}</strong>
            <code>{{ formatTs(a.fired_at) }} — {{ t('alerts.count', a.count) }}</code>
            <em v-if="a.last">{{ a.last }}</em>
          </div>
        </li>
      </ul>
    </section>

    <section class="dashboard">
      <h2>{{ t('stats.title') }}</h2>
      <p v-if="!statsEnabled" class="err">{{ t('stats.disabled') }}</p>
      <template v-else-if="stats">
        <div class="stat-total">
          {{ t('stats.total') }} <strong>{{ stats.total }}</strong>
          <span class="hint">{{ t('stats.last24h') }}</span>
        </div>

        <h3 class="stat-h">{{ t('stats.hourly') }}</h3>
        <div class="bars">
          <div v-for="b in stats.hourly" :key="b.label" class="bar-row">
            <span class="bar-label">{{ b.label }}</span>
            <div class="bar-track">
              <div class="bar-fill" :style="{ width: barWidth(b, stats.total) }" :title="`${b.label}: ${b.count}`"></div>
            </div>
            <span class="bar-count">{{ b.count }}</span>
          </div>
        </div>

        <h3 class="stat-h">{{ t('stats.bySource') }}</h3>
        <div class="bars">
          <div v-for="s in stats.by_source" :key="s.source" class="bar-row">
            <span class="bar-label">{{ s.source }}</span>
            <div class="bar-track">
              <div class="bar-fill src" :style="{ width: barWidth({ label: s.source, count: s.count }, stats.total) }" :title="`${s.source}: ${s.count}`"></div>
            </div>
            <span class="bar-count">{{ s.count }}</span>
          </div>
        </div>

        <h3 class="stat-h">{{ t('stats.topPaths') }}</h3>
        <ul>
          <li v-for="p in stats.top_paths" :key="p.path">
            <div class="w-info">
              <code>{{ p.path }}</code>
              <em>{{ t('stats.count', p.count) }}</em>
            </div>
          </li>
        </ul>
      </template>
    </section>

    <section class="dashboard">
      <h2>{{ t('tree.title') }}</h2>
      <p v-if="!treeEnabled" class="err">{{ t('tree.disabled') }}</p>
      <template v-else-if="tree">
        <p class="hint">{{ t('tree.hint') }}</p>
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
      <h2>{{ t('diffs.title') }}</h2>
      <p class="hint">{{ t('diffs.hint') }}</p>
      <p v-if="!diffsEnabled" class="err">{{ t('diffs.disabled') }}</p>
      <p v-else-if="diffs.length === 0" class="hint">{{ t('diffs.none') }}</p>
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
        <h2>{{ t('search.title') }}</h2>
      <div class="row">
        <input v-model="keyword" type="text" :placeholder="t('search.keywordPlaceholder')" @keyup.enter="doSearch" />
        <select v-model="src">
          <option value="">{{ t('search.allSources') }}</option>
          <option v-for="w in watchers" :key="w.name" :value="w.name">{{ w.name }}</option>
        </select>
      </div>
      <div class="row">
        <label>{{ t('search.from') }} <input v-model="since" type="datetime-local" /></label>
        <label>{{ t('search.to') }} <input v-model="until" type="datetime-local" /></label>
        <label>{{ t('search.limit') }} <input v-model.number="limit" type="number" min="1" max="5000" /></label>
      </div>
      <div class="row">
        <button class="on" :disabled="searching" @click="doSearch">{{ t('search.search') }}</button>
        <button class="ex" :disabled="!searchEnabled" @click="doExport">{{ t('search.export') }}</button>
        <select v-model="exportFormat" class="fmt">
          <option value="json">JSON</option>
          <option value="csv">CSV</option>
        </select>
        <span v-if="!searchEnabled" class="err">{{ t('search.disabled') }}</span>
        <span v-else class="count">{{ t('search.count', searchResults.length) }}</span>
      </div>
      <pre id="search-view">
        <span v-if="searchResults.length === 0 && !searching">{{ t('search.hint') }}</span>
        <span v-for="(l, i) in searchResults" :key="'s' + i">
[{{ formatTs(l.ts) }}] [{{ l.source }}] {{ l.message }}
</span>
      </pre>
      </section>
      <section class="logs">
        <h2>{{ t('logs.title') }}</h2>
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
button.lang { background: #1e293b; color: #e2e8f0; border: 1px solid #334155; font-size: 0.75rem; padding: 0.3rem 0.7rem; }
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
.bar-fill.mem { background: #fbbf24; }
.perf-warn { background: rgba(248, 113, 113, 0.15); color: #fecaca; border: 1px solid #f87171; border-radius: 6px; padding: 0.4rem 0.6rem; margin-bottom: 0.5rem; font-size: 0.8rem; }
.bar-count { min-width: 2.5rem; color: #e2e8f0; }
.hint { color: #94a3b8; font-size: 0.75rem; }
.tree { display: flex; flex-direction: column; gap: 2px; font-size: 0.72rem; }
.tree-row { display: flex; align-items: center; gap: 0.4rem; border-radius: 4px; padding: 2px 6px; cursor: pointer; }
.tree-folder { background: none; border: none; color: #94a3b8; padding: 0 2px; cursor: pointer; font-size: 0.7rem; }
.tree-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #e2e8f0; }
.tree-count { color: #94a3b8; min-width: 3rem; text-align: right; }
.preset-select { min-width: 10rem; width: auto !important; }
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
