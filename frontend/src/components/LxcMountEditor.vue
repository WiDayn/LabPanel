<script setup>
import { onMounted, ref } from 'vue'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'

const props = defineProps({ name: { type: String, required: true } })
const emit = defineEmits(['busy', 'changed'])
const endpoint = `/lxc/storage/mounts/${encodeURIComponent(props.name)}`
const mounts = ref([])
const loading = ref(false)
const saving = ref(false)
const loaded = ref(false)
const error = ref('')
const draft = ref(null)
const editing = ref(false)
const removing = ref(null)

async function loadMounts() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.get(endpoint, { timeout: 15000 })
    mounts.value = data.mounts || []
    loaded.value = true
  } catch (err) {
    loaded.value = false
    error.value = err.response?.data?.error || '无法加载目录挂载'
  } finally {
    loading.value = false
  }
}

function edit(mount) {
  removing.value = null
  error.value = ''
  editing.value = !!mount
  draft.value = mount ? { ...mount } : { name: '', source: '', path: '', readonly: true }
}

async function save() {
  const { name, source, path, readonly } = draft.value
  const payload = { name: name.trim(), source, path, readonly }
  await submit(() => editing.value
    ? api.put(`${endpoint}/${encodeURIComponent(name)}`, payload, { timeout: 45000 })
    : api.post(endpoint, payload, { timeout: 45000 }))
}

async function remove() {
  const name = removing.value.name
  await submit(() => api.delete(`${endpoint}/${encodeURIComponent(name)}`, { timeout: 45000 }))
}

async function submit(request) {
  if (saving.value) return
  saving.value = true
  emit('busy', true)
  error.value = ''
  try {
    await request()
    draft.value = null
    removing.value = null
    emit('changed')
    await loadMounts()
  } catch (err) {
    error.value = err.response?.data?.error || '无法确认操作结果，请刷新后核对'
  } finally {
    saving.value = false
    emit('busy', false)
  }
}

onMounted(loadMounts)
</script>

<template>
  <section class="space-y-3">
    <div class="flex flex-wrap justify-end gap-2">
      <Button size="sm" variant="outline" :disabled="loading || saving" @click="loadMounts">刷新</Button>
      <Button size="sm" :disabled="!loaded || loading || saving" @click="edit()">新增挂载</Button>
    </div>
    <p v-if="error" role="alert" class="break-words text-sm text-red-700">{{ error }}</p>
    <p v-if="loading" role="status" class="text-sm text-gray-600">正在读取目录挂载…</p>
    <p v-else-if="loaded && !mounts.length" class="text-sm text-gray-600">暂无目录挂载</p>
    <ul v-if="mounts.length" class="divide-y rounded-md border border-gray-200">
      <li v-for="mount in mounts" :key="mount.name" class="flex flex-wrap items-center justify-between gap-3 p-3">
        <div class="min-w-0 text-sm">
          <div class="break-all font-medium text-gray-900">{{ mount.name }} · {{ mount.readonly ? '只读' : '读写' }}</div>
          <div class="mt-1 break-all text-gray-700">{{ mount.pool ? `${mount.pool} / ` : '' }}{{ mount.source }} → {{ mount.path }}</div>
        </div>
        <div v-if="mount.editable" class="flex gap-2">
          <Button size="sm" variant="outline" :disabled="!loaded || loading || saving" @click="edit(mount)">编辑</Button>
          <Button size="sm" variant="destructive" :disabled="!loaded || loading || saving" @click="draft = null; removing = mount; error = ''">移除</Button>
        </div>
        <span v-else class="text-xs text-gray-600">{{ mount.pool ? '存储池卷' : '只可查看' }}</span>
      </li>
    </ul>
    <form v-if="draft" class="space-y-3 border-t pt-3" @submit.prevent="save">
      <fieldset :disabled="saving" class="space-y-3">
        <legend class="mb-2 text-sm font-semibold">{{ editing ? '编辑挂载' : '新增挂载' }}</legend>
        <label class="block text-sm font-medium">
          设备名
          <Input v-model="draft.name" class="mt-1" maxlength="63" :disabled="editing" required />
        </label>
        <div class="grid gap-3 sm:grid-cols-2">
          <label class="block text-sm font-medium">
            宿主机目录
            <Input v-model="draft.source" class="mt-1" placeholder="/mnt/sata0n1/Development" required />
          </label>
          <label class="block text-sm font-medium">
            容器路径
            <Input v-model="draft.path" class="mt-1" placeholder="/root/Development" required />
          </label>
        </div>
        <label class="flex items-center gap-2 text-sm"><input v-model="draft.readonly" type="checkbox" /> 只读</label>
      </fieldset>
      <div class="flex justify-end gap-2">
        <Button variant="outline" :disabled="saving" @click="draft = null">取消</Button>
        <Button type="submit" :disabled="saving || !loaded">{{ saving ? '保存中…' : '保存挂载' }}</Button>
      </div>
    </form>
    <div v-if="removing" class="space-y-3 border-t pt-3">
      <p class="break-words text-sm">移除挂载「{{ removing.name }}」？宿主机目录和文件会保留。</p>
      <div class="flex justify-end gap-2">
        <Button variant="outline" :disabled="saving" @click="removing = null">取消</Button>
        <Button variant="destructive" :disabled="saving || !loaded" @click="remove">{{ saving ? '移除中…' : '确认移除' }}</Button>
      </div>
    </div>
  </section>
</template>
