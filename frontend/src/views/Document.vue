<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
        <h1 class="text-xl font-semibold">系统文档</h1>
        <Button variant="outline" @click="handleLogout">退出登录</Button>
      </div>
      <Navigation />
    </header>

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div class="flex gap-6">
        <!-- 左侧文档列表 -->
        <div class="w-64 bg-white rounded-lg shadow-sm p-4">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold">文档列表</h2>
            <Button size="sm" @click="showCreateDialog = true">新增</Button>
          </div>
          <div v-if="loading" class="text-center py-4 text-gray-500">加载中...</div>
          <div v-else-if="documents.length === 0" class="text-center py-4 text-gray-500">暂无文档</div>
          <div v-else class="space-y-0">
            <div
              v-for="doc in documents"
              :key="doc.id"
              @click="selectDocument(doc.id)"
              :class="[
                'w-full px-4 py-3 cursor-pointer transition-colors border-b border-gray-200 hover:bg-gray-50 hover:border-blue-300',
                selectedDocId === doc.id ? 'bg-blue-50 border-b-2 border-blue-500' : ''
              ]"
            >
              <div class="font-medium text-sm">{{ doc.title }}</div>
            </div>
          </div>
        </div>

        <!-- 右侧文档内容 -->
        <div class="flex-1 bg-white rounded-lg shadow-sm p-6">
          <div v-if="!selectedDocument" class="text-center py-20 text-gray-500">
            请从左侧选择文档或创建新文档
          </div>
          <div v-else>
            <div class="flex items-center justify-between mb-4">
              <h2 class="text-2xl font-semibold">{{ selectedDocument.title }}</h2>
              <div class="flex gap-2">
                <Button variant="outline" size="sm" @click="handleToggleEdit" :disabled="toggling">
                  {{ editMode ? '查看' : '编辑' }}
                </Button>
                <Button variant="destructive" size="sm" @click="deleteDocument">删除</Button>
              </div>
            </div>

            <div v-if="editMode" class="space-y-4">
              <div>
                <label class="block text-sm font-medium mb-2">标题</label>
                <Input v-model="editingDoc.title" placeholder="文档标题" />
              </div>
              <div>
                <label class="block text-sm font-medium mb-2">内容 (Markdown)</label>
                <div class="flex gap-2 mb-2">
                  <Button variant="outline" size="sm" @click="insertImage">插入图片</Button>
                </div>
                <textarea
                  v-model="editingDoc.content"
                  class="w-full h-96 p-3 border rounded-md font-mono text-sm"
                  placeholder="输入Markdown内容..."
                ></textarea>
              </div>
              <div class="flex gap-2">
                <Button @click="saveDocument" :disabled="saving">
                  {{ saving ? '保存中...' : '保存' }}
                </Button>
                <Button variant="outline" @click="cancelEdit">取消</Button>
              </div>
            </div>
            <div v-else class="prose max-w-none">
              <div v-html="renderedContent"></div>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- 创建文档对话框 -->
    <div
      v-if="showCreateDialog"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="closeCreateDialog"
    >
      <Card class="w-full max-w-md p-6 m-4">
        <h3 class="text-lg font-semibold mb-4">新增文档</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-2">标题</label>
            <Input v-model="newDoc.title" placeholder="请输入文档标题" />
          </div>
          <div class="flex gap-2 justify-end">
            <Button variant="outline" @click="closeCreateDialog">取消</Button>
            <Button @click="createDocument" :disabled="creating">
              {{ creating ? '创建中...' : '创建' }}
            </Button>
          </div>
        </div>
      </Card>
    </div>

    <!-- 图片上传对话框 -->
    <input
      ref="fileInput"
      type="file"
      accept="image/*"
      class="hidden"
      @change="handleFileSelect"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { marked } from 'marked'
import api from '@/utils/api'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Navigation from '@/components/Navigation.vue'

const router = useRouter()
const documents = ref([])
const selectedDocId = ref(null)
const selectedDocument = ref(null)
const editMode = ref(false)
const loading = ref(false)
const saving = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
const fileInput = ref(null)
const newDoc = ref({ title: '' })
const editingDoc = ref({ id: '', title: '', content: '', order: 0 })
const toggling = ref(false) // 防止重复切换

