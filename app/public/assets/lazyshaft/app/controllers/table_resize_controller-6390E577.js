import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  connect() {
    this.table = this.element
    this.boundMove = event => this.move(event)
    this.boundStop = () => this.stop()
    this.boundSelectRow = event => this.selectRow(event)
    this.installColumns()
    this.installHandles()
    this.installRowSelection()
    this.installResizeObserver()
  }

  disconnect() {
    this.stop()
    this.table?.removeEventListener("click", this.boundSelectRow)
    this.resizeObserver?.disconnect()
    this.removeHandles()
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
    this.headers = this.headerCells()
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
    this.setWidths(this.widthsForAvailableSpace(this.storedWidths() || widths))
  }

  installHandles() {
    this.removeHandles()
    this.headers?.forEach((header, index) => {
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

  removeHandles() {
    this.table?.querySelectorAll("thead .table-resize-handle")?.forEach(handle => handle.remove())
  }

  headerCells() {
    const rows = Array.from(this.table.tHead?.rows || [])
    if (rows.length === 0) return []

    const grid = []
    const cells = []
    rows.forEach((row, rowIndex) => {
      grid[rowIndex] ||= []
      let columnIndex = 0
      Array.from(row.cells).forEach(header => {
        while (grid[rowIndex][columnIndex]) columnIndex++

        const colSpan = Math.max(1, Number.parseInt(header.getAttribute("colspan") || header.colSpan || "1", 10) || 1)
        const rowSpan = Math.max(1, Number.parseInt(header.getAttribute("rowspan") || header.rowSpan || "1", 10) || 1)
        cells.push({ header, rowIndex, columnIndex, colSpan, rowSpan })

        for (let rowOffset = 0; rowOffset < rowSpan; rowOffset++) {
          const targetRow = rowIndex + rowOffset
          grid[targetRow] ||= []
          for (let columnOffset = 0; columnOffset < colSpan; columnOffset++) {
            grid[targetRow][columnIndex + columnOffset] = header
          }
        }
        columnIndex += colSpan
      })
    })

    return cells
      .filter(cell => cell.colSpan === 1 && cell.rowIndex + cell.rowSpan >= rows.length)
      .sort((left, right) => left.columnIndex - right.columnIndex)
      .map(cell => cell.header)
  }

  installRowSelection() {
    this.table.addEventListener("click", this.boundSelectRow)
  }

  installResizeObserver() {
    if (!window.ResizeObserver) return

    const target = this.resizeTarget()
    this.lastAvailableWidth = Math.round(this.availableWidth())
    this.resizeObserver = new window.ResizeObserver(() => this.fitAvailableSpace())
    this.resizeObserver.observe(target)
  }

  selectRow(event) {
    const link = event.target?.closest?.("a[data-turbo-frame]")
    if (!link || !this.table.contains(link) || link.dataset.turboFrame === "_top") return

    const row = link.closest("tbody tr")
    if (!row || !this.table.contains(row)) return

    const rows = Array.from(row.parentElement.querySelectorAll("tr[aria-selected], tr.is-selected"))
    const useClassSelection = rows.some(candidate => candidate.classList.contains("is-selected")) || !row.hasAttribute("aria-selected")
    rows.forEach(candidate => {
      if (candidate.hasAttribute("aria-selected")) {
        candidate.setAttribute("aria-selected", candidate === row ? "true" : "false")
      }
      if (useClassSelection) {
        candidate.classList.toggle("is-selected", candidate === row)
      }
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

  fitAvailableSpace() {
    if (this.activeIndex !== undefined || !this.cols?.length) return

    const available = Math.round(this.availableWidth())
    if (!Number.isFinite(available) || available <= 0 || available === this.lastAvailableWidth) return
    this.lastAvailableWidth = available
    this.setWidths(this.widthsForAvailableSpace(this.widths()))
  }

  widthsForAvailableSpace(widths) {
    const available = this.availableWidth()
    if (!Number.isFinite(available) || available <= 0) return widths
    return this.widthsForTotal(widths, Math.round(available))
  }

  widthsForTotal(widths, targetTotal) {
    const minimums = this.minimums || this.minimumWidths()
    const minimumTotal = minimums.reduce((sum, width) => sum + width, 0)
    const target = Math.max(minimumTotal, targetTotal)
    const normalized = widths.map((width, index) => Math.max(minimums[index], Number(width) || minimums[index]))
    const currentTotal = normalized.reduce((sum, width) => sum + width, 0)
    if (!Number.isFinite(currentTotal) || currentTotal <= 0) return minimums
    if (Math.abs(currentTotal - target) < 1) return this.roundWidths(normalized, target, minimums)

    if (target > currentTotal) {
      return this.roundWidths(normalized.map(width => width * target / currentTotal), target, minimums)
    }

    const adjusted = normalized.slice()
    let remainingShrink = currentTotal - target
    let active = adjusted.map((_, index) => index).filter(index => adjusted[index] > minimums[index])
    while (remainingShrink > 0.01 && active.length > 0) {
      const activeTotal = active.reduce((sum, index) => sum + adjusted[index], 0)
      let clamped = false
      for (const index of active) {
        const shrink = remainingShrink * adjusted[index] / activeTotal
        if (adjusted[index] - shrink <= minimums[index]) {
          remainingShrink -= adjusted[index] - minimums[index]
          adjusted[index] = minimums[index]
          clamped = true
        }
      }
      active = active.filter(index => adjusted[index] > minimums[index])
      if (!clamped) {
        for (const index of active) {
          adjusted[index] -= remainingShrink * adjusted[index] / activeTotal
        }
        remainingShrink = 0
      }
    }
    return this.roundWidths(adjusted, target, minimums)
  }

  roundWidths(widths, targetTotal, minimums) {
    const rounded = widths.map((width, index) => Math.max(minimums[index], Math.round(width)))
    let delta = targetTotal - rounded.reduce((sum, width) => sum + width, 0)
    let index = 0
    while (delta > 0 && rounded.length > 0) {
      rounded[index % rounded.length] += 1
      delta--
      index++
    }
    while (delta < 0) {
      const shrinkable = rounded.findIndex((width, widthIndex) => width > minimums[widthIndex])
      if (shrinkable < 0) break
      rounded[shrinkable] -= 1
      delta++
    }
    return rounded
  }

  resizeTarget() {
    return this.table.parentElement || this.table
  }

  availableWidth() {
    return this.resizeTarget().getBoundingClientRect().width
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
