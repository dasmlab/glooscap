<template>
  <q-page padding class="settings-page">
    <q-card flat bordered class="q-pa-lg">
      <q-card-section>
        <div class="text-h5 text-primary">{{ $t('settings.title') }}</div>
        <div class="text-subtitle2 text-grey-7">
          {{ $t('settings.subtitle') }}
        </div>
      </q-card-section>

      <q-separator />

      <q-tabs v-model="tab" class="text-grey" active-color="primary" indicator-color="primary" align="justify">
        <q-tab name="general" :label="$t('settings.tabs.general')" />
        <q-tab name="translation" :label="$t('settings.tabs.translation')" />
        <q-tab name="wikitargets" :label="$t('settings.tabs.wikitargets')" />
      </q-tabs>

      <q-separator />

      <q-tab-panels v-model="tab" animated>
        <!-- General Settings Tab -->
        <q-tab-panel name="general">
          <q-form @submit.prevent="save">
            <div class="row q-col-gutter-md">
              <div class="col-12">
                <q-checkbox
                  v-model="form.remoteWikiTarget"
                  label="Remote Wiki Target"
                  color="primary"
                />
              </div>
            </div>
            
            <div class="row q-col-gutter-md q-mt-md">
              <div class="col-12 col-md-4">
                <q-input
                  v-model="form.destinationTarget"
                  :label="$t('settings.destinationTarget')"
                  outlined
                  dense
                  :disable="!form.remoteWikiTarget"
                />
              </div>
              <div class="col-12 col-md-4" v-if="!form.remoteWikiTarget">
                <q-input
                  v-model="form.pathPrefix"
                  :label="$t('settings.pathPrefix')"
                  outlined
                  dense
                  :hint="$t('settings.pathPrefixHint')"
                />
              </div>
              <div class="col-12 col-md-4">
                <q-input
                  v-model="form.defaultLanguage"
                  :label="$t('settings.languageTag')"
                  outlined
                  dense
                />
              </div>
            </div>

            <div class="row q-col-gutter-md q-mt-md">
              <div class="col-12 col-md-6">
                <q-item>
                  <q-item-section avatar>
                    <q-icon
                      :name="telemetryStatusIcon"
                      :color="telemetryStatusColor"
                      size="md"
                    />
                  </q-item-section>
                  <q-item-section>
                    <q-input
                      v-model="otlpEndpoint"
                      :label="$t('settings.telemetryEndpoint')"
                      outlined
                      dense
                      disable
                      :hint="$t('settings.telemetryHint')"
                    />
                  </q-item-section>
                </q-item>
              </div>
              <div class="col-12 col-md-6">
                <q-input
                  v-model="securityBadge"
                  :label="$t('settings.securityBanner')"
                  outlined
                  dense
                  disable
                />
              </div>
            </div>

            <div class="row justify-end q-gutter-sm q-mt-lg">
              <q-btn :label="$t('common.cancel')" color="white" text-color="primary" outline @click="reset" />
              <q-btn :label="$t('common.save')" color="primary" type="submit" />
            </div>
          </q-form>
        </q-tab-panel>

        <!-- Translation Service Configuration Tab -->
        <q-tab-panel name="translation">
          <div class="row q-col-gutter-md">
            <!-- Current Status -->
            <div class="col-12">
              <q-card flat bordered>
                <q-card-section>
                  <div class="text-subtitle1 q-mb-md">Translation Service Status</div>
                  <q-item>
                    <q-item-section avatar>
                      <q-icon
                        :name="nanabushStatusIcon"
                        :color="nanabushStatusColor"
                        size="md"
                      />
                    </q-item-section>
                    <q-item-section>
                      <q-item-label class="text-weight-medium">
                        Translation Service Connection
                      </q-item-label>
                      <q-item-label caption>
                        {{ nanabushStatusText }}
                      </q-item-label>
                    </q-item-section>
                    <q-item-section side v-if="nanabushStatus.clientId">
                      <q-item-label caption class="text-grey-6">
                        ID: {{ nanabushStatus.clientId }}
                      </q-item-label>
                    </q-item-section>
                  </q-item>
                </q-card-section>
              </q-card>
            </div>

            <!-- Configuration Form -->
            <div class="col-12">
              <q-card flat bordered>
                <q-card-section>
                  <div class="text-subtitle1 q-mb-md">Configure Translation Service</div>
                  
                  <!-- Current Active Configuration Display -->
                  <q-banner v-if="translationServiceConfig.address" class="bg-blue-1 q-mb-md">
                    <template #avatar>
                      <q-icon name="info" color="primary" />
                    </template>
                    <div class="text-weight-medium">Current Active Configuration</div>
                    <div class="text-body2">
                      <strong>Address:</strong> {{ translationServiceConfig.address }}<br>
                      <strong>Type:</strong> {{ translationServiceConfig.type }}<br>
                      <strong>TLS:</strong> {{ translationServiceConfig.secure ? 'Enabled' : 'Disabled' }}
                    </div>
                  </q-banner>
                  
                  <q-form @submit.prevent="saveTranslationService">
                    <div class="row q-col-gutter-md">
                      <div class="col-12 col-md-6">
                        <q-input
                          v-model="translationServiceConfig.address"
                          label="Service Address"
                          hint="gRPC address (e.g., iskoces-service.iskoces.svc:50051)"
                          outlined
                          dense
                          :rules="[val => !!val || 'Address is required']"
                        />
                      </div>
                      <div class="col-12 col-md-3">
                        <q-select
                          v-model="translationServiceConfig.type"
                          :options="translationServiceTypes"
                          label="Service Type"
                          outlined
                          dense
                          emit-value
                          map-options
                        />
                      </div>
                      <div class="col-12 col-md-3">
                        <q-checkbox
                          v-model="translationServiceConfig.secure"
                          label="Use TLS/mTLS"
                        />
                      </div>
                    </div>
                    <div class="row q-gutter-sm q-mt-md">
                      <q-btn
                        type="submit"
                        color="primary"
                        :label="translationServiceConfig.address ? 'Update Configuration' : 'Set Configuration'"
                        :loading="savingTranslationService"
                      />
                      <q-btn
                        v-if="translationServiceConfig.address"
                        color="negative"
                        outline
                        label="Clear Configuration"
                        @click="clearTranslationService"
                        :loading="savingTranslationService"
                      />
                    </div>
                  </q-form>
                </q-card-section>
              </q-card>
            </div>
          </div>
        </q-tab-panel>

        <!-- WikiTargets Management Tab -->
        <q-tab-panel name="wikitargets">
          <div class="row q-col-gutter-md">
            <!-- WikiTargets List -->
            <div class="col-12">
              <q-card flat bordered>
                <q-card-section>
                  <div class="row items-center q-mb-md">
                    <div class="col">
                      <div class="text-subtitle1">WikiTargets</div>
                    </div>
                    <div class="col-auto">
                      <q-btn
                        color="primary"
                        icon="add"
                        label="Add WikiTarget"
                        @click="showWikiTargetDialog = true; editingWikiTarget = null"
                      />
                    </div>
                  </div>

                  <q-table
                    :rows="wikiTargets"
                    :columns="wikiTargetColumns"
                    row-key="name"
                    :loading="loadingWikiTargets"
                    flat
                    bordered
                  >
                    <template v-slot:body-cell-actions="props">
                      <q-td :props="props">
                        <q-btn
                          flat
                          dense
                          round
                          icon="edit"
                          color="primary"
                          @click="editWikiTarget(props.row)"
                        />
                        <q-btn
                          flat
                          dense
                          round
                          icon="delete"
                          color="negative"
                          @click="deleteWikiTarget(props.row)"
                        />
                      </q-td>
                    </template>
                  </q-table>
                </q-card-section>
              </q-card>
            </div>
          </div>
        </q-tab-panel>
      </q-tab-panels>
    </q-card>

    <!-- WikiTarget Dialog -->
    <q-dialog v-model="showWikiTargetDialog" persistent>
      <q-card style="min-width: 500px">
        <q-card-section>
          <div class="text-h6">{{ editingWikiTarget ? 'Edit WikiTarget' : 'Add WikiTarget' }}</div>
        </q-card-section>

        <q-card-section>
          <q-form @submit.prevent="saveWikiTarget">
            <q-input
              v-model="wikiTargetForm.name"
              label="Name"
              outlined
              dense
              :rules="[val => !!val || 'Name is required']"
              :disable="!!editingWikiTarget"
            />
            <q-input
              v-model="wikiTargetForm.namespace"
              label="Namespace"
              outlined
              dense
              hint="Default: glooscap-system"
              class="q-mt-md"
            />
            <q-select
              v-model="wikiTargetForm.spec.wikiType"
              :options="wikiTypeOptions"
              label="Wiki Type"
              outlined
              dense
              emit-value
              map-options
              :rules="[val => !!val || 'Wiki type is required']"
              class="q-mt-md"
            />
            <q-input
              v-model="wikiTargetForm.spec.uri"
              label="Wiki URI"
              outlined
              dense
              :rules="[val => !!val || 'URI is required']"
              class="q-mt-md"
            />
            <q-input
              v-model="wikiTargetForm.spec.serviceAccountSecretRef.name"
              label="Secret Name"
              outlined
              dense
              :rules="[val => !!val || 'Secret name is required']"
              class="q-mt-md"
            />
            <q-input
              v-model="wikiTargetForm.spec.serviceAccountSecretRef.key"
              label="Secret Key"
              outlined
              dense
              hint="Default: token"
              class="q-mt-md"
            />
            <q-select
              v-model="wikiTargetForm.spec.mode"
              :options="wikiTargetModes"
              label="Mode"
              outlined
              dense
              emit-value
              map-options
              :rules="[val => !!val || 'Mode is required']"
              class="q-mt-md"
            />
            <div class="row q-gutter-sm q-mt-lg">
              <q-btn
                type="submit"
                color="primary"
                :label="editingWikiTarget ? 'Update' : 'Create'"
                :loading="savingWikiTarget"
              />
              <q-btn
                color="white"
                text-color="primary"
                outline
                label="Cancel"
                @click="showWikiTargetDialog = false"
              />
            </div>
          </q-form>
        </q-card-section>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { reactive, ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useSettingsStore } from 'src/stores/settings-store'
