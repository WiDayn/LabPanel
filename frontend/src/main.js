import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Login from './views/Login.vue'
import Dashboard from './views/Dashboard.vue'
import Lxc from './views/Lxc.vue'
import HostMonitor from './views/HostMonitor.vue'
import LxcMonitor from './views/LxcMonitor.vue'
import GpuMonitor from './views/GpuMonitor.vue'
import Document from './views/Document.vue'
import SystemSettings from './views/SystemSettings.vue'
import { loadPublicConfig } from './utils/publicConfig'
import './style.css'

const routes = [
  { path: '/', redirect: '/overview' },
  { path: '/login', name: 'Login', component: Login },
  { path: '/dashboard', name: 'Dashboard', component: Dashboard, meta: { requiresAuth: true } },
  { path: '/lxc', name: 'Lxc', component: Lxc, meta: { requiresAuth: true } },
  { path: '/overview', name: 'HostMonitor', component: HostMonitor, meta: { requiresAuth: true } },
  { path: '/lxc-monitor', name: 'LxcMonitor', component: LxcMonitor, meta: { requiresAuth: true } },
  { path: '/gpu-monitor', name: 'GpuMonitor', component: GpuMonitor, meta: { requiresAuth: true } },
  { path: '/document', name: 'Document', component: Document, meta: { requiresAuth: true } },
  { path: '/settings', name: 'SystemSettings', component: SystemSettings, meta: { requiresAuth: true } },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    next('/login')
  } else {
    next()
  }
})

loadPublicConfig().finally(() => {
  createApp(App).use(router).mount('#app')
})
