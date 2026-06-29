import { Controller } from "@hotwired/stimulus"

const svgNamespace = "http://www.w3.org/2000/svg"

export default class extends Controller {
  static targets = ["table", "output", "ready", "activeRequests", "activeConnections", "phase", "message"]
  static values = { eventsUrl: String }

  connect() {
    this.render()
    this.connectEvents()
  }

  disconnect() {
    this.disconnectEvents()
  }

  render() {
    if (!this.hasTableTarget || !this.hasOutputTarget) return

    const graph = this.graphFromRows()
    if (graph.nodes.length === 0) return

    this.outputTarget.replaceChildren(this.svgForGraph(graph))
    this.outputTarget.hidden = false
    this.tableTarget.hidden = true
  }

  graphFromRows() {
    const nodeMap = new Map()
    const edges = []
    const seenEdges = new Set()

    this.tableTarget.querySelectorAll("tbody tr[data-depgraph-name], tbody tr[data-controller-depgraph-name]").forEach(row => {
      const name = this.valueFor(row, "name")
      if (!name) return

      const node = nodeMap.get(name) || { name, dependsOn: [], usedBy: [], state: "running" }
      node.dependsOn = this.namesFromValue(this.valueFor(row, "dependsOn"))
      node.usedBy = this.namesFromValue(this.valueFor(row, "usedBy"))
      node.state = this.valueFor(row, "state") || "running"
      nodeMap.set(name, node)

      node.dependsOn.forEach(dependency => {
        if (!nodeMap.has(dependency)) {
          nodeMap.set(dependency, { name: dependency, dependsOn: [], usedBy: [], state: "running" })
        }
        const key = `${name}\u0000${dependency}`
        if (seenEdges.has(key)) return
        seenEdges.add(key)
        edges.push({ from: name, to: dependency })
      })
    })

    return {
      nodes: Array.from(nodeMap.values()).sort((left, right) => this.nodeSort(left, right)),
      edges
    }
  }

  valueFor(row, name) {
    const dataValue = row.dataset[`depgraph${this.capitalize(name)}`]
    if (dataValue !== undefined) return dataValue
    return row.getAttribute(`data-controller-depgraph-${this.dasherize(name)}`) || ""
  }

  namesFromValue(value) {
    return String(value || "")
      .split(",")
      .map(name => name.trim())
      .filter(name => name !== "" && name !== "-")
  }

  svgForGraph(graph) {
    const positions = this.positionsFor(graph)
    const svg = document.createElementNS(svgNamespace, "svg")
    svg.classList.add("depgraph-svg")
    svg.setAttribute("role", "img")
    svg.setAttribute("aria-label", "Application dependency graph")
    svg.setAttribute("viewBox", `0 0 ${positions.width} ${positions.height}`)

    svg.append(this.markerDefinition())
    graph.edges.forEach(edge => {
      const from = positions.nodes.get(edge.from)
      const to = positions.nodes.get(edge.to)
      if (!from || !to) return
      svg.append(this.edgePath(from, to))
    })
    graph.nodes.forEach(node => {
      const position = positions.nodes.get(node.name)
      if (!position) return
      svg.append(this.nodeGroup(node, position))
    })

    return svg
  }

  positionsFor(graph) {
    const depths = this.depthsFor(graph)
    const columns = new Map()
    graph.nodes.forEach(node => {
      const depth = depths.get(node.name) || 0
      if (!columns.has(depth)) columns.set(depth, [])
      columns.get(depth).push(node)
    })

    const sortedDepths = Array.from(columns.keys()).sort((left, right) => left - right)
    const maxRows = Math.max(1, ...Array.from(columns.values()).map(nodes => nodes.length))
    const width = Math.max(520, sortedDepths.length * 220 + 80)
    const height = Math.max(180, maxRows * 74 + 48)
    const positions = new Map()

    sortedDepths.forEach((depth, columnIndex) => {
      const nodes = columns.get(depth).sort((left, right) => this.nodeSort(left, right))
      const gap = height / (nodes.length + 1)
      nodes.forEach((node, rowIndex) => {
        positions.set(node.name, {
          x: 40 + columnIndex * 220,
          y: Math.round(gap * (rowIndex + 1) - 22),
          width: 150,
          height: 44
        })
      })
    })

    return { width, height, nodes: positions }
  }

  depthsFor(graph) {
    const depths = new Map(graph.nodes.map(node => [node.name, node.name === "app" ? 0 : 1]))
    let changed = true
    for (let pass = 0; pass < graph.nodes.length && changed; pass++) {
      changed = false
      graph.edges.forEach(edge => {
        const fromDepth = depths.get(edge.from) ?? 0
        const nextDepth = fromDepth + 1
        if ((depths.get(edge.to) ?? 0) >= nextDepth) return
        depths.set(edge.to, nextDepth)
        changed = true
      })
    }
    return depths
  }

