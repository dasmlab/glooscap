import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import api from 'src/services/api'

export const useCatalogueStore = defineStore('catalogue', () => {
  const targets = ref([])
  const pages = ref([])

  const selectedTargetId = ref(null)
  const selectedPages = ref(new Set())
  const search = ref('')
  const loading = ref(false)
  const error = ref(null)

  const filteredPages = computed(() => {
    const activeTarget = selectedTargetId.value
    const term = search.value.trim().toLowerCase()

    return pages.value.filter((page) => {
      const matchesTarget = !activeTarget || page.targetId === activeTarget
      const matchesTerm =
        !term ||
        page.title.toLowerCase().includes(term) ||
        page.slug.toLowerCase().includes(term)
      return matchesTarget && matchesTerm
    })
  })

  const selectionCount = computed(() => selectedPages.value.size)

  async function setTarget(targetId) {
    selectedTargetId.value = targetId
    selectedPages.value.clear()
    await refreshCatalogue()
  }

  function toggleSelection(pageId) {
    const next = new Set(selectedPages.value)
    next.has(pageId) ? next.delete(pageId) : next.add(pageId)
    selectedPages.value = next
  }

  function clearSelection() {
    selectedPages.value = new Set()
  }

  function setSelection(pageIds) {
    selectedPages.value = new Set(pageIds)
  }

  async function refreshCatalogue() {
    loading.value = true
    error.value = null
    try {
      await ensureTargets()
      const params = {}
      if (selectedTargetId.value) {
        params.target = selectedTargetId.value
      }
      const { data } = await api.get('/catalogue', { params })
      pages.value = Array.isArray(data)
        ? data.map((page) => ({
            status: page.status || 'Discovered',
            ...page,
            targetId: selectedTargetId.value,
          }))
        : []
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
  }

  async function ensureTargets() {
    if (targets.value.length > 0) {
      return
    }
    const { data } = await api.get('/targets')
    targets.value = Array.isArray(data)
      ? data.map((target) => ({
          id: target.id,
          name: target.name || target.id,
          uri: target.uri,
          mode: target.mode,
          namespace: target.namespace,
          resourceName: target.name,
        }))
      : []
    if (!selectedTargetId.value && targets.value.length > 0) {
      selectedTargetId.value = targets.value[0].id
    }
  }

  async function initialise() {
    await ensureTargets()
    await refreshCatalogue()
  }

  return {
    targets,
    pages,
    filteredPages,
    selectedTargetId,
    selectedPages,
    selectionCount,
    search,
    loading,
    error,
    initialise,
    setTarget,
    toggleSelection,
    clearSelection,
    setSelection,
    refreshCatalogue,
  }
})

