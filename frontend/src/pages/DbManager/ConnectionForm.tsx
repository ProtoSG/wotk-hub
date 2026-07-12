import { useEffect, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useDbStore } from '@/store/dbStore'
import { useDbApi } from '@/hooks/useDbApi'
import type { SavedConnection } from '@/types/db.types'

const schema = z.object({
  name: z.string().min(1, 'Required'),
  dialect: z.enum(['postgres', 'mysql']),
  host: z.string().min(1, 'Required'),
  port: z.number().int().min(1).max(65535),
  user: z.string().min(1, 'Required'),
  password: z.string(),
  database: z.string().min(1, 'Required'),
})

type FormValues = z.infer<typeof schema>

const emptyDefaults: FormValues = {
  name: '',
  dialect: 'postgres',
  host: 'localhost',
  port: 5432,
  user: '',
  password: '',
  database: '',
}

interface Props {
  open: boolean
  onClose: () => void
  editing?: SavedConnection | null
}

export default function ConnectionForm({ open, onClose, editing }: Props) {
  const [testing, setTesting] = useState(false)
  const { addConnection, updateConnection } = useDbStore()
  const { testConnection } = useDbApi()

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: editing ?? emptyDefaults,
  })

  useEffect(() => {
    if (open) {
      reset(editing ?? emptyDefaults)
    }
  }, [editing, open, reset])

  const dialect = watch('dialect')

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setTesting(true)
    try {
      const conn: SavedConnection = { id: editing?.id ?? crypto.randomUUID(), ...values }
      await testConnection(conn)
      if (editing) {
        updateConnection(editing.id, values)
      } else {
        addConnection(conn)
      }
      toast.success('Connection saved')
      reset()
      onClose()
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Connection failed'
      toast.error(msg)
    } finally {
      setTesting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Connection' : 'New Connection'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Name</Label>
            <Input placeholder="My Postgres DB" {...register('name')} />
            {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
          </div>
          <div className="space-y-1">
            <Label>Dialect</Label>
            <Select
              value={dialect}
              onValueChange={(v) => {
                setValue('dialect', v as 'postgres' | 'mysql')
                setValue('port', v === 'mysql' ? 3306 : 5432)
              }}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="postgres">PostgreSQL</SelectItem>
                <SelectItem value="mysql">MySQL</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="grid grid-cols-3 gap-2">
            <div className="col-span-2 space-y-1">
              <Label>Host</Label>
              <Input placeholder="localhost" {...register('host')} />
            </div>
            <div className="space-y-1">
              <Label>Port</Label>
              <Input type="number" {...register('port', { valueAsNumber: true })} />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="space-y-1">
              <Label>User</Label>
              <Input {...register('user')} />
            </div>
            <div className="space-y-1">
              <Label>Password</Label>
              <Input type="password" {...register('password')} />
            </div>
          </div>
          <div className="space-y-1">
            <Label>Database</Label>
            <Input {...register('database')} />
            {errors.database && <p className="text-xs text-destructive">{errors.database.message}</p>}
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => { reset(emptyDefaults); onClose() }}>
              Cancel
            </Button>
            <Button type="submit" disabled={testing}>
              {testing && <Loader2 size={14} className="animate-spin" />}
              {testing ? 'Testing…' : 'Test & Save'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
