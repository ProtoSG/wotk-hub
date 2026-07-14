import axios from 'axios'
import publicApi from '@/lib/publicApi'
import { parseFilename } from '@/lib/contentDisposition'

/** With responseType: 'blob', axios also delivers error bodies as a Blob
 *  instead of parsed JSON — read it back out so we can tell "missing token"
 *  apart from "invalid token" instead of collapsing both into one message. */
async function readBackendError(data: unknown): Promise<string | undefined> {
  if (!(data instanceof Blob)) return undefined
  try {
    const parsed = JSON.parse(await data.text()) as { error?: string }
    return parsed.error
  } catch {
    return undefined
  }
}

export function usePublicYtdlpApi() {
  async function downloadMp3(url: string, token: string): Promise<{ blob: Blob; filename: string }> {
    try {
      const res = await publicApi.post(
        '/api/ytdlp/public/download',
        { url },
        { params: { token }, responseType: 'blob' }
      )
      return { blob: res.data as Blob, filename: parseFilename(res.headers['content-disposition']) }
    } catch (err) {
      if (axios.isAxiosError(err)) {
        const backendMsg = await readBackendError(err.response?.data)
        if (err.response?.status === 401) {
          if (backendMsg === 'missing token') throw new Error('Falta el token en el link.')
          throw new Error('El token del link es incorrecto.')
        }
        if (err.response?.status === 429) throw new Error('El servidor está ocupado, probá de nuevo en un momento.')
        if (err.response?.status === 400) throw new Error('Ese link no es de YouTube.')
      }
      throw new Error('No se pudo descargar el audio.')
    }
  }

  return { downloadMp3 }
}
