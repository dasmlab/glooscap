import axios from 'axios'

// Support both build-time (VITE_) and runtime (window.__API_BASE_URL__) configuration
const getApiBaseURL = () => {
  // Runtime override (set via window.__API_BASE_URL__ or environment variable in container)
  if (typeof window !== 'undefined' && window.__API_BASE_URL__) {
    return window.__API_BASE_URL__
  }
  // Build-time variable
  // Default to Kubernetes service DNS format: name.namespace.svc.cluster.local:port
  // For external access, this will be overridden via environment variable
  return import.meta.env.VITE_API_BASE_URL || '/api/v1'
}

const api = axios.create({
  baseURL: getApiBaseURL(),
  timeout: 10_000,
})

// Add response interceptor to handle connection errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    // Emit custom event for connection status tracking
    if (typeof window !== 'undefined') {
      window.dispatchEvent(
        new CustomEvent('api-connection-status', {
          detail: {
            connected: error.response !== undefined, // If we got a response, we're connected (even if error)
            error: error.code === 'ECONNREFUSED' || error.code === 'ERR_NETWORK',
            message: error.message,
          },
        }),
      )
    }
    return Promise.reject(error)
  },
)

api.interceptors.request.use((config) => {
  const token = window.localStorage.getItem('glooscap-token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export default api

