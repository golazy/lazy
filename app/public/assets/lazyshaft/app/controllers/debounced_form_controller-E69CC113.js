import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static values = {
    countSource: String,
    countTarget: String,
    delay: { type: Number, default: 250 },
    frame: String,
    streamSource: String,
  }

  connect() {
    this.sequence = 0
    this.timeout = null
    this.abortController = null
  }

  disconnect() {
    this.clearTimer()
    this.abortCurrent()
  }

  queue() {
    this.clearTimer()
    this.timeout = window.setTimeout(() => this.request(), this.delayValue)
  }

  submit(event) {
    event?.preventDefault()
    this.clearTimer()
    this.request()
  }

  request() {
    if (this.hasStreamSourceValue) {
      this.replaceStreamSource()
      return
    }
    this.fetchFrame()
  }

  fetchFrame() {
    const frame = this.targetFrame()
    if (!frame) return

    const sequence = ++this.sequence
    this.abortCurrent()
    this.abortController = new AbortController()

    window.fetch(this.requestURL(), {
      credentials: "same-origin",
      headers: {
        Accept: "text/html, application/xhtml+xml",
        "Turbo-Frame": frame.id,
      },
      method: "GET",
      signal: this.abortController.signal,
    }).then(response => {
      if (!response.ok) throw new Error(`Frame request failed with ${response.status}`)
      return response.text()
    }).then(html => {
      if (sequence !== this.sequence) return
      this.replaceFrame(frame, html)
    }).catch(error => {
      if (error.name === "AbortError") return
      console.error(error)
    })
  }

  requestURL() {
    const url = new URL(this.element.action || window.location.href, window.location.href)
    const params = new URLSearchParams(new FormData(this.element))
    url.search = params.toString()
    return url
  }

  targetFrame() {
    const id = this.frameValue || this.element.dataset.turboFrame
    return id ? document.getElementById(id) : null
  }

  targetStreamSource() {
    return this.streamSourceValue ? document.querySelector(this.streamSourceValue) : null
  }

  replaceStreamSource() {
    const source = this.targetStreamSource()
    if (!source) return

    const replacement = source.cloneNode(false)
    replacement.setAttribute("src", this.requestURL().toString())
    source.replaceWith(replacement)
  }

  replaceFrame(frame, html) {
    const template = document.createElement("template")
    template.innerHTML = html.trim()
    const replacement = Array.from(template.content.querySelectorAll("turbo-frame")).find(candidate => candidate.id === frame.id)
    if (!replacement) return

    if (this.hasCountSourceValue && this.hasCountTargetValue) {
      const count = replacement.querySelector(this.countSourceValue)
      const visibleCount = document.querySelector(this.countTargetValue)
      if (count && visibleCount) {
        visibleCount.textContent = count.textContent
      }
    }

    frame.replaceChildren(...Array.from(replacement.childNodes))
  }

  clearTimer() {
    if (this.timeout === null) return
    window.clearTimeout(this.timeout)
    this.timeout = null
  }

  abortCurrent() {
    this.abortController?.abort()
    this.abortController = null
  }
}
