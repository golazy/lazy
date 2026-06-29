import DevPanelController from "/_golazy/assets/devpanel_controller.js"

const panelRootID = "golazy-dev-panel-root"
const clientState = window.__golazyDevPanelClient || {}
window.__golazyDevPanelClient = clientState

const controller = DevPanelController.connect(document.getElementById(panelRootID), { state: clientState })
window.disableDevPanel = () => {
  if (controller) {
    controller.disable()
    return
  }
  clientState.devToolsOpen = true
  window.__golazyDevToolsOpen = true
}

installReloadClient()

function installReloadClient() {
  if (location.pathname.startsWith("/_golazy")) return

  const client = clientState.reload || {}
  if (!client.source || client.source.readyState === EventSource.CLOSED) {
    const existingSource = window.__lazyReloadSource
    client.source = existingSource && existingSource.readyState !== EventSource.CLOSED ? existingSource : new EventSource("/__lazy/reload")
    window.__lazyReloadSource = client.source
    client.listenerInstalled = false
  }
  if (!client.onReload) {
    client.onReload = () => {
      if (!location.pathname.startsWith("/_golazy")) {
        location.reload()
      }
    }
  }
  if (!client.listenerInstalled) {
    client.source.addEventListener("reload", client.onReload)
    client.listenerInstalled = true
  }
  clientState.reload = client
}
