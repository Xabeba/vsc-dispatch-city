export default defineEventHandler(() => ({
  instance: process.env.HOSTNAME || 'local-dashboard',
}))
