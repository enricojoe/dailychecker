/**
 * Profile hooks.
 *
 * useUpdateProfile writes the returned user straight into the ['me'] cache so
 * the AppShell header reflects name/username changes immediately.
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { UserDto } from '@/auth/api'
import { profileApi } from './api'
import type { UpdateProfileDto } from './api'

export function useUpdateProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (dto: UpdateProfileDto) => profileApi.update(dto),
    onSuccess: (user: UserDto) => {
      qc.setQueryData(['me'], user)
    },
  })
}
