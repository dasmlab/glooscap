import { defineStore } from 'pinia'
import { ref } from 'vue'
import i18n from 'src/i18n'

export const useSettingsStore = defineStore('settings', () => {
  const defaultLanguage = ref('fr-CA')
  const destinationTarget = ref('outline-fr')
  const pathPrefix = ref('/fr')
  const remoteWikiTarget = ref(false)
  const operatorEnabled = ref(true)
  const operatorEndpoint = ref('glooscap-operator.testdev.dasmlab.org:8080')
  const telemetryEnabled = ref(true)
  const telemetryEndpoint = ref('otel-collector.glooscap-system.svc.cluster.local:4317')
  const nokomisEnabled = ref(false)
  const nokomisEndpoint = ref('nokomis-service.nokomis.svc.cluster.local:8080')
  // securityBadge is now translated via i18n, so we don't need a ref for it
  
  // UI language preference (separate from translation target language)
  const uiLocale = ref(localStorage.getItem('glooscap-locale') || 'en-US')
  
  function setUILocale(lang) {
    uiLocale.value = lang
    i18n.global.locale.value = lang
    localStorage.setItem('glooscap-locale', lang)
  }

  function updateSettings(partial) {
    if (partial.defaultLanguage) defaultLanguage.value = partial.defaultLanguage
    if (partial.destinationTarget) destinationTarget.value = partial.destinationTarget
    if (partial.pathPrefix !== undefined) pathPrefix.value = partial.pathPrefix
    if (partial.remoteWikiTarget !== undefined) remoteWikiTarget.value = partial.remoteWikiTarget
    if (partial.operatorEnabled !== undefined) operatorEnabled.value = partial.operatorEnabled
    if (partial.operatorEndpoint) operatorEndpoint.value = partial.operatorEndpoint
    if (partial.telemetryEnabled !== undefined) telemetryEnabled.value = partial.telemetryEnabled
    if (partial.telemetryEndpoint) telemetryEndpoint.value = partial.telemetryEndpoint
    if (partial.nokomisEnabled !== undefined) nokomisEnabled.value = partial.nokomisEnabled
    if (partial.nokomisEndpoint) nokomisEndpoint.value = partial.nokomisEndpoint
  }

  return {
    defaultLanguage,
    destinationTarget,
    pathPrefix,
    remoteWikiTarget,
    operatorEnabled,
    operatorEndpoint,
    telemetryEnabled,
    telemetryEndpoint,
    nokomisEnabled,
    nokomisEndpoint,
    uiLocale,
    updateSettings,
    setUILocale,
  }
})

