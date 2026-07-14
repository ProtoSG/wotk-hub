import api from '@/lib/axios'
import { parseFilename } from '@/lib/contentDisposition'

export function useYtdlpApi() {
  async function downloadMp3(url: string): Promise<{ blob: Blob; filename: string }> {
    const res = await api.post('/api/ytdlp/download', { url }, { responseType: 'blob' })
    return { blob: res.data as Blob, filename: parseFilename(res.headers['content-disposition']) }
  }

  return { downloadMp3 }
}
