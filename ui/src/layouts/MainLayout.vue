<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated>
      <q-toolbar>
        <q-btn flat dense round icon="menu" aria-label="Menu" @click="toggleLeftDrawer" />

        <q-toolbar-title> {{ $t('app.title') }} </q-toolbar-title>
        <div class="row items-center q-gutter-xs">
          <q-chip square dense color="primary" text-color="white">
            vLLM air-gap compliant
          </q-chip>
          <q-chip square dense color="grey-7" text-color="white" size="sm">
            {{ buildInfo }}
          </q-chip>
          <q-select
            v-model="currentLocale"
            :options="localeOptions"
            dense
            outlined
            emit-value
            map-options
            style="min-width: 120px;"
            class="q-mr-xs"
          >
            <template #option="scope">
              <q-item v-bind="scope.itemProps">
                <q-item-section avatar>
                  <q-icon :name="scope.opt.icon" />
                </q-item-section>
                <q-item-section>
                  <q-item-label>{{ scope.opt.label }}</q-item-label>
                </q-item-section>
              </q-item>
            </template>
          </q-select>
          <q-btn
            flat
            dense
            round
            :icon="consoleOpen ? 'keyboard_arrow_down' : 'keyboard_arrow_up'"
            @click="consoleOpen = !consoleOpen"
            :title="$t('console.title')"
          />
        </div>
      </q-toolbar>
    </q-header>

    <q-drawer v-model="leftDrawerOpen" show-if-above bordered>
      <q-list>
        <q-item-label header> {{ $t('navigation.catalogue') }} </q-item-label>
        <q-item
          v-for="item in navItems"
          :key="item.to.name"
          clickable
          v-ripple
          :to="item.to"
          exact
          active-class="text-primary bg-grey-2"
        >
          <q-item-section avatar>
            <q-icon :name="item.icon" />
          </q-item-section>
          <q-item-section>
            <q-item-label>{{ item.label }}</q-item-label>
            <q-item-label caption>{{ item.caption }}</q-item-label>
          </q-item-section>
        </q-item>
      </q-list>
    </q-drawer>

    <q-page-container>
      <router-view />
    </q-page-container>
    
    <ConsoleLog v-model="consoleOpen" ref="consoleRef" />
  </q-layout>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted, provide } from 'vue'
import { useI18n } from 'vue-i18n'
import ConsoleLog from 'src/components/ConsoleLog.vue'
import { useCatalogueStore } from 'src/stores/catalogue-store'
import { useSettingsStore } from 'src/stores/settings-store'

const { t } = useI18n()
const leftDrawerOpen = ref(false)
const consoleOpen = ref(true) // Show console by default for debugging
const consoleRef = ref(null)
const catalogueStore = useCatalogueStore()
const settingsStore = useSettingsStore()

// Provide console ref to child components
provide('console', consoleRef)

// Build version info
const buildVersion = import.meta.env.VITE_BUILD_VERSION || 'dev'
const buildNumber = import.meta.env.VITE_BUILD_NUMBER || ''
const buildSha = import.meta.env.VITE_BUILD_SHA || ''

const buildInfo = computed(() => {
  if (buildVersion && buildVersion !== 'dev') {
    // Show compact version: "v0.2.1-a1b2c3" or "v0.2.1" if no SHA
    // Extract version number from "0.2.1-alpha" -> "0.2.1"
    const versionNum = buildVersion.replace(/-alpha$/, '')
    if (buildSha && buildSha !== 'unknown') {
      return `v${versionNum}-${buildSha}`
    }
    return `v${versionNum}`
  }
  // Fallback: show build number or dev
  return buildNumber ? `#${buildNumber}` : 'dev'
})

const navItems = computed(() => [
  {
    label: t('navigation.catalogue'),
    caption: t('navigation.catalogueDesc'),
    icon: 'travel_explore',
    to: { name: 'catalogue' },
  },
  {
    label: t('navigation.author'),
    caption: t('navigation.authorDesc'),
    icon: 'edit_document',
    to: { name: 'author' },
  },
  {
    label: t('navigation.jobs'),
    caption: t('navigation.jobsDesc'),
    icon: 'list_alt',
    to: { name: 'jobs' },
  },
  {
    label: t('navigation.settings'),
    caption: t('navigation.settingsDesc'),
    icon: 'tune',
    to: { name: 'settings' },
  },
])

const localeOptions = [
  { label: 'English', value: 'en-US', icon: 'language' },
  { label: 'Français', value: 'fr-CA', icon: 'language' },
]

const currentLocale = computed({
  get: () => settingsStore.uiLocale,
  set: (value) => settingsStore.setUILocale(value),
})

function toggleLeftDrawer() {
  leftDrawerOpen.value = !leftDrawerOpen.value
}

function logToConsole(level, message, data = null) {
  // Also log to browser console for debugging
  if (level === 'ERROR') {
    console.error(`[${level}]`, message, data || '')
  } else if (level === 'WARN') {
    console.warn(`[${level}]`, message, data || '')
  } else if (level === 'DEBUG') {
    console.debug(`[${level}]`, message, data || '')
  } else {
    console.log(`[${level}]`, message, data || '')
  }
  
  // Log to console component if available
  if (consoleRef.value && typeof consoleRef.value.addLog === 'function') {
    try {
      consoleRef.value.addLog(level, message, data)
    } catch (err) {
      console.error('Failed to log to console component:', err)
    }
  }
}

onMounted(() => {
  // Subscribe to SSE events with logging
  catalogueStore.subscribeToEvents(logToConsole)
  
  // Log initial connection
  logToConsole('INFO', 'UI initialized, connecting to operator API...')
})

onUnmounted(() => {
  catalogueStore.unsubscribeFromEvents()
})
</script>
