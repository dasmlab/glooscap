<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated>
      <q-toolbar>
        <q-btn flat dense round icon="menu" aria-label="Menu" @click="toggleLeftDrawer" />

        <q-toolbar-title> Glooscap ETL Console </q-toolbar-title>
        <q-chip square dense color="primary" text-color="white">
          vLLM air-gap compliant
        </q-chip>
      </q-toolbar>
    </q-header>

    <q-drawer v-model="leftDrawerOpen" show-if-above bordered>
      <q-list>
        <q-item-label header> Navigation </q-item-label>
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
  </q-layout>
</template>

<script setup>
import { computed, ref } from 'vue'
const leftDrawerOpen = ref(false)

const navItems = computed(() => [
  {
    label: 'Catalogue',
    caption: 'Discover pages',
    icon: 'travel_explore',
    to: { name: 'catalogue' },
  },
  {
    label: 'Jobs',
    caption: 'Translation queue',
    icon: 'list_alt',
    to: { name: 'jobs' },
  },
  {
    label: 'Settings',
    caption: 'Defaults & destinations',
    icon: 'tune',
    to: { name: 'settings' },
  },
])

function toggleLeftDrawer() {
  leftDrawerOpen.value = !leftDrawerOpen.value
}
</script>
