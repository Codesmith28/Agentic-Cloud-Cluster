import axios from 'axios';

const BASE_URL =
  import.meta.env.VITE_API_BASE_URL ||
  (import.meta.env.DEV ? '' : window.location.origin);

const apiClient = axios.create({
  baseURL: BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 10000,
  withCredentials: true, // Enable sending cookies with requests
});

// Request interceptor
apiClient.interceptors.request.use(
  (config) => {
    // Auth token is automatically sent via cookies (withCredentials: true)
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Redirect will be handled by AuthContext
    }
    if (import.meta.env.DEV) {
      console.error('API Error:', error.response?.status, error.message);
    }
    return Promise.reject(error);
  }
);

export default apiClient;
