const panelPath = "/_golazy/"
const embeddedPanelID = "golazy-dev-panel"
const embeddedPanelHeight = "320px"
const embeddedPanelClosedKey = "golazy:devpanel:closed"

installEmbeddedPanel()
installTurboPersistence()
installEmbeddedPanelMessages()

const reloadSource = window.__lazyReloadSource || new EventSource("/__lazy/reload")
window.__lazyReloadSource = reloadSource
reloadSource.addEventListener("reload", () => {
  if (!location.pathname.startsWith("/_golazy")) {
    location.reload()
  }
})

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