import api from 'src/services/api'

const { t } = useI18n()
const $q = useQuasar()
const settingsStore = useSettingsStore()

// Get console ref from parent layout for logging
const consoleRef = inject('console', null)

// Tab management
const tab = ref('general')

// General settings form
const form = reactive({
  remoteWikiTarget: settingsStore.remoteWikiTarget || false,
  destinationTarget: settingsStore.destinationTarget,
  pathPrefix: settingsStore.pathPrefix,
  defaultLanguage: settingsStore.defaultLanguage,
})

const otlpEndpoint = ref('otel-collector.glooscap.svc:4317')
const securityBadge = computed(() => t('app.securityBadge'))

// Telemetry endpoint status
const telemetryStatus = ref({
  connected: false,
  status: 'unknown',
})

// Computed properties for telemetry status display
const telemetryStatusIcon = computed(() => {
  switch (telemetryStatus.value.status) {
    case 'connected':
      return 'fiber_manual_record'
    case 'error':
      return 'fiber_manual_record'
    default:
      return 'help_outline'
  }
})

const telemetryStatusColor = computed(() => {
  switch (telemetryStatus.value.status) {
    case 'connected':
      return 'positive'
    case 'error':
      return 'negative'
    default:
      return 'grey'
  }
})

// Fetch telemetry endpoint status
async function fetchTelemetryStatus() {
  try {
    // Try to ping the telemetry endpoint or check if it's reachable
    // For now, we'll assume it's connected if the endpoint is configured
    if (otlpEndpoint.value) {
      telemetryStatus.value = {
        connected: true,
        status: 'connected',
      }
    } else {
      telemetryStatus.value = {
        connected: false,
        status: 'error',
      }
    }
  } catch (error) {
    logToConsole('WARN', 'Failed to check telemetry endpoint status', error.message)
    telemetryStatus.value = {
      connected: false,
      status: 'error',
    }
  }
}

