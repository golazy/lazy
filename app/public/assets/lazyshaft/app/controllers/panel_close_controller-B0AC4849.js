import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["button"]

  connect() {
    this.embedded = window.parent && window.parent !== window
    document.documentElement.dataset.golazyEmbedded = String(this.embedded)
    if (this.hasButtonTarget) {
      this.buttonTarget.hidden = !this.embedded
    }
  }

  close() {
    if (!this.embedded) return
    window.parent.postMessage({ type: "golazy:devpanel:close" }, window.location.origin)
  }
}
