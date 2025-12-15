<template>
  <q-page padding class="catalogue-page">
    <div class="row items-center q-gutter-md q-mb-md">
      <div class="col-xs-12 col-sm-4">
        <q-select
          v-model="selectedTarget"
          :options="targetOptions"
          :label="$t('catalogue.wikiTarget')"
          emit-value
          map-options
          dense
          outlined
        />
      </div>
      <div class="col-xs-12 col-sm-4">
        <q-input
          v-model="catalogueStore.search"
          :label="$t('catalogue.searchPages')"
          dense
          outlined
          clearable
          debounce="200"
        >
          <template #append>
            <q-icon name="search" />
          </template>
        </q-input>
      </div>
      <div class="col-xs-12 col-sm-auto">
        <q-btn
          color="primary"
          icon="sync"
          :label="$t('catalogue.refreshCatalogue')"
          :loading="catalogueStore.loading"
          @click="refresh"
        />
      </div>
      <div class="col-xs-12 col-sm-auto text-right">
        <q-badge color="positive" class="q-pa-sm">
          {{ $t('app.securityBadge') }}
        </q-badge>
      </div>
    </div>

    <q-banner v-if="catalogueStore.error" class="bg-negative text-white q-mb-md">
      <q-icon name="warning" class="q-mr-sm" />
      {{ catalogueStore.error }}
    </q-banner>

    <!-- WikiTarget Status Banner -->
    <q-banner
      v-if="targetStatus"
      :class="statusBannerClass"
      class="q-mb-md"
    >
      <template #avatar>
        <q-icon :name="statusIcon" size="md" />
      </template>
      <div class="text-weight-bold q-mb-xs">{{ $t('catalogue.wikitargetStatus') }}: {{ targetStatusReason }}</div>
      <div class="text-body2">{{ targetStatusMessage }}</div>
      <div v-if="targetStatusLastSync" class="text-caption q-mt-xs">
        {{ $t('catalogue.lastSync') }}: {{ formatDate(targetStatusLastSync) }}
      </div>
      <div v-if="targetStatusCatalogRevision" class="text-caption">
        {{ $t('catalogue.catalogRevision') }}: {{ targetStatusCatalogRevision }}
      </div>
    </q-banner>

    <q-table
      v-model:selected="selectedRowKeys"
      :rows="catalogueStore.filteredPages"
      :columns="columns"
      :no-data-label="$t('catalogue.noData')"
      row-key="id"
      selection="multiple"
      flat
      bordered
      :loading="catalogueStore.loading"
      :rows-per-page-options="[10, 25, 50]"
      :row-class="(row) => row.isTemplate ? 'template-row' : ''"
      @update:selected="(val) => { console.log('[CataloguePage] Table selection updated:', val); selectedRowKeys = val }"
    >
      <template #top-right>
        <!-- Removed Queue Translation button - now using per-row controls -->
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <div class="row q-gutter-xs">
            <q-btn
              size="sm"
              color="primary"
              icon="search"
              label="Analyze"
              dense
              @click="analyzePage(props.row)"
            />
            <q-btn
              size="sm"
              color="secondary"
              icon="translate"
              label="Translate"
              dense
              @click="showTranslateDialog(props.row)"
            />
          </div>
        </q-td>
      </template>
      <template #body-cell-updatedAt="props">
        <q-td :props="props">
          {{ formatDate(props.row.updatedAt) }}
        </q-td>
      </template>
      <template #body-cell-status="props">
        <q-td :props="props">
          <q-chip
            :color="statusColor(props.row.status)"
            text-color="white"
            size="sm"
            square
          >
            {{ props.row.status }}
          </q-chip>
        </q-td>
      </template>
      <template #body-cell-title="props">
        <q-td :props="props">
          <div class="row items-center q-gutter-xs">
            <span :class="{ 'text-grey-6': props.row.isTemplate }">{{ props.row.title || '—' }}</span>
            <q-icon v-if="props.row.isTemplate" name="description" color="grey-6" size="sm" />
          </div>
        </q-td>
      </template>
    </q-table>

    <!-- Translate Confirmation Dialog -->
    <q-dialog v-model="showTranslateDialogRef">
      <q-card style="min-width: 350px">
        <q-card-section>
          <div class="text-h6">Generate Translation?</div>
        </q-card-section>

        <q-card-section v-if="translatePageRef">
          <div class="text-body2">
            <strong>Page:</strong> {{ translatePageRef.title }}<br>
            <strong>ID:</strong> {{ translatePageRef.id }}<br>
            <strong>Language:</strong> {{ translatePageRef.language || 'EN' }} → 
            {{ typeof settingsStore.defaultLanguage === 'string' 
              ? settingsStore.defaultLanguage 
              : settingsStore.defaultLanguage?.value ?? 'fr-CA' }}
          </div>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat label="Cancel" color="primary" @click="showTranslateDialogRef = false" />
          <q-btn flat label="Yes" color="primary" @click="confirmTranslate" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { computed, onMounted, ref, watch, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useCatalogueStore } from 'src/stores/catalogue-store'
