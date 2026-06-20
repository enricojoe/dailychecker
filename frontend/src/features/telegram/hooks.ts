import { useMutation, useQueryClient } from '@tanstack/react-query'
import { telegramApi } from './api'
import type { TelegramLinkResponse } from './api'

export function useTelegramLink() {
  const qc = useQueryClient()
  return useMutation<TelegramLinkResponse, Error>({
    mutationFn: () => telegramApi.link(),
    onSuccess: () => {
      // Refetch /me so user.telegram_chat_id populates when the user links
      void qc.invalidateQueries({ queryKey: ['me'] })
    },
  })
}
