// tailwind.config.js
const {heroui} = require("@heroui/react");

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{js,ts,jsx,tsx,mdx}",
    "./node_modules/@heroui/theme/dist/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      keyframes: {
        
        fadeInUp: {
          '0%': { opacity: 0, transform: 'translateY(20px)' },
          '100%': { opacity: 1, transform: 'translateY(0)' },
        },
        expand: {
          '0%': { opacity: 0, transform: 'scale(0.95)' },
          '100%': { opacity: 1, transform: 'scale(1)' },
        },
        fadeIn: {
          '0%': { opacity: 0 },
          '100%': { opacity: 1 },
        },
        bounceIn: {
          '0%, 20%, 40%, 60%, 80%, 100%': {
            transform: 'translateY(0)',
          },
          '50%': {
            transform: 'translateY(-10px)',
          },    
        },
        slideIn: {
          '0%': { transform: 'translateX(-100%)' },
          '100%': { transform: 'translateX(0)' },
        },
        scaleUp: {
          '0%': { transform: 'scale(0.5)' },
          '100%': { transform: 'scale(1)' },
        },
      },
      animation: {
        fadeInUp: 'fadeInUp 1s ease-out',
        fadeInUp2: 'fadeInUp 1.5s ease-out',
        fadeIn: 'fadeIn 1s ease-out',
        expand: 'expand 1s ease-out',
        bounceIn: 'bounceIn 1s ease-in-out',
        slideIn: 'slideIn 0.5s ease-out',
        'fade-in': 'fadeIn 2s ease-in-out',
        'scale-up': 'scaleUp 2s ease-in-out',
      },
      colors: {
        primary: {
          50: "rgb(var(--theme-50) / <alpha-value>)",
          100: "rgb(var(--theme-100) / <alpha-value>)",
          200: "rgb(var(--theme-200) / <alpha-value>)",
          300: "rgb(var(--theme-300) / <alpha-value>)",
          400: "rgb(var(--theme-400) / <alpha-value>)",
          500: "rgb(var(--theme-500) / <alpha-value>)",
          600: "rgb(var(--theme-600) / <alpha-value>)",
          700: "rgb(var(--theme-700) / <alpha-value>)",
          800: "rgb(var(--theme-800) / <alpha-value>)",
          900: "rgb(var(--theme-900) / <alpha-value>)",
          950: "rgb(var(--theme-950) / <alpha-value>)",
          DEFAULT: "rgb(var(--theme-500) / <alpha-value>)",
        },
      },
    },
},
  darkMode: "class",
  plugins: [
    require('@tailwindcss/typography'),
    heroui({
      themes: {
        light: {
          colors: {
            background: "rgba(250, 250, 250, 0.9)", 
            foreground: "#27272a",
            primary: {
              50: "rgb(var(--theme-50) / <alpha-value>)",
              100: "rgb(var(--theme-100) / <alpha-value>)",
              200: "rgb(var(--theme-200) / <alpha-value>)",
              300: "rgb(var(--theme-300) / <alpha-value>)",
              400: "rgb(var(--theme-400) / <alpha-value>)",
              500: "rgb(var(--theme-500) / <alpha-value>)",
              600: "rgb(var(--theme-600) / <alpha-value>)",
              700: "rgb(var(--theme-700) / <alpha-value>)",
              800: "rgb(var(--theme-800) / <alpha-value>)",
              900: "rgb(var(--theme-900) / <alpha-value>)",
              DEFAULT: "rgb(var(--theme-500) / <alpha-value>)",
              foreground: "#ffffff",
            },
          },
        },
        dark: {
          colors: {
            background: "rgba(5, 5, 5, 0.9)", 
            primary: {
              50: "rgb(var(--theme-50) / <alpha-value>)",
              100: "rgb(var(--theme-100) / <alpha-value>)",
              200: "rgb(var(--theme-200) / <alpha-value>)",
              300: "rgb(var(--theme-300) / <alpha-value>)",
              400: "rgb(var(--theme-400) / <alpha-value>)",
              500: "rgb(var(--theme-500) / <alpha-value>)",
              600: "rgb(var(--theme-600) / <alpha-value>)",
              700: "rgb(var(--theme-700) / <alpha-value>)",
              800: "rgb(var(--theme-800) / <alpha-value>)",
              900: "rgb(var(--theme-900) / <alpha-value>)",
              DEFAULT: "rgb(var(--theme-500) / <alpha-value>)",
              foreground: "#ffffff",
            },
          },
        },
      },
    }),
  ],
};
