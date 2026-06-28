import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["handle", "primary"]
  static values = {
    direction: { type: String, default: "right" },
    max: String,
    min: { type: String, default: "220px" },
    size: String,
  }

  connect() {
    this.boundMove = event => this.move(event)
    this.boundStop = event => this.stop(event)
    this.configureHandle()
    if (!this.hasPrimaryTarget) return

    if (this.hasSizeValue && this.sizeValue.trim() !== "") {
      this.updateSize(this.parseSize(this.sizeValue, this.currentSize()))
    } else if (this.boundsSize() > 0) {
      this.updateSize(this.currentSize())
    }
  }

  disconnect() {
    this.stop()
  }

  start(event) {
    if (event.button !== undefined && event.button !== 0) return
    if (!this.hasPrimaryTarget) return

    event.preventDefault()
    this.dragging = true
    this.pointerID = event.pointerId
    this.startPoint = this.coordinate(event)
    this.startSize = this.currentSize()
    this.element.classList.add("is-resizing")
    this.handleTarget.setPointerCapture?.(event.pointerId)
    window.addEventListener("pointermove", this.boundMove)
    window.addEventListener("pointerup", this.boundStop)
    window.addEventListener("pointercancel", this.boundStop)
  }

  move(event) {
    if (!this.dragging) return
    event.preventDefault()
    this.updateSize(this.startSize + (this.coordinate(event) - this.startPoint) * this.directionMultiplier())
  }

  stop() {
    if (!this.dragging) return
    this.dragging = false
    this.element.classList.remove("is-resizing")
    window.removeEventListener("pointermove", this.boundMove)
    window.removeEventListener("pointerup", this.boundStop)
    window.removeEventListener("pointercancel", this.boundStop)
  }

  nudge(event) {
    const direction = this.keyboardDirection(event.key)
    if (direction === 0 && event.key !== "Home" && event.key !== "End") {
      return
    }

    event.preventDefault()
    if (event.key === "Home") {
      this.updateSize(this.minSize())
      return
    }
    if (event.key === "End") {
      this.updateSize(this.maxSize())
      return
    }
    this.updateSize(this.currentSize() + direction * this.directionMultiplier() * (event.shiftKey ? 40 : 10))
  }

  configureHandle() {
    if (!this.hasHandleTarget) return
    this.handleTarget.setAttribute("role", "separator")
    this.handleTarget.setAttribute("tabindex", "0")
    this.handleTarget.setAttribute("aria-orientation", this.horizontal() ? "vertical" : "horizontal")
    this.handleTarget.setAttribute("aria-valuemin", String(Math.round(this.minSize())))
  }

  updateSize(value) {
    const size = Math.round(this.clamp(value))
    this.sizeValue = `${size}px`
    this.element.style.setProperty("--panel-resize-primary-size", `${size}px`)
    if (this.hasHandleTarget) {
      this.handleTarget.setAttribute("aria-valuenow", String(size))
      this.handleTarget.setAttribute("aria-valuemax", String(Math.round(this.maxSize())))
    }
  }

  currentSize() {
    const rect = this.primaryTarget.getBoundingClientRect()
    return this.horizontal() ? rect.width : rect.height
  }

  boundsSize() {
    const rect = this.element.getBoundingClientRect()
    return this.horizontal() ? rect.width : rect.height
  }

  coordinate(event) {
    return this.horizontal() ? event.clientX : event.clientY
  }

  keyboardDirection(key) {
    if (this.horizontal()) {
      if (key === "ArrowLeft") return -1
      if (key === "ArrowRight") return 1
    } else {
      if (key === "ArrowUp") return -1
      if (key === "ArrowDown") return 1
    }
    return 0
  }

  minSize() {
    return Math.max(0, this.parseSize(this.minValue, 0))
  }

  maxSize() {
    const min = this.minSize()
    const total = this.boundsSize()
    if (this.hasMaxValue && this.maxValue.trim() !== "") {
      return Math.max(min, this.parseSize(this.maxValue, total))
    }
    return Math.max(min, total - min)
  }

  clamp(value) {
    return Math.min(Math.max(value, this.minSize()), this.maxSize())
  }

  parseSize(value, fallback) {
    const text = String(value ?? "").trim()
    if (text === "") return fallback

    if (text.endsWith("%")) {
      const percent = Number.parseFloat(text.slice(0, -1))
      return Number.isFinite(percent) ? this.boundsSize() * percent / 100 : fallback
    }
    if (text.endsWith("px")) {
      const pixels = Number.parseFloat(text.slice(0, -2))
      return Number.isFinite(pixels) ? pixels : fallback
    }

    const pixels = Number.parseFloat(text)
    return Number.isFinite(pixels) ? pixels : fallback
  }

  horizontal() {
    return this.directionValue === "left" || this.directionValue === "right"
  }

  directionMultiplier() {
    return this.directionValue === "left" || this.directionValue === "top" ? -1 : 1
  }
}
