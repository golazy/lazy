import { Controller } from "/_golazy/assets/lazyshaft/stimulus-FNW3RRR2.js"

export default class extends Controller {
  static targets = ["handle", "primary"]
  static values = {
    axis: { type: String, default: "horizontal" },
    max: Number,
    min: { type: Number, default: 220 },
    size: Number,
  }

  connect() {
    this.boundMove = event => this.move(event)
    this.boundStop = event => this.stop(event)
    this.configureHandle()
    if (this.hasSizeValue) {
      this.updateSize(this.sizeValue)
    } else if (this.hasPrimaryTarget) {
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
    this.updateSize(this.startSize + this.coordinate(event) - this.startPoint)
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
    if (direction === 0 && event.key !== "Home" && event.key !== "End") return

    event.preventDefault()
    if (event.key === "Home") {
      this.updateSize(this.minValue)
      return
    }
    if (event.key === "End") {
      this.updateSize(this.maxSize())
      return
    }
    this.updateSize(this.currentSize() + direction * (event.shiftKey ? 40 : 10))
  }

  configureHandle() {
    if (!this.hasHandleTarget) return
    this.handleTarget.setAttribute("role", "separator")
    this.handleTarget.setAttribute("tabindex", "0")
    this.handleTarget.setAttribute("aria-orientation", this.horizontal() ? "vertical" : "horizontal")
    this.handleTarget.setAttribute("aria-valuemin", String(this.minValue))
  }

  updateSize(value) {
    const size = Math.round(this.clamp(value))
    this.sizeValue = size
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

  maxSize() {
    if (this.hasMaxValue) return Math.max(this.minValue, this.maxValue)
    const rect = this.element.getBoundingClientRect()
    const total = this.horizontal() ? rect.width : rect.height
    return Math.max(this.minValue, total - this.minValue)
  }

  clamp(value) {
    return Math.min(Math.max(value, this.minValue), this.maxSize())
  }

  horizontal() {
    return this.axisValue !== "vertical"
  }
}
