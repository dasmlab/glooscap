import { defineStore } from 'pinia'
import { ref } from 'vue'
import i18n from 'src/i18n'

export const useSettingsStore = defineStore('settings', () => {
  const defaultLanguage = ref('fr-CA')
  const destinationTarget = ref('outline-fr')
  const pathPrefix = ref('/fr')
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
  }

  return {
    defaultLanguage,
    destinationTarget,
    pathPrefix,
    uiLocale,
    updateSettings,
    setUILocale,
  }
})

