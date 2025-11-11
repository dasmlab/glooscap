<template>
  <q-page padding class="jobs-page">
    <div class="row items-center q-mb-md">
      <div class="col">
        <div class="text-h5 text-primary">Translation Queue</div>
        <div class="text-subtitle2 text-grey-7">
          Monitor queued, running, and completed translation jobs.
        </div>
      </div>
      <div class="col-auto">
        <q-btn
          color="primary"
          icon="refresh"
          label="Refresh"
          :loading="jobStore.loading"
          @click="refresh"
        />
      </div>
    </div>

    <q-banner v-if="jobStore.error" class="bg-negative text-white q-mb-md">
      <q-icon name="warning" class="q-mr-sm" />
      {{ jobStore.error }}
    </q-banner>

    <q-card flat bordered>
      <q-card-section class="row items-center justify-between">
        <div class="text-subtitle1">Active jobs: {{ jobStore.activeCount }}</div>
        <q-btn icon="download" color="secondary" flat label="Export Audit Trail" />
      </q-card-section>

      <q-separator />

      <q-card-section>
        <q-timeline color="primary" layout="dense">
          <q-timeline-entry
            v-for="job in jobStore.recentJobs"
            :key="job.id"
            :title="job.pageTitle"
            :subtitle="formatSubtitle(job)"
            :color="statusColor(job.state)"
            :icon="statusIcon(job.state)"
          >
            <div class="text-body2">
              <q-badge :color="statusColor(job.state)" text-color="white" class="q-mr-sm">
                {{ job.state }}
              </q-badge>
              Pipeline: {{ job.pipeline }}
            </div>
            <div class="text-caption text-grey-7">
              Job ID: {{ job.id }} • Target: {{ job.targetId }}
            </div>
            <div v-if="job.message" class="text-negative q-mt-xs">
              {{ job.message }}
            </div>
          </q-timeline-entry>
        </q-timeline>
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup>
import { useQuasar } from 'quasar'
import { useJobStore } from 'src/stores/job-store'

const $q = useQuasar()
const jobStore = useJobStore()

function formatSubtitle(job) {
  const created = new Date(job.createdAt).toLocaleString()
  const updated = new Date(job.updatedAt).toLocaleString()
  return `Created ${created} • Updated ${updated}`
}

function statusColor(state) {
  switch (state) {
    case 'Completed':
      return 'positive'
    case 'Publishing':
      return 'primary'
    case 'Running':
    case 'Dispatching':
      return 'warning'
    case 'Failed':
      return 'negative'
    default:
      return 'secondary'
  }
}

function statusIcon(state) {
  switch (state) {
    case 'Completed':
      return 'check_circle'
    case 'Publishing':
      return 'cloud_upload'
    case 'Running':
      return 'play_arrow'
    case 'Dispatching':
      return 'schedule'
    case 'Failed':
      return 'error'
    default:
      return 'translate'
  }
}

async function refresh() {
  await jobStore.refreshJobs()
  $q.notify({
    type: 'info',
    message: 'Job list refreshed',
  })
}
</script>

<style scoped>
.jobs-page {
  background: #f9fafc;
}
</style>