// Translation Service Configuration
const translationServiceConfig = reactive({
  address: '',
  type: 'nanabush',
  secure: false,
})

const translationServiceTypes = [
  { label: 'Nanabush', value: 'nanabush' },
  { label: 'Iskoces', value: 'iskoces' },
]

const savingTranslationService = ref(false)

// Nanabush connection status
const nanabushStatus = ref({
  connected: false,
  registered: false,
  clientId: '',
  status: 'error',
  missedHeartbeats: 0,
  lastHeartbeat: null,
})

// WikiTargets Management
const wikiTargets = ref([])
const loadingWikiTargets = ref(false)
const showWikiTargetDialog = ref(false)
const editingWikiTarget = ref(null)
const savingWikiTarget = ref(false)

const wikiTargetForm = reactive({
  name: '',
  namespace: 'glooscap-system',
  spec: {
    wikiType: 'outline',
    uri: '',
    serviceAccountSecretRef: {
      name: '',
      key: 'token',
    },
    mode: 'ReadOnly',
  },
})

const wikiTypeOptions = [
  { label: 'Outline', value: 'outline' },
  { label: 'Confluence', value: 'confluence' },
]

const wikiTargetModes = [
  { label: 'Read Only', value: 'ReadOnly' },
  { label: 'Read Write', value: 'ReadWrite' },
  { label: 'Push Only', value: 'PushOnly' },
]

