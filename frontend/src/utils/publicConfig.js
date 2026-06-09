import { ref } from 'vue'
import api from './api'

const fallbackTitle = 'LabPanel 管理面板'
const initialTitle = document.title && document.title !== '%APP_TITLE%' ? document.title : fallbackTitle

export const appTitle = ref(initialTitle)

let publicConfigRequest = null

export const loadPublicConfig = async () => {
  if (!publicConfigRequest) {
    publicConfigRequest = api
      .get('/public-config')
      .then((response) => {
        const title = response.data?.title?.trim() || fallbackTitle
        appTitle.value = title
        document.title = title
        return response.data
      })
      .catch(() => {
        document.title = appTitle.value
        return null
      })
  }

  return publicConfigRequest
}

export const setAppTitle = (title) => {
  const nextTitle = title?.trim() || fallbackTitle
  appTitle.value = nextTitle
  document.title = nextTitle
}
