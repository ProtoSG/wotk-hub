import { CreditCard } from 'lucide-react'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { CardFormFields } from './CardForm'

// Page-level onboarding gate (spec finance-onboarding / design #40). Blocks
// ALL Finances tabs until the owner has ≥1 card, regardless of card type.
// Reuses the existing listCards result + the CardFormFields body so the
// user creates their first card inline without ever seeing the tabbed content.
export default function OnboardingGate({ onSaved }: { onSaved: () => void }) {
  return (
    <CozyCard className="animate-card-in mx-auto mt-12 max-w-md">
      <CardContent className="flex flex-col items-center gap-4 py-8 text-center">
        <CreditCard className="h-12 w-12 opacity-30" />
        <div className="space-y-1">
          <h2 className="text-xl font-semibold">Para iniciar con tus finanzas</h2>
          <p className="text-sm text-muted-foreground">
            Agregá una tarjeta para empezar a registrar tus movimientos.
          </p>
        </div>
        <div className="w-full text-left">
          <CardFormFields onSaved={onSaved} />
        </div>
      </CardContent>
    </CozyCard>
  )
}
