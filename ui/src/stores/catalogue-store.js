import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import api from 'src/services/api'

export const useCatalogueStore = defineStore('catalogue', () => {
  const targets = ref([])
  const pages = ref([])
  const wikitargets = ref([])

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

  const selectionCount = computed(() => {
    try {
      if (!selectedPages.value) {
        selectedPages.value = new Set()
        return 0
      }
      // Check if it's actually a Set
      if (typeof selectedPages.value.size === 'undefined' || typeof selectedPages.value.has !== 'function') {
        selectedPages.value = new Set()
        return 0
      }
      return selectedPages.value.size || 0
    } catch (err) {
      console.error('[CatalogueStore] Error getting selectionCount:', err)
      selectedPages.value = new Set()
      return 0
    }
  })

  async function setTarget(targetId) {
    selectedTargetId.value = targetId
    // Ensure selectedPages is a Set before clearing
    if (!selectedPages.value || typeof selectedPages.value.clear !== 'function') {
      selectedPages.value = new Set()
    } else {
      selectedPages.value.clear()
    }
    await refreshCatalogue()
  }

  function toggleSelection(pageId) {
    try {
      // Ensure selectedPages is a Set
      if (!selectedPages.value || typeof selectedPages.value.has !== 'function') {
        selectedPages.value = new Set()
      }
      const next = new Set(selectedPages.value)
      next.has(pageId) ? next.delete(pageId) : next.add(pageId)
      selectedPages.value = next
    } catch (err) {
      console.error('[CatalogueStore] Error in toggleSelection:', err)
      selectedPages.value = new Set()
    }
  }

  function clearSelection() {
    selectedPages.value = new Set()
  }

  function setSelection(pageIds) {
    // Ensure we always have a Set, even if pageIds is undefined/null
    if (!pageIds || !Array.isArray(pageIds)) {
      selectedPages.value = new Set()
    } else {
      selectedPages.value = new Set(pageIds)
    }
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
      console.log('[CatalogueStore] Fetching catalogue with params:', params)
      const { data } = await api.get('/catalogue', { params })
      console.log('[CatalogueStore] Received pages:', data)
      pages.value = Array.isArray(data)
        ? data.map((page) => ({
            status: page.status || 'Discovered',
            ...page,
            targetId: selectedTargetId.value,
          }))
        : []
      console.log('[CatalogueStore] Processed pages:', pages.value.length)
    } catch (err) {
      console.error('[CatalogueStore] Error refreshing catalogue:', err)
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
  }

  async function ensureTargets() {
    if (targets.value.length > 0) {
      return
    }
    try {
      console.log('[CatalogueStore] Fetching targets from /api/v1/targets')
      const { data } = await api.get('/targets')
      console.log('[CatalogueStore] Received targets:', data)
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
      console.log('[CatalogueStore] Processed targets:', targets.value)
      if (!selectedTargetId.value && targets.value.length > 0) {
        selectedTargetId.value = targets.value[0].id
        console.log('[CatalogueStore] Auto-selected target:', selectedTargetId.value)
      }
    } catch (err) {
      console.error('[CatalogueStore] Error fetching targets:', err)
      throw err
    }
  }

  async function fetchWikiTargets() {
    loading.value = true
    error.value = null
    try {
      console.log('[CatalogueStore] Fetching WikiTargets from /api/v1/wikitargets')
      const { data } = await api.get('/wikitargets', {
        params: { namespace: 'glooscap-system' },
      })
      console.log('[CatalogueStore] Received WikiTargets:', data)
      wikitargets.value = Array.isArray(data?.items) ? data.items : []
      console.log('[CatalogueStore] Processed WikiTargets:', wikitargets.value.length)
    } catch (err) {
      console.error('[CatalogueStore] Error fetching WikiTargets:', err)
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
  }

  const selectedWikiTargetStatus = computed(() => {
    if (!selectedTargetId.value) return null
    const target = wikitargets.value.find(
      (wt) => `${wt.namespace}/${wt.name}` === selectedTargetId.value,
    )
    return target?.status || null
  })

  let eventSource = null
  let logCallback = null

  function subscribeToEvents(callback) {
    logCallback = callback
    
    if (eventSource) {
      eventSource.close()
    }
    
    // Construct SSE URL properly - support both build-time and runtime config
    const getApiBaseURL = () => {
      if (typeof window !== 'undefined' && window.__API_BASE_URL__) {
        return window.__API_BASE_URL__
      }
      return import.meta.env.VITE_API_BASE_URL || '/api/v1'
    }
    const apiBaseUrl = getApiBaseURL()
    let baseUrl = apiBaseUrl
    
    // Remove /api/v1 suffix if present
    if (baseUrl.endsWith('/api/v1')) {
      baseUrl = baseUrl.slice(0, -7)
    } else if (baseUrl.endsWith('/api/v1/')) {
      baseUrl = baseUrl.slice(0, -8)
    }
    // Ensure baseUrl doesn't end with /
    baseUrl = baseUrl.replace(/\/$/, '')
    
    // Construct events URL
    const eventsUrl = `${baseUrl}/api/v1/events`
    
    logCallback?.('INFO', `Connecting to SSE endpoint: ${eventsUrl}`)
    
    try {
      eventSource = new EventSource(eventsUrl)
    } catch (err) {
      logCallback?.('ERROR', `Failed to create EventSource: ${err.message}`)
      return
    }
    
    eventSource.onopen = () => {
      logCallback?.('INFO', 'SSE connection opened')
    }
    
    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        logCallback?.('DEBUG', 'Received SSE event', data)
        
        // Process nanabush status from event
        if (data.nanabush && typeof data.nanabush === 'object') {
          logCallback?.('DEBUG', 'Received nanabush status from SSE', data.nanabush)
          // Emit nanabush status event for SettingsPage to consume
          // We'll use a custom event or store the status in a reactive ref
          // For now, emit a custom event
          if (typeof window !== 'undefined') {
            window.dispatchEvent(new CustomEvent('nanabush-status', { detail: data.nanabush }))
          }
        }
        
        // Process WikiTargets and pages from event
        if (data.wikitargets && Array.isArray(data.wikitargets)) {
          // Update targets
          const newTargets = []
          const newPages = []
          
          data.wikitargets.forEach((wt) => {
            const targetId = wt.targetId || `${wt.namespace}/${wt.name}`
            newTargets.push({
              id: targetId,
              name: wt.name || targetId,
              uri: wt.wikitarget,
              mode: wt.mode,
              namespace: wt.namespace,
              resourceName: wt.name,
            })
            
            // Add pages
            if (wt.pages && Array.isArray(wt.pages)) {
              wt.pages.forEach((page) => {
                newPages.push({
                  ...page,
                  title: page.name || page.title,
                  targetId,
                  status: page.state || 'Discovered',
                })
              })
            }
          })
          
          // Update store
          if (newTargets.length > 0) {
            targets.value = newTargets
          }
          if (newPages.length > 0) {
            // Preserve selectedPages Set when updating pages
            let currentSelection = new Set()
            try {
              if (selectedPages.value && typeof selectedPages.value.has === 'function') {
                currentSelection = new Set(selectedPages.value)
              }
            } catch (err) {
              console.error('[CatalogueStore] Error preserving selection:', err)
            }
            
            pages.value = newPages
            
            // Restore selection for pages that still exist
            try {
              selectedPages.value = new Set(
                Array.from(currentSelection).filter((id) =>
                  newPages.some((p) => p.id === id)
                )
              )
            } catch (err) {
              console.error('[CatalogueStore] Error restoring selection:', err)
              selectedPages.value = new Set()
            }
            
            logCallback?.('INFO', `Updated ${newPages.length} pages from SSE`)
          }
        }
      } catch (err) {
        logCallback?.('ERROR', 'Failed to parse SSE event', err.message)
        console.error('[CatalogueStore] SSE parse error:', err)
      }
    }
    
    eventSource.onerror = (err) => {
      const state = eventSource?.readyState
      
      // Only log errors for actual failures, not during normal connection attempts
      // CONNECTING (0) is normal during initial connection - don't log as error
      if (state === EventSource.CONNECTING) {
        // This is normal during connection - just log as debug
        logCallback?.('DEBUG', 'SSE connecting...')
        return
      }
      
      // CLOSED (2) means connection failed or was closed - this is an error
      if (state === EventSource.CLOSED) {
        logCallback?.('ERROR', 'SSE connection closed', err)
        console.error('[CatalogueStore] SSE connection closed:', err)
        
        // Try to reconnect after a delay
        setTimeout(() => {
          logCallback?.('INFO', 'Attempting to reconnect SSE...')
          subscribeToEvents(callback)
        }, 5000)
      } else {
        // OPEN (1) or unknown state - log as warning
        logCallback?.('WARN', 'SSE error in unexpected state', { state, error: err })
      }
    }
  }
  
  function unsubscribeFromEvents() {
    if (eventSource) {
      eventSource.close()
      eventSource = null
      logCallback?.('INFO', 'SSE connection closed')
    }
    logCallback = null
  }

  async function initialise() {
    await ensureTargets()
    await fetchWikiTargets()
    await refreshCatalogue()
    // SSE subscription will be started by the component
  }

  return {
    targets,
    pages,
    filteredPages,
    wikitargets,
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
    fetchWikiTargets,
    selectedWikiTargetStatus,
    subscribeToEvents,
    unsubscribeFromEvents,
  }
})

