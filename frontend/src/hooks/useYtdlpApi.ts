import api from '@/lib/axios'
import { parseFilename } from '@/lib/contentDisposition'

export function useYtdlpApi() {
  async function downloadMp3(url: string): Promise<{ blob: Blob; filename: string }> {
    // Streams the actual audio file back — easily exceeds the global 30s
    // API timeout (that default exists to fail hung *JSON* requests fast,
    // not to cap a real download). 0 = no timeout, just for this call.
    const res = await api.post('/api/ytdlp/download', { url }, { responseType: 'blob', timeout: 0 })
    return { blob: res.data as Blob, filename: parseFilename(res.headers['content-disposition']) }
  }

  return { downloadMp3 }
}