  markerDefinition() {
    const defs = document.createElementNS(svgNamespace, "defs")
    const marker = document.createElementNS(svgNamespace, "marker")
    marker.id = "depgraph-arrow"
    marker.setAttribute("viewBox", "0 0 10 10")
    marker.setAttribute("refX", "9")
    marker.setAttribute("refY", "5")
    marker.setAttribute("markerWidth", "7")
    marker.setAttribute("markerHeight", "7")
    marker.setAttribute("orient", "auto-start-reverse")

    const path = document.createElementNS(svgNamespace, "path")
    path.setAttribute("d", "M 0 0 L 10 5 L 0 10 z")
    marker.append(path)
    defs.append(marker)
    return defs
  }

  edgePath(from, to) {
    const startX = from.x + from.width
    const startY = from.y + from.height / 2
    const endX = to.x
    const endY = to.y + to.height / 2
    const control = Math.max(40, Math.abs(endX - startX) / 2)
    const path = document.createElementNS(svgNamespace, "path")
    path.classList.add("depgraph-edge")
    path.setAttribute("d", `M ${startX} ${startY} C ${startX + control} ${startY}, ${endX - control} ${endY}, ${endX - 8} ${endY}`)
    path.setAttribute("marker-end", "url(#depgraph-arrow)")
    return path
  }

  nodeGroup(node, position) {
    const group = document.createElementNS(svgNamespace, "g")
    group.classList.add("depgraph-node")
    group.classList.add(`depgraph-node-${this.stateClass(node.state)}`)
    if (node.name === "app") group.classList.add("depgraph-node-root")
    group.setAttribute("transform", `translate(${position.x} ${position.y})`)

    const rect = document.createElementNS(svgNamespace, "rect")
    rect.setAttribute("width", position.width)
    rect.setAttribute("height", position.height)
    rect.setAttribute("rx", "4")
    group.append(rect)

    const name = document.createElementNS(svgNamespace, "text")
    name.classList.add("depgraph-node-name")
    name.setAttribute("x", "12")
    name.setAttribute("y", "19")
    name.textContent = node.name
    group.append(name)

    const meta = document.createElementNS(svgNamespace, "text")
    meta.classList.add("depgraph-node-meta")
    meta.setAttribute("x", "12")
    meta.setAttribute("y", "34")
    meta.textContent = this.metaText(node)
    group.append(meta)

    return group
  }

  metaText(node) {
    const deps = node.dependsOn.length
    const users = node.usedBy.length
    const state = node.state && node.state !== "running" ? `${node.state} · ` : ""
    return `${state}${deps} dep${deps === 1 ? "" : "s"} · ${users} user${users === 1 ? "" : "s"}`
  }

  connectEvents() {
    if (!this.hasEventsUrlValue || this.eventsUrlValue === "") return
    this.disconnectEvents()
    this.events = new EventSource(this.eventsUrlValue)
    this.events.addEventListener("shutdown", event => {
      try {
        this.applyShutdownState(JSON.parse(event.data))
      } catch (error) {
        console.error("dependency shutdown event could not be parsed", error)
      }
    })
  }

  disconnectEvents() {
    if (!this.events) return
    this.events.close()
    this.events = null
  }

  applyShutdownState(state) {
    if (!state) return
    this.updateTextTarget("ready", state.ready_text || this.readyText(state))
    this.updateTextTarget("activeRequests", String(state.active_requests ?? 0))
    this.updateTextTarget("activeConnections", String(state.active_connections ?? 0))
    this.updateTextTarget("phase", state.phase || "idle")
    this.updateTextTarget("message", state.message || "")

    const states = new Map()
    const nodes = state.nodes || []
    nodes.forEach(node => {
      if (node && node.name) states.set(node.name, node.state || "running")
    })
    if (!this.hasTableTarget) return
    this.tableTarget.querySelectorAll("tbody tr[data-depgraph-name], tbody tr[data-controller-depgraph-name]").forEach(row => {
      const name = this.valueFor(row, "name")
      const nodeState = states.get(name) || "running"
      row.dataset.depgraphState = nodeState
      row.setAttribute("data-controller-depgraph-state", nodeState)
    })
    this.render()
  }

  updateTextTarget(name, value) {
    const targetName = `${name}Target`
    const hasTargetName = `has${this.capitalize(name)}Target`
    if (!this[hasTargetName]) return
    this[targetName].textContent = value
  }

  readyText(state) {
    if (state.ready) return "GET /readyz => 200 ready"
    const status = state.ready_status || 503
    return `GET /readyz => ${status} not ready`
  }

  stateClass(state) {
    const normalized = String(state || "running").toLowerCase().replace(/[^a-z0-9_-]/g, "-")
    return normalized || "running"
  }

  nodeSort(left, right) {
    if (left.name === "app") return -1
    if (right.name === "app") return 1
    return left.name.localeCompare(right.name)
  }

  capitalize(value) {
    return value.charAt(0).toUpperCase() + value.slice(1)
  }

  dasherize(value) {
    return value.replace(/[A-Z]/g, letter => `-${letter.toLowerCase()}`)
  }
}
