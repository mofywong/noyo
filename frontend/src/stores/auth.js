import { defineStore } from 'pinia';
import axios from 'axios';
import {
  hasUserPermission,
  isProjectAdminUser,
  isSystemAdminUser,
  isTenantAdminUser,
} from '../utils/authIdentity.js';

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('access_token') || '',
    refreshToken: localStorage.getItem('refresh_token') || '',
    user: JSON.parse(localStorage.getItem('user_info') || 'null'),
  }),
  getters: {
    isLoggedIn: (state) => !!state.token,
    userRole: (state) => (state.user ? state.user.role : ''),
    isSystemAdmin: (state) => isSystemAdminUser(state.user),
    isTenantAdmin: (state) => isTenantAdminUser(state.user),
    isProjectAdmin: (state) => isProjectAdminUser(state.user),
    hasPermission: (state) => {
      return (code) => hasUserPermission(state.user, code)
    }
  },
  actions: {
    async login(username, password, login_suffix = '') {
      const response = await axios.post('/api/auth/login', { username, password, login_suffix });
      if (response.data.code === 0) {
        const { access_token, refresh_token, user_info } = response.data.data;
        this.token = access_token;
        this.refreshToken = refresh_token;
        this.user = user_info;
        
        localStorage.setItem('access_token', access_token);
        localStorage.setItem('refresh_token', refresh_token);
        localStorage.setItem('user_info', JSON.stringify(user_info));
      }
      return response.data;
    },
    
    logout() {
      this.token = '';
      this.refreshToken = '';
      this.user = null;
      localStorage.removeItem('access_token');
      localStorage.removeItem('refresh_token');
      localStorage.removeItem('user_info');
      localStorage.removeItem('current_project_id');
    },

    async refreshProfile() {
      const response = await axios.get('/api/auth/profile');
      if (response.data.code === 0) {
        this.user = response.data.data;
        localStorage.setItem('user_info', JSON.stringify(this.user));
      }
      return response.data;
    },

    async changePassword(oldPassword, newPassword) {
      const response = await axios.put('/api/auth/password', {
        old_password: oldPassword,
        new_password: newPassword
      });
      return response.data;
    }
  }
});
