<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { GetStatus, OpenResultsFolder, RunBenchmark, RunPreflight, StartInfrastructure, StopGoServices } from '../wailsjs/go/main/App'

type LogLine = { stream: string; line: string; at: string }

const status = ref<any>({ repoRoot: 'detecting...', platform: '', scriptsReady: false, isRunning: false, resultsSummary: '' })
const logs = ref<LogLine[]>([])
const busy = ref(false)
const lastResult = ref('Ready')

const form = reactive({
  backend: 'postgres',
  mode: 'mixed',
  eps: 100,
  batch: 20,
  durationSec: 60,
  queryIntervalSec: 1,
  queryWarmupSec: 5,
  queryConcurrency: 2,
  workerReadCount: 150,
  writeMode: 'batch',
  workloadPath: 'scenarios/query-default.json',
  resetStorage: true,
  buildSummary: true
})

const backends = ['postgres', 'clickhouse', 'elasticsearch', 'cassandra']
const modes = ['ingest-only', 'query-only', 'mixed', 'longrun-ingest', 'longrun-mixed']
const isQueryMode = computed(() => ['query-only', 'mixed', 'longrun-mixed'].includes(form.mode))
const isIngestMode = computed(() => ['ingest-only', 'mixed', 'longrun-ingest', 'longrun-mixed'].includes(form.mode))

function pushLog(stream: string, line: string) {
  logs.value.push({ stream, line, at: new Date().toLocaleTimeString() })
  if (logs.value.length > 600) logs.value.splice(0, logs.value.length - 600)
}

async function refreshStatus() {
  status.value = await GetStatus()
  busy.value = !!status.value.isRunning
}

async function runAction(label: string, action: () => Promise<any>) {
  try {
    busy.value = true
    lastResult.value = `${label} running...`
    pushLog('system', `▶ ${label}`)
    const result = await action()
    lastResult.value = `${label} finished: exit ${result?.exitCode ?? 0}`
    await refreshStatus()
  } catch (error: any) {
    lastResult.value = `${label} failed`
    pushLog('stderr', error?.message ?? String(error))
  } finally {
    busy.value = false
  }
}

function startInfra() { return runAction('Start infrastructure', StartInfrastructure) }
function stopServices() { return runAction('Stop Go services', StopGoServices) }
function preflight() { return runAction('Preflight', RunPreflight) }
function runBenchmark() { return runAction(`Benchmark ${form.backend}/${form.mode}`, () => RunBenchmark({ ...form })) }
function clearLogs() { logs.value = [] }

onMounted(async () => {
  EventsOn('command-log', (line: LogLine) => pushLog(line.stream, line.line))
  EventsOn('command-finished', (result: any) => pushLog('system', `✓ ${result.name} finished in ${result.duration}, exit=${result.exitCode}`))
  await refreshStatus()
})
</script>

<template>
  <main class="shell">
    <section class="hero glass">
      <div>
        <p class="eyebrow">SIEM-like benchmark stand</p>
        <h1>Control Center</h1>
        <p class="subtitle">Премиальная панель управления инфраструктурой, прогонами и результатами PostgreSQL · ClickHouse · Elasticsearch · Cassandra.</p>
      </div>
      <div class="status-pill" :class="busy ? 'busy' : 'ready'">
        <span></span>{{ busy ? 'RUNNING' : 'READY' }}
      </div>
    </section>

    <section class="grid stats">
      <article class="card"><span>Repo</span><strong>{{ status.repoRoot }}</strong></article>
      <article class="card"><span>Platform</span><strong>{{ status.platform }}</strong></article>
      <article class="card"><span>Scripts</span><strong>{{ status.scriptsReady ? 'Ready' : 'Missing' }}</strong></article>
      <article class="card"><span>Results</span><strong>{{ status.resultsSummary }}</strong></article>
    </section>

    <section class="layout">
      <aside class="panel glass">
        <h2>Quick actions</h2>
        <button class="premium" :disabled="busy" @click="startInfra">Start infrastructure</button>
        <button :disabled="busy" @click="stopServices">Stop Go services</button>
        <button :disabled="busy" @click="preflight">Run preflight</button>
        <button :disabled="busy" @click="OpenResultsFolder">Open results</button>
        <button class="ghost" @click="clearLogs">Clear logs</button>
        <div class="hint">{{ lastResult }}</div>
      </aside>

      <section class="panel glass form-panel">
        <div class="section-head">
          <div><p class="eyebrow">Benchmark launcher</p><h2>Новый прогон</h2></div>
          <button class="premium run" :disabled="busy" @click="runBenchmark">Launch run</button>
        </div>

        <div class="form-grid">
          <label>СУБД<select v-model="form.backend"><option v-for="b in backends" :key="b" :value="b">{{ b }}</option></select></label>
          <label>Режим<select v-model="form.mode"><option v-for="m in modes" :key="m" :value="m">{{ m }}</option></select></label>
          <label v-if="isIngestMode">EPS<input v-model.number="form.eps" type="number" min="1" /></label>
          <label v-if="isIngestMode">Batch<input v-model.number="form.batch" type="number" min="1" /></label>
          <label>Duration, sec<input v-model.number="form.durationSec" type="number" min="1" /></label>
          <label>Worker read count<input v-model.number="form.workerReadCount" type="number" min="1" /></label>
          <label>Write mode<select v-model="form.writeMode"><option>batch</option><option>row</option></select></label>
          <label v-if="isQueryMode">Query interval<input v-model.number="form.queryIntervalSec" type="number" min="1" /></label>
          <label v-if="isQueryMode">Warmup<input v-model.number="form.queryWarmupSec" type="number" min="0" /></label>
          <label v-if="isQueryMode">Concurrency<input v-model.number="form.queryConcurrency" type="number" min="1" /></label>
          <label v-if="isQueryMode" class="wide">Workload<input v-model="form.workloadPath" /></label>
        </div>

        <div class="switches">
          <label><input v-model="form.resetStorage" type="checkbox" /> Reset storage</label>
          <label><input v-model="form.buildSummary" type="checkbox" /> Build summary</label>
        </div>
      </section>
    </section>

    <section class="console glass">
      <div class="section-head"><h2>Live logs</h2><button class="ghost" @click="refreshStatus">Refresh status</button></div>
      <div class="terminal">
        <div v-for="(log, index) in logs" :key="index" :class="['log', log.stream]">
          <span>{{ log.at }}</span><b>{{ log.stream }}</b><code>{{ log.line }}</code>
        </div>
      </div>
    </section>
  </main>
</template>
