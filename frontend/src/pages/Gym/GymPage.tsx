import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import ExerciseCatalog from './ExerciseCatalog'

/**
 * P1 of the gym module (see GYM_SPEC.md): the seeded exercise catalog, browsable
 * and filterable. Session logging, routines and progress charts land in later
 * phases and will turn this into a tabbed shell.
 */
export default function GymPage() {
  return (
    <div className="space-y-6 pb-24 sm:pb-0">
      <h1 className="text-2xl font-bold">Gimnasio</h1>

      <CozyCard>
        <CardHeader>
          <CardTitle>Ejercicios</CardTitle>
        </CardHeader>
        <CardContent>
          <ExerciseCatalog />
        </CardContent>
      </CozyCard>
    </div>
  )
}
