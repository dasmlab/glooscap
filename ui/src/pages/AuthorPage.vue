<template>
  <q-page padding class="author-page">
    <!-- Top Controls -->
    <div class="row items-center q-gutter-md q-mb-md">
      <div class="col-xs-12 col-sm-3">
        <q-select
          v-model="selectedTarget"
          :options="targetOptions"
          :label="$t('author.wikiTarget')"
          emit-value
          map-options
          dense
          outlined
        />
      </div>
      <div class="col-xs-12 col-sm-3">
        <q-select
          v-model="selectedArea"
          :options="areaOptions"
          :label="$t('author.selectArea')"
          emit-value
          map-options
          dense
          outlined
        />
      </div>
      <div class="col-xs-12 col-sm-3">
        <q-input
          v-model="search"
          :label="$t('author.searchPages')"
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
          :label="$t('author.refreshCatalogue')"
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

    <!-- Dual Panel Layout -->
    <div class="row q-gutter-md author-panels-container">
      <!-- English Panel -->
      <div class="col-12 col-md-6 author-panel-wrapper">
        <q-card class="author-panel">
          <q-card-section class="bg-blue-1">
            <div class="row items-center justify-between">
              <div class="text-h6 text-weight-bold">
                <q-toggle
                  v-model="leftPanelLang"
                  :true-value="'en'"
                  :false-value="'fr'"
                  :label="leftPanelLang === 'en' ? $t('author.panelEnglish') : $t('author.panelFrench')"
                  color="primary"
                  size="lg"
                />
              </div>
            </div>
          </q-card-section>
          <q-card-section>
            <q-select
              v-model="selectedLeftPage"
              :options="leftPanelPageOptions"
              :label="$t('author.selectPage')"
              emit-value
              map-options
              dense
              outlined
              clearable
              @update:model-value="onLeftPageSelected"
            />
          </q-card-section>
          <q-separator />
          <q-card-section class="markdown-container">
            <div v-if="leftPageContent" class="markdown-view">
              <pre class="markdown-content">{{ leftPageContent }}</pre>
            </div>
            <div v-else-if="selectedLeftPage" class="text-center text-grey-6 q-pa-lg">
              <q-spinner v-if="loadingLeftContent" color="primary" size="3em" />
              <div v-else>{{ $t('author.loadingContent') }}</div>
            </div>
            <div v-else class="text-center text-grey-6 q-pa-lg">
              {{ $t('author.noPageSelected') }}
            </div>
          </q-card-section>
          <q-separator />
          <q-card-section>
            <div class="row q-gutter-sm">
              <q-btn
                color="primary"
                :label="$t('author.edit')"
                outline
                :disable="!selectedLeftPage"
                @click="onEdit('left')"
              />
              <q-btn
                color="positive"
                :label="$t('author.create')"
                outline
                :disable="selectedLeftPage || !selectedRightPage"
                @click="onCreate('left')"
              />
              <q-btn
                color="secondary"
                :label="$t('author.publish')"
                outline
                :disable="!selectedLeftPage"
                @click="onPublish('left')"
              />
              <q-btn
                color="warning"
                :label="$t('author.revert')"
                outline
                :disable="!selectedLeftPage"
                @click="onRevert('left')"
              />
              <q-btn
                color="info"
                :label="$t('author.info')"
                outline
                :disable="!selectedLeftPage"
                @click="onInfo('left')"
              />
            </div>
          </q-card-section>
        </q-card>
      </div>

      <!-- French Panel -->
      <div class="col-12 col-md-6 author-panel-wrapper">
        <q-card class="author-panel">
          <q-card-section class="bg-red-1">
            <div class="row items-center justify-between">
              <div class="text-h6 text-weight-bold">
                <q-toggle
                  v-model="rightPanelLang"
                  :true-value="'fr'"
                  :false-value="'en'"
                  :label="rightPanelLang === 'fr' ? $t('author.panelFrench') : $t('author.panelEnglish')"
                  color="primary"
                  size="lg"
                />
              </div>
            </div>
          </q-card-section>
          <q-card-section>
            <q-select
              v-model="selectedRightPage"
              :options="rightPanelPageOptions"
              :label="$t('author.selectPage')"
              emit-value
              map-options
              dense
              outlined
              clearable
              @update:model-value="onRightPageSelected"
            />
          </q-card-section>
          <q-separator />
          <q-card-section class="markdown-container">
            <div v-if="rightPageContent" class="markdown-view">
              <pre class="markdown-content">{{ rightPageContent }}</pre>
            </div>
            <div v-else-if="selectedRightPage" class="text-center text-grey-6 q-pa-lg">
              <q-spinner v-if="loadingRightContent" color="primary" size="3em" />
              <div v-else>{{ $t('author.loadingContent') }}</div>
            </div>
            <div v-else class="text-center text-grey-6 q-pa-lg">
              {{ $t('author.noPageSelected') }}
            </div>
          </q-card-section>
          <q-separator />
          <q-card-section>
            <div class="row q-gutter-sm">
              <q-btn
                color="primary"
                :label="$t('author.edit')"
                outline
                :disable="!selectedRightPage"
                @click="onEdit('right')"
              />
              <q-btn
                color="positive"
                :label="$t('author.create')"
                outline
                :disable="selectedRightPage || !selectedLeftPage"
                @click="onCreate('right')"
              />
              <q-btn
                color="secondary"
                :label="$t('author.publish')"
                outline
                :disable="!selectedRightPage"
                @click="onPublish('right')"
              />
              <q-btn
                color="warning"
                :label="$t('author.revert')"
                outline
                :disable="!selectedRightPage"
                @click="onRevert('right')"
              />
              <q-btn
                color="info"
                :label="$t('author.info')"
                outline
                :disable="!selectedRightPage"
                @click="onInfo('right')"
              />
            </div>
          </q-card-section>
        </q-card>
      </div>
    </div>
  </q-page>
