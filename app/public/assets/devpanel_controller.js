const embeddedPanelID = "golazy-dev-panel"
const embeddedPanelLauncherID = "golazy-dev-panel-launcher"
const embeddedPanelPaddingID = "golazy-dev-panel-padding"
const embeddedPanelDefaultHeight = 320
const embeddedPanelMinHeight = 180
const embeddedPanelMaxRatio = 0.85
const embeddedPanelResizeHandleHeight = 8
const embeddedPanelHeightKey = "golazy:devpanel:height"
const embeddedPanelClosedKey = "golazy:devpanel:closed"
const pageReadyMessage = "golazy:page:devpanel-ready"

export default class DevPanelController {
  static connect(root, options = {}) {
    if (!root || location.pathname.startsWith("/_golazy")) return null

    const state = options.state || {}
    if (state.devPanelController) {
      state.devPanelController.sync()
      return state.devPanelController
    }

    const controller = new DevPanelController(root, state)
    state.devPanelController = controller
    controller.connect()
    return controller
  }

  constructor(root, state) {
    this.root = root
    this.state = state
    this.panel = root.querySelector(`#${embeddedPanelID}`)
    this.padding = root.querySelector(`#${embeddedPanelPaddingID}`)
    this.launcher = root.querySelector(`#${embeddedPanelLauncherID}`)
    this.handle = root.querySelector("[data-golazy-dev-panel-resize-handle]")
    this.moveResize = this.moveResize.bind(this)
    this.stopResize = this.stopResize.bind(this)
    this.sync = this.sync.bind(this)
  }

  connect() {
    this.installMessages()
    this.installResize()
    this.installViewportResize()
    this.installTurboPersistence()
    this.announceReady()
    if (window.__golazyDevToolsOpen === true) {
      this.setDevToolsPanelOpen(true)
      return
    }
    this.sync()
  }

  disable() {
    this.state.extensionInstalled = true
    this.setDevToolsPanelOpen(true)
  }

  announceReady() {
    window.postMessage({ type: pageReadyMessage }, window.location.origin)
  }

  sync() {
    if (this.isDevToolsPanelOpen()) {
      this.hideAll()
      return
    }
    if (this.isEmbeddedPanelClosed()) {
      this.hidePanel()
      this.showLauncher()
      return
    }
    this.showPanel()
  }

  installMessages() {
    if (this.state.embeddedPanelMessagesInstalled) return
    this.state.embeddedPanelMessagesInstalled = true
    window.addEventListener("message", event => {
      if (event.origin !== window.location.origin) return
      switch (event.data?.type) {
      case "golazy:devpanel:close":
        this.closePanel()
        break
      case "golazy:extension:installed":
        this.setExtensionInstalled(true)
        break
      case "golazy:extension:toggle-inpage-panel":
        this.togglePanel()
        break
      case "golazy:extension:devtools-open":
        this.setDevToolsPanelOpen(event.data.open === true)
        break
      }
    })
  }

  installResize() {
    if (!this.handle || this.state.embeddedPanelResizeInstalled) return
    this.state.embeddedPanelResizeInstalled = true
    this.handle.addEventListener("pointerdown", event => {
      if (event.button !== undefined && event.button !== 0) return
      if (this.panel?.hidden) return
      event.preventDefault()
      this.resizing = true
      this.startY = event.clientY
      this.startHeight = this.currentPanelHeight()
      this.handle.classList.add("is-resizing")
      this.handle.setPointerCapture?.(event.pointerId)
      window.addEventListener("pointermove", this.moveResize)
      window.addEventListener("pointerup", this.stopResize)
      window.addEventListener("pointercancel", this.stopResize)
    })
    this.handle.addEventListener("keydown", event => {
      if (this.panel?.hidden) return
      const current = this.currentPanelHeight()
      if (event.key === "Home") {
        event.preventDefault()
        this.setPanelHeight(embeddedPanelMinHeight)
        return
      }
      if (event.key === "End") {
        event.preventDefault()
        this.setPanelHeight(this.maxPanelHeight())
        return
      }
      if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return
      event.preventDefault()
      const step = event.shiftKey ? 40 : 10
      this.setPanelHeight(current + (event.key === "ArrowUp" ? step : -step))
    })
  }

  moveResize(event) {
    if (!this.resizing) return
    event.preventDefault()
    this.setPanelHeight(this.startHeight + this.startY - event.clientY)
  }

  stopResize() {
    if (!this.resizing) return
    this.resizing = false
    this.handle?.classList.remove("is-resizing")
    window.removeEventListener("pointermove", this.moveResize)
    window.removeEventListener("pointerup", this.stopResize)
    window.removeEventListener("pointercancel", this.stopResize)
  }

  installViewportResize() {
    if (this.state.embeddedPanelViewportResizeInstalled) return
    this.state.embeddedPanelViewportResizeInstalled = true
    window.addEventListener("resize", () => {
      if (!this.panel?.hidden) this.setPanelHeight(this.currentPanelHeight())
    })
  }

