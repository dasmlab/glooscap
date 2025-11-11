<template>
  <q-page padding class="catalogue-page">
    <div class="row items-center q-gutter-md q-mb-md">
      <div class="col-xs-12 col-sm-4">
        <q-select
          v-model="selectedTarget"
          :options="targetOptions"
          label="Wiki Target"
          emit-value
          map-options
          dense
          outlined
        />
      </div>
      <div class="col-xs-12 col-sm-4">
        <q-input
          v-model="catalogueStore.search"
          label="Search Pages"
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
          label="Refresh Catalogue"
          :loading="catalogueStore.loading"
          @click="refresh"
        />
      </div>
      <div class="col-xs-12 col-sm-auto text-right">
        <q-badge color="positive" class="q-pa-sm">
          {{ securityBadge }}
        </q-badge>
      </div>
    </div>

    <q-banner v-if="catalogueStore.error" class="bg-negative text-white q-mb-md">
      <q-icon name="warning" class="q-mr-sm" />
      {{ catalogueStore.error }}
    </q-banner>

    <q-table
      v-model:selected="selectedRowKeys"
      :rows="catalogueStore.filteredPages"
      :columns="columns"
      row-key="id"
      selection="multiple"
      flat
      bordered
      :loading="catalogueStore.loading"
      :rows-per-page-options="[10, 25, 50]"
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
              Queue Translation
              <q-badge color="grey-9" text-color="white" class="q-ml-xs">
                {{ selectedRowKeys.length }}
              </q-badge>
            </div>
          </q-btn>
          <q-btn
            color="white"
            text-color="primary"
            outline
            label="Clear"
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
    </q-table>
  </q-page>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useQuasar } from 'quasar'
import { useCatalogueStore } from 'src/stores/catalogue-store'
import { useJobStore } from 'src/stores/job-store'
import { useSettingsStore } from 'src/stores/settings-store'

const $q = useQuasar()
const catalogueStore = useCatalogueStore()
const jobStore = useJobStore()
const settingsStore = useSettingsStore()

onMounted(() => {
  catalogueStore.initialise().catch((err) => {
    $q.notify({ type: 'negative', message: err.message || 'Failed to load catalogue' })
  })
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
  get: () => Array.from(catalogueStore.selectedPages),
  set: (value) => catalogueStore.setSelection(value ?? []),
})

const columns = [
  { name: 'title', label: 'Page Title', field: 'title', align: 'left', sortable: true },
  { name: 'slug', label: 'Slug', field: 'slug', align: 'left' },
  { name: 'language', label: 'Language', field: 'language', align: 'center', sortable: true },
  { name: 'updatedAt', label: 'Last Updated', field: 'updatedAt', align: 'left', sortable: true },
  { name: 'status', label: 'Status', field: 'status', align: 'left', sortable: true },
]

const securityBadge = computed(() => settingsStore.securityBadge)

function formatDate(value) {
  if (!value) return '—'
  return new Date(value).toLocaleString()
}

async function refresh() {
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
  const target = activeTarget.value
  const namespace = target?.namespace || 'glooscap-system'
  const targetRef = target?.resourceName || target?.id || ''
  const languageTag =
    typeof settingsStore.defaultLanguage === 'string'
      ? settingsStore.defaultLanguage
      : settingsStore.defaultLanguage?.value ?? 'fr-CA'

  const requests = selectedRowKeys.value.map((pageId) => {
    const page = catalogueStore.pages.find((item) => item.id === pageId)
    if (!page) return null
    return jobStore
      .submitJob({
        namespace,
        targetRef,
        pageId: page.id,
        pipeline: 'TektonJob',
        languageTag,
        pageTitle: page.title,
      })
      .catch((err) => {
        $q.notify({ type: 'negative', message: err.message || 'Failed to queue job' })
      })
  })

  await Promise.all(requests.filter(Boolean))
  catalogueStore.clearSelection()
  $q.notify({
    type: 'positive',
    message: 'Translation jobs queued',
  })
}

function clearSelection() {
  catalogueStore.clearSelection()
}
</script>

<style scoped>
.catalogue-page {
  background: #f4f7fb;
}
</style>

