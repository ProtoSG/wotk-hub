import { Bus, Coins, CreditCard, Landmark, Smartphone, Wallet, type LucideIcon } from 'lucide-react'

// No literal Yape/Metropolitano glyphs exist in lucide (or realistically in
// any general icon set) — these map to the closest generic concept instead:
// Yape → a mobile wallet (Smartphone), Metropolitano → transit (Bus).
export const CARD_ICON_OPTIONS = [
  { value: 'credit-card', label: 'Tarjeta' },
  { value: 'wallet', label: 'Billetera' },
  { value: 'coins', label: 'Efectivo' },
  { value: 'smartphone', label: 'Yape / billetera digital' },
  { value: 'bus', label: 'Metropolitano / transporte' },
  { value: 'landmark', label: 'Banco' },
]

// card.icon only ever holds one of CARD_ICON_OPTIONS' values.
export const CARD_ICON_MAP: Record<string, LucideIcon> = {
  'credit-card': CreditCard,
  wallet: Wallet,
  coins: Coins,
  smartphone: Smartphone,
  bus: Bus,
  landmark: Landmark,
}
