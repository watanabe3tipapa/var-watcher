<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

interface Watcher { name: string; enabled: boolean }
interface LogLine { ts: string; source: string; message: string }
interface Preset { id: string; name: string; target: string; db: string }
interface PerfSample { ev: number; heap: number }

const DOCS_URL = 'https://watanabe3tipapa.github.io/var-watcher/'

type Lang = 'ja' | 'en'
const dict = {
  ja: {
    sub: 'モックデータによる動作イメージ(実際の監視はしていません)',
    connOk: '● WebSocket 接続中',
    alerts: '⚠ {n} 件のアラート',
    switchTo: 'English',
    watchers: 'Watchers',
    presets: 'プリセット',
    presetsHint: 'プリセットは target と有効エンジンをまとめて切り替えます。',
    apply: '適用',
    target: 'target:',
    perf: 'パフォーマンス',
    eventsPerSec: 'イベント/s:',
    heap: 'ヒープ:',
    goroutines: 'goroutines:',
    search: 'ログ検索 (永続化)',
    keyword: 'キーワード (path / message)',
    searchBtn: '検索',
    exportBtn: 'エクスポート',
    count: '{n} 件',
    diffs: '差分 (Diffs)',
    diffAdded: '追加',
    diffRemoved: '削除',
    logs: 'ログ (Logs)',
    presetApplied: 'プリセット適用: {id} → {target}',
  },
  en: {
    sub: 'Mock UI for demonstration purposes (not actually watching)',
    connOk: '● WebSocket connected',
    alerts: '⚠ {n} alerts',
    switchTo: '日本語',
    watchers: 'Watchers',
    presets: 'Presets',
    presetsHint: 'Presets switch the target and enabled engines together.',
    apply: 'Apply',
    target: 'target:',
    perf: 'Performance',
    eventsPerSec: 'events/s:',
    heap: 'heap:',
    goroutines: 'goroutines:',
    search: 'Log Search (persisted)',
    keyword: 'keyword (path / message)',
    searchBtn: 'Search',
    exportBtn: 'Export',
    count: '{n}',
    diffs: 'Diffs',
    diffAdded: 'added',
    diffRemoved: 'removed',
    logs: 'Logs',
    presetApplied: 'preset applied: {id} → {target}',
  },
}

const lang = ref<Lang>((localStorage.getItem('varwatch.demo.lang') as Lang) || 'ja')
const t = computed(() => dict[lang.value])
function setLang(l: Lang) {
  lang.value = l
  localStorage.setItem('varwatch.demo.lang', l)
}

const watchers = ref<Watcher[]>([
  { name: 'fswatch', enabled: true },
  { name: 'watchman', enabled: false },
  { name: 'entr', enabled: true },
  { name: 'logstream', enabled: false },
])

const presets = ref<Preset[]>([
  { id: 'logs', name: 'Log files', target: '/var/log', db: 'fswatch + logstream' },
  { id: 'caches', name: 'Caches', target: '/var/folders', db: 'fswatch + watchman' },
  { id: 'temp', name: 'Temp files', target: '/tmp', db: 'fswatch' },
])
const toast = ref('')
const activePreset = ref('')

function applyPreset(p: Preset) {
  activePreset.value = p.id
  const key = 'presetApplied'
  toast.value = t.value[key].replace('{id}', p.id).replace('{target}', p.target)
  window.setTimeout(() => (toast.value = ''), 3000)
}

const samples = ref<PerfSample[]>([])
function tickPerf() {
  const last = samples.value[samples.value.length - 1] || { ev: 14, heap: 38 }
  samples.value.push({
    ev: Math.max(2, last.ev + Math.round((Math.random() - 0.45) * 8)),
    heap: Math.max(12, Math.min(88, last.heap + Math.round((Math.random() - 0.5) * 6))),
  })
  if (samples.value.length > 20) samples.value.shift()
}
const perf = computed(() => {
  const cur = samples.value[samples.value.length - 1] || { ev: 0, heap: 0 }
  const max = Math.max(10, ...samples.value.map((s) => s.ev))
  return { cur, max }
})

const logs = ref<LogLine[]>([])
const maxLogs = 120
const keyword = ref('')
let timer: number | undefined
let perfTimer: number | undefined

