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
          v-model="selectedPage"
          :options="pageOptions"
          :label="$t('author.selectPage')"
          emit-value
          map-options
          dense
          outlined
          clearable
          @update:model-value="onPageSelected"
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
            <div v-if="leftPageUri" class="text-caption text-grey-7 q-mt-xs">
              <q-icon name="link" size="xs" />
              {{ leftPageUri }}
            </div>
            <!-- Action Buttons -->
            <div class="row q-gutter-sm q-mt-md">
              <q-btn
                color="primary"
                :label="$t('author.edit')"
                outline
                size="sm"
                :disable="!selectedLeftPage"
                @click="onEdit('left')"
              />
              <q-btn
                color="positive"
                :label="$t('author.create')"
                outline
                size="sm"
                :disable="selectedLeftPage || !selectedRightPage"
                @click="onCreate('left')"
              />
              <q-btn
                color="secondary"
                :label="$t('author.publish')"
                outline
                size="sm"
                :disable="!selectedLeftPage"
                @click="onPublish('left')"
              />
              <q-btn
                color="warning"
                :label="$t('author.revert')"
                outline
                size="sm"
                :disable="!selectedLeftPage"
                @click="onRevert('left')"
              />
              <q-btn
                color="info"
                :label="$t('author.info')"
                outline
                size="sm"
                :disable="!selectedLeftPage"
                @click="onInfo('left')"
              />
              <q-btn
                color="accent"
                icon="translate"
                :label="$t('author.translate')"
                outline
                size="sm"
                :disable="!selectedLeftPage"
                @click="showTranslateDialog('left')"
              />
            </div>
          </q-card-section>
          <q-separator />
          <q-card-section class="markdown-container">
            <div v-if="leftPageContent" class="markdown-view">
              <pre class="markdown-content">{{ leftPageContent }}</pre>
              <div class="text-caption text-grey-6 q-mt-xs">
                Content length: {{ leftPageContent.length }} characters
              </div>
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
            <div v-if="rightPageUri" class="text-caption text-grey-7 q-mt-xs">
              <q-icon name="link" size="xs" />
              {{ rightPageUri }}
            </div>
            <!-- Action Buttons -->
            <div class="row q-gutter-sm q-mt-md">
              <q-btn
                color="primary"
                :label="$t('author.edit')"
                outline
                size="sm"
                :disable="!selectedRightPage"
                @click="onEdit('right')"
              />
              <q-btn
                color="positive"
                :label="$t('author.create')"
                outline
                size="sm"
                :disable="selectedRightPage || !selectedLeftPage"
                @click="onCreate('right')"
              />
              <q-btn
                color="secondary"
                :label="$t('author.publish')"
                outline
                size="sm"
                :disable="!selectedRightPage"
                @click="onPublish('right')"
              />
              <q-btn
                color="warning"
                :label="$t('author.revert')"
                outline
                size="sm"
                :disable="!selectedRightPage"
                @click="onRevert('right')"
              />
              <q-btn
                color="info"
                :label="$t('author.info')"
                outline
                size="sm"
                :disable="!selectedRightPage"
                @click="onInfo('right')"
              />
              <q-btn
                color="accent"
                icon="translate"
                :label="$t('author.translate')"
                outline
                size="sm"
                :disable="!selectedRightPage"
                @click="showTranslateDialog('right')"
              />
            </div>
          </q-card-section>
          <q-separator />
          <q-card-section class="markdown-container">
            <div v-if="rightPageContent" class="markdown-view">
              <pre class="markdown-content">{{ rightPageContent }}</pre>
              <div class="text-caption text-grey-6 q-mt-xs">
                Content length: {{ rightPageContent.length }} characters
              </div>
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
    <!-- Translate Confirmation Dialog -->
    <q-dialog v-model="showTranslateDialogRef">
      <q-card style="min-width: 350px">
        <q-card-section>
          <div class="text-h6">Generate Translation?</div>
        </q-card-section>

        <q-card-section v-if="translatePageRef">
          <div class="text-body2">
            <strong>Page:</strong> {{ translatePageRef.title }}<br>
            <strong>ID:</strong> {{ translatePageRef.id }}<br>
            <strong>Language:</strong> {{ translatePageRef.language || 'EN' }} → 
            {{ typeof settingsStore.defaultLanguage === 'string' 
              ? settingsStore.defaultLanguage 
              : settingsStore.defaultLanguage?.value ?? 'fr-CA' }}
          </div>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat label="Cancel" color="primary" @click="showTranslateDialogRef = false" />
          <q-btn flat label="Yes" color="primary" @click="confirmTranslate" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { computed, ref, watch, onMounted, inject } from 'vue'