  installTurboPersistence() {
    if (this.state.turboPersistenceInstalled) return
    this.state.turboPersistenceInstalled = true
    document.addEventListener("turbo:before-render", event => {
      const newBody = event.detail?.newBody
      if (this.root && newBody && !newBody.querySelector(`#${this.root.id}`)) {
        newBody.append(this.root)
      }
    })
    document.addEventListener("turbo:render", this.sync)
    document.addEventListener("turbo:load", this.sync)
  }

  showPanel() {
    if (!this.root || !this.panel || !this.padding) return
    this.root.hidden = false
    this.hideLauncher()
    this.setPanelHeight(this.currentPanelHeight())
    this.padding.hidden = false
    this.panel.hidden = false
  }

  hidePanel() {
    this.panel?.setAttribute("hidden", "")
    this.padding?.setAttribute("hidden", "")
  }

  hideAll() {
    this.hidePanel()
    this.hideLauncher()
    if (this.root) this.root.hidden = true
  }

  showLauncher() {
    if (!this.root || !this.launcher) return
    if (!this.shouldShowLauncher()) {
      this.hideLauncher()
      if (!this.isPanelOpen()) this.root.hidden = true
      return
    }
    this.root.hidden = false
    this.launcher.hidden = false
  }

  hideLauncher() {
    this.launcher?.setAttribute("hidden", "")
  }

  closePanel() {
    this.rememberPanelClosed()
    this.hidePanel()
    this.showLauncher()
  }

  openPanel() {
    if (this.isDevToolsPanelOpen()) return
    this.rememberPanelOpen()
    this.showPanel()
  }

  togglePanel() {
    if (this.isDevToolsPanelOpen()) {
      this.hideAll()
      return
    }
    if (this.panel && !this.panel.hidden) {
      this.closePanel()
      return
    }
    this.openPanel()
  }

  setDevToolsPanelOpen(open) {
    this.state.devToolsOpen = open === true
    window.__golazyDevToolsOpen = this.state.devToolsOpen
    this.sync()
  }

  setExtensionInstalled(installed) {
    this.state.extensionInstalled = installed === true
    this.sync()
  }

  setPanelHeight(height) {
    const next = this.clampPanelHeight(height)
    this.state.embeddedPanelHeight = next
    this.rememberPanelHeight(next)
    this.root?.style.setProperty("--golazy-dev-panel-height", `${next}px`)
    this.root?.style.setProperty("--golazy-dev-panel-resize-handle-height", `${embeddedPanelResizeHandleHeight}px`)
    this.updateResizeHandleA11y(next)
  }

  updateResizeHandleA11y(height) {
    if (!this.handle) return
    this.handle.setAttribute("aria-valuemin", String(embeddedPanelMinHeight))
    this.handle.setAttribute("aria-valuemax", String(this.maxPanelHeight()))
    this.handle.setAttribute("aria-valuenow", String(this.clampPanelHeight(height)))
  }

  currentPanelHeight() {
    if (Number.isFinite(this.state.embeddedPanelHeight)) {
      return this.clampPanelHeight(this.state.embeddedPanelHeight)
    }
    try {
      const value = Number.parseInt(window.sessionStorage.getItem(embeddedPanelHeightKey) || "", 10)
      if (Number.isFinite(value)) {
        this.state.embeddedPanelHeight = value
        return this.clampPanelHeight(value)
      }
    } catch {
      // The default height still works when sessionStorage is unavailable.
    }
    return this.clampPanelHeight(embeddedPanelDefaultHeight)
  }

  rememberPanelHeight(height) {
    try {
      window.sessionStorage.setItem(embeddedPanelHeightKey, String(this.clampPanelHeight(height)))
    } catch {
      // Resizing should still work when sessionStorage is unavailable.
    }
  }

  clampPanelHeight(height) {
    const value = Number.parseInt(height, 10)
    if (!Number.isFinite(value)) return embeddedPanelDefaultHeight
    return Math.min(Math.max(value, embeddedPanelMinHeight), this.maxPanelHeight())
  }

  maxPanelHeight() {
    const viewport = window.innerHeight || embeddedPanelDefaultHeight
    return Math.max(embeddedPanelMinHeight, Math.floor(viewport * embeddedPanelMaxRatio))
  }

  rememberPanelOpen() {
    try {
      window.sessionStorage.removeItem(embeddedPanelClosedKey)
    } catch {
      // Opening should still work when sessionStorage is unavailable.
    }
  }

  rememberPanelClosed() {
    try {
      window.sessionStorage.setItem(embeddedPanelClosedKey, "true")
    } catch {
      // Closing should still work when sessionStorage is unavailable.
    }
  }

  isEmbeddedPanelClosed() {
    try {
      return window.sessionStorage.getItem(embeddedPanelClosedKey) === "true"
    } catch {
      return false
    }
  }

  isExtensionInstalled() {
    return this.state.extensionInstalled === true
  }

  isDevToolsPanelOpen() {
    return this.state.devToolsOpen === true || window.__golazyDevToolsOpen === true
  }

  isPanelOpen() {
    return this.panel?.hidden === false
  }

  shouldShowLauncher() {
    return this.isEmbeddedPanelClosed() && !this.isPanelOpen() && !this.isExtensionInstalled() && !this.isDevToolsPanelOpen()
  }
}
