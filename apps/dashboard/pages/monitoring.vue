<script setup lang="ts">
const api = useApi()
const data = ref<any>(null)
const circuits = ref<any[]>([])
const pools = ref<any[]>([])
const loading = ref(true)
const refreshedAt = ref<Date | null>(null)

async function load() {
  loading.value = true
  try {
    const [overview, circuitRows, poolRows] = await Promise.all([
      api.get('/api/overview'),
      api.get('/api/circuit-breakers').catch(() => []),
      api.get('/api/pool-stats').catch(() => []),
    ])
    data.value = overview
    circuits.value = circuitRows || []
    pools.value = poolRows || []
    refreshedAt.value = new Date()
  } finally { loading.value = false }
}
onMounted(load)

const pct = (n: number) => `${(n ?? 0).toFixed(1)}%`
const refreshedLabel = computed(() => refreshedAt.value
  ? refreshedAt.value.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  : '—')
const unhealthyCount = computed(() => (data.value?.connectors || []).filter((c: any) =>
  !['healthy', 'active', 'closed'].includes(String(c.health_status || '').toLowerCase())
).length)
const openCircuitCount = computed(() => circuits.value.filter((c: any) => String(c.state).toLowerCase() !== 'closed').length)
const platformState = computed(() => unhealthyCount.value || openCircuitCount.value ? 'Attention needed' : 'Operational')
const platformTone = computed(() => unhealthyCount.value || openCircuitCount.value ? 'warn' : 'good')
</script>

