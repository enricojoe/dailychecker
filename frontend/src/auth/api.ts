/**
 * Typed auth API calls and DTOs.
 * All calls go through apiClient so the Bearer header + refresh flow applies.
 */

import { apiClient } from '@/api/apiClient'

// ── DTOs ──────────────────────────────────────────────────────────────────────

export interface UserDto {
  id: string
  name: string
  phone: string
  telegram_chat_id?: number
  telegram_linked_at?: string
  created_at: string
  updated_at: string
}

export interface TokenPair {
  access: string
  refresh: string
}

export interface RegisterDto {
  name: string
  phone: string
  password: string
}

// ── Calls ─────────────────────────────────────────────────────────────────────

export const authApi = {
  register: (dto: RegisterDto) =>
    apiClient.post<UserDto>('/auth/register', dto),

  login: (phone: string, password: string) =>
    apiClient.post<TokenPair>('/auth/login', { phone, password }),

  logout: (refresh: string) =>
    apiClient.post<void>('/auth/logout', { refresh }),

  me: () => apiClient.get<UserDto>('/me'),
}