const wikiTargetColumns = [
  { name: 'name', label: 'Name', field: 'name', align: 'left' },
  { name: 'namespace', label: 'Namespace', field: 'namespace', align: 'left' },
  { name: 'wikiType', label: 'Wiki Type', field: 'wikiType', align: 'left' },
  { name: 'uri', label: 'URI', field: 'uri', align: 'left' },
  { name: 'mode', label: 'Mode', field: 'mode', align: 'left' },
  { name: 'actions', label: 'Actions', field: 'actions', align: 'right' },
]

// Log to console component if available
function logToConsole(level, message, data = null) {
  if (consoleRef && typeof consoleRef.addLog === 'function') {
    try {
      consoleRef.addLog(level, message, data)
    } catch (err) {
      console.error('Failed to log to console:', err)
    }
  }
  if (level === 'ERROR') {
    console.error(`[${level}]`, message, data || '')
  } else if (level === 'WARN') {
    console.warn(`[${level}]`, message, data || '')
  } else {
    console.log(`[${level}]`, message, data || '')
  }
}

// Fetch translation service configuration
async function fetchTranslationServiceConfig() {
  try {
    const response = await api.get('/translation-service')
    if (response.data && response.data.address) {
      translationServiceConfig.address = response.data.address || ''
      translationServiceConfig.type = response.data.type || 'nanabush'
      translationServiceConfig.secure = response.data.secure || false
    }
  } catch (error) {
    logToConsole('WARN', 'Failed to fetch translation service config', error.message)
  }
}

// Save translation service configuration
async function saveTranslationService() {
  savingTranslationService.value = true
  
  // Show loading notification
  const loadingNotify = $q.notify({
    type: 'info',
    message: 'Updating translation service configuration...',
    timeout: 0,
    position: 'top',
    spinner: true,
  })
  
  try {
    const config = {
      address: translationServiceConfig.address,
      type: translationServiceConfig.type,
      secure: translationServiceConfig.secure,
    }
    
    const response = await api.post('/translation-service', config)
    logToConsole('INFO', 'Translation service configured', response.data)
    
    loadingNotify({
      type: 'positive',
      message: 'Translation service configuration saved and applied',
      timeout: 3000,
    })
    
    // Refresh status after a short delay
    setTimeout(() => {
      fetchNanabushStatus()
      fetchTranslationServiceConfig() // Refresh to show updated config
    }, 1000)
  } catch (error) {
    logToConsole('ERROR', 'Failed to save translation service config', error.message)
    loadingNotify({
      type: 'negative',
      message: `Failed to save configuration: ${error.response?.data?.message || error.message}`,
      timeout: 5000,
    })
  } finally {
    savingTranslationService.value = false
  }
}

