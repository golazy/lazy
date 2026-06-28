const panelPath = "/_golazy/"
const embeddedPanelID = "golazy-dev-panel"
const embeddedPanelHeight = "320px"
const embeddedPanelClosedKey = "golazy:devpanel:closed"
const clientState = window.__golazyDevPanelClient || {}
window.__golazyDevPanelClient = clientState

installEmbeddedPanel()
installTurboPersistence()
installEmbeddedPanelMessages()
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

function installEmbeddedPanel() {
  if (location.pathname.startsWith("/_golazy")) return
  if (isEmbeddedPanelClosed()) {
    releasePanelSpace()
    return
  }

  if (document.getElementById(embeddedPanelID)) return
  if (!document.body) {
    document.addEventListener("DOMContentLoaded", installEmbeddedPanel, { once: true })
    return
  }

  reservePanelSpace()
  const host = document.createElement("golazy-dev-panel")
  host.id = embeddedPanelID
  host.setAttribute("data-turbo-permanent", "")
  host.setAttribute("aria-label", "GoLazy development panel")

  const shadow = host.attachShadow({ mode: "open" })
  const style = document.createElement("style")
  style.textContent = `
    :host {
      --golazy-dev-panel-height: ${embeddedPanelHeight};
      bottom: 0;
      display: block;
      height: calc(var(--golazy-dev-panel-height) + env(safe-area-inset-bottom));
      left: 0;
      position: fixed;
      right: 0;
      z-index: 2147483647;
    }
    iframe {
      background: #202124;
      border: 0;
      border-top: 1px solid #3c4043;
      display: block;
      height: 100%;
      width: 100%;
    }
  `
  const frame = document.createElement("iframe")
  frame.src = panelPath
  frame.title = "GoLazy development panel"
  shadow.append(style, frame)
  document.body.append(host)
}

function reservePanelSpace() {
  const root = document.documentElement
  if (!root.dataset.golazyDevPanelPaddingBase) {
    root.dataset.golazyDevPanelPaddingBase = getComputedStyle(root).paddingBottom || "0px"
  }
  root.style.setProperty("--golazy-dev-panel-height", embeddedPanelHeight)
  root.style.paddingBottom = `calc(${root.dataset.golazyDevPanelPaddingBase} + var(--golazy-dev-panel-height) + env(safe-area-inset-bottom))`
}

function releasePanelSpace() {
  const root = document.documentElement
  if (!root.dataset.golazyDevPanelPaddingBase) return
  root.style.paddingBottom = root.dataset.golazyDevPanelPaddingBase
  root.style.removeProperty("--golazy-dev-panel-height")
}

function installEmbeddedPanelMessages() {
  if (clientState.embeddedPanelMessagesInstalled) return
  clientState.embeddedPanelMessagesInstalled = true
  window.addEventListener("message", event => {
    if (event.origin !== window.location.origin) return
    if (event.data?.type !== "golazy:devpanel:close") return
    closeEmbeddedPanel()
  })
}

function closeEmbeddedPanel() {
  rememberEmbeddedPanelClosed()
  document.getElementById(embeddedPanelID)?.remove()
  releasePanelSpace()
}

function isEmbeddedPanelClosed() {
  try {
    return window.sessionStorage.getItem(embeddedPanelClosedKey) === "true"
  } catch {
    return false
  }
}

function rememberEmbeddedPanelClosed() {
  try {
    window.sessionStorage.setItem(embeddedPanelClosedKey, "true")
  } catch {
    // Closing should still work when sessionStorage is unavailable.
  }
}

function installTurboPersistence() {
  if (clientState.turboPersistenceInstalled) return
  clientState.turboPersistenceInstalled = true
  document.addEventListener("turbo:before-render", event => {
    const host = document.getElementById(embeddedPanelID)
    const newBody = event.detail?.newBody
    if (host && newBody && !newBody.querySelector(`#${embeddedPanelID}`)) {
      newBody.append(host)
    }
  })
  document.addEventListener("turbo:render", installEmbeddedPanel)
  document.addEventListener("turbo:load", installEmbeddedPanel)
}
