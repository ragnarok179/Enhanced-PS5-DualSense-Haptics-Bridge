import { lua } from "@/bridge"
import { ROUTE_SOURCE_ID, routeRecords } from "./routes.js"

const BUTTON_ID = "dualsensehaptics.settings"
const COMPONENT_PATH = "/ui/ui-vue/mods/dualsensehaptics/DualSenseHapticsSettings.vue"

function toLuaRoutes(records) {
  return records.map(record => ({
    name: record.name,
    path: record.path,
    meta: record.meta,
    ...(record.children ? { children: toLuaRoutes(record.children) } : {}),
  }))
}

async function registerRoutes() {
  window.bngRoutes.add([{ path: ROUTE_SOURCE_ID, routes: routeRecords }])
  const result = await lua.extensions.ui_router_routeManager.registerModRoutes(
    ROUTE_SOURCE_ID,
    toLuaRoutes(routeRecords),
  )
  if (!result?.success) {
    window.bngRoutes.remove([ROUTE_SOURCE_ID])
    console.error("[Enhanced PS5 DualSense Haptics] Failed to register settings route", result?.errors)
  }
  return result
}

export async function onLoad() {
  const routeResult = await registerRoutes()
  if (!routeResult?.success) return

  const result = await lua.extensions.ui_pause_actions.registerModButton({
    id: BUTTON_ID,
    tabId: "mods",
    label: "Enhanced PS5 DualSense Settings",
    icon: "wrench",
    componentName: COMPONENT_PATH,
  })
  if (!result?.success) {
    console.error("[Enhanced PS5 DualSense Haptics] Failed to register pause button", result?.reason)
  }
}

export async function onUnload() {
  await lua.extensions.ui_pause_actions.unregisterModButton(BUTTON_ID)
  window.bngRoutes.remove([ROUTE_SOURCE_ID])
  await lua.extensions.ui_router_routeManager.unregisterModRoutes(
    ROUTE_SOURCE_ID,
    { fallbackRoute: "play" },
  )
}
