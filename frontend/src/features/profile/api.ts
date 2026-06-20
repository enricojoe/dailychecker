/**
 * Typed profile API calls and DTOs.
 * All calls go through apiClient so the Bearer header + refresh flow applies.
 */

import { apiClient } from '@/api'
import type { UserDto } from '@/auth/api'

export interface UpdateProfileDto {
  name?: string
  username?: string
  current_password?: string
  new_password?: string
}

export interface UsernameAvailability {
  available: boolean
}

export const profileApi = {
  update: (dto: UpdateProfileDto): Promise<UserDto> =>
    apiClient.patch<UserDto>('/me', dto),

  checkUsername: (username: string): Promise<UsernameAvailability> =>
    apiClient.get<UsernameAvailability>(
      `/auth/check-username?username=${encodeURIComponent(username)}`
    ),
}
