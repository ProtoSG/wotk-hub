// Matte cotton-paper texture — feTurbulence height-mapped through
// feDiffuseLighting with a flat surfaceScale and near-overhead light, so it
// reads as soft paper fiber rather than sandpaper grain or a glossy emboss
// (both tried and rejected before this). Must render as an <img>, not a CSS
// background-image: Chromium doesn't tile inline-SVG data-URI backgrounds
// correctly at small sizes, it silently washes out to nothing.
const PAPER_SVG = encodeURIComponent(
  `<svg xmlns='http://www.w3.org/2000/svg' width='320' height='320'>` +
    `<filter id='n' x='-20%' y='-20%' width='140%' height='140%'>` +
    `<feTurbulence type='fractalNoise' baseFrequency='0.18' numOctaves='5' seed='7' stitchTiles='stitch' result='noise'/>` +
    `<feDiffuseLighting in='noise' lighting-color='#ffffff' surfaceScale='1.1' diffuseConstant='1.0' result='light'>` +
    `<feDistantLight azimuth='235' elevation='68'/>` +
    `</feDiffuseLighting>` +
    `</filter>` +
    `<rect width='100%' height='100%' filter='url(#n)'/>` +
    `</svg>`
)

export function CardTextureOverlay() {
  return (
    <img
      aria-hidden
      alt=""
      src={`data:image/svg+xml,${PAPER_SVG}`}
      className="pointer-events-none absolute inset-0 h-full w-full rounded-[inherit] object-cover opacity-[0.55] mix-blend-soft-light"
    />
  )
}