// Clear translation service configuration
async function clearTranslationService() {
  $q.dialog({
    title: 'Confirm',
    message: 'Are you sure you want to clear the translation service configuration?',
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    savingTranslationService.value = true
    try {
      await api.delete('/translation-service')
      translationServiceConfig.address = ''
      translationServiceConfig.type = 'nanabush'
      translationServiceConfig.secure = false
      logToConsole('INFO', 'Translation service configuration cleared')
      $q.notify({
        type: 'positive',
        message: 'Translation service configuration cleared',
      })
      fetchNanabushStatus()
    } catch (error) {
      logToConsole('ERROR', 'Failed to clear translation service config', error.message)
      $q.notify({
        type: 'negative',
        message: `Failed to clear configuration: ${error.message}`,
      })
    } finally {
      savingTranslationService.value = false
    }
  })
}

// Fetch nanabush connection status
async function fetchNanabushStatus() {
  try {
    const response = await api.get('/status/nanabush')
    const status = response.data
    nanabushStatus.value = status
    
    logToConsole('INFO', `Translation service status: ${status.status}`, {
      connected: status.connected,
      registered: status.registered,
      clientId: status.clientId,
    })
  } catch (error) {
    logToConsole('ERROR', 'Failed to fetch translation service status', error.message)
    nanabushStatus.value = {
      connected: false,
      registered: false,
      status: 'error',
    }
  }
}

// Computed properties for status display
const nanabushStatusIcon = computed(() => {
  switch (nanabushStatus.value.status) {
    case 'healthy':
      return 'fiber_manual_record'
    case 'warning':
      return 'fiber_manual_record'
    case 'error':
      return 'fiber_manual_record'
    default:
      return 'help_outline'
  }
})

const nanabushStatusColor = computed(() => {
  switch (nanabushStatus.value.status) {
    case 'healthy':
      return 'positive'
    case 'warning':
      return 'warning'
    case 'error':
      return 'negative'
    default:
      return 'grey'
  }
})

const nanabushStatusText = computed(() => {
  if (!nanabushStatus.value.connected) {
    return 'Disconnected'
  }
  if (!nanabushStatus.value.registered) {
    return 'Not Registered'
  }
  switch (nanabushStatus.value.status) {
    case 'healthy':
      return 'Connected and Healthy'
    case 'warning':
      return `Warning (${nanabushStatus.value.missedHeartbeats} missed heartbeats)`
    case 'error':
      return `Error (${nanabushStatus.value.missedHeartbeats} missed heartbeats)`
    default:
      return 'Unknown Status'
  }
})

// Fetch WikiTargets
async function fetchWikiTargets() {
  loadingWikiTargets.value = true
  try {
    const response = await api.get('/wikitargets')
    wikiTargets.value = (response.data.items || []).map(item => ({
      name: item.name,
      namespace: item.namespace,
      uri: item.uri,
      mode: item.mode,
      wikiType: item.wikiType || 'outline',
    }))
  } catch (error) {
    logToConsole('ERROR', 'Failed to fetch WikiTargets', error.message)
    $q.notify({
      type: 'negative',
      message: `Failed to fetch WikiTargets: ${error.message}`,
    })
  } finally {
    loadingWikiTargets.value = false
  }
}

// Edit WikiTarget
function editWikiTarget(target) {
  editingWikiTarget.value = target
  wikiTargetForm.name = target.name
  wikiTargetForm.namespace = target.namespace || 'glooscap-system'
  // Note: We'd need to fetch the full spec to edit properly
  // For now, just open the dialog with basic info
  showWikiTargetDialog.value = true
}

// Delete WikiTarget
async function deleteWikiTarget(target) {
  $q.dialog({
    title: 'Confirm Delete',
    message: `Are you sure you want to delete WikiTarget "${target.name}"?`,
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    try {
      await api.delete(`/wikitargets/${target.namespace}/${target.name}`)
      logToConsole('INFO', 'WikiTarget deleted', target.name)
      $q.notify({
        type: 'positive',
        message: 'WikiTarget deleted successfully',
      })
      fetchWikiTargets()
    } catch (error) {
      logToConsole('ERROR', 'Failed to delete WikiTarget', error.message)
      $q.notify({
        type: 'negative',
        message: `Failed to delete WikiTarget: ${error.message}`,
      })
    }
  })
}

