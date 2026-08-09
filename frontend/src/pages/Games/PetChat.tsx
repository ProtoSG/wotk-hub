import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Mic, MicOff, Send, Volume2, VolumeX } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { usePetChat } from '@/hooks/usePetChat'
import type { PetMood } from '@/types/pet.types'
import { MOOD_SPRITE } from './petSprites'

// es-PE matches this project's existing locale convention everywhere else a
// locale string appears (see lib/currency.ts's Intl.NumberFormat/
// toLocaleDateString calls and ConfigurationPage.tsx) — this app targets a
// couple in Peru (see backend's America/Lima timezone), not es-AR.
const SPEECH_LANG = 'es-PE'

// Whether to speak the pet's replies out loud, persisted across sessions —
// same direct localStorage read/write pattern CoupleCover.tsx already uses
// for its own single string preference, no dedicated settings store exists
// for a one-off boolean like this.
const TTS_STORAGE_KEY = 'pet-chat-tts-enabled'

// Voice output is on standby: ElevenLabs' free plan rejects Voice Library
// voices via the API (402 payment_required) — see pet/speak.go's 502 log.
// Rather than rip out the working /speak plumbing, this single flag hides
// the toggle button and short-circuits playback until the plan gets
// upgraded. Flip back to true (and delete this comment) once that happens —
// no other code changes needed.
const VOICE_OUTPUT_ENABLED = false

interface ChatMessage {
  role: 'user' | 'pet'
  text: string
}

// The Web Speech API's SpeechRecognition side isn't part of TS's standard
// DOM lib (Chrome only ships it vendor-prefixed, still non-standard) — this
// declares just the slice actually used here rather than pulling in a whole
// speech-api type-defs package for one feature-detected mic button. TTS
// output no longer touches this API at all (see handleSend/playReply below)
// — replies are voiced through ElevenLabs + a plain <audio> element, which
// is standard and universally supported, so there's nothing to feature-
// detect or declare for that half anymore.
interface MinimalSpeechRecognition extends EventTarget {
  lang: string
  interimResults: boolean
  maxAlternatives: number
  onresult: ((event: { results: { [i: number]: { [j: number]: { transcript: string } } } }) => void) | null
  onerror: (() => void) | null
  onend: (() => void) | null
  start(): void
  stop(): void
}
type SpeechRecognitionCtor = new () => MinimalSpeechRecognition

// Feature-detected once at module load, not per-render — neither vendor
// exposes this outside Chrome, so SpeechRecognitionImpl stays undefined
// (and the mic button hides entirely, see the render below) everywhere else.
const SpeechRecognitionImpl: SpeechRecognitionCtor | undefined =
  typeof window === 'undefined'
    ? undefined
    : (
        window as unknown as {
          SpeechRecognition?: SpeechRecognitionCtor
          webkitSpeechRecognition?: SpeechRecognitionCtor
        }
      ).SpeechRecognition ??
      (window as unknown as { webkitSpeechRecognition?: SpeechRecognitionCtor }).webkitSpeechRecognition

interface PetChatProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  mood: PetMood
  petName: string
}

