import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { useAuthApi } from '@/hooks/useAuthApi'
import { useAuthStore } from '@/store/authStore'

const schema = z.object({
  email: z.string().min(1, 'Requerido').email('Email inválido'),
  password: z.string().min(1, 'Requerido'),
})

type FormValues = z.infer<typeof schema>

export default function LoginPage() {
  const [signingIn, setSigningIn] = useState(false)
  const { login } = useAuthApi()
  const setUser = useAuthStore((s) => s.setUser)
  const setHasHydrated = useAuthStore((s) => s.setHasHydrated)
  const navigate = useNavigate()

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '' },
  })

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSigningIn(true)
    try {
      const user = await login(values.email, values.password)
      setUser(user)
      setHasHydrated(true)
      navigate('/dashboard', { replace: true })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo iniciar sesión')
    } finally {
      setSigningIn(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <CozyCard className="animate-card-in w-full max-w-sm">
        <CardHeader>
          <CardTitle>Work Hub</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-1">
              <Label>Email</Label>
              <Input type="email" autoComplete="email" {...register('email')} />
              {errors.email && <p className="text-xs text-destructive">{errors.email.message}</p>}
            </div>
            <div className="space-y-1">
              <Label>Contraseña</Label>
              <Input type="password" autoComplete="current-password" {...register('password')} />
              {errors.password && <p className="text-xs text-destructive">{errors.password.message}</p>}
            </div>
            <Button type="submit" className="w-full" disabled={signingIn}>
              {signingIn && <Loader2 size={14} className="animate-spin" />}
              {signingIn ? 'Ingresando…' : 'Ingresar'}
            </Button>
          </form>
        </CardContent>
      </CozyCard>
    </div>
  )
}
