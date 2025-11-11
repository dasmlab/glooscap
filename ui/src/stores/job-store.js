import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

const mockJobs = [
  {
    id: 'job-1001',
    pageTitle: 'Platform Onboarding',
    targetId: 'outline-en',
    pipeline: 'TektonJob',
    state: 'Queued',
    createdAt: '2025-11-10T15:30:00Z',
    updatedAt: '2025-11-10T15:30:00Z',
  },
  {
    id: 'job-1002',
    pageTitle: 'Disaster Recovery Playbook',
    targetId: 'outline-en',
    pipeline: 'TektonJob',
    state: 'Publishing',
    createdAt: '2025-11-09T09:32:00Z',
    updatedAt: '2025-11-09T10:01:00Z',
  },
  {
    id: 'job-1003',
    pageTitle: 'Secure Coding Checklist',
    targetId: 'outline-en',
    pipeline: 'InlineLLM',
    state: 'Failed',
    createdAt: '2025-11-08T18:32:00Z',
    updatedAt: '2025-11-08T18:45:00Z',
    message: 'WikiTarget not found',
  },
]

export const useJobStore = defineStore('jobs', () => {
  const jobs = ref(mockJobs)
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
      await new Promise((resolve) => setTimeout(resolve, 300))
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
  }

  function enqueueJob(payload) {
    const newJob = {
      id: `job-${Math.floor(Math.random() * 10_000)}`,
      pageTitle: payload.pageTitle,
      targetId: payload.targetId,
      pipeline: payload.pipeline || 'TektonJob',
      state: 'Queued',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    }
    jobs.value = [newJob, ...jobs.value]
  }

  return {
    jobs,
    recentJobs,
    activeCount,
    loading,
    error,
    refreshJobs,
    enqueueJob,
  }
})

