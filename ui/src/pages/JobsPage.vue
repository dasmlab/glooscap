<template>
  <q-page padding class="jobs-page">
    <div class="row items-center q-mb-md">
      <div class="col">
        <div class="text-h5 text-primary">{{ $t('jobs.title') }}</div>
        <div class="text-subtitle2 text-grey-7">
          {{ $t('jobs.subtitle') }}
        </div>
      </div>
      <div class="col-auto">
        <q-btn
          color="primary"
          icon="refresh"
          :label="$t('jobs.refresh')"
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
        <div class="text-subtitle1">{{ $t('jobs.activeJobs') }}: {{ jobStore.activeCount }}</div>
        <q-btn icon="download" color="secondary" flat :label="$t('jobs.exportAudit')" />
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
              {{ $t('jobs.pipeline') }}: {{ job.pipeline }}
            </div>
            <div class="text-caption text-grey-7">
              {{ $t('jobs.jobId') }}: {{ job.id }} • {{ $t('jobs.target') }}: {{ job.targetId }}
            </div>
            <div v-if="job.message" class="text-negative q-mt-xs">
              {{ job.message }}
            </div>
            <!-- Duplicate Approval Banner -->
            <div v-if="job.state === 'AwaitingApproval' && job.duplicateInfo" class="q-mt-sm">
              <q-banner class="bg-warning text-dark">
                <template #avatar>
                  <q-icon name="warning" color="dark" />
                </template>
                <div class="text-body2">
                  {{ $t('jobs.duplicateFound') }}: {{ job.duplicateInfo.pageTitle }}
                </div>
                <div class="text-caption q-mt-xs">
                  {{ job.duplicateInfo.message }}
                </div>
                <template #action>
                  <q-btn
                    flat
                    dense
                    :label="$t('jobs.approve')"
                    color="primary"
                    @click="showApprovalDialog(job)"
                  />
                </template>
              </q-banner>
            </div>
            <!-- Draft Approval Banner -->
            <div v-else-if="job.state === 'AwaitingApproval'" class="q-mt-sm">
              <q-banner class="bg-info text-white">
                <template #avatar>
                  <q-icon name="description" color="white" />
                </template>
                <div class="text-body2">
                  {{ $t('jobs.draftReady') }}
                </div>
                <div class="text-caption q-mt-xs">
                  {{ job.message || $t('jobs.draftReadyMessage') }}
                </div>
                <template #action>
                  <q-btn
                    flat
                    dense
                    :label="$t('jobs.publish')"
                    color="white"
                    text-color="primary"
                    :loading="jobStore.loading"
                    @click="handlePublishApproval(job)"
                  />
                </template>
              </q-banner>
            </div>
          </q-timeline-entry>
        </q-timeline>
      </q-card-section>
    </q-card>

    <!-- Duplicate Approval Dialog -->
    <q-dialog v-model="approvalDialog.show" persistent>
      <q-card style="min-width: 400px">
        <q-card-section class="row items-center q-pb-none">
          <q-icon name="warning" color="warning" size="md" class="q-mr-sm" />
          <div class="text-h6">{{ $t('jobs.duplicateApproval') }}</div>
        </q-card-section>

        <q-card-section>
          <div class="text-body1 q-mb-md">
            {{ $t('jobs.duplicateApprovalMessage') }}
          </div>
          <div v-if="approvalDialog.duplicateInfo" class="q-pa-md bg-grey-2 rounded-borders">
            <div class="text-weight-bold q-mb-xs">{{ $t('jobs.existingPage') }}:</div>
            <div class="text-body2">{{ approvalDialog.duplicateInfo.pageTitle }}</div>
            <div class="text-caption text-grey-7 q-mt-xs">
              {{ approvalDialog.duplicateInfo.pageUri }}
            </div>
          </div>
        </q-card-section>

        <q-card-actions align="right" class="q-pa-md">
          <q-btn
            flat
            :label="$t('common.cancel')"
            color="grey-7"
            @click="approvalDialog.show = false"
          />
          <q-btn
            :label="$t('jobs.approveOverwrite')"
            color="warning"
            @click="handleApproval"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useJobStore } from 'src/stores/job-store'

const { t } = useI18n()
const $q = useQuasar()
const jobStore = useJobStore()

const approvalDialog = ref({
  show: false,
  jobId: null,
  namespace: null,
  duplicateInfo: null,
})

onMounted(() => {
  jobStore.refreshJobs().catch((err) => {
    $q.notify({ type: 'negative', message: err.message || 'Failed to load jobs' })
  })
})

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
    case 'AwaitingApproval':
      return 'orange'
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
    case 'AwaitingApproval':
      return 'pause_circle'
    case 'Failed':
      return 'error'
    default:
      return 'translate'
  }
}

function showApprovalDialog(job) {
  approvalDialog.value = {
    show: true,
    jobId: job.id,
    namespace: job.namespace || 'glooscap-system',
    duplicateInfo: job.duplicateInfo,
  }
}

async function handleApproval() {
  try {
    await jobStore.approveDuplicate(approvalDialog.value.jobId, approvalDialog.value.namespace)
    approvalDialog.value.show = false
    $q.notify({
      type: 'positive',
      message: t('jobs.duplicateApproved'),
    })
  } catch (err) {
    $q.notify({
      type: 'negative',
      message: err.message || t('jobs.approvalFailed'),
    })
  }
}

async function handlePublishApproval(job) {
  try {
    await jobStore.approveTranslation(job.id, job.namespace || 'glooscap-system')
    $q.notify({
      type: 'positive',
      message: t('jobs.publishJobCreated'),
      icon: 'check_circle',
    })
  } catch (err) {
    $q.notify({
      type: 'negative',
      message: err.message || t('jobs.publishFailed'),
      icon: 'error',
    })
  }
}

async function refresh() {
  await jobStore.refreshJobs()
  $q.notify({
    type: 'info',
    message: t('common.refresh'),
  })
}
</script>

<style scoped>
.jobs-page {
  background: #f9fafc;
}
</style>

