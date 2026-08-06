import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Eye, EyeOff, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { useAuthApi } from '@/hooks/useAuthApi'
import { useAuthStore } from '@/store/authStore'
import { defaultRouteFor } from '@/router/defaultRoute'

const schema = z.object({
  email: z.string().min(1, 'Requerido'),
  password: z.string().min(1, 'Requerido'),
})

type FormValues = z.infer<typeof schema>

// Users can log in with just their username ("jadet.perez") — no need to
// type the full email every time. Only a bare username gets the domain
// appended; anything with an '@' already is assumed to be a full email and
// is sent through untouched.
const defaultEmailDomain = 'workhub.com'

function withDefaultDomain(email: string): string {
  const trimmed = email.trim()
  return trimmed.includes('@') ? trimmed : `${trimmed}@${defaultEmailDomain}`
}

export default function LoginPage() {
  const [signingIn, setSigningIn] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
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
      const user = await login(withDefaultDomain(values.email), values.password)
      setUser(user)
      setHasHydrated(true)
      navigate(defaultRouteFor(user.role), { replace: true })
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
              <Input type="text" autoComplete="username" {...register('email')} />
              {errors.email && <p className="text-xs text-destructive">{errors.email.message}</p>}
            </div>
            <div className="space-y-1">
              <Label>Contraseña</Label>
              <div className="relative">
                <Input
                  type={showPassword ? 'text' : 'password'}
                  autoComplete="current-password"
                  className="pr-10"
                  {...register('password')}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute inset-y-0 right-0 h-full w-9 text-muted-foreground hover:bg-transparent hover:text-foreground"
                  onClick={() => setShowPassword((v) => !v)}
                  aria-label={showPassword ? 'Ocultar contraseña' : 'Mostrar contraseña'}
                  tabIndex={-1}
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </Button>
              </div>
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