// Session-only chat panel for the shared couple pet — no persistence, the
// message list resets every time this closes/reopens. Reuses MascotaTab's
// Dialog primitive and pixel-art flat material (hard 2px borders, no
// gradients/glow) rather than introducing a new visual language for this
// one feature.
export default function PetChat({ open, onOpenChange, mood, petName }: PetChatProps) {
  const { sendMessage, speak } = usePetChat()
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [listening, setListening] = useState(false)
  const [ttsEnabled, setTtsEnabled] = useState(() => localStorage.getItem(TTS_STORAGE_KEY) === 'true')
  const recognitionRef = useRef<MinimalSpeechRecognition | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' })
  }, [messages, sending])

  useEffect(() => {
    localStorage.setItem(TTS_STORAGE_KEY, String(ttsEnabled))
  }, [ttsEnabled])

  // Stop any in-flight recognition the moment the panel closes — otherwise
  // it'd keep listening in the background after the couple dismisses it.
  // Deliberately NOT calling setListening(false) directly here: .stop()
  // fires the recognition's own onend callback (wired in handleMicClick),
  // which already flips `listening` back to false — updating state from
  // that async callback instead of synchronously in the effect body avoids
  // the cascading-render footgun react-hooks/set-state-in-effect flags.
  useEffect(() => {
    if (!open) {
      recognitionRef.current?.stop()
    }
  }, [open])

  async function handleSend(rawText: string) {
    const trimmed = rawText.trim()
    if (!trimmed || sending) return
    setMessages((prev) => [...prev, { role: 'user', text: trimmed }])
    setInput('')
    setSending(true)
    try {
      const reply = await sendMessage(trimmed)
      setMessages((prev) => [...prev, { role: 'pet', text: reply }])
      // Fire-and-forget: voice playback shouldn't block `sending` (and thus
      // the input/send button) while the audio downloads and plays — the
      // text reply above is already the source of truth for the chat.
      if (VOICE_OUTPUT_ENABLED && ttsEnabled) {
        void playReply(reply)
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo enviar el mensaje')
    } finally {
      setSending(false)
    }
  }

  // Fetches the ElevenLabs audio for `text` and plays it via a plain <audio>
  // element, revoking the object URL once playback ends (or fails to start)
  // to avoid leaking it. Best-effort: a failed/rate-limited/unconfigured
  // voice call is reported via toast but never rethrown — the chat message
  // itself already rendered by the time this runs, so voice is a bonus, not
  // a requirement.
  async function playReply(text: string) {
    try {
      const blob = await speak(text)
      const url = URL.createObjectURL(blob)
      const audio = new Audio(url)
      audio.addEventListener('ended', () => URL.revokeObjectURL(url))
      await audio.play().catch(() => {
        URL.revokeObjectURL(url)
        throw new Error('No se pudo reproducir la voz')
      })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo reproducir la voz')
    }
  }

  function handleMicClick() {
    if (!SpeechRecognitionImpl) return
    if (listening) {
      recognitionRef.current?.stop()
      return
    }
    const recognition = new SpeechRecognitionImpl()
    recognition.lang = SPEECH_LANG
    recognition.interimResults = false
    recognition.maxAlternatives = 1
    recognition.onresult = (event) => {
      const transcript = event.results[0]?.[0]?.transcript
      if (transcript) handleSend(transcript)
    }
    recognition.onerror = () => {
      toast.error('No se pudo reconocer la voz')
      setListening(false)
    }
    recognition.onend = () => setListening(false)
    recognitionRef.current = recognition
    recognition.start()
    setListening(true)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[80vh] flex-col sm:max-w-md">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <img
              src={MOOD_SPRITE[mood]}
              alt={petName || 'Nuestra mascota'}
              width={32}
              height={32}
              style={{ imageRendering: 'pixelated' }}
            />
            <DialogTitle className="font-pixel text-xl tracking-wide">{petName || 'Nuestra mascota'}</DialogTitle>
          </div>
        </DialogHeader>

        <div ref={scrollRef} className="min-h-[240px] flex-1 space-y-2 overflow-y-auto py-2">
          {messages.length === 0 && (
            <p className="py-8 text-center text-sm text-muted-foreground">Escribile algo a tu mascota</p>
          )}
          {messages.map((m, i) => (
            <ChatBubble key={i} message={m} />
          ))}
          {sending && <ChatBubble message={{ role: 'pet', text: '…' }} />}
        </div>

        <div className="flex items-center gap-2 border-t-2 pt-3">
          {VOICE_OUTPUT_ENABLED && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={ttsEnabled ? 'Desactivar voz de la mascota' : 'Activar voz de la mascota'}
              title={ttsEnabled ? 'Desactivar voz' : 'Activar voz'}
              onClick={() => setTtsEnabled((v) => !v)}
            >
              {ttsEnabled ? <Volume2 className="h-4 w-4" /> : <VolumeX className="h-4 w-4 text-muted-foreground" />}
            </Button>
          )}
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleSend(input)
            }}
            placeholder="Escribile algo..."
            disabled={sending}
            className="font-pixel"
          />
          {/* Hidden entirely (not just disabled) when the browser doesn't
              support SpeechRecognition — this is a Chrome-only API, and a
              visible-but-broken mic button would be worse than no button. */}
          {SpeechRecognitionImpl && (
            <Button
              type="button"
              variant={listening ? 'default' : 'ghost'}
              size="icon"
              aria-label={listening ? 'Detener grabación' : 'Hablar con la mascota'}
              onClick={handleMicClick}
              disabled={sending}
            >
              {listening ? <MicOff className="h-4 w-4" /> : <Mic className="h-4 w-4" />}
            </Button>
          )}
          <Button
            type="button"
            size="icon"
            aria-label="Enviar mensaje"
            onClick={() => handleSend(input)}
            disabled={sending || !input.trim()}
          >
            <Send className="h-4 w-4" />
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

// Flat pixel-art bubble — hard 2px border, no gradient/glow/soft shadow,
// same accent-tint-over-card material as the rest of this feature (compare
// petToast/TurnCard in MascotaTab.tsx). --chart-3 for the pet's side mirrors
// the shop/rename items' identity color; --primary for the human side
// mirrors the care actions' default accent.
function ChatBubble({ message }: { message: ChatMessage }) {
  const isPet = message.role === 'pet'
  return (
    <div className={cn('flex', isPet ? 'justify-start' : 'justify-end')}>
      <div
        className="max-w-[80%] rounded-sm border-2 px-3 py-2 text-sm"
        style={{
          backgroundColor: isPet
            ? 'color-mix(in oklch, var(--chart-3) 16%, var(--card))'
            : 'color-mix(in oklch, var(--primary) 16%, var(--card))',
          borderColor: isPet
            ? 'color-mix(in oklch, var(--chart-3) 50%, var(--border))'
            : 'color-mix(in oklch, var(--primary) 50%, var(--border))',
          color: isPet ? 'var(--chart-3)' : 'var(--primary)',
        }}
      >
        {message.text}
      </div>
    </div>
  )
}