</template>

<script setup>
import { computed, ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useCatalogueStore } from 'src/stores/catalogue-store'

const { t } = useI18n()
const $q = useQuasar()
const catalogueStore = useCatalogueStore()

// Panel language toggles (opposite of each other)
const leftPanelLang = ref('en')
const rightPanelLang = ref('fr')

// Selected pages
const selectedLeftPage = ref(null)
const selectedRightPage = ref(null)

// Page content
const leftPageContent = ref('')
const rightPageContent = ref('')
const loadingLeftContent = ref(false)
const loadingRightContent = ref(false)

// Filters
const selectedTarget = ref(null)
const selectedArea = ref(null)
const search = ref('')

// Area options
const areaOptions = [
  { label: t('author.areaIaaS'), value: 'IaaS' },
  { label: t('author.areaSC'), value: 'SC' },
  { label: t('author.areaPaaS'), value: 'PaaS' },
]

// Target options
const targetOptions = computed(() =>
  catalogueStore.targets.map((target) => ({
    label: target.name || target.id,
    value: target.id,
    caption: [target.mode, target.uri].filter(Boolean).join(' • '),
  })),
)

// Filtered pages by area and language
const filteredPagesByArea = computed(() => {
  if (!selectedArea.value) return []
  
  const term = search.value.trim().toLowerCase()
  const areaPrefix = selectedArea.value === 'IaaS' ? 'IaaS' : selectedArea.value === 'SC' ? 'SC' : 'PaaS'
  
  return catalogueStore.pages.filter((page) => {
    // Filter by target
    const matchesTarget = !selectedTarget.value || page.targetId === selectedTarget.value
    
    // Filter by area - check if title starts with "EN -" or "FR -" followed by area
    const title = page.title || ''
    const matchesArea = title.includes(`${areaPrefix}`) || 
                       title.includes(`EN - ${areaPrefix}`) || 
                       title.includes(`FR - ${areaPrefix}`) ||
                       title.includes(`EN-${areaPrefix}`) ||
                       title.includes(`FR-${areaPrefix}`)
    
    // Filter by search term
    const matchesTerm =
      !term ||
      title.toLowerCase().includes(term) ||
      (page.slug && page.slug.toLowerCase().includes(term))
    
    return matchesTarget && matchesArea && matchesTerm
  })
})

