import { Controller } from "@hotwired/stimulus"

const lastTabKey = "golazy:devpanel:last-tab"

export default class extends Controller {
  static values = {
    defaultPath: String,
  }

  initialize() {
    this.rememberClickedTab = this.rememberClickedTab.bind(this)
  }

  connect() {
    document.addEventListener("click", this.rememberClickedTab, true)
    this.restoreOrRememberCurrentTab()
  }

  disconnect() {
    document.removeEventListener("click", this.rememberClickedTab, true)
  }

  rememberClickedTab(event) {
    const target = event.target
    const link = target instanceof Element ? target.closest("a[href]") : null
    if (link) this.rememberTab(link.href)
  }

  restoreOrRememberCurrentTab() {
    const current = this.panelPath(window.location.href)
    const remembered = this.rememberedTab()
    if (current && this.isDefaultTab(current) && remembered && remembered !== current) {
      this.visitPanel(remembered)
      return
    }
    if (current) this.writeLastTab(current)
  }

  isDefaultTab(path) {
    return path === this.defaultPanelPath()
  }

  defaultPanelPath() {
    return this.panelPath(this.defaultPathValue || "/_golazy/app") || "/_golazy/app"
  }

  rememberTab(value) {
    const path = this.panelPath(value)
    if (!this.validPanelTab(path)) return
    this.writeLastTab(path)
  }

  rememberedTab() {
    try {
      const path = this.panelPath(window.sessionStorage.getItem(lastTabKey) || "")
      if (this.validPanelTab(path)) return path
    } catch {
      // The panel still opens at its default tab when sessionStorage is unavailable.
    }
    return ""
  }

  writeLastTab(path) {
    try {
      window.sessionStorage.setItem(lastTabKey, path)
    } catch {
      // Navigation should keep working when sessionStorage is unavailable.
    }
  }

  validPanelTab(path) {
    return path.startsWith("/_golazy") && path !== "/_golazy/status"
  }

  panelPath(value) {
    try {
      const url = new URL(value, window.location.href)
      if (url.origin !== window.location.origin) return ""
      const path = url.pathname.replace(/\/+$/, "")
      return path || "/"
    } catch {
      return ""
    }
  }

  visitPanel(path) {
    const turbo = window.Turbo
    if (turbo && typeof turbo.visit === "function") {
      turbo.visit(path, { action: "replace" })
      return
    }
    window.location.replace(path)
  }
}
