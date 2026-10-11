import { runRaw } from "@/bridge/libs/Lua.js"

function escapeLuaString(value) {
  return String(value).replace(/\\/g, "\\\\").replace(/'/g, "\\'").replace(/\r/g, "\\r").replace(/\n/g, "\\n")
}

let queue = Promise.resolve()
export function flushModSettings() { return queue }

export async function getModSetting(key, fallback = null) {
  await queue
  const raw = await runRaw(`extensions.dualsenseSettingsStore.getJson('${escapeLuaString(key)}')`)
  if (raw == null || raw === "") return fallback
  try { return JSON.parse(String(raw)) } catch { return fallback }
}

export function setModSetting(key, value) {
  const encoded = JSON.stringify(value)
  const command = `extensions.dualsenseSettingsStore.setJson('${escapeLuaString(key)}','${escapeLuaString(encoded)}')`
  const task = queue.then(async () => {
    const result = await runRaw(command)
    if (result === false || String(result).trim() === "false") {
      throw new Error(`Unable to persist ${key}`)
    }
    return result
  })
  queue = task.catch(() => undefined)
  return task
}

export async function patchModSettings(values) {
  for (const [key, value] of Object.entries(values || {})) await setModSetting(key, value)
}