// 配置 marked 选项
marked.setOptions({
  breaks: true, // 支持 GitHub 风格的换行
  gfm: true, // 启用 GitHub 风格的 Markdown
})

// Markdown渲染
const renderedContent = computed(() => {
  if (!selectedDocument.value?.content) return ''
  
  try {
    // 使用 marked 库渲染 Markdown
    const html = marked.parse(selectedDocument.value.content)
    return typeof html === 'string' ? html : String(html || '')
  } catch (err) {
    console.error('Markdown 渲染失败:', err)
    return selectedDocument.value.content
  }
})

const loadDocuments = async () => {
  loading.value = true
  try {
    const response = await api.get('/documents')
    documents.value = response.data.documents || []
    // 如果有选中的文档，重新加载
    if (selectedDocId.value) {
      await loadDocument(selectedDocId.value)
    }
  } catch (err) {
    console.error('加载文档列表失败:', err)
  } finally {
    loading.value = false
  }
}

const loadDocument = async (id) => {
  try {
    const response = await api.get(`/documents/${id}`)
    selectedDocument.value = response.data
    // 只有在非编辑模式下才更新 editingDoc，避免覆盖用户正在编辑的内容
    if (!editMode.value) {
      editingDoc.value = { ...response.data }
    }
  } catch (err) {
    console.error('加载文档失败:', err)
  }
}

const selectDocument = async (id) => {
  selectedDocId.value = id
  editMode.value = false
  await loadDocument(id)
}

const createDocument = async () => {
  if (!newDoc.value.title || creating.value) {
    return
  }

  creating.value = true
  try {
    const response = await api.post('/documents', {
      title: newDoc.value.title,
      content: '',
      order: documents.value.length,
    })
    closeCreateDialog()
    // 先更新列表，避免重复加载
    documents.value.push({
      id: response.data.id,
      title: response.data.title,
      content: '',
      order: response.data.order,
    })
    // 直接选择新创建的文档，避免重复加载
    selectedDocId.value = response.data.id
    selectedDocument.value = {
      id: response.data.id,
      title: response.data.title,
      content: '',
      order: response.data.order,
    }
    editingDoc.value = {
      id: response.data.id,
      title: response.data.title,
      content: '',
      order: response.data.order,
    }
    editMode.value = true
  } catch (err) {
    console.error('创建文档失败:', err)
    alert('创建文档失败: ' + (err.response?.data?.error || err.message))
  } finally {
    creating.value = false
  }
}

const saveDocument = async () => {
  if (!editingDoc.value.id || saving.value) return

  saving.value = true
  try {
    await api.put('/documents', {
      id: editingDoc.value.id,
      title: editingDoc.value.title,
      content: editingDoc.value.content,
      order: editingDoc.value.order,
    })
    // 更新本地状态，避免重复加载
    if (selectedDocument.value) {
      selectedDocument.value.title = editingDoc.value.title
      selectedDocument.value.content = editingDoc.value.content
      selectedDocument.value.order = editingDoc.value.order
    }
    // 更新列表中的文档标题
    const docIndex = documents.value.findIndex(d => d.id === editingDoc.value.id)
    if (docIndex !== -1) {
      documents.value[docIndex].title = editingDoc.value.title
    }
    editMode.value = false
  } catch (err) {
    console.error('保存文档失败:', err)
    alert('保存文档失败: ' + (err.response?.data?.error || err.message))
  } finally {
    saving.value = false
  }
}

const handleToggleEdit = async (event) => {
  // 防止重复点击
  if (toggling.value) {
    return
  }
  
  if (event) {
    event.stopPropagation()
    event.preventDefault()
  }
  
  toggling.value = true
  try {
    await toggleEditMode()
  } finally {
    // 延迟重置，防止快速连续点击
    setTimeout(() => {
      toggling.value = false
    }, 300)
  }
}

const toggleEditMode = async () => {
  if (editMode.value) {
    // 从编辑模式切换到查看模式
    editMode.value = false
    // 重新加载文档以恢复原始内容
    if (selectedDocId.value) {
      await loadDocument(selectedDocId.value)
    }
  } else {
    // 从查看模式切换到编辑模式
    // 确保编辑数据是最新的
    if (!selectedDocument.value && selectedDocId.value) {
      // 如果 selectedDocument 为空，重新加载
      await loadDocument(selectedDocId.value)
    }
    
    if (selectedDocument.value) {
      editingDoc.value = {
        id: selectedDocument.value.id,
        title: selectedDocument.value.title,
        content: selectedDocument.value.content || '',
        order: selectedDocument.value.order || 0,
      }
      // 设置编辑模式
      editMode.value = true
    }
  }
}