import { useJobStore } from 'src/stores/job-store'
import { useSettingsStore } from 'src/stores/settings-store'
import api from 'src/services/api'

const { t } = useI18n()
const $q = useQuasar()
const catalogueStore = useCatalogueStore()
const jobStore = useJobStore()
const settingsStore = useSettingsStore()
const consoleRef = inject('console', null)

// Log to console component if available
function logToConsole(level, message, data = null) {
  if (consoleRef && consoleRef.value && typeof consoleRef.value.addLog === 'function') {
    try {
      consoleRef.value.addLog(level, message, data)
    } catch (err) {
      console.error('Failed to log to console:', err)
    }
  }
  // Also log to browser console
  if (level === 'ERROR') {
    console.error(`[${level}]`, message, data || '')
  } else if (level === 'WARN') {
    console.warn(`[${level}]`, message, data || '')
  } else {
    console.log(`[${level}]`, message, data || '')
  }
}

onMounted(async () => {
  try {
    console.log('[CataloguePage] Initializing...')
    await catalogueStore.initialise()
    console.log('[CataloguePage] Initialized', {
      targets: catalogueStore.targets.length,
      pages: catalogueStore.pages.length,
      selectedTarget: catalogueStore.selectedTargetId,
    })
  } catch (err) {
    console.error('[CataloguePage] Initialization error:', err)
    if ($q && typeof $q.notify === 'function') {
      $q.notify({ type: 'negative', message: err.message || 'Failed to load catalogue' })
    } else {
      console.error('Quasar notify not available:', $q)
    }
  }
})

const targetOptions = computed(() =>
  catalogueStore.targets.map((target) => ({
    label: target.name || target.id,
    value: target.id,
    caption: [target.mode, target.uri].filter(Boolean).join(' • '),
  })),
)

const selectedTarget = computed({
  get: () => catalogueStore.selectedTargetId,
  set: (value) =>
    catalogueStore.setTarget(value).catch((err) => {
      $q.notify({ type: 'negative', message: err.message || 'Failed to load target' })
    }),
})

const activeTarget = computed(() =>
  catalogueStore.targets.find((target) => target.id === catalogueStore.selectedTargetId),
)

// Use a ref for selectedRowKeys to ensure reactivity with q-table
const selectedRowKeys = ref([])

// Sync with store when store changes (for external updates)
watch(() => catalogueStore.selectedPages, (newSet) => {
  if (newSet && typeof newSet.size !== 'undefined') {
    const newArray = Array.from(newSet)
    if (JSON.stringify(newArray.sort()) !== JSON.stringify(selectedRowKeys.value.sort())) {
      selectedRowKeys.value = newArray
    }
  } else {
    selectedRowKeys.value = []
  }
}, { deep: true, immediate: true })

// Sync to store when table selection changes
watch(selectedRowKeys, (newValue) => {
  catalogueStore.setSelection(newValue ?? [])
}, { deep: true })

