export default defineEventHandler(async () => {
  const token = await mintMapKitToken()
  return { token }
})