// Left panel pages (filtered by leftPanelLang)
const leftPanelPageOptions = computed(() => {
  const lang = leftPanelLang.value.toUpperCase()
  return filteredPagesByArea.value
    .filter((page) => {
      const title = page.title || ''
      const langTag = page.language || ''
      // Check if page title starts with language prefix or language tag matches
      return title.startsWith(`${lang} -`) || 
             title.startsWith(`${lang}-`) ||
             langTag === lang ||
             langTag === lang.toLowerCase()
    })
    .map((page) => ({
      label: page.title || page.slug || 'Untitled',
      value: page.id,
      caption: page.slug || '',
    }))
})

// Right panel pages (filtered by rightPanelLang)
const rightPanelPageOptions = computed(() => {
  const lang = rightPanelLang.value.toUpperCase()
  return filteredPagesByArea.value
    .filter((page) => {
      const title = page.title || ''
      const langTag = page.language || ''
      // Check if page title starts with language prefix or language tag matches
      return title.startsWith(`${lang} -`) || 
             title.startsWith(`${lang}-`) ||
             langTag === lang ||
             langTag === lang.toLowerCase()
    })
    .map((page) => ({
      label: page.title || page.slug || 'Untitled',
      value: page.id,
      caption: page.slug || '',
    }))
})

// Watch for language toggle changes - ensure panels are opposite
watch(leftPanelLang, (newVal) => {
  rightPanelLang.value = newVal === 'en' ? 'fr' : 'en'
  // Clear selections when language changes
  selectedLeftPage.value = null
  selectedRightPage.value = null
  leftPageContent.value = ''
  rightPageContent.value = ''
})

watch(rightPanelLang, (newVal) => {
  leftPanelLang.value = newVal === 'fr' ? 'en' : 'fr'
  // Clear selections when language changes
  selectedLeftPage.value = null
  selectedRightPage.value = null
  leftPageContent.value = ''
  rightPageContent.value = ''
})

// Find matching page in opposite language
function findMatchingPage(pageId) {
  const page = catalogueStore.pages.find((p) => p.id === pageId)
  if (!page) return null
  
  const title = page.title || ''
  // Extract base title (remove language prefix)
  const baseTitle = title.replace(/^(EN|FR)\s*-\s*/i, '').trim()
  
  // Find matching page in opposite language
  const oppositeLang = leftPanelLang.value === 'en' ? 'FR' : 'EN'
  const matchingPage = catalogueStore.pages.find((p) => {
    if (p.id === pageId) return false
    const pTitle = p.title || ''
    const pBaseTitle = pTitle.replace(/^(EN|FR)\s*-\s*/i, '').trim()
    return pBaseTitle === baseTitle && 
           (pTitle.startsWith(`${oppositeLang} -`) || pTitle.startsWith(`${oppositeLang}-`))
  })
  
  return matchingPage
}

// Handle left panel page selection
function onLeftPageSelected(pageId) {
  if (!pageId) {
    leftPageContent.value = ''
    return
  }
  
  // Load content (stub for now)
  loadingLeftContent.value = true
  setTimeout(() => {
    // TODO: Fetch actual page content from API
    leftPageContent.value = '# Sample Markdown Content\n\nThis is a placeholder for the actual page content.\n\n**Note:** Content loading will be implemented when the API endpoint is ready.'
    loadingLeftContent.value = false
  }, 500)
  
  // Try to find matching page in right panel
  const matchingPage = findMatchingPage(pageId)
  if (matchingPage) {
    selectedRightPage.value = matchingPage.id
    onRightPageSelected(matchingPage.id)
  } else {
    // Clear right panel if no match
    selectedRightPage.value = null
    rightPageContent.value = ''
  }
}