const columns = computed(() => [
  { name: 'title', label: t('catalogue.pageTitle'), field: 'title', align: 'left', sortable: true },
  { name: 'slug', label: t('catalogue.slug'), field: 'slug', align: 'left' },
  { name: 'collection', label: t('catalogue.collection'), field: 'collection', align: 'left', sortable: true },
  { name: 'template', label: t('catalogue.template'), field: 'template', align: 'left', sortable: true },
  { name: 'language', label: t('catalogue.language'), field: 'language', align: 'center', sortable: true },
  { name: 'updatedAt', label: t('catalogue.lastUpdated'), field: 'updatedAt', align: 'left', sortable: true },
  { name: 'status', label: t('catalogue.status'), field: 'status', align: 'left', sortable: true },
  { name: 'actions', label: 'Actions', field: 'actions', align: 'right', sortable: false },
])

const targetStatus = computed(() => catalogueStore.selectedWikiTargetStatus)

const targetStatusCondition = computed(() => {
  const status = targetStatus.value
  if (!status?.conditions || status.conditions.length === 0) return null
  return status.conditions.find((c) => c.type === 'Ready') || status.conditions[0]
})

const targetStatusReason = computed(() => {
  const cond = targetStatusCondition.value
  return cond?.reason || 'Unknown'
})

const targetStatusMessage = computed(() => {
  const cond = targetStatusCondition.value
  return cond?.message || 'Status not available'
})

const targetStatusLastSync = computed(() => {
  return targetStatus.value?.lastSyncTime || null
})

const targetStatusCatalogRevision = computed(() => {
  return targetStatus.value?.catalogRevision || null
})

const statusBannerClass = computed(() => {
  const cond = targetStatusCondition.value
  if (!cond) return 'bg-grey-3 text-dark'
  if (cond.status === 'True') return 'bg-green-1 text-green-8'
  if (cond.reason === 'DiscoveryFailed') return 'bg-negative text-white'
  return 'bg-warning text-dark'
})

const statusIcon = computed(() => {
  const cond = targetStatusCondition.value
  if (!cond) return 'help'
  if (cond.status === 'True') return 'check_circle'
  if (cond.reason === 'DiscoveryFailed') return 'error'
  return 'schedule'
})

function formatDate(value) {
  if (!value) return '—'
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}

async function refresh() {
  try {
    await catalogueStore.fetchWikiTargets()
    
    // Trigger a refresh on the operator side for the selected target
    const target = activeTarget.value
    if (target) {
      const namespace = target.namespace || 'glooscap-system'
      const targetName = target.resourceName || target.name || target.id
      try {
        await api.post(`/wikitargets/${namespace}/${targetName}/refresh`)
        logToConsole('INFO', `Triggered refresh for WikiTarget: ${namespace}/${targetName}`)
      } catch (err) {
        logToConsole('WARN', `Failed to trigger WikiTarget refresh: ${err.message}`)
        // Continue anyway - we'll still refresh the catalogue
      }
    }
    
    // Wait a bit for the operator to process the refresh
    await new Promise(resolve => setTimeout(resolve, 1000))
    
    // Refresh the catalogue from the operator
    await catalogueStore.refreshCatalogue()
    
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'positive',
        message: 'Catalogue refresh triggered',
        timeout: 3000,
      })
    }
  } catch (err) {
    logToConsole('ERROR', `Failed to refresh catalogue: ${err.message}`)
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'negative',
        message: `Failed to refresh catalogue: ${err.message || 'Unknown error'}`,
        timeout: 5000,
      })
    } else {
      console.error('Quasar notify not available:', $q)
    }
  }
}

function statusColor(status) {
  switch (status) {
    case 'Translated':
      return 'positive'
    case 'InProgress':
      return 'warning'
    case 'Failed':
      return 'negative'
    default:
      return 'primary'
  }
}

// Removed queueSelection() - replaced with per-row Translate button
// Removed clearSelection() - not used anymore

