import { createI18n } from 'vue-i18n'
import enUS from './en-US.js'
import frCA from './fr-CA.js'

// Get saved language preference or default to browser language
function getDefaultLocale() {
  const saved = localStorage.getItem('glooscap-locale')
  if (saved) return saved
  
  // Try to match browser language
  const browserLang = navigator.language || navigator.userLanguage
  if (browserLang.startsWith('fr')) {
    return 'fr-CA'
  }
  return 'en-US'
}

const i18n = createI18n({
  locale: getDefaultLocale(),
  fallbackLocale: 'en-US',
  messages: {
    'en-US': enUS,
    'fr-CA': frCA,
  },
  legacy: false, // Use Composition API mode
})

export default i18n

