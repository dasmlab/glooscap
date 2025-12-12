<template>
  <q-page padding class="nokomis-page">
    <div class="row items-center q-mb-md">
      <div class="col">
        <div class="text-h5 text-primary">{{ $t('nokomis.title') }}</div>
        <div class="text-subtitle2 text-grey-7">
          {{ $t('nokomis.subtitle') }}
        </div>
      </div>
      <div class="col-auto q-gutter-sm">
        <q-btn
          color="secondary"
          icon="folder_special"
          :label="$t('nokomis.loadTemplate')"
          @click="showTemplateDialog = true"
        />
        <q-btn
          color="primary"
          icon="add"
          :label="$t('nokomis.createStructure')"
          @click="showCreateDialog = true"
        />
      </div>
    </div>

    <!-- Connection Status -->
    <q-banner
      v-if="!connected"
      class="bg-warning text-dark q-mb-md"
    >
      <template #avatar>
        <q-icon name="warning" />
      </template>
      {{ $t('nokomis.notConnected') }}
    </q-banner>

    <!-- Canvas Area -->
    <q-card flat bordered class="nokomis-canvas">
      <q-card-section>
        <div class="text-subtitle1 q-mb-md">{{ $t('nokomis.organizationStructures') }}</div>
        
        <!-- Canvas placeholder -->
        <div v-if="structures.length === 0" class="canvas-placeholder">
          <q-icon name="account_tree" size="64px" color="grey-4" />
          <div class="text-h6 text-grey-6 q-mt-md">{{ $t('nokomis.noStructures') }}</div>
          <div class="text-body2 text-grey-6 q-mt-sm">{{ $t('nokomis.createFirstStructure') }}</div>
        </div>

        <!-- Structure Tree View -->
        <div v-else class="structure-tree">
          <q-tree
            :nodes="structureTree"
            node-key="id"
            default-expand-all
            :label-key="'label'"
            :children-key="'children'"
          >
            <template v-slot:default-header="prop">
              <div class="row items-center q-gutter-sm full-width">
                <q-icon :name="prop.node.icon || 'folder'" :color="prop.node.color || 'primary'" />
                <span class="text-weight-medium">{{ prop.node.label }}</span>
                <q-chip
                  v-if="prop.node.type"
                  size="sm"
                  :color="getTypeColor(prop.node.type)"
                  text-color="white"
                >
                  {{ prop.node.type }}
                </q-chip>
                <q-space />
                <q-btn
                  flat
                  dense
                  round
                  icon="edit"
                  size="sm"
                  color="primary"
                  @click.stop="editStructure(prop.node)"
                />
                <q-btn
                  flat
                  dense
                  round
                  icon="delete"
                  size="sm"
                  color="negative"
                  @click.stop="deleteStructure(prop.node)"
                />
              </div>
            </template>
          </q-tree>
        </div>
      </q-card-section>
    </q-card>

    <!-- Template Selection Dialog -->
    <q-dialog v-model="showTemplateDialog" persistent>
      <q-card style="min-width: 700px; max-width: 900px">
        <q-card-section>
          <div class="text-h6">{{ $t('nokomis.selectTemplate') }}</div>
        </q-card-section>

        <q-card-section>
          <q-tabs v-model="selectedTemplateCategory" align="left" class="text-grey" active-color="primary">
            <q-tab
              v-for="(category, key) in templateCategories"
              :key="key"
              :name="key"
              :label="category.name"
              :icon="category.icon"
            />
          </q-tabs>

          <q-separator />

          <q-tab-panels v-model="selectedTemplateCategory" animated class="q-mt-md">
            <q-tab-panel
              v-for="(group, categoryKey) in templatesByCategory"
              :key="categoryKey"
              :name="categoryKey"
            >
              <div class="row q-col-gutter-md">
                <div
                  v-for="template in group.templates"
                  :key="template.id"
                  class="col-12 col-md-6"
                >
                  <q-card
                    flat
                    bordered
                    class="template-card cursor-pointer"
                    @click="loadTemplate(template)"
                  >
                    <q-card-section>
                      <div class="row items-center q-mb-sm">
                        <q-icon
                          :name="group.category.icon"
                          :color="group.category.color"
                          size="md"
                          class="q-mr-sm"
                        />
                        <div class="text-h6">{{ template.name }}</div>
                      </div>
                      <div class="text-body2 text-grey-7">
                        {{ template.description }}
                      </div>
                      <q-separator class="q-my-sm" />
                      <div class="text-caption">
                        <q-chip
                          v-for="(value, key) in template.metadata"
                          :key="key"
                          size="sm"
                          dense
                          class="q-mr-xs q-mb-xs"
                        >
                          <strong>{{ key }}:</strong> {{ value }}
                        </q-chip>
                      </div>
                    </q-card-section>
                  </q-card>
                </div>
              </div>
            </q-tab-panel>
          </q-tab-panels>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn
            flat
            :label="$t('common.cancel')"
            color="grey-7"
            @click="showTemplateDialog = false"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Create/Edit Structure Dialog -->
    <q-dialog v-model="showCreateDialog" persistent>
      <q-card style="min-width: 500px">
        <q-card-section>
          <div class="text-h6">
            {{ editingStructure ? $t('nokomis.editStructure') : $t('nokomis.createStructure') }}
          </div>
        </q-card-section>

        <q-card-section>
          <q-form @submit.prevent="saveStructure">
            <q-input
              v-model="structureForm.name"
              :label="$t('nokomis.structureName')"
              outlined
              dense
              :rules="[val => !!val || $t('nokomis.nameRequired')]"
            />
            <q-select
              v-model="structureForm.type"
              :options="structureTypes"
              :label="$t('nokomis.structureType')"
              outlined
              dense
              emit-value
              map-options
              :rules="[val => !!val || $t('nokomis.typeRequired')]"
              class="q-mt-md"
            />
            <q-select
              v-if="structures.length > 0"
              v-model="structureForm.parentId"
              :options="parentOptions"
              :label="$t('nokomis.parentStructure')"
              outlined
              dense
              emit-value
              map-options
              clearable
              hint="Optional - leave empty for root level"
              class="q-mt-md"
            />
            <q-input
              v-model="structureForm.description"
              :label="$t('nokomis.description')"
              outlined
              dense
              type="textarea"
              rows="3"
              class="q-mt-md"
            />
            <div class="row q-gutter-sm q-mt-lg">
              <q-btn
                type="submit"
                color="primary"
                :label="editingStructure ? $t('common.save') : $t('nokomis.create')"
                :loading="savingStructure"
              />
              <q-btn
                color="white"
                text-color="primary"
                outline
                :label="$t('common.cancel')"
                @click="cancelEdit"
              />
            </div>
          </q-form>
        </q-card-section>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useSettingsStore } from 'src/stores/settings-store'
