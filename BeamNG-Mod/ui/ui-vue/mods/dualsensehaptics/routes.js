import DualSenseHapticsSettings from "./DualSenseHapticsSettings.vue"

export const ROUTE_SOURCE_ID = "dualsensehaptics.routes"
export const SETTINGS_ROUTE_NAME = "dualsenseHaptics.settings"

export const routeRecords = [
  {
    path: "/dualsense-haptics-settings",
    name: SETTINGS_ROUTE_NAME,
    component: DualSenseHapticsSettings,
    meta: {
      luaRoute: {
        title: "Enhanced PS5 DualSense Settings",
        backTarget: "play",
        scopeTree: { "dualsense-haptics-settings": {} },
      },
    },
  },
]
