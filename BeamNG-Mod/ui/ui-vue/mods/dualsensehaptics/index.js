import { lua } from "@/bridge"
import { ROUTE_SOURCE_ID, routeRecords } from "./routes.js"

const BUTTON_ID = "dualsensehaptics.settings"
const COMPONENT_PATH = "/ui/ui-vue/mods/dualsensehaptics/DualSenseHapticsSettings.vue"
const LOG_TAG = "[Enhanced PS5 DualSense Haptics]"

let transition = Promise.resolve()
let loaded = false

function toLuaRoutes(records) {
  return records.map(record => ({
    name: record.name,
    path: record.path,
    meta: record.meta,
    ...(record.children ? { children: toLuaRoutes(record.children) } : {}),
  }))
}

function scheduleTransition(operation) {
  // BeamNG may begin unloading while route/button registration is awaiting Lua.
  // Serialize both directions so a late onLoad cannot re-register the UI.
  transition = transition.then(operation, operation).catch(error => {
    console.error(LOG_TAG, "UI lifecycle transition failed", error)
  })
  return transition
}

async function unregisterUI() {
  try {
    await lua.extensions.ui_pause_actions.unregisterModButton(BUTTON_ID)
  } catch (error) {
    console.warn(LOG_TAG, "Pause button cleanup failed", error)
  }

  // Always remove the local route, even when BeamNG's Lua routing is unavailable
  // because the ZIP has already been unmounted.
  try {
    window.bngRoutes.remove([ROUTE_SOURCE_ID])
  } catch (error) {
    console.warn(LOG_TAG, "Local route cleanup failed", error)
  }

  try {
    await lua.extensions.ui_router_routeManager.unregisterModRoutes(
      ROUTE_SOURCE_ID,
      { fallbackRoute: "play" },
    )
  } catch (error) {
    console.warn(LOG_TAG, "Lua route cleanup failed", error)
  }
  loaded = false
}

async function registerUI() {
  if (loaded) return

  try {
    window.bngRoutes.add([{ path: ROUTE_SOURCE_ID, routes: routeRecords }])
    const routeResult = await lua.extensions.ui_router_routeManager.registerModRoutes(
      ROUTE_SOURCE_ID,
      toLuaRoutes(routeRecords),
    )
    if (!routeResult?.success) {
      console.error(LOG_TAG, "Failed to register settings route", routeResult?.errors)
      await unregisterUI()
      return
    }

    const buttonResult = await lua.extensions.ui_pause_actions.registerModButton({
      id: BUTTON_ID,
      tabId: "mods",
      label: "Enhanced PS5 DualSense Settings",
      icon: "wrench",
      componentName: COMPONENT_PATH,
    })
    if (!buttonResult?.success) {
      console.error(LOG_TAG, "Failed to register pause button", buttonResult?.reason)
      await unregisterUI()
      return
    }
    loaded = true
  } catch (error) {
    console.error(LOG_TAG, "UI registration failed", error)
    await unregisterUI()
  }
}

export function onLoad() {
  return scheduleTransition(registerUI)
}

export function onUnload() {
  return scheduleTransition(unregisterUI)
}