import api from 'src/services/api'
import {
  structureTemplates,
  templateCategories,
  getTemplatesByCategory,
  flattenTemplateStructure,
} from 'src/stores/nokomis-templates'

const { t } = useI18n()
const $q = useQuasar()
const settingsStore = useSettingsStore()

// Connection status
const connected = ref(false)
const eventSource = ref(null)

// Structures
const structures = ref([])
const loading = ref(false)
const savingStructure = ref(false)
const showCreateDialog = ref(false)
const showTemplateDialog = ref(false)
const selectedTemplateCategory = ref('operational')
const editingStructure = ref(null)

// Templates
const templatesByCategory = computed(() => getTemplatesByCategory())

const structureForm = ref({
  name: '',
  type: '',
  parentId: null,
  description: '',
})

const structureTypes = [
  { label: 'Organization', value: 'org' },
  { label: 'Division', value: 'division' },
  { label: 'Project', value: 'project' },
  { label: 'Area', value: 'area' },
  { label: 'Product', value: 'product' },
  { label: 'Feature', value: 'feature' },
  { label: 'Ops', value: 'ops' },
  { label: 'Dev', value: 'dev' },
]

// Convert flat structures to tree format
const structureTree = computed(() => {
  const buildTree = (items, parentId = null) => {
    return items
      .filter(item => item.parentId === parentId)
      .map(item => ({
        id: item.id,
        label: item.name,
        type: item.type,
        icon: getTypeIcon(item.type),
        color: getTypeColor(item.type),
        description: item.description,
        children: buildTree(items, item.id),
      }))
  }
  return buildTree(structures.value)
})

const parentOptions = computed(() => {
  return structures.value
    .filter(s => !editingStructure.value || s.id !== editingStructure.value.id)
    .map(s => ({
      label: s.name,
      value: s.id,
      caption: s.type,
    }))
})

function getTypeIcon(type) {
  const icons = {
    org: 'business',
    division: 'corporate_fare',
    project: 'folder',
    area: 'category',
    product: 'inventory',
    feature: 'extension',
    ops: 'settings',
    dev: 'code',
  }
  return icons[type] || 'folder'
}

function getTypeColor(type) {
  const colors = {
    org: 'purple',
    division: 'indigo',
    project: 'blue',
    area: 'cyan',
    product: 'teal',
    feature: 'green',
    ops: 'orange',
    dev: 'deep-orange',
  }
  return colors[type] || 'grey'
}

// Connect to Nokomis SSE
function connectToNokomis() {
  if (!settingsStore.nokomisEnabled || !settingsStore.nokomisEndpoint) {
    connected.value = false
    return
  }

  try {
    // For now, stub the connection - will be implemented when Nokomis service is ready
    // const url = `http://${settingsStore.nokomisEndpoint}/api/v1/events`
    // eventSource.value = new EventSource(url)
    
    // eventSource.value.onopen = () => {
    //   connected.value = true
    //   console.log('[NokomisPage] Connected to Nokomis SSE')
    // }
    
    // eventSource.value.onmessage = (event) => {
    //   const data = JSON.parse(event.data)
    //   handleNokomisEvent(data)
    // }
    
    // eventSource.value.onerror = () => {
    //   connected.value = false
    //   console.error('[NokomisPage] SSE connection error')
    // }
    
    // Stub: Assume connected for now
    connected.value = true
    fetchStructures()
  } catch (error) {
    console.error('[NokomisPage] Failed to connect to Nokomis:', error)
    connected.value = false
  }
}

