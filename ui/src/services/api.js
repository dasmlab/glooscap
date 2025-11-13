import axios from 'axios'

// Support both build-time (VITE_) and runtime (window.__API_BASE_URL__) configuration
const getApiBaseURL = () => {
  // Runtime override (set via window.__API_BASE_URL__ or environment variable in container)
  if (typeof window !== 'undefined' && window.__API_BASE_URL__) {
    return window.__API_BASE_URL__
  }
  // Build-time variable
  return import.meta.env.VITE_API_BASE_URL || '/api/v1'
}

const api = axios.create({
  baseURL: getApiBaseURL(),
  timeout: 10_000,
})

api.interceptors.request.use((config) => {
  const token = window.localStorage.getItem('glooscap-token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export default api

