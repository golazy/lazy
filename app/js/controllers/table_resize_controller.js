import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  connect() {
    this.table = this.element
    this.boundMove = event => this.move(event)
    this.boundStop = () => this.stop()
    this.installColumns()
    this.installHandles()
  }

  disconnect() {
    this.stop()
    this.headers?.forEach(header => header.querySelector(".table-resize-handle")?.remove())
  }

  start(event) {
    if (event.button !== undefined && event.button !== 0) return
    const handle = event.currentTarget
    this.activeIndex = Number.parseInt(handle.dataset.columnIndex, 10)
    if (!Number.isFinite(this.activeIndex)) return

    event.preventDefault()
    this.startX = event.clientX
    this.startWidths = this.widths()
    this.minimums = this.minimumWidths()
    this.table.classList.add("is-column-resizing")
    handle.setPointerCapture?.(event.pointerId)
    window.addEventListener("pointermove", this.boundMove)
    window.addEventListener("pointerup", this.boundStop)
    window.addEventListener("pointercancel", this.boundStop)
  }

  move(event) {
    if (this.activeIndex === undefined) return
    event.preventDefault()
    this.applyResize(event.clientX - this.startX)
  }

  stop() {
    if (this.activeIndex === undefined) return
    this.activeIndex = undefined
    this.table.classList.remove("is-column-resizing")
    window.removeEventListener("pointermove", this.boundMove)
    window.removeEventListener("pointerup", this.boundStop)
    window.removeEventListener("pointercancel", this.boundStop)
  }

  installColumns() {
    this.headers = Array.from(this.table.querySelectorAll("thead th"))
    if (this.headers.length === 0) return

    let colgroup = this.table.querySelector("colgroup")
    if (!colgroup) {
      colgroup = document.createElement("colgroup")
      this.table.insertBefore(colgroup, this.table.firstChild)
    }
    while (colgroup.children.length < this.headers.length) {
      colgroup.append(document.createElement("col"))
    }
    while (colgroup.children.length > this.headers.length) {
      colgroup.lastElementChild.remove()
    }
    this.cols = Array.from(colgroup.children)
    const widths = this.headers.map(header => Math.max(10, header.getBoundingClientRect().width))
    this.setWidths(widths)
  }

  installHandles() {
    this.headers?.forEach((header, index) => {
      header.querySelector(".table-resize-handle")?.remove()
      const handle = document.createElement("span")
      handle.className = "table-resize-handle"
      handle.dataset.columnIndex = String(index)
      handle.setAttribute("role", "separator")
      handle.setAttribute("aria-orientation", "vertical")
      handle.setAttribute("tabindex", "-1")
      handle.addEventListener("pointerdown", event => this.start(event))
      header.append(handle)
    })
  }

  applyResize(delta) {
    const widths = this.startWidths.slice()
    if (delta >= 0) {
      if (this.activeIndex === widths.length - 1) {
        widths[this.activeIndex] += delta
      } else {
        const right = this.activeIndex + 1
        const shrink = Math.min(delta, Math.max(0, widths[right] - this.minimums[right]))
        widths[this.activeIndex] += shrink
        widths[right] -= shrink
      }
      this.setWidths(widths)
      return
    }

    let remaining = -delta
    let index = this.activeIndex
    let totalShrink = 0
    while (remaining > 0 && index >= 0) {
      const shrink = Math.min(remaining, Math.max(0, widths[index] - this.minimums[index]))
      widths[index] -= shrink
      totalShrink += shrink
      remaining -= shrink
      if (remaining > 0) index--
    }
    if (this.activeIndex < widths.length - 1) {
      widths[this.activeIndex + 1] += totalShrink
    }
    this.setWidths(widths)
  }

  widths() {
    return this.cols.map(col => Number.parseFloat(col.style.width) || col.getBoundingClientRect().width)
  }

  minimumWidths() {
    return this.headers.map(header => {
      const value = Number.parseFloat(header.dataset.tableResizeMinWidthValue || header.dataset.tableResizeMinWidth)
      return Number.isFinite(value) ? Math.max(10, value) : 10
    })
  }

  setWidths(widths) {
    const minimums = this.minimums || this.minimumWidths()
    widths.forEach((width, index) => {
      this.cols[index].style.width = `${Math.max(minimums[index], Math.round(width))}px`
    })
    const total = widths.reduce((sum, width, index) => sum + Math.max(minimums[index], Math.round(width)), 0)
    this.table.style.width = `${total}px`
  }
}