// Handle SSE events from Nokomis
function handleNokomisEvent(data) {
  switch (data.type) {
    case 'structure.created':
    case 'structure.updated':
      fetchStructures()
      break
    case 'structure.deleted':
      fetchStructures()
      break
    default:
      console.log('[NokomisPage] Unknown event type:', data.type)
  }
}

// Fetch structures
async function fetchStructures() {
  if (!connected.value) return
  
  loading.value = true
  try {
    // Stub: For now, use mock data
    // const response = await api.get('/nokomis/structures')
    // structures.value = response.data.items || []
    
    // Mock data for now
    structures.value = []
  } catch (error) {
    console.error('[NokomisPage] Failed to fetch structures:', error)
    $q.notify({
      type: 'negative',
      message: `Failed to fetch structures: ${error.message}`,
    })
  } finally {
    loading.value = false
  }
}

// Create/Edit structure
function editStructure(structure) {
  editingStructure.value = structure
  structureForm.value = {
    name: structure.label,
    type: structure.type,
    parentId: null, // Will need to find parent from structures
    description: structure.description || '',
  }
  showCreateDialog.value = true
}

function cancelEdit() {
  showCreateDialog.value = false
  editingStructure.value = null
  structureForm.value = {
    name: '',
    type: '',
    parentId: null,
    description: '',
  }
}

async function saveStructure() {
  savingStructure.value = true
  try {
    const payload = {
      name: structureForm.value.name,
      type: structureForm.value.type,
      parentId: structureForm.value.parentId || null,
      description: structureForm.value.description,
    }

    if (editingStructure.value) {
      // Update existing
      // await api.put(`/nokomis/structures/${editingStructure.value.id}`, payload)
      console.log('[NokomisPage] Update structure:', payload)
    } else {
      // Create new
      // await api.post('/nokomis/structures', payload)
      console.log('[NokomisPage] Create structure:', payload)
      
      // Add to local list for now (stub)
      structures.value.push({
        id: `temp-${Date.now()}`,
        ...payload,
      })
    }

    $q.notify({
      type: 'positive',
      message: editingStructure.value
        ? t('nokomis.structureUpdated')
        : t('nokomis.structureCreated'),
    })

    cancelEdit()
    fetchStructures()
  } catch (error) {
    console.error('[NokomisPage] Failed to save structure:', error)
    $q.notify({
      type: 'negative',
      message: `Failed to save structure: ${error.message}`,
    })
  } finally {
    savingStructure.value = false
  }
}

async function deleteStructure(structure) {
  $q.dialog({
    title: t('nokomis.deleteStructure'),
    message: t('nokomis.deleteConfirm', { name: structure.label }),
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    try {
      // await api.delete(`/nokomis/structures/${structure.id}`)
      console.log('[NokomisPage] Delete structure:', structure.id)
      
      // Remove from local list for now (stub)
      structures.value = structures.value.filter(s => s.id !== structure.id)
      
      $q.notify({
        type: 'positive',
        message: t('nokomis.structureDeleted'),
      })
      
      fetchStructures()
    } catch (error) {
      console.error('[NokomisPage] Failed to delete structure:', error)
      $q.notify({
        type: 'negative',
        message: `Failed to delete structure: ${error.message}`,
      })
    }
  })
}

// Load structure from template
async function loadTemplate(template) {
  $q.dialog({
    title: t('nokomis.loadTemplateConfirm'),
    message: t('nokomis.loadTemplateMessage', { name: template.name }),
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    try {
      // Flatten template structure
      const templateStructures = flattenTemplateStructure(template)
      
      // Create all structures from template
      // In real implementation, this would be a batch API call
      for (const structure of templateStructures) {
        // await api.post('/nokomis/structures', structure)
        console.log('[NokomisPage] Create structure from template:', structure)
        
        // Add to local list for now (stub)
        structures.value.push({
          ...structure,
          id: structure.id.replace('template-', `struct-${Date.now()}-`),
        })
      }
      
      $q.notify({
        type: 'positive',
        message: t('nokomis.templateLoaded', {
          name: template.name,
          count: templateStructures.length,
        }),
      })
      
      showTemplateDialog.value = false
      fetchStructures()
    } catch (error) {
      console.error('[NokomisPage] Failed to load template:', error)
      $q.notify({
        type: 'negative',
        message: `Failed to load template: ${error.message}`,
      })
    }
  })
}

onMounted(() => {
  connectToNokomis()
})

onUnmounted(() => {
  if (eventSource.value) {
    eventSource.value.close()
    eventSource.value = null
  }
})
</script>

<style scoped>
.nokomis-page {
  background: #f4f7fb;
}

.nokomis-canvas {
  min-height: 500px;
  background: white;
}

.canvas-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  padding: 48px;
}

.structure-tree {
  padding: 16px;
}
</style>

