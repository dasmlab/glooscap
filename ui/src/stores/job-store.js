import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import api from 'src/services/api'

export const useJobStore = defineStore('jobs', () => {
  const jobs = ref([])
  const loading = ref(false)
  const error = ref(null)

  const recentJobs = computed(() =>
    [...jobs.value].sort(
      (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
    ),
  )

  const activeCount = computed(
    () => jobs.value.filter((job) => ['Queued', 'Dispatching', 'Running'].includes(job.state)).length,
  )

  async function refreshJobs() {
    loading.value = true
    error.value = null
    try {
      const { data } = await api.get('/jobs')
      const items = data?.items ?? {}
      jobs.value = Object.entries(items).map(([id, item]) => {
        const status = item?.status ?? {}
        return {
          id,
          pageTitle: item?.pageTitle || status?.message || id,
          pipeline: item?.pipeline || 'TektonJob',
          state: status?.state || 'Queued',
          createdAt: status?.startedAt,
          updatedAt: status?.finishedAt || status?.startedAt,
          message: status?.message || '',
          targetId: item?.targetRef || '',
          namespace: item?.namespace || 'glooscap-system',
          duplicateInfo: status?.duplicateInfo || null,
        }
      })
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
  }

  async function submitJob(payload) {
    loading.value = true
    try {
      const response = await api.post('/jobs', payload)
      await refreshJobs()
      // Return the response so caller can get the job name/ID
      return response.data || response
    } finally {
      loading.value = false
    }
  }

  async function approveDuplicate(jobId, namespace) {
    loading.value = true
    try {
      // Patch the TranslationJob to add the approval annotation
      // Note: This requires a PATCH endpoint on the operator API
      await api.patch(`/jobs/${namespace}/${jobId}/approve-duplicate`, {})
      await refreshJobs()
    } finally {
      loading.value = false
    }
  }

  async function approveTranslation(jobId, namespace) {
    loading.value = true
    try {
      // Call the approve-translation endpoint to create a publish job
      await api.post('/approve-translation', {
        jobName: jobId,
        namespace: namespace,
      })
      await refreshJobs()
    } finally {
      loading.value = false
    }
  }

  return {
    jobs,
    recentJobs,
    activeCount,
    loading,
    error,
    refreshJobs,
    submitJob,
    approveDuplicate,
    approveTranslation,
  }
})

