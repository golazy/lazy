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
    this.storeWidths()
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
    this.storageKey = this.storageKeyForTable()
    this.setWidths(this.storedWidths() || widths)
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
        let remaining = delta
        let index = this.activeIndex + 1
        let totalShrink = 0
        while (remaining > 0 && index < widths.length) {
          const shrink = Math.min(remaining, Math.max(0, widths[index] - this.minimums[index]))
          widths[index] -= shrink
          totalShrink += shrink
          remaining -= shrink
          if (remaining > 0) index++
        }
        widths[this.activeIndex] += totalShrink
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
    return this.currentWidths?.slice() || this.cols.map(col => Number.parseFloat(col.style.width) || col.getBoundingClientRect().width)
  }

  minimumWidths() {
    return this.headers.map(header => {
      const value = Number.parseFloat(header.dataset.tableResizeMinWidthValue || header.dataset.tableResizeMinWidth)
      return Number.isFinite(value) ? Math.max(10, value) : 10
    })
  }

  setWidths(widths) {
    const minimums = this.minimums || this.minimumWidths()
    const normalized = widths.map((width, index) => Math.max(minimums[index], Math.round(width)))
    normalized.forEach((width, index) => {
      this.cols[index].style.width = `${width}px`
    })
    this.currentWidths = normalized
    const total = normalized.reduce((sum, width) => sum + width, 0)
    this.table.style.width = `${total}px`
  }

  storageKeyForTable() {
    const explicit = this.table.dataset.tableResizeStorageKeyValue || this.table.dataset.tableResizeStorageKey
    if (explicit) return `golazy:devpanel:table-widths:${explicit}`

    const id = this.table.id ? `#${this.table.id}` : ""
    const classes = Array.from(this.table.classList)
      .filter(name => name !== "is-column-resizing")
      .sort()
      .join(".")
    const headers = this.headers
      .map(header => header.textContent.trim().replace(/\s+/g, " "))
      .join("|")
    return `golazy:devpanel:table-widths:${id || classes || "table"}:${headers}`
  }

  storedWidths() {
    try {
      const raw = window.localStorage?.getItem(this.storageKey)
      if (!raw) return null
      const parsed = JSON.parse(raw)
      if (!Array.isArray(parsed) || parsed.length !== this.cols.length) return null
      const widths = parsed.map(width => Number(width))
      return widths.every(width => Number.isFinite(width)) ? widths : null
    } catch {
      return null
    }
  }

  storeWidths() {
    try {
      window.localStorage?.setItem(this.storageKey, JSON.stringify(this.widths()))
    } catch {
      // localStorage may be unavailable in private or restricted browser modes.
    }
  }
}