import { useQuasar } from 'quasar'
import { useCatalogueStore } from 'src/stores/catalogue-store'
import api from 'src/services/api'
import { useJobStore } from 'src/stores/job-store'
import { useSettingsStore } from 'src/stores/settings-store'

const $q = useQuasar()
const catalogueStore = useCatalogueStore()
const jobStore = useJobStore()
const settingsStore = useSettingsStore()
const consoleRef = inject('console', null)

// Log to console component if available
function logToConsole(level, message, data = null) {
  if (consoleRef && consoleRef.value && typeof consoleRef.value.addLog === 'function') {
    try {
      consoleRef.value.addLog(level, message, data)
    } catch (err) {
      console.error('Failed to log to console:', err)
    }
  }
  // Also log to browser console
  if (level === 'ERROR') {
    console.error(`[${level}]`, message, data || '')
  } else if (level === 'WARN') {
    console.warn(`[${level}]`, message, data || '')
  } else {
    console.log(`[${level}]`, message, data || '')
  }
}

// Panel language toggles (opposite of each other)
const leftPanelLang = ref('en')
const rightPanelLang = ref('fr')

// Selected pages
const selectedLeftPage = ref(null)
const selectedRightPage = ref(null)
const selectedPage = ref(null) // Main page selector at top

// Page content and metadata
const leftPageContent = ref('')
const rightPageContent = ref('')
const leftPageUri = ref('')
const rightPageUri = ref('')
const leftPageMetadata = ref(null)
const rightPageMetadata = ref(null)
const loadingLeftContent = ref(false)
const loadingRightContent = ref(false)


// Translate dialog
const showTranslateDialogRef = ref(false)
const translatePageRef = ref(null)
const translatePanelRef = ref(null)

// Filters
const selectedTarget = ref(null)
const search = ref('')

// Target options
const targetOptions = computed(() =>
  catalogueStore.targets.map((target) => ({
    label: target.name || target.id,
    value: target.id,
    caption: [target.mode, target.uri].filter(Boolean).join(' • '),
  })),
)

// All pages filtered by target and search (like catalog)
const filteredPages = computed(() => {
  const term = search.value.trim().toLowerCase()
  return catalogueStore.filteredPages.filter((page) => {
    const matchesTarget = !selectedTarget.value || page.targetId === selectedTarget.value
    const matchesTerm =
      !term ||
      (page.title && page.title.toLowerCase().includes(term)) ||
      (page.slug && page.slug.toLowerCase().includes(term))
    return matchesTarget && matchesTerm
  })
})

// Page options for main selector (all pages)
const pageOptions = computed(() =>
  filteredPages.value.map((page) => ({
    label: page.title || page.slug || 'Untitled',
    value: page.id,
    caption: `${page.language || 'N/A'} • ${page.slug || ''}`,
  })),
)

