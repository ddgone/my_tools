/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{vue,ts,js}',
  ],
  theme: {
    extend: {
      colors: {
        dracula: {
          bg: '#282a36',
          panel: '#1e1f29',
          soft: '#44475a',
          text: '#f8f8f2',
          cyan: '#8be9fd',
          green: '#50fa7b',
          pink: '#ff79c6',
          yellow: '#f1fa8c',
          orange: '#ffb86c',
          red: '#ff5555'
        }
      }
    },
  },
  plugins: [],
}
