import client from './client';

/**
 * Register a new user
 * @param {Object} userData - User registration data
 * @param {string} userData.name - User's full name
 * @param {string} userData.email - User's email address
 * @param {string} userData.password - User's password
 * @returns {Promise<Object>} Registration response with user data
 */
export const register = async (userData) => {
  const response = await client.post('/api/auth/register', userData);
  return response.data;
};

/**
 * Login user
 * @param {Object} credentials - Login credentials
 * @param {string} credentials.email - User's email address
 * @param {string} credentials.password - User's password
 * @returns {Promise<Object>} Login response with user data and visit count
 */
export const login = async (credentials) => {
  const response = await client.post('/api/auth/login', credentials);
  return response.data;
};

/**
 * Logout current user
 * @returns {Promise<Object>} Logout response
 */
export const logout = async () => {
  const response = await client.post('/api/auth/logout');
  return response.data;
};

/**
 * Get current authenticated user
 * @returns {Promise<Object>} Current user data
 */
export const getMe = async () => {
  const response = await client.get('/api/auth/me');
  return response.data;
};
