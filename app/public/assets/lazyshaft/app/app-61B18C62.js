import "@hotwired/turbo"
import { Application } from "@hotwired/stimulus"
import PanelCloseController from "/js/controllers/panel_close_controller.js"
import PanelResizeController from "/js/controllers/panel_resize_controller.js"

const application = Application.start()
application.register("panel-close", PanelCloseController)
application.register("panel-resize", PanelResizeController)

const panelPath = "/_golazy/"
const embeddedPanelID = "golazy-dev-panel"
const embeddedPanelDefaultHeight = 320
const embeddedPanelMinHeight = 180
const embeddedPanelMaxRatio = 0.85
const embeddedPanelResizeHandleHeight = 8
const embeddedPanelHeightKey = "golazy:devpanel:height"
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

  const existing = document.getElementById(embeddedPanelID)
  if (existing) {
    const height = currentEmbeddedPanelHeight()
    reservePanelSpace(height)
    setPanelHostHeight(existing, height)
    return
  }
  if (!document.body) {
    document.addEventListener("DOMContentLoaded", installEmbeddedPanel, { once: true })
    return
  }

  const height = currentEmbeddedPanelHeight()
  reservePanelSpace(height)
  const host = document.createElement("golazy-dev-panel")
  host.id = embeddedPanelID
  host.setAttribute("data-turbo-permanent", "")
  host.setAttribute("aria-label", "GoLazy development panel")
  setPanelHostHeight(host, height)

  const shadow = host.attachShadow({ mode: "open" })
  const style = document.createElement("style")
  style.textContent = `
    :host {
      --golazy-dev-panel-height: ${height}px;
      --golazy-dev-panel-resize-handle-height: ${embeddedPanelResizeHandleHeight}px;
      bottom: 0;
      display: block;
      height: calc(var(--golazy-dev-panel-height) + var(--golazy-dev-panel-resize-handle-height) + env(safe-area-inset-bottom));
      left: 0;
      position: fixed;
      right: 0;
      z-index: 2147483647;
    }
    .resize-handle {
      align-items: center;
      background: #202124;
      border-top: 1px solid #3c4043;
      cursor: ns-resize;
      display: flex;
      height: var(--golazy-dev-panel-resize-handle-height);
      justify-content: center;
      outline: none;
      touch-action: none;
      user-select: none;
      width: 100%;
    }
    .resize-handle::before {
      background: #8ab4f8;
      border-radius: 999px;
      content: "";
      display: block;
      height: 2px;
      opacity: 0.72;
      width: 44px;
    }
    .resize-handle:hover::before,
    .resize-handle:focus-visible::before,
    .resize-handle.is-resizing::before {
      opacity: 1;
    }
    iframe {
      background: #202124;
      border: 0;
      display: block;
      height: calc(100% - var(--golazy-dev-panel-resize-handle-height));
      width: 100%;
    }
  `
  const handle = document.createElement("div")
  handle.className = "resize-handle"
  handle.setAttribute("aria-label", "Resize GoLazy development panel")
  handle.setAttribute("aria-orientation", "horizontal")
  handle.setAttribute("role", "separator")
  handle.setAttribute("tabindex", "0")
  installEmbeddedPanelResize(handle, host)
  const frame = document.createElement("iframe")
  frame.src = panelPath
  frame.title = "GoLazy development panel"
  shadow.append(style, handle, frame)
  document.body.append(host)
  updateResizeHandleA11y(handle, height)
  installEmbeddedPanelViewportResize()
}

function reservePanelSpace(height) {
  const root = document.documentElement
  if (!root.dataset.golazyDevPanelPaddingBase) {
    root.dataset.golazyDevPanelPaddingBase = getComputedStyle(root).paddingBottom || "0px"
  }
  root.style.setProperty("--golazy-dev-panel-height", `${clampEmbeddedPanelHeight(height)}px`)
  root.style.setProperty("--golazy-dev-panel-resize-handle-height", `${embeddedPanelResizeHandleHeight}px`)
  root.style.paddingBottom = `calc(${root.dataset.golazyDevPanelPaddingBase} + var(--golazy-dev-panel-height) + var(--golazy-dev-panel-resize-handle-height) + env(safe-area-inset-bottom))`
}

function releasePanelSpace() {
  const root = document.documentElement
  if (!root.dataset.golazyDevPanelPaddingBase) return
  root.style.paddingBottom = root.dataset.golazyDevPanelPaddingBase
  root.style.removeProperty("--golazy-dev-panel-height")
  root.style.removeProperty("--golazy-dev-panel-resize-handle-height")
}

