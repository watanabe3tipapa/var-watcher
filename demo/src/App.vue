<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

interface Watcher {
  name: string
  enabled: boolean
}

interface LogLine {
  ts: string
  source: string
  message: string
}

const DOCS_URL = 'https://watanabe3tipapa.github.io/var-watcher/'

const watchers = ref<Watcher[]>([
  { name: 'fswatch', enabled: true },
  { name: 'watchman', enabled: false },
  { name: 'entr', enabled: true },
  { name: 'logstream', enabled: false },
])

const logs = ref<LogLine[]>([])
const maxLogs = 200
let timer: number | undefined

async function poll() {
  try {
    const res = await fetch('/.netlify/functions/log')
    if (!res.ok) return
    const data = await res.json()
    for (const l of data.logs as LogLine[]) {
      logs.value.push(l)
      if (logs.value.length > maxLogs) logs.value.shift()
    }
  } catch {
    /* demo 中は無視 */
  }
}

onMounted(() => {
  poll()
  timer = window.setInterval(poll, 2000)
})

onBeforeUnmount(() => window.clearInterval(timer))
</script>

<template>
  <header>
    <h1>var-watcher Live Demo</h1>
    <p class="sub">モックデータによる動作イメージ(実際の監視はしていません)</p>
  </header>

  <main>
    <section class="watchers">
      <h2>Watchers</h2>
      <ul>
        <li v-for="w in watchers" :key="w.name">
          <strong>{{ w.name }}</strong>
          <span class="tag" :class="w.enabled ? 'on' : 'off'">{{ w.enabled ? 'ON' : 'OFF' }}</span>
        </li>
      </ul>
    </section>

    <section class="logs">
      <h2>Logs (2 秒間隔で更新)</h2>
      <pre id="log-view"><span v-for="(l, i) in logs" :key="i">[{{ l.ts }}] [{{ l.source }}] {{ l.message }}
</span></pre>
    </section>
  </main>

  <footer>
    <a :href="DOCS_URL" target="_blank" rel="noopener noreferrer">ドキュメント・教材はこちら (GitHub Pages)</a>
  </footer>
</template>

<style>
* { box-sizing: border-box; }
body { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
header { padding: 1.25rem 1.5rem; border-bottom: 1px solid #334155; display: flex; align-items: baseline; gap: 1rem; }
header h1 { margin: 0; font-size: 1.4rem; }
.sub { margin: 0; color: #94a3b8; }
main { padding: 1rem 1.5rem; display: grid; grid-template-columns: 320px 1fr; gap: 1.5rem; }
@media (max-width: 800px) { main { grid-template-columns: 1fr; } }
section { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 1rem; }
section h2 { margin: 0 0 0.75rem; font-size: 0.85rem; text-transform: uppercase; color: #94a3b8; }
ul { list-style: none; margin: 0; padding: 0; }
li { display: flex; align-items: center; justify-content: space-between; padding: 0.6rem 0; border-bottom: 1px dashed #1e293b; }
li:last-child { border-bottom: none; }
.tag { font-size: 0.7rem; padding: 0.15rem 0.5rem; border-radius: 4px; }
.tag.on { background: #16a34a; color: #fff; }
.tag.off { background: #334155; color: #94a3b8; }
#log-view { max-height: 60vh; overflow-y: auto; font-size: 0.8rem; line-height: 1.5; margin: 0; white-space: pre-wrap; word-break: break-all; }
footer { border-top: 1px solid #334155; padding: 1rem 1.5rem; text-align: center; }
footer a { color: #ffcc00; }
</style>
