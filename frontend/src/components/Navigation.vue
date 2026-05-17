<template>
  <nav class="bg-white shadow-sm border-b">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex space-x-8">
        <router-link
          to="/dashboard"
          class="border-b-2 py-4 px-1 text-sm font-medium transition-colors"
          :class="[
            $route.path === '/dashboard' || $route.path === '/'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
          ]"
        >
          Frp 管理
        </router-link>
        <router-link
          to="/lxc"
          class="border-b-2 py-4 px-1 text-sm font-medium transition-colors"
          :class="[
            $route.path === '/lxc'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
          ]"
        >
          LXC 容器
        </router-link>
        <router-link
          to="/lxc-monitor"
          class="border-b-2 py-4 px-1 text-sm font-medium transition-colors"
          :class="[
            $route.path === '/lxc-monitor'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
          ]"
        >
          容器监控
        </router-link>
        <router-link
          to="/gpu-monitor"
          class="border-b-2 py-4 px-1 text-sm font-medium transition-colors"
          :class="[
            $route.path === '/gpu-monitor'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
          ]"
        >
          显卡监控
        </router-link>
        <router-link
          to="/document"
          class="border-b-2 py-4 px-1 text-sm font-medium transition-colors"
          :class="[
            $route.path === '/document'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
          ]"
        >
          系统文档
        </router-link>
      </div>
    </div>
  </nav>
</template>

<script setup>
</script>