const filteredLogs = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return logs.value
  return logs.value.filter((l) => l.message.toLowerCase().includes(k) || l.source.includes(k))
})

const diffItems = ref([
  { path: '/var/tmp/tmp1234/app.conf', dir: 'A', lines: ['+ hello var-watcher', '+ line two'] },
  { path: '/var/db/periodic/x.plist', dir: 'D', lines: ['- LaunchInterval = 1800'] },
])

async function poll() {
  try {
    const res = await fetch('/.netlify/functions/log')
    if (!res.ok) return
    const data = await res.json()
    for (const l of data.logs as LogLine[]) {
      logs.value.unshift(l)
      if (logs.value.length > maxLogs) logs.value.pop()
    }
  } catch {
    /* demo 中は無視 */
  }
}

function exportCsv() {
  const rows = [['timestamp', 'source', 'message']]
  for (const { ts, source, message } of filteredLogs.value) rows.push([ts, source, message])
  const csv = rows.map((r) => r.map((c) => `"${String(c).replaceAll('"', '""')}"`).join(',')).join('\n')
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv' }))
  const a = document.createElement('a')
  a.href = url
  a.download = `varwatch-export-${new Date().toISOString().replace(/[:.]/g, '-')}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  for (let i = 0; i < 16; i++) tickPerf()
  samples.value = samples.value.slice(-8)
  poll()
  timer = window.setInterval(poll, 2000)
  perfTimer = window.setInterval(tickPerf, 1000)
})

onBeforeUnmount(() => {
  window.clearInterval(timer)
  window.clearInterval(perfTimer)
})
</script>

<template>
  <header>
    <h1>var-watcher <span class="ver">v0.2.1</span></h1>
    <p class="sub">{{ t.sub }}</p>
    <span class="conn">{{ t.connOk }}</span>
    <span class="alerts">{{ t.alerts.replace('{n}', '2') }}</span>
    <button class="lang" @click="setLang(lang === 'ja' ? 'en' : 'ja')">{{ t.switchTo }}</button>
  </header>

  <main>
    <aside class="col">
      <section>
        <h2>{{ t.watchers }}</h2>
        <ul>
          <li v-for="w in watchers" :key="w.name">
            <strong>{{ w.name }}</strong>
            <span class="tag" :class="w.enabled ? 'on' : 'off'">{{ w.enabled ? 'ON' : 'OFF' }}</span>
          </li>
        </ul>
      </section>

      <section>
        <h2>{{ t.presets }}</h2>
        <p class="hint">{{ t.presetsHint }}</p>
        <ul>
          <li v-for="p in presets" :key="p.id">
            <div class="preset">
              <strong>{{ p.name }}</strong>
              <span class="subtarget">{{ t.target }} <code>{{ p.target }}</code> · {{ p.db }}</span>
            </div>
            <button class="apply" :class="{ done: activePreset === p.id }" @click="applyPreset(p)">{{ t.apply }}</button>
          </li>
        </ul>
        <p v-if="toast" class="toast">{{ toast }}</p>
      </section>
    </aside>

    <div class="col">
      <section>
        <h2>{{ t.perf }}</h2>
        <div class="perf">
          <div v-for="(s, i) in samples" :key="i" class="bar" :style="{ height: (16 + (s.ev / perf.max) * 90) + '%' }" :title="s.ev + ' events/s'"></div>
        </div>
        <p class="mono">{{ t.eventsPerSec }} <b>{{ perf.cur.ev }}</b> · {{ t.heap }} {{ perf.cur.heap }} MiB · {{ t.goroutines }} 12</p>
      </section>

      <section>
        <h2>{{ t.search }}</h2>
        <div class="row">
          <input v-model="keyword" :placeholder="t.keyword" />
          <button class="apply" disabled>{{ t.searchBtn }}</button>
          <button class="apply" @click="exportCsv">{{ t.exportBtn }}</button>
        </div>
        <p class="mono">{{ t.count.replace('{n}', String(filteredLogs.length)) }}</p>
      </section>

      <section>
        <h2>{{ t.diffs }}</h2>
        <div v-for="d in diffItems" :key="d.path" class="diff">
          <p class="mono"><b>{{ d.path }}</b></p>
          <pre><span v-for="(l, i) in d.lines" :key="i" :class="l.startsWith('+') ? 'add' : 'del'">{{ l }}
</span></pre>
        </div>
      </section>
    </div>

    <section class="logs">
      <h2>{{ t.logs }} (2 秒間隔で更新)</h2>
      <pre id="log-view"><span v-for="(l, i) in filteredLogs" :key="i">[{{ l.ts }}] [{{ l.source }}] {{ l.message }}
</span></pre>
    </section>
  </main>

  <footer>
    <a :href="DOCS_URL" target="_blank" rel="noopener noreferrer">ドキュメント・教材はこちら (GitHub Pages)</a>
  </footer>
</template>

<style>
* { box-sizing: border-box; }
body { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; background: #0b1120; color: #e2e8f0; }
header { padding: 1rem 1.5rem; border-bottom: 1px solid #334155; display: flex; flex-wrap: wrap; align-items: center; gap: 0.75rem 1rem; }
header h1 { margin: 0; font-size: 1.3rem; }
.ver { color: #38bdf8; font-size: 0.75rem; }
.sub { margin: 0; color: #94a3b8; font-size: 0.85rem; flex-basis: 100%; }
.conn { color: #16a34a; font-size: 0.8rem; }
.alerts { color: #f59e0b; font-size: 0.8rem; }
.lang { margin-left: auto; background: #1e293b; color: #e2e8f0; border: 1px solid #475569; border-radius: 6px; padding: 0.3rem 0.8rem; cursor: pointer; }
main { padding: 1rem 1.5rem; display: grid; grid-template-columns: 300px 1fr; gap: 1.25rem; align-items: start; }
@media (max-width: 900px) { main { grid-template-columns: 1fr; } }
section { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 1rem; }
section h2 { margin: 0 0 0.75rem; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.05em; color: #94a3b8; }
.col { display: flex; flex-direction: column; gap: 1.25rem; }
ul { list-style: none; margin: 0; padding: 0; }
li { display: flex; align-items: center; justify-content: space-between; gap: 0.5rem; padding: 0.55rem 0; border-bottom: 1px dashed #1e293b; }
li:last-child { border-bottom: none; }
.tag { font-size: 0.7rem; padding: 0.15rem 0.5rem; border-radius: 4px; }
.tag.on { background: #16a34a; color: #fff; }
.tag.off { background: #334155; color: #94a3b8; }
.hint { margin: 0 0 0.4rem; color: #64748b; font-size: 0.75rem; }
.preset { display: flex; flex-direction: column; }
.subtarget { color: #64748b; font-size: 0.75rem; }
code { background: #1e293b; padding: 1px 4px; border-radius: 4px; }
.apply { background: #2563eb; color: #fff; border: 0; border-radius: 6px; padding: 0.3rem 0.7rem; cursor: pointer; font-size: 0.75rem; }
.apply.done { background: #16a34a; }
.apply:disabled { background: #334155; color: #94a3b8; cursor: not-allowed; }
.toast { margin: 0.4rem 0 0; color: #4ade80; font-size: 0.75rem; }
.perf { display: flex; align-items: flex-end; gap: 2px; height: 72px; padding-top: 4px; }
.bar { flex: 1; background: #38bdf8; border-radius: 2px 2px 0 0; }
.mono { margin: 0.4rem 0 0; font-size: 0.75rem; color: #94a3b8; }
.row { display: flex; gap: 0.5rem; }
.row input { flex: 1; background: #1e293b; color: #e2e8f0; border: 1px solid #475569; border-radius: 6px; padding: 0.3rem 0.6rem; }
.diff { margin-bottom: 0.75rem; }
.diff p { margin: 0.2rem 0; }
.diff pre { margin: 0; background: #0b1120; border-radius: 6px; padding: 0.4rem 0.6rem; font-size: 0.75rem; overflow-x: auto; }
.add { color: #4ade80; }
.del { color: #f87171; }
.logs { grid-column: 1 / -1; }
#log-view { max-height: 44vh; overflow-y: auto; font-size: 0.8rem; line-height: 1.5; margin: 0; white-space: pre-wrap; word-break: break-all; }
footer { border-top: 1px solid #334155; padding: 1rem 1.5rem; text-align: center; }
footer a { color: #ffcc00; }
</style>