// Analyze page - fetch content and show in console for testing parser/stripper
async function analyzePage(page) {
  logToConsole('INFO', `Analyzing page: ${page.title} (ID: ${page.id})`)
  
  const target = activeTarget.value
  if (!target) {
    logToConsole('ERROR', 'No wiki target selected')
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'negative',
        message: 'No wiki target selected',
      })
    }
    return
  }

  const namespace = target?.namespace || 'glooscap-system'
  const targetRef = target?.resourceName || target?.id || ''

  logToConsole('DEBUG', 'Fetching page content', {
    pageId: page.id,
    pageTitle: page.title,
    targetRef,
    namespace,
  })

  try {
    const response = await api.get(`/pages/${targetRef}/${page.id}/content`, {
      params: { namespace },
    })

    const content = response.data
    logToConsole('INFO', `Page content fetched successfully`, {
      pageId: content.pageId,
      title: content.title,
      slug: content.slug,
      rawLength: content.rawLength,
      hasMarkdown: !!content.markdown,
      metadata: content.metadata,
    })

    // Log the markdown content for analysis
    logToConsole('DEBUG', 'Page markdown content', {
      markdown: content.markdown,
      length: content.markdown?.length || 0,
      preview: content.markdown?.substring(0, 200) || '',
    })

    // TODO: Add markdown parser/stripper analysis here
    // For now, just show the raw content
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'positive',
        message: `Page analyzed: ${content.title} (${content.rawLength} chars)`,
        timeout: 3000,
      })
    }
  } catch (err) {
    logToConsole('ERROR', `Failed to analyze page: ${page.title}`, {
      error: err.message,
      pageId: page.id,
    })
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'negative',
        message: `Failed to analyze page: ${err.message || 'Unknown error'}`,
        timeout: 5000,
      })
    }
  }
}

// Show translate dialog and handle translation
const showTranslateDialogRef = ref(false)
const translatePageRef = ref(null)

function showTranslateDialog(page) {
  // Check if page is a template
  if (page.isTemplate) {
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'warning',
        message: 'Templates cannot be translated',
        timeout: 3000,
      })
    }
    return
  }

  translatePageRef.value = page
  showTranslateDialogRef.value = true
}

async function confirmTranslate() {
  const page = translatePageRef.value
  if (!page) {
    return
  }

  showTranslateDialogRef.value = false

  logToConsole('INFO', `Starting translation for page: ${page.title} (ID: ${page.id})`)

  const target = activeTarget.value
  if (!target) {
    logToConsole('ERROR', 'No wiki target selected')
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'negative',
        message: 'No wiki target selected',
      })
    }
    return
  }

  const namespace = target?.namespace || 'glooscap-system'
  const targetRef = target?.resourceName || target?.id || ''
  const languageTag =
    typeof settingsStore.defaultLanguage === 'string'
      ? settingsStore.defaultLanguage
      : settingsStore.defaultLanguage?.value ?? 'fr-CA'

  logToConsole('DEBUG', 'Creating TranslationJob', {
    pageId: page.id,
    pageTitle: page.title,
    targetRef,
    namespace,
    languageTag,
  })

  try {
    const result = await jobStore.submitJob({
      namespace,
      targetRef,
      pageId: page.id,
      pipeline: 'TektonJob',
      languageTag,
      pageTitle: page.title,
    })

    const jobName = result?.name || result?.data?.name || 'unknown'
    logToConsole('INFO', `TranslationJob created successfully`, {
      jobName,
      pageId: page.id,
      pageTitle: page.title,
    })

    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'positive',
        message: `Translation Scheduled: ${jobName}`,
        timeout: 5000,
        actions: [{ icon: 'close', color: 'white' }],
      })
    }
  } catch (err) {
    logToConsole('ERROR', `Failed to create TranslationJob`, {
      error: err.message,
      pageId: page.id,
      pageTitle: page.title,
    })
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'negative',
        message: `Failed to schedule translation: ${err.message || 'Unknown error'}`,
        timeout: 5000,
      })
    }
  }

  translatePageRef.value = null
}
</script>

<style scoped>
.catalogue-page {
  background: #f4f7fb;
}

.template-row {
  opacity: 0.6;
  background-color: #f5f5f5;
}

.template-row:hover {
  opacity: 0.8;
}
</style>

