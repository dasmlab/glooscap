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

      <q-card-section>
        <q-form @submit.prevent="save">
          <div class="row q-col-gutter-md">
            <div class="col-12 col-md-4">
              <q-input
                v-model="form.destinationTarget"
                :label="$t('settings.destinationTarget')"
                outlined
                dense
              />
            </div>
            <div class="col-12 col-md-4">
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
              <q-input
                v-model="otlpEndpoint"
                :label="$t('settings.telemetryEndpoint')"
                outlined
                dense
                disable
                :hint="$t('settings.telemetryHint')"
              />
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
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup>
import { reactive, ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useSettingsStore } from 'src/stores/settings-store'

const { t } = useI18n()
const $q = useQuasar()
const settingsStore = useSettingsStore()

const form = reactive({
  destinationTarget: settingsStore.destinationTarget,
  pathPrefix: settingsStore.pathPrefix,
  defaultLanguage: settingsStore.defaultLanguage,
})

const otlpEndpoint = ref('otel-collector.glooscap.svc:4317')
const securityBadge = computed(() => t('app.securityBadge'))

function reset() {
  form.destinationTarget = settingsStore.destinationTarget
  form.pathPrefix = settingsStore.pathPrefix
  form.defaultLanguage = settingsStore.defaultLanguage
  $q.notify({ type: 'info', message: t('common.cancel') })
}

function save() {
  settingsStore.updateSettings({
    destinationTarget: form.destinationTarget,
    pathPrefix: form.pathPrefix,
    defaultLanguage: form.defaultLanguage,
  })
  $q.notify({ type: 'positive', message: t('common.success') })
}
</script>

<style scoped>
.settings-page {
  max-width: 960px;
  margin: 0 auto;
}
</style>