// Left panel pages (filtered by leftPanelLang)
const leftPanelPageOptions = computed(() => {
  const lang = leftPanelLang.value.toUpperCase()
  return filteredPages.value
    .filter((page) => {
      return detectPageLanguage(page) === lang.toLowerCase()
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
  return filteredPages.value
    .filter((page) => {
      return detectPageLanguage(page) === lang.toLowerCase()
    })
    .map((page) => ({
      label: page.title || page.slug || 'Untitled',
      value: page.id,
      caption: page.slug || '',
    }))
})

// Detect page language from title or language field
function detectPageLanguage(page) {
  if (!page) return null
  
  // Check language field first
  if (page.language) {
    const lang = page.language.toUpperCase()
    if (lang === 'EN' || lang === 'EN-US' || lang.startsWith('EN')) return 'en'
    if (lang === 'FR' || lang === 'FR-CA' || lang.startsWith('FR')) return 'fr'
  }
  
  // Check title prefix
  const title = (page.title || '').toUpperCase()
  if (title.startsWith('EN -') || title.startsWith('EN-')) return 'en'
  if (title.startsWith('FR -') || title.startsWith('FR-')) return 'fr'
  
  // Default to English if unsure
  return 'en'
}

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
function findMatchingPage(pageId, targetLang) {
  const page = catalogueStore.pages.find((p) => p.id === pageId)
  if (!page) return null
  
  const title = page.title || ''
  // Extract base title (remove language prefix)
  const baseTitle = title.replace(/^(EN|FR)\s*-\s*/i, '').trim()
  
  // Find matching pages in target language
  const candidates = catalogueStore.pages.filter((p) => {
    if (p.id === pageId) return false
    const pTitle = p.title || ''
    const pBaseTitle = pTitle.replace(/^(EN|FR)\s*-\s*/i, '').trim()
    const pLang = detectPageLanguage(p)
    return pBaseTitle === baseTitle && pLang === targetLang
  })
  
  // If multiple candidates, return the first one (we can prompt user later)
  return candidates.length > 0 ? candidates[0] : null
}

// Main page selector handler
async function onPageSelected(pageId) {
  if (!pageId) {
    selectedLeftPage.value = null
    selectedRightPage.value = null
    leftPageContent.value = ''
    rightPageContent.value = ''
    leftPageUri.value = ''
    rightPageUri.value = ''
    return
  }

  logToConsole('INFO', `Page selected: ${pageId}`)
  
  const page = catalogueStore.pages.find((p) => p.id === pageId)
  if (!page) {
    logToConsole('WARN', `Page not found: ${pageId}`)
    return
  }

  const pageLang = detectPageLanguage(page)
  logToConsole('DEBUG', `Detected page language: ${pageLang}`, {
    pageId,
    title: page.title,
    language: page.language,
  })

  // Determine which panel to display in
  if (pageLang === 'en') {
    // Display in left panel (English)
    selectedLeftPage.value = pageId
    await loadPageContent('left', pageId, page)
    
    // Try to find matching French page
    const matchingFr = findMatchingPage(pageId, 'fr')
    if (matchingFr) {
      logToConsole('INFO', `Found matching French page: ${matchingFr.id}`, {
        title: matchingFr.title,
      })
      selectedRightPage.value = matchingFr.id
      await loadPageContent('right', matchingFr.id, matchingFr)
    } else {
      logToConsole('INFO', 'No matching French page found')
      selectedRightPage.value = null
      rightPageContent.value = ''
      rightPageUri.value = ''
    }
  } else if (pageLang === 'fr') {
    // Display in right panel (French)
    selectedRightPage.value = pageId
    await loadPageContent('right', pageId, page)
    
    // Try to find matching English page
    const matchingEn = findMatchingPage(pageId, 'en')
    if (matchingEn) {
      logToConsole('INFO', `Found matching English page: ${matchingEn.id}`, {
        title: matchingEn.title,
      })
      selectedLeftPage.value = matchingEn.id
      await loadPageContent('left', matchingEn.id, matchingEn)
    } else {
      logToConsole('INFO', 'No matching English page found')
      selectedLeftPage.value = null
      leftPageContent.value = ''
      leftPageUri.value = ''
    }
  } else {
    // Unknown language - prompt user or default to left panel
    logToConsole('WARN', `Unknown page language, defaulting to left panel`, {
      pageId,
      title: page.title,
    })
    selectedLeftPage.value = pageId
    await loadPageContent('left', pageId, page)
  }
}

// Load page content from API
async function loadPageContent(panel, pageId, pageMetadata) {
  const loadingRef = panel === 'left' ? loadingLeftContent : loadingRightContent
  const contentRef = panel === 'left' ? leftPageContent : rightPageContent
  const uriRef = panel === 'left' ? leftPageUri : rightPageUri
  const metadataRef = panel === 'left' ? leftPageMetadata : rightPageMetadata

  loadingRef.value = true
  contentRef.value = ''
  uriRef.value = ''

  try {
    const target = catalogueStore.targets.find((t) => t.id === selectedTarget.value)
    if (!target) {
      throw new Error('No target selected')
    }

    const namespace = target?.namespace || 'glooscap-system'
    const targetRef = target?.resourceName || target?.id || ''

    logToConsole('DEBUG', `Fetching page content for ${panel} panel`, {
      pageId,
      targetRef,
      namespace,
    })

    const response = await api.get(`/pages/${targetRef}/${pageId}/content`, {
      params: { namespace },
    })

    const content = response.data
    const markdownContent = content.markdown || ''
    contentRef.value = markdownContent
    uriRef.value = content.metadata?.uri || pageMetadata?.uri || `/${content.slug || pageId}`
    metadataRef.value = content.metadata || pageMetadata

    logToConsole('INFO', `Page content loaded for ${panel} panel`, {
      pageId,
      title: content.title,
      uri: uriRef.value,
      contentLength: content.rawLength || 0,
      markdownLength: markdownContent.length,
      markdownPreview: markdownContent.substring(0, 200),
    })
  } catch (err) {
    logToConsole('ERROR', `Failed to load page content for ${panel} panel`, {
      pageId,
      error: err.message,
    })
    contentRef.value = `Error loading content: ${err.message || 'Unknown error'}`
    uriRef.value = pageMetadata?.uri || `/${pageMetadata?.slug || pageId}`
    if ($q && typeof $q.notify === 'function') {
      $q.notify({
        type: 'negative',
        message: `Failed to load page content: ${err.message || 'Unknown error'}`,
        timeout: 5000,
      })
    }
  } finally {
    loadingRef.value = false
  }
}

// Handle left panel page selection
async function onLeftPageSelected(pageId) {
  if (!pageId) {
    leftPageContent.value = ''
    leftPageUri.value = ''
    return
  }
  
  const page = catalogueStore.pages.find((p) => p.id === pageId)
  await loadPageContent('left', pageId, page)
  
  // Try to find matching page in right panel
  const matchingPage = findMatchingPage(pageId, 'fr')
  if (matchingPage) {
    selectedRightPage.value = matchingPage.id
    await onRightPageSelected(matchingPage.id)
  } else {
    // Clear right panel if no match
    selectedRightPage.value = null
    rightPageContent.value = ''
    rightPageUri.value = ''
  }
}

// Handle right panel page selection
async function onRightPageSelected(pageId) {
  if (!pageId) {
    rightPageContent.value = ''
    rightPageUri.value = ''
    return
  }
  
  const page = catalogueStore.pages.find((p) => p.id === pageId)
  await loadPageContent('right', pageId, page)
  
  // Try to find matching page in left panel
  const matchingPage = findMatchingPage(pageId, 'en')
  if (matchingPage) {
    selectedLeftPage.value = matchingPage.id
    await onLeftPageSelected(matchingPage.id)
  } else {
    // Clear left panel if no match
    selectedLeftPage.value = null
    leftPageContent.value = ''
    leftPageUri.value = ''
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

// Translate dialog functions
function showTranslateDialog(panel) {
  const selectedPageId = panel === 'left' ? selectedLeftPage.value : selectedRightPage.value
  if (!selectedPageId) {
    $q.notify({
      type: 'warning',
      message: 'No page selected in this panel',
      timeout: 3000,
    })
    return
  }

  const page = catalogueStore.pages.find((p) => p.id === selectedPageId)
  if (!page) {
    $q.notify({
      type: 'warning',
      message: 'Page not found',
      timeout: 3000,
    })
    return
  }

  // Check if page is a template
  if (page.isTemplate) {
    $q.notify({
      type: 'warning',
      message: 'Templates cannot be translated',
      timeout: 3000,
    })
    return
  }

  translatePageRef.value = page
  translatePanelRef.value = panel
  showTranslateDialogRef.value = true
}

async function confirmTranslate() {
  const page = translatePageRef.value
  if (!page) {
    return
  }

  showTranslateDialogRef.value = false

  logToConsole('INFO', `Starting translation for page: ${page.title} (ID: ${page.id})`)

  const target = catalogueStore.targets.find((t) => t.id === selectedTarget.value)
  if (!target) {
    logToConsole('ERROR', 'No wiki target selected')
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

  logToConsole('DEBUG', 'Creating TranslationJob', {
    pageId: page.id,
    pageTitle: page.title,
    targetRef,
    namespace,
    languageTag,
  })

  try {
    const result = await jobStore.submitJob({
      namespace,
      targetRef,
      pageId: page.id,
      pipeline: 'TektonJob',
      languageTag,
      pageTitle: page.title,
    })

    const jobName = result?.name || result?.data?.name || 'unknown'
    logToConsole('INFO', `TranslationJob created successfully`, {
      jobName,
      pageId: page.id,
      pageTitle: page.title,
    })

    $q.notify({
      type: 'positive',
      message: `Translation Scheduled: ${jobName}`,
      timeout: 5000,
      actions: [{ icon: 'close', color: 'white' }],
    })
  } catch (err) {
    logToConsole('ERROR', `Failed to create TranslationJob`, {
      error: err.message,
      pageId: page.id,
      pageTitle: page.title,
    })
    $q.notify({
      type: 'negative',
      message: `Failed to schedule translation: ${err.message || 'Unknown error'}`,
      timeout: 5000,
    })
  }

  translatePageRef.value = null
  translatePanelRef.value = null
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
  display: flex;
  flex-wrap: nowrap;
  align-items: stretch;
}

.author-panel-wrapper {
  display: flex;
  flex-direction: column;
  flex: 1 1 50%;
  min-width: 0;
}

.author-panel {
  display: flex;
  flex-direction: column;
  min-height: 600px;
  height: 100%;
  width: 100%;
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

