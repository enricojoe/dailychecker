import { apiClient } from '@/api'

export interface TelegramLinkResponse {
  url: string
  token: string
}

export const telegramApi = {
  link: (): Promise<TelegramLinkResponse> =>
    apiClient.post<TelegramLinkResponse>('/telegram/link', {}),
}
