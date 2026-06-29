import { Controller } from "@hotwired/stimulus"

const visitMessage = "golazy:devpanel:visit"

export default class extends Controller {
  static values = {
    url: String,
  }

  visit(event) {
    if (!this.embedded()) return
    const url = this.urlValue || this.element.getAttribute("href")
    if (!url) return
    event.preventDefault()
    window.parent.postMessage({ type: visitMessage, url }, window.location.origin)
  }

  embedded() {
    return window.parent && window.parent !== window
  }
}
