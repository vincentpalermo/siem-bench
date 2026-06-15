<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { GetStatus, OpenResultsFolder, RunBenchmark, RunPreflight, StartInfrastructure, StopGoServices } from '../wailsjs/go/main/App'

type LogLine = { stream: string; line: string; at: string }

const status = ref<any>({ repoRoot: 'detecting...', platform: '', scriptsReady: false, isRunning: false, resultsSummary: '' })
const logs = ref<LogLine[]>([])
const busy = ref(false)
const lastResult = ref('Готово')

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

const paramHints = {
  backend: 'Исследуемая СУБД. Возможные значения: PostgreSQL, ClickHouse, Elasticsearch, Cassandra. Определяет, какой worker, stream Redis и storage backend будут использоваться в прогоне.',
  mode: 'Сценарий эксперимента. ingest-only — только запись, query-only — только запросы, mixed — запись и запросы одновременно, longrun-* — длительные варианты этих режимов.',
  eps: 'Events per second — целевая скорость генерации событий. Принимает положительные целые значения. Чем выше EPS, тем сильнее нагрузка на collector, Redis, worker и выбранную СУБД.',
  batch: 'Размер пачки событий, отправляемой генератором в collector одним HTTP-запросом. Принимает положительные целые значения. Влияет на число HTTP-запросов и эффективность записи.',
  durationSec: 'Длительность прогона в секундах. Принимает положительные целые значения. Влияет на общий объём событий: sent_events = EPS × DurationSec.',
  workerReadCount: 'Сколько сообщений worker пытается читать из Redis Stream за один раз. Принимает положительные целые значения. Влияет на скорость выгрузки очереди и размер внутренних batch-операций.',
  writeMode: 'Режим записи worker в СУБД. batch — пакетная запись, row — построчная запись. Для ClickHouse, Elasticsearch и Cassandra используется batch.',
  queryIntervalSec: 'Интервал между итерациями query-runner в секундах. Принимает положительные целые значения. Чем меньше интервал, тем выше частота аналитических запросов.',
  queryWarmupSec: 'Время прогрева query-runner перед фиксацией результатов. Принимает 0 или положительное число. Позволяет не учитывать стартовые задержки.',
  queryConcurrency: 'Количество параллельных query-worker потоков. Принимает положительные целые значения. Чем выше значение, тем сильнее нагрузка аналитическими запросами.',
  workloadPath: 'Путь к JSON-файлу workload-сценария. Например: scenarios/query-default.json или scenarios/query-heavy.json. Определяет набор запросов для query-only и mixed.',
  resetStorage: 'Если включено, перед прогоном очищается выбранная СУБД и Redis Stream. Используется для чистого сравнения без влияния старых данных.',
  buildSummary: 'Если включено, после прогона пересобирается CSV summary по результатам. Нужно для таблиц, графиков и дальнейшего анализа.'
}

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
    lastResult.value = `${label}: выполняется...`
    pushLog('system', `▶ ${label}`)
    const result = await action()
    lastResult.value = `${label}: завершено, код ${result?.exitCode ?? 0}`
    await refreshStatus()
  } catch (error: any) {
    lastResult.value = `${label}: ошибка`
    pushLog('stderr', error?.message ?? String(error))
  } finally {
    busy.value = false
  }
}