// Save WikiTarget
async function saveWikiTarget() {
  savingWikiTarget.value = true
  
  // Show loading notification
  const loadingNotify = $q.notify({
    type: 'info',
    message: editingWikiTarget.value ? 'Updating WikiTarget...' : 'Creating WikiTarget...',
    timeout: 0,
    position: 'top',
    spinner: true,
  })
  
  try {
    const payload = {
      metadata: {
        name: wikiTargetForm.name,
        namespace: wikiTargetForm.namespace || 'glooscap-system',
      },
      spec: {
        wikiType: wikiTargetForm.spec.wikiType || 'outline',
        uri: wikiTargetForm.spec.uri,
        serviceAccountSecretRef: {
          name: wikiTargetForm.spec.serviceAccountSecretRef.name,
          key: wikiTargetForm.spec.serviceAccountSecretRef.key || 'token',
        },
        mode: wikiTargetForm.spec.mode,
      },
    }

    if (editingWikiTarget.value) {
      // Update existing
      await api.put(`/wikitargets/${wikiTargetForm.namespace}/${wikiTargetForm.name}`, payload)
      logToConsole('INFO', 'WikiTarget updated', payload.metadata.name)
      loadingNotify({
        type: 'positive',
        message: 'WikiTarget updated successfully',
        timeout: 3000,
      })
    } else {
      // Create new
      await api.post('/wikitargets', payload)
      logToConsole('INFO', 'WikiTarget created', payload.metadata.name)
      loadingNotify({
        type: 'positive',
        message: 'WikiTarget created successfully',
        timeout: 3000,
      })
    }

    showWikiTargetDialog.value = false
    editingWikiTarget.value = null
    // Reset form
    wikiTargetForm.name = ''
    wikiTargetForm.namespace = 'glooscap-system'
    wikiTargetForm.spec.wikiType = 'outline'
    wikiTargetForm.spec.uri = ''
    wikiTargetForm.spec.serviceAccountSecretRef.name = ''
    wikiTargetForm.spec.serviceAccountSecretRef.key = 'token'
    wikiTargetForm.spec.mode = 'ReadOnly'
    
    fetchWikiTargets()
  } catch (error) {
    logToConsole('ERROR', 'Failed to save WikiTarget', error.message)
    loadingNotify({
      type: 'negative',
      message: `Failed to save WikiTarget: ${error.response?.data?.message || error.message}`,
      timeout: 5000,
    })
  } finally {
    savingWikiTarget.value = false
  }
}

// Listen for nanabush status events from SSE
function handleNanabushStatusEvent(event) {
  const status = event.detail
  nanabushStatus.value = {
    connected: status.connected || false,
    registered: status.registered || false,
    clientId: status.clientId || '',
    status: status.status || 'error',
    missedHeartbeats: status.missedHeartbeats || 0,
    lastHeartbeat: status.lastHeartbeat || null,
  }
}

// Lifecycle hooks
onMounted(() => {
  fetchNanabushStatus()
  fetchTranslationServiceConfig()
  fetchWikiTargets()
  fetchTelemetryStatus()
  window.addEventListener('nanabush-status', handleNanabushStatusEvent)
})

onUnmounted(() => {
  window.removeEventListener('nanabush-status', handleNanabushStatusEvent)
})

function reset() {
  form.remoteWikiTarget = settingsStore.remoteWikiTarget || false
  form.destinationTarget = settingsStore.destinationTarget
  form.pathPrefix = settingsStore.pathPrefix
  form.defaultLanguage = settingsStore.defaultLanguage
  $q.notify({ type: 'info', message: t('common.cancel') })
}

function save() {
  settingsStore.updateSettings({
    remoteWikiTarget: form.remoteWikiTarget,
    destinationTarget: form.destinationTarget,
    pathPrefix: form.remoteWikiTarget ? '' : form.pathPrefix, // Clear path prefix if remote
    defaultLanguage: form.defaultLanguage,
  })
  $q.notify({ type: 'positive', message: t('common.success') })
}
</script>

<style scoped>
.settings-page {
  max-width: 1200px;
  margin: 0 auto;
}
</style>
