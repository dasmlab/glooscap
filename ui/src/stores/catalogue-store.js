import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

const mockTargets = [
  {
    id: 'outline-en',
    name: 'Outline English',
    uri: 'https://wiki.infra.dasmlab.org',
    mode: 'ReadOnly',
    defaultLanguage: 'en',
  },
  {
    id: 'outline-fr',
    name: 'Outline French',
    uri: 'https://wiki-fr.infra.dasmlab.org',
    mode: 'ReadWrite',
    defaultLanguage: 'fr',
  },
]

const mockPages = [
  {
    id: 'page-001',
    targetId: 'outline-en',
    title: 'Platform Onboarding',
    slug: 'platform-onboarding',
    updatedAt: '2025-11-10T13:45:00Z',
    language: 'en',
    hasAssets: true,
    status: 'New',
  },
  {
    id: 'page-002',
    targetId: 'outline-en',
    title: 'Disaster Recovery Playbook',
    slug: 'dr-playbook',
    updatedAt: '2025-11-08T09:15:00Z',
    language: 'en',
    hasAssets: false,
    status: 'Translated',
  },
  {
    id: 'page-003',
    targetId: 'outline-en',
    title: 'Secure Coding Checklist',
    slug: 'secure-coding-checklist',
    updatedAt: '2025-11-05T18:05:00Z',
    language: 'en',
    hasAssets: true,
    status: 'InProgress',
  },
]

export const useCatalogueStore = defineStore('catalogue', () => {
  const targets = ref(mockTargets)
  const pages = ref(mockPages)

  const selectedTargetId = ref(targets.value[0]?.id ?? null)
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

  function setTarget(targetId) {
    selectedTargetId.value = targetId
    selectedPages.value.clear()
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
      // TODO: replace mock with API call
      await new Promise((resolve) => setTimeout(resolve, 300))
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
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
    setTarget,
    toggleSelection,
    clearSelection,
    setSelection,
    refreshCatalogue,
  }
})

