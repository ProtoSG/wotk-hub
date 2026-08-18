import api from '@/lib/axios'

export interface ModulePermission {
  module: string
  enabled: boolean
}

export function usePermissionsApi() {
  async function listMine(): Promise<string[]> {
    const res = await api.get<{ modules: string[] }>('/api/permissions/mine')
    return res.data.modules
  }

  async function listGuest(): Promise<ModulePermission[]> {
    const res = await api.get<{ modules: ModulePermission[] }>('/api/permissions/guest')
    return res.data.modules
  }

  async function updateGuest(modules: Record<string, boolean>): Promise<ModulePermission[]> {
    const res = await api.put<{ modules: ModulePermission[] }>('/api/permissions/guest', { modules })
    return res.data.modules
  }

  return { listMine, listGuest, updateGuest }
}