function installEmbeddedPanelResize(handle, host) {
  let startY = 0
  let startHeight = 0
  let dragging = false

  const move = event => {
    if (!dragging) return
    event.preventDefault()
    setEmbeddedPanelHeight(startHeight + startY - event.clientY, { host, handle })
  }
  const stop = () => {
    if (!dragging) return
    dragging = false
    handle.classList.remove("is-resizing")
    window.removeEventListener("pointermove", move)
    window.removeEventListener("pointerup", stop)
    window.removeEventListener("pointercancel", stop)
  }

  handle.addEventListener("pointerdown", event => {
    if (event.button !== undefined && event.button !== 0) return
    event.preventDefault()
    dragging = true
    startY = event.clientY
    startHeight = currentEmbeddedPanelHeight()
    handle.classList.add("is-resizing")
    handle.setPointerCapture?.(event.pointerId)
    window.addEventListener("pointermove", move)
    window.addEventListener("pointerup", stop)
    window.addEventListener("pointercancel", stop)
  })
  handle.addEventListener("keydown", event => {
    const current = currentEmbeddedPanelHeight()
    if (event.key === "Home") {
      event.preventDefault()
      setEmbeddedPanelHeight(embeddedPanelMinHeight, { host, handle })
      return
    }
    if (event.key === "End") {
      event.preventDefault()
      setEmbeddedPanelHeight(maxEmbeddedPanelHeight(), { host, handle })
      return
    }
    if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return
    event.preventDefault()
    const step = event.shiftKey ? 40 : 10
    setEmbeddedPanelHeight(current + (event.key === "ArrowUp" ? step : -step), { host, handle })
  })
}

function setEmbeddedPanelHeight(height, options = {}) {
  const next = clampEmbeddedPanelHeight(height)
  clientState.embeddedPanelHeight = next
  rememberEmbeddedPanelHeight(next)
  reservePanelSpace(next)
  setPanelHostHeight(options.host || document.getElementById(embeddedPanelID), next)
  updateResizeHandleA11y(options.handle, next)
}

function setPanelHostHeight(host, height) {
  if (!host) return
  host.style.setProperty("--golazy-dev-panel-height", `${clampEmbeddedPanelHeight(height)}px`)
  host.style.setProperty("--golazy-dev-panel-resize-handle-height", `${embeddedPanelResizeHandleHeight}px`)
}

function updateResizeHandleA11y(handle, height) {
  if (!handle) return
  handle.setAttribute("aria-valuemin", String(embeddedPanelMinHeight))
  handle.setAttribute("aria-valuemax", String(maxEmbeddedPanelHeight()))
  handle.setAttribute("aria-valuenow", String(clampEmbeddedPanelHeight(height)))
}

function installEmbeddedPanelViewportResize() {
  if (clientState.embeddedPanelViewportResizeInstalled) return
  clientState.embeddedPanelViewportResizeInstalled = true
  window.addEventListener("resize", () => setEmbeddedPanelHeight(currentEmbeddedPanelHeight()))
}

function currentEmbeddedPanelHeight() {
  if (Number.isFinite(clientState.embeddedPanelHeight)) {
    return clampEmbeddedPanelHeight(clientState.embeddedPanelHeight)
  }
  try {
    const value = Number.parseInt(window.sessionStorage.getItem(embeddedPanelHeightKey) || "", 10)
    if (Number.isFinite(value)) {
      clientState.embeddedPanelHeight = value
      return clampEmbeddedPanelHeight(value)
    }
  } catch {
    // The default height still works when sessionStorage is unavailable.
  }
  return clampEmbeddedPanelHeight(embeddedPanelDefaultHeight)
}

function rememberEmbeddedPanelHeight(height) {
  try {
    window.sessionStorage.setItem(embeddedPanelHeightKey, String(clampEmbeddedPanelHeight(height)))
  } catch {
    // Resizing should still work when sessionStorage is unavailable.
  }
}

function clampEmbeddedPanelHeight(height) {
  const value = Number.parseInt(height, 10)
  if (!Number.isFinite(value)) return embeddedPanelDefaultHeight
  return Math.min(Math.max(value, embeddedPanelMinHeight), maxEmbeddedPanelHeight())
}

function maxEmbeddedPanelHeight() {
  const viewport = window.innerHeight || embeddedPanelDefaultHeight
  return Math.max(embeddedPanelMinHeight, Math.floor(viewport * embeddedPanelMaxRatio))
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
