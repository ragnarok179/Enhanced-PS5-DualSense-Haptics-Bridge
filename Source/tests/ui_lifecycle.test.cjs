// Test UI activation/deactivation against controlled (possibly slow) BeamNG Lua APIs.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')

const file = path.resolve(__dirname, '../../BeamNG-Mod/ui/ui-vue/mods/dualsensehaptics/index.js')
const source = fs.readFileSync(file, 'utf8')
  .replace('import { lua } from "@/bridge"', 'const lua = globalThis.__testLua')
  .replace('import { ROUTE_SOURCE_ID, routeRecords } from "./routes.js"',
    'const ROUTE_SOURCE_ID = "test.routes"; const routeRecords = [{ name: "test.page", path: "/test", meta: {} }]')
  .replaceAll('export function ', 'function ')
  + '\nmodule.exports = { onLoad, onUnload }\n'

function deferred() {
  let resolve
  const promise = new Promise(done => { resolve = done })
  return { promise, resolve }
}

function fixture() {
  const state = { local: false, remote: false, button: false, calls: [], messages: [] }
  const gates = { register: null, button: null, throwRoute: false, refuseRoute: false }
  const uiRouter = {
    registerModRoutes: async () => {
      state.calls.push('registerRoute')
      if (gates.register) await gates.register.promise
      if (gates.throwRoute) throw Error('route registration failed')
      if (gates.refuseRoute) return { success: false, errors: ['invalid'] }
      state.remote = true
      return { success: true }
    },
    unregisterModRoutes: async () => {
      state.calls.push('unregisterRoute')
      state.remote = false
    },
  }
  const pauseActions = {
    registerModButton: async () => {
      state.calls.push('registerButton')
      if (gates.button) await gates.button.promise
      state.button = true
      return { success: true }
    },
    unregisterModButton: async () => {
      state.calls.push('unregisterButton')
      state.button = false
    },
  }
  const context = vm.createContext({
    module: { exports: {} }, Promise,
    console: { error: (...msg) => state.messages.push(msg), warn: (...msg) => state.messages.push(msg) },
    __testLua: { extensions: { ui_router_routeManager: uiRouter, ui_pause_actions: pauseActions } },
    window: { bngRoutes: {
      add: () => { state.calls.push('addLocal'); state.local = true },
      remove: () => { state.calls.push('removeLocal'); state.local = false },
    } },
  })
  vm.runInContext(source, context, { filename: file })
  return { state, gates, ...context.module.exports }
}

function assertClean(state, explanation) {
  assert.equal(state.local, false, explanation + ' local route')
  assert.equal(state.remote, false, explanation + ' Lua route')
  assert.equal(state.button, false, explanation + ' pause button')
}

async function main() {
  {
    const { state, onLoad, onUnload } = fixture()
    await onLoad()
    assert.equal(state.local, true)
    assert.equal(state.remote, true)
    assert.equal(state.button, true)
    await onLoad()
    assert.equal(state.calls.filter(v => v === 'registerButton').length, 1, 'idempotent load')
    await onUnload()
    await onUnload()
    assertClean(state, 'normal repeated unload')
  }
  {
    const { state, gates, onLoad, onUnload } = fixture()
    gates.register = deferred()
    const loading = onLoad()
    const unloading = onUnload()
    // Both intents are queued. Once the registration finishes, cleanup runs.
    gates.register.resolve()
    await Promise.all([loading, unloading])
    assertClean(state, 'slow route registration followed by disable')
    assert.ok(state.calls.indexOf('unregisterRoute') > state.calls.indexOf('registerRoute'))
  }
  {
    const { state, gates, onLoad, onUnload } = fixture()
    gates.button = deferred()
    const loading = onLoad()
    const unloading = onUnload()
    gates.button.resolve()
    await Promise.all([loading, unloading])
    assertClean(state, 'slow pause button registration followed by disable')
  }
  {
    const { state, gates, onLoad, onUnload } = fixture()
    gates.refuseRoute = true
    await onLoad()
    assertClean(state, 'rejected Lua route')
    gates.refuseRoute = false
    await onLoad()
    assert.equal(state.button, true, 'retry after failed UI registration')
    await onUnload()
    assertClean(state, 'retry cleanup')
  }
  {
    const { state, gates, onLoad, onUnload } = fixture()
    gates.throwRoute = true
    await onLoad()
    assertClean(state, 'thrown Lua route')
    await onUnload()
    assertClean(state, 'unload following throw')
  }
  process.stdout.write('UI lifecycle registration/unregistration: PASS\n')
}
main().catch(error => { console.error(error); process.exitCode = 1 })
