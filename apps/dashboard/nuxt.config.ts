export default defineNuxtConfig({
  compatibilityDate: '2026-08-03',
  devtools: { enabled: false },
  css: ['~/assets/css/main.css'],
  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || '',
    },
  },
  app: {
    head: {
      title: 'Dispatch City | Live Delivery System',
      meta: [
        { name: 'description', content: 'Live visualization of a distributed food delivery system on Kubernetes.' },
        { name: 'theme-color', content: '#101412' },
      ],
    },
  },
})
