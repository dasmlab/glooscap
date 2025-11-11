<template>
  <q-page padding class="settings-page">
    <q-card flat bordered class="q-pa-lg">
      <q-card-section>
        <div class="text-h5 text-primary">Translation Defaults</div>
        <div class="text-subtitle2 text-grey-7">
          Configure default destinations and language tags for queued jobs.
        </div>
      </q-card-section>

      <q-separator />

      <q-card-section>
        <q-form @submit.prevent="save">
          <div class="row q-col-gutter-md">
            <div class="col-12 col-md-4">
              <q-input
                v-model="form.destinationTarget"
                label="Destination Wiki Target"
                outlined
                dense
              />
            </div>
            <div class="col-12 col-md-4">
              <q-input
                v-model="form.pathPrefix"
                label="Destination Path Prefix"
                outlined
                dense
                hint="Example: /fr"
              />
            </div>
            <div class="col-12 col-md-4">
              <q-input
                v-model="form.defaultLanguage"
                label="Language Tag (BCP 47)"
                outlined
                dense
              />
            </div>
          </div>

          <div class="row q-col-gutter-md q-mt-md">
            <div class="col-12 col-md-6">
              <q-input
                v-model="otlpEndpoint"
                label="Telemetry Endpoint"
                outlined
                dense
                disable
                hint="Configured in operator deployment"
              />
            </div>
            <div class="col-12 col-md-6">
              <q-input
                v-model="securityBadge"
                label="Security Banner"
                outlined
                dense
                disable
              />
            </div>
          </div>

          <div class="row justify-end q-gutter-sm q-mt-lg">
            <q-btn label="Reset" color="white" text-color="primary" outline @click="reset" />
            <q-btn label="Save" color="primary" type="submit" />
          </div>
        </q-form>
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useQuasar } from 'quasar'
import { useSettingsStore } from 'src/stores/settings-store'

const $q = useQuasar()
const settingsStore = useSettingsStore()

const form = reactive({
  destinationTarget: settingsStore.destinationTarget,
  pathPrefix: settingsStore.pathPrefix,
  defaultLanguage: settingsStore.defaultLanguage,
})

const otlpEndpoint = ref('otel-collector.glooscap.svc:4317')
const securityBadge = ref(settingsStore.securityBadge)

function reset() {
  form.destinationTarget = settingsStore.destinationTarget
  form.pathPrefix = settingsStore.pathPrefix
  form.defaultLanguage = settingsStore.defaultLanguage
  $q.notify({ type: 'info', message: 'Changes reverted' })
}

function save() {
  settingsStore.updateSettings({
    destinationTarget: form.destinationTarget,
    pathPrefix: form.pathPrefix,
    defaultLanguage: form.defaultLanguage,
  })
  $q.notify({ type: 'positive', message: 'Defaults updated' })
}
</script>

<style scoped>
.settings-page {
  max-width: 960px;
  margin: 0 auto;
}
</style>

