export const ANIM = {
  duration: {
    fast: 0.15,
    normal: 0.25,
    slow: 0.4,
    reveal: 0.5,
  },
  ease: {
    out: 'power2.out' as const,
    inOut: 'power2.inOut' as const,
    backOut: 'back.out(1.4)' as const,
    elastic: 'elastic.out(1, 0.5)' as const,
  },
}