// Handle right panel page selection
function onRightPageSelected(pageId) {
  if (!pageId) {
    rightPageContent.value = ''
    return
  }
  
  // Load content (stub for now)
  loadingRightContent.value = true
  setTimeout(() => {
    // TODO: Fetch actual page content from API
    rightPageContent.value = '# Contenu Markdown Exemple\n\nCeci est un espace réservé pour le contenu réel de la page.\n\n**Note:** Le chargement du contenu sera implémenté lorsque le point de terminaison API sera prêt.'
    loadingRightContent.value = false
  }, 500)
  
  // Try to find matching page in left panel
  const matchingPage = findMatchingPage(pageId)
  if (matchingPage) {
    selectedLeftPage.value = matchingPage.id
    onLeftPageSelected(matchingPage.id)
  } else {
    // Clear left panel if no match
    selectedLeftPage.value = null
    leftPageContent.value = ''
  }
}

// Action handlers (stubs for now)
function onEdit(panel) {
  $q.notify({
    type: 'info',
    message: `Edit action for ${panel} panel (not yet implemented)`,
  })
}

function onCreate(panel) {
  $q.notify({
    type: 'info',
    message: `Create action for ${panel} panel (not yet implemented)`,
  })
}

function onPublish(panel) {
  $q.notify({
    type: 'info',
    message: `Publish action for ${panel} panel (not yet implemented)`,
  })
}

function onRevert(panel) {
  $q.notify({
    type: 'info',
    message: `Revert action for ${panel} panel (not yet implemented)`,
  })
}

function onInfo(panel) {
  $q.notify({
    type: 'info',
    message: `Info action for ${panel} panel (not yet implemented)`,
  })
}

async function refresh() {
  await catalogueStore.fetchWikiTargets()
  await catalogueStore.refreshCatalogue()
  $q.notify({
    type: 'info',
    message: 'Catalogue refresh triggered',
  })
}

// Initialize
onMounted(async () => {
  try {
    await catalogueStore.initialise()
    if (catalogueStore.targets.length > 0 && !selectedTarget.value) {
      selectedTarget.value = catalogueStore.targets[0].id
    }
  } catch (err) {
    console.error('[AuthorPage] Initialization error:', err)
    $q.notify({ type: 'negative', message: err.message || 'Failed to load catalogue' })
  }
})

// Watch for target changes
watch(
  () => catalogueStore.selectedTargetId,
  (newTargetId) => {
    if (newTargetId) {
      selectedTarget.value = newTargetId
    }
  },
  { immediate: true },
)

watch(selectedTarget, (newTarget) => {
  if (newTarget) {
    catalogueStore.setTarget(newTarget).catch((err) => {
      $q.notify({ type: 'negative', message: err.message || 'Failed to load target' })
    })
  }
})
</script>

<style scoped>
.author-page {
  background: #f4f7fb;
}

.author-panels-container {
  align-items: stretch;
}

.author-panel-wrapper {
  display: flex;
  flex-direction: column;
}

.author-panel {
  display: flex;
  flex-direction: column;
  min-height: 600px;
  height: 100%;
}

.author-panel .q-card__section {
  flex-shrink: 0;
}

.markdown-container {
  flex: 1 1 auto;
  overflow: auto;
  min-height: 300px;
  display: flex;
  flex-direction: column;
}

.markdown-view {
  width: 100%;
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
}

.markdown-content {
  font-family: 'Courier New', monospace;
  font-size: 14px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-wrap: break-word;
  padding: 16px;
  margin: 0;
  background: #fafafa;
  border-radius: 4px;
  border: 1px solid #e0e0e0;
  flex: 1 1 auto;
  overflow: auto;
}
</style>

