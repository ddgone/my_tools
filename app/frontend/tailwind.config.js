function withOpacity(cssVariable) {
  return ({ opacityValue }) => {
    if (opacityValue === undefined) {
      return `rgb(var(${cssVariable}) / 1)`
    }
    return `rgb(var(${cssVariable}) / ${opacityValue})`
  }
}

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{vue,ts,js}',
  ],
  theme: {
    extend: {
      colors: {
        white: withOpacity('--color-fg-base'),
        dracula: {
          bg: withOpacity('--color-bg-app'),
          panel: withOpacity('--color-bg-panel'),
          soft: withOpacity('--color-fg-muted'),
          text: withOpacity('--color-fg-base'),
          cyan: withOpacity('--color-brand-primary'),
          green: withOpacity('--color-success'),
          pink: withOpacity('--color-mode-remote'),
          yellow: withOpacity('--color-warning'),
          orange: withOpacity('--color-kind-rust'),
          red: withOpacity('--color-error'),
        },
      },
    },
  },
  plugins: [],
}