<template>
  <section class="monitoring-intro">
    <div>
      <div class="eyebrow"><span class="live-dot" /> Live telemetry</div>
      <h2>Platform health, without the noise.</h2>
      <p>Gateway traffic, connector resilience, and database pool pressure in one operational view.</p>
    </div>
    <div class="monitoring-actions">
      <div class="sync-state"><span class="sync-label">Last sync</span><strong>{{ refreshedLabel }}</strong></div>
      <button class="btn primary refresh-btn" :disabled="loading" @click="load"><span :class="{ spin: loading }">↻</span> Refresh</button>
    </div>
  </section>

  <div v-if="loading && !data" class="loading monitoring-loading"><span class="spin" /> Loading live telemetry…</div>
  <div v-else-if="data">
    <section class="monitoring-healthbar" :class="platformTone">
      <div class="healthbar-status"><span class="health-pulse" /><div><strong>{{ platformState }}</strong><span>Gateway and connector control plane</span></div></div>
      <div class="healthbar-stats">
        <div><strong>{{ data.connectors?.length || 0 }}</strong><span>connections</span></div>
        <div><strong>{{ openCircuitCount }}</strong><span>open circuits</span></div>
        <div><strong>{{ unhealthyCount }}</strong><span>needs review</span></div>
      </div>
    </section>

    <section class="metrics-grid" aria-label="Key platform metrics">
      <article class="metric-tile">
        <span class="metric-label">Requests today</span>
        <strong>{{ data.requests_today || 0 }}</strong>
        <span class="metric-note">Active day window</span>
      </article>
      <article class="metric-tile">
        <span class="metric-label">Average latency</span>
        <strong>{{ Math.round(data.avg_latency_ms || 0) }}<small>ms</small></strong>
        <span class="metric-note">End-to-end gateway time</span>
      </article>
      <article class="metric-tile" :class="{ alert: (data.error_rate_today || 0) > 2 }">
        <span class="metric-label">Error rate</span>
        <strong>{{ pct(data.error_rate_today) }}</strong>
        <span class="metric-note">Real traffic · gateway / upstream 5xx</span>
      </article>
      <article class="metric-tile">
        <span class="metric-label">Cache hit ratio</span>
        <strong>{{ pct(data.cache_hit_ratio) }}</strong>
        <span class="metric-note">Responses served from cache</span>
      </article>
    </section>

    <section class="monitoring-section">
      <div class="section-heading"><div><span class="eyebrow">Request performance</span><h3>Where traffic is slowing down</h3></div><span class="section-meta">Current active window</span></div>
      <div class="grid cols-2 monitoring-grid">
        <div class="card monitoring-card">
          <div class="card-head"><div><h3>Slowest endpoints</h3><p>Highest average response time</p></div><span class="badge amber">Latency</span></div>
          <div class="table-wrap"><table class="tbl">
            <thead><tr><th>Endpoint</th><th class="align-right">Avg. latency</th><th class="align-right">Requests</th></tr></thead>
            <tbody>
              <tr v-for="(e, i) in data.top_slow" :key="i"><td class="mono endpoint-cell">{{ e.label }}</td><td class="align-right mono">{{ Math.round(e.avg_ms) }} ms</td><td class="align-right">{{ e.count }}</td></tr>
              <tr v-if="!data.top_slow?.length"><td colspan="3"><div class="table-empty"><span>✓</span><div><strong>No slow endpoints</strong><p>Traffic will appear here once requests are received.</p></div></div></td></tr>
            </tbody>
          </table></div>
        </div>
        <div class="card monitoring-card">
          <div class="card-head"><div><h3>Error concentration</h3><p>Endpoints with the highest error ratio</p></div><span class="badge coral">Errors</span></div>
          <div class="table-wrap"><table class="tbl">
            <thead><tr><th>Endpoint</th><th class="align-right">Error rate</th><th class="align-right">Requests</th></tr></thead>
            <tbody>
              <tr v-for="(e, i) in data.top_errors" :key="i"><td class="mono endpoint-cell">{{ e.label }}</td><td class="align-right mono">{{ e.error_rate.toFixed(1) }}%</td><td class="align-right">{{ e.count }}</td></tr>
              <tr v-if="!data.top_errors?.length"><td colspan="3"><div class="table-empty"><span>✓</span><div><strong>No platform errors</strong><p>Gateway and upstream 5xx failures will be surfaced here.</p></div></div></td></tr>
            </tbody>
          </table></div>
        </div>
      </div>
    </section>

    <section class="monitoring-section">
      <div class="section-heading"><div><span class="eyebrow">Data connectivity</span><h3>Connector and pool resilience</h3></div><span class="section-meta">Internal service telemetry</span></div>
      <div class="grid cols-2 monitoring-grid">
        <div class="card monitoring-card">
          <div class="card-head"><div><h3>Connector health</h3><p>Connection state and circuit protection</p></div></div>
          <div class="table-wrap"><table class="tbl">
            <thead><tr><th>Connection</th><th>Engine</th><th>Health</th><th>Circuit</th></tr></thead>
            <tbody>
              <tr v-for="(c, i) in data.connectors" :key="i"><td>{{ c.name }}</td><td><span class="badge blue">{{ c.database_type }}</span></td><td><StatusBadge :status="c.health_status" /></td><td><StatusBadge :status="circuits.find((x) => x.connection_id === c.id)?.state || 'closed'" /></td></tr>
              <tr v-if="!data.connectors?.length"><td colspan="4" class="faint">No connections configured.</td></tr>
            </tbody>
          </table></div>
        </div>
        <div class="card monitoring-card">
          <div class="card-head"><div><h3>Pool capacity</h3><p>Current connector allocation and wait pressure</p></div></div>
          <div class="table-wrap"><table class="tbl">
            <thead><tr><th>Connection</th><th class="align-right">In use</th><th class="align-right">Idle</th><th class="align-right">Max</th><th class="align-right">Timeout</th></tr></thead>
            <tbody>
              <tr v-for="p in pools" :key="p.connection_id"><td>{{ p.connection }}</td><td class="align-right">{{ p.in_use }}</td><td class="align-right">{{ p.idle }}</td><td class="align-right">{{ p.max }}</td><td class="align-right">{{ p.timeout_count }}</td></tr>
              <tr v-if="!pools.length"><td colspan="5" class="faint">No active pools yet.</td></tr>
            </tbody>
          </table></div>
        </div>
      </div>
    </section>

    <details class="metrics-reference">
      <summary><div><span class="eyebrow">Reference</span><strong>Metrics catalog & telemetry contract</strong><p>Standard metric names used across overview, Grafana, and request correlation.</p></div><span class="details-caret">⌄</span></summary>
      <div class="table-wrap"><table class="tbl">
        <thead><tr><th>Metric</th><th>Meaning</th><th>Used in</th></tr></thead>
        <tbody>
          <tr><td class="mono">requests_today</td><td>Total requests in the active day window</td><td>Overview</td></tr>
          <tr><td class="mono">avg_latency_ms</td><td>Average end-to-end request latency</td><td>Overview / slow endpoints</td></tr>
          <tr><td class="mono">error_rate_today</td><td>4xx/5xx ratio in current day window</td><td>Overview / endpoint errors</td></tr>
          <tr><td class="mono">cache_hit_ratio</td><td>Share of responses served from cache</td><td>Overview</td></tr>
          <tr><td class="mono">connector_pool_*</td><td>Pool pressure, allocation, and timeout counts</td><td>Pool capacity</td></tr>
          <tr><td class="mono">circuit_state</td><td>Current circuit-breaker state per connector</td><td>Connector health</td></tr>
        </tbody>
      </table></div>
      <p class="reference-footnote">All core services expose <span class="mono">/healthz</span>, <span class="mono">/readyz</span>, and <span class="mono">/metrics</span>. Use request IDs to correlate gateway, audit, and service telemetry.</p>
    </details>
  </div>
</template>
