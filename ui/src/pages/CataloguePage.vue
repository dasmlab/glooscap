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
    >
      <template #top-right>
        <div class="row items-center q-gutter-sm">
          <q-btn
            color="secondary"
            icon="translate"
            :disable="selectedRowKeys.length === 0"
            @click="queueSelection"
          >
            <div class="q-ml-sm">
              {{ $t('catalogue.queueTranslation') }}
              <q-badge color="grey-9" text-color="white" class="q-ml-xs">
                {{ selectedRowKeys.length }}
              </q-badge>
            </div>
          </q-btn>
          <q-btn
            color="white"
            text-color="primary"
            outline
            :label="$t('catalogue.clear')"
            :disable="selectedRowKeys.length === 0"
            @click="clearSelection"
          />
        </div>
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
  </q-page>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useCatalogueStore } from 'src/stores/catalogue-store'
import { useJobStore } from 'src/stores/job-store'
import { useSettingsStore } from 'src/stores/settings-store'

const { t } = useI18n()
const $q = useQuasar()
const catalogueStore = useCatalogueStore()
const jobStore = useJobStore()
const settingsStore = useSettingsStore()

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

const selectedRowKeys = computed({
  get: () => {
    const pages = catalogueStore.selectedPages
    if (!pages || typeof pages.size === 'undefined') {
      return []
    }
    return Array.from(pages)
  },
  set: (value) => catalogueStore.setSelection(value ?? []),
})

const columns = computed(() => [
  { name: 'title', label: t('catalogue.pageTitle'), field: 'title', align: 'left', sortable: true },
  { name: 'slug', label: t('catalogue.slug'), field: 'slug', align: 'left' },
  { name: 'collection', label: t('catalogue.collection'), field: 'collection', align: 'left', sortable: true },
  { name: 'template', label: t('catalogue.template'), field: 'template', align: 'left', sortable: true },
  { name: 'language', label: t('catalogue.language'), field: 'language', align: 'center', sortable: true },
  { name: 'updatedAt', label: t('catalogue.lastUpdated'), field: 'updatedAt', align: 'left', sortable: true },
  { name: 'status', label: t('catalogue.status'), field: 'status', align: 'left', sortable: true },
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
  await catalogueStore.fetchWikiTargets()
  await catalogueStore.refreshCatalogue()
  $q.notify({
    type: 'info',
    message: 'Catalogue refresh triggered',
  })
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

async function queueSelection() {
  if (!$q || typeof $q.notify !== 'function') {
    console.error('Quasar notify not available')
    return
  }

  const target = activeTarget.value
  if (!target) {
    $q.notify({
      type: 'negative',
      message: 'No wiki target selected',
    })
    return
  }

  const namespace = target?.namespace || 'glooscap-system'
  const targetRef = target?.resourceName || target?.id || ''
  const languageTag =
    typeof settingsStore.defaultLanguage === 'string'
      ? settingsStore.defaultLanguage
      : settingsStore.defaultLanguage?.value ?? 'fr-CA'

  // Filter out templates - they shouldn't be translated
  const validPages = selectedRowKeys.value
    .map((pageId) => catalogueStore.pages.find((item) => item.id === pageId))
    .filter((page) => page && !page.isTemplate)

  if (validPages.length === 0) {
    $q.notify({
      type: 'warning',
      message: 'Templates cannot be queued for translation',
    })
    return
  }

  if (validPages.length < selectedRowKeys.value.length) {
    $q.notify({
      type: 'info',
      message: `${selectedRowKeys.value.length - validPages.length} template(s) skipped`,
    })
  }

  // For MVP: Use direct translation endpoint instead of creating TranslationJob
  try {
    for (const page of validPages) {
      try {
        // Call direct translation endpoint
        const response = await api.post('/translate', {
          targetRef,
          namespace,
          pageId: page.id,
          pageTitle: page.title,
          languageTag,
        })

        if (response.data && response.data.success) {
          $q.notify({
            type: 'positive',
            message: `Translated: ${page.title}`,
          })
        } else {
          $q.notify({
            type: 'warning',
            message: `Translation may have failed for: ${page.title}`,
          })
        }
      } catch (err) {
        console.error(`Failed to translate page ${page.id}:`, err)
        $q.notify({
          type: 'negative',
          message: `Failed to translate ${page.title}: ${err.message || 'Unknown error'}`,
        })
      }
    }

    catalogueStore.clearSelection()
  } catch (err) {
    console.error('Translation error:', err)
    $q.notify({
      type: 'negative',
      message: err.message || 'Failed to translate pages',
    })
  }
}

function clearSelection() {
  catalogueStore.clearSelection()
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