function startInfra() { return runAction('Запуск инфраструктуры', StartInfrastructure) }
function stopServices() { return runAction('Остановить сервисы', StopGoServices) }
function preflight() { return runAction('Тест системы', RunPreflight) }
function runBenchmark() { return runAction(`Запуск ${form.backend}/${form.mode}`, () => RunBenchmark({ ...form })) }
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
        <p class="eyebrow">SIEM benchmark stand</p>
        <h1>Control Center</h1>
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
        <h2>Быстрые действия</h2>
        <button class="premium" :disabled="busy" @click="startInfra">Запуск инфраструктуры</button>
        <button :disabled="busy" @click="stopServices">Остановить сервисы</button>
        <button :disabled="busy" @click="preflight">Тест системы</button>
        <button :disabled="busy" @click="OpenResultsFolder">Открыть результаты</button>
        <button class="ghost" @click="clearLogs">Очистить логи</button>
        <div class="hint">{{ lastResult }}</div>
      </aside>

      <section class="panel glass form-panel">
        <div class="section-head">
          <div><p class="eyebrow">Benchmark launcher</p><h2>Новый прогон</h2></div>
          <button class="premium run" :disabled="busy" @click="runBenchmark">Запуск</button>
        </div>

        <div class="form-grid">
          <label><span class="field-title">СУБД<span class="tooltip" :data-tooltip="paramHints.backend">?</span></span><select v-model="form.backend"><option v-for="b in backends" :key="b" :value="b">{{ b }}</option></select></label>
          <label><span class="field-title">Режим<span class="tooltip" :data-tooltip="paramHints.mode">?</span></span><select v-model="form.mode"><option v-for="m in modes" :key="m" :value="m">{{ m }}</option></select></label>
          <label v-if="isIngestMode"><span class="field-title">EPS<span class="tooltip" :data-tooltip="paramHints.eps">?</span></span><input v-model.number="form.eps" type="number" min="1" /></label>
          <label v-if="isIngestMode"><span class="field-title">Batch<span class="tooltip" :data-tooltip="paramHints.batch">?</span></span><input v-model.number="form.batch" type="number" min="1" /></label>
          <label><span class="field-title">Duration, sec<span class="tooltip" :data-tooltip="paramHints.durationSec">?</span></span><input v-model.number="form.durationSec" type="number" min="1" /></label>
          <label><span class="field-title">Worker read count<span class="tooltip" :data-tooltip="paramHints.workerReadCount">?</span></span><input v-model.number="form.workerReadCount" type="number" min="1" /></label>
          <label><span class="field-title">Write mode<span class="tooltip" :data-tooltip="paramHints.writeMode">?</span></span><select v-model="form.writeMode"><option>batch</option><option>row</option></select></label>
          <label v-if="isQueryMode"><span class="field-title">Query interval<span class="tooltip" :data-tooltip="paramHints.queryIntervalSec">?</span></span><input v-model.number="form.queryIntervalSec" type="number" min="1" /></label>
          <label v-if="isQueryMode"><span class="field-title">Warmup<span class="tooltip" :data-tooltip="paramHints.queryWarmupSec">?</span></span><input v-model.number="form.queryWarmupSec" type="number" min="0" /></label>
          <label v-if="isQueryMode"><span class="field-title">Concurrency<span class="tooltip" :data-tooltip="paramHints.queryConcurrency">?</span></span><input v-model.number="form.queryConcurrency" type="number" min="1" /></label>
          <label v-if="isQueryMode" class="wide"><span class="field-title">Workload<span class="tooltip" :data-tooltip="paramHints.workloadPath">?</span></span><input v-model="form.workloadPath" /></label>
        </div>

        <div class="switches">
          <label><input v-model="form.resetStorage" type="checkbox" /> Reset storage <span class="tooltip" :data-tooltip="paramHints.resetStorage">?</span></label>
          <label><input v-model="form.buildSummary" type="checkbox" /> Build summary <span class="tooltip" :data-tooltip="paramHints.buildSummary">?</span></label>
        </div>
      </section>
    </section>

    <section class="console glass">
      <div class="section-head"><h2>Live logs</h2><button class="ghost" @click="refreshStatus">Обновить статус</button></div>
      <div class="terminal">
        <div v-for="(log, index) in logs" :key="index" :class="['log', log.stream]">
          <span>{{ log.at }}</span><b>{{ log.stream }}</b><code>{{ log.line }}</code>
        </div>
      </div>
    </section>
  </main>
</template>