const cancelEdit = () => {
  editMode.value = false
  if (selectedDocId.value) {
    loadDocument(selectedDocId.value)
  }
}

const deleteDocument = async () => {
  if (!selectedDocument.value || !confirm('确定要删除这个文档吗？')) {
    return
  }

  try {
    await api.delete('/documents', {
      data: { id: selectedDocument.value.id },
      headers: { 'Content-Type': 'application/json' }
    })
    selectedDocId.value = null
    selectedDocument.value = null
    await loadDocuments()
  } catch (err) {
    console.error('删除文档失败:', err)
    alert('删除文档失败: ' + (err.response?.data?.error || err.message))
  }
}

const insertImage = () => {
  fileInput.value?.click()
}

const handleFileSelect = async (event) => {
  const file = event.target.files[0]
  if (!file) return

  const formData = new FormData()
  formData.append('file', file)

  try {
    const response = await api.post('/upload/image', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    })
    
    // 插入图片Markdown语法
    const imageMarkdown = `\n![${file.name}](${response.data.url})\n`
    
    // 查找textarea元素
    const textarea = document.querySelector('textarea')
    if (textarea) {
      const start = textarea.selectionStart || 0
      const end = textarea.selectionEnd || 0
      const text = editingDoc.value.content || ''
      editingDoc.value.content = text.substring(0, start) + imageMarkdown + text.substring(end)
      
      // 更新光标位置
      setTimeout(() => {
        textarea.focus()
        const newPos = start + imageMarkdown.length
        textarea.setSelectionRange(newPos, newPos)
      }, 0)
    } else {
      editingDoc.value.content = (editingDoc.value.content || '') + imageMarkdown
    }
  } catch (err) {
    console.error('上传图片失败:', err)
    alert('上传图片失败: ' + (err.response?.data?.error || err.message))
  }
  
  // 重置文件输入
  event.target.value = ''
}

const closeCreateDialog = () => {
  showCreateDialog.value = false
  newDoc.value = { title: '' }
}

const handleLogout = () => {
  localStorage.removeItem('token')
  router.push('/login')
}

onMounted(() => {
  loadDocuments()
})
</script>

<style scoped>
.prose {
  @apply text-gray-800;
}

.prose :deep(h1) {
  @apply text-3xl font-bold mt-6 mb-4;
  display: block;
}

.prose :deep(h2) {
  @apply text-2xl font-bold mt-5 mb-3;
  display: block;
}

.prose :deep(h3) {
  @apply text-xl font-bold mt-4 mb-2;
  display: block;
}

.prose :deep(h4) {
  @apply text-lg font-bold mt-3 mb-2;
  display: block;
}

.prose :deep(h5) {
  @apply text-base font-bold mt-2 mb-1;
  display: block;
}

.prose :deep(h6) {
  @apply text-sm font-bold mt-2 mb-1;
  display: block;
}

.prose :deep(p) {
  @apply mb-4;
  display: block;
}

.prose :deep(ul) {
  @apply list-disc list-inside mb-4;
  display: block;
}

.prose :deep(ol) {
  @apply list-decimal list-inside mb-4;
  display: block;
}

.prose :deep(li) {
  @apply mb-1;
  display: list-item;
}

.prose :deep(pre) {
  @apply bg-gray-100 p-4 rounded mb-4;
  display: block;
  overflow-x: auto;
  white-space: pre-wrap;
  word-wrap: break-word;
  word-break: break-all;
}

.prose :deep(pre code) {
  @apply bg-transparent p-0;
  white-space: pre-wrap;
  word-wrap: break-word;
  word-break: break-all;
}

.prose :deep(code) {
  @apply bg-gray-100 px-1 py-0.5 rounded text-sm font-mono;
  word-break: break-all;
  white-space: pre-wrap;
}

.prose :deep(img) {
  @apply my-4 rounded;
  display: block;
  max-width: 100%;
}

.prose :deep(a) {
  @apply text-blue-600 hover:underline;
}
</style>

