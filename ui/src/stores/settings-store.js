import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useSettingsStore = defineStore('settings', () => {
  const defaultLanguage = ref('fr-CA')
  const destinationTarget = ref('outline-fr')
  const pathPrefix = ref('/fr')
  const securityBadge = ref('Data contained on-cluster')

  function updateSettings(partial) {
    if (partial.defaultLanguage) defaultLanguage.value = partial.defaultLanguage
    if (partial.destinationTarget) destinationTarget.value = partial.destinationTarget
    if (partial.pathPrefix !== undefined) pathPrefix.value = partial.pathPrefix
  }

  return {
    defaultLanguage,
    destinationTarget,
    pathPrefix,
    securityBadge,
    updateSettings,
  }
})

