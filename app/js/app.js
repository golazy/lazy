// golazy:turbo
// golazy:stimulus

const panelPath = "/_golazy/"
const embeddedPanelID = "golazy-dev-panel"
const embeddedPanelDefaultHeight = 320
const embeddedPanelMinHeight = 180
const embeddedPanelMaxRatio = 0.85
const embeddedPanelResizeHandleHeight = 8
const embeddedPanelHeightKey = "golazy:devpanel:height"
const embeddedPanelClosedKey = "golazy:devpanel:closed"
const clientState = window.__golazyDevPanelClient || {}
window.__golazyDevPanelClient = clientState

let selectedService = clientState.selectedService || ""
let currentPanelState = clientState.currentPanelState || null
let stateSource = clientState.stateSource || null
let serviceNavigationInstalled = Boolean(clientState.serviceNavigationInstalled)
let cacheActionsInstalled = Boolean(clientState.cacheActionsInstalled)
let requestMonitoringInstalled = Boolean(clientState.requestMonitoringInstalled)

installEmbeddedPanel()
installTurboPersistence()
installEmbeddedPanelMessages()
installReloadClient()
bootPanelPage()
installPanelBoot()

function installReloadClient() {
  if (location.pathname.startsWith("/_golazy")) return

  const client = clientState.reload || {}
  if (!client.source || client.source.readyState === EventSource.CLOSED) {
    const existingSource = window.__lazyReloadSource
    client.source = existingSource && existingSource.readyState !== EventSource.CLOSED ? existingSource : new EventSource("/__lazy/reload")
    window.__lazyReloadSource = client.source
    client.listenerInstalled = false
  }
  if (!client.onReload) {
    client.onReload = () => {
      if (!location.pathname.startsWith("/_golazy")) {
        location.reload()
      }
    }
  }
  if (!client.listenerInstalled) {
    client.source.addEventListener("reload", client.onReload)
    client.listenerInstalled = true
  }
  clientState.reload = client
}

function installPanelBoot() {
  if (clientState.panelBootInstalled) return
  document.addEventListener("turbo:load", bootPanelPage)
  clientState.panelBootInstalled = true
}

function bootPanelPage() {
  if (!panelElement()) return
  installServiceNavigation()
  installCacheActions()
  installRequestMonitoringActions()
  installPanelClose()
  refreshPanelState()
  refreshCache()
  refreshRequestMonitoring()
  refreshJobs()
  startPanelEvents()
}

function panelElement() {
  return document.querySelector("[data-panel]")
}

function startPanelEvents() {
  stateSource = clientState.stateSource || stateSource
  if (stateSource && stateSource.readyState !== EventSource.CLOSED) return
  stateSource = new EventSource("/_golazy/events")
  clientState.stateSource = stateSource
  stateSource.addEventListener("turbo-stream", renderTurboStream)
  stateSource.addEventListener("state", refreshState)
  stateSource.addEventListener("output", updateFromOutputEvent)
  stateSource.addEventListener("file_change", refreshState)
  stateSource.addEventListener("reload", updateFromOutputEvent)
  stateSource.addEventListener("manual", refreshState)
}

function installEmbeddedPanel() {
  if (location.pathname.startsWith("/_golazy")) return
  if (isEmbeddedPanelClosed()) {
    releasePanelSpace()
    return
  }

  const existing = document.getElementById(embeddedPanelID)
  if (existing) {
    const height = currentEmbeddedPanelHeight()
    reservePanelSpace(height)
    setPanelHostHeight(existing, height)
    return
  }
  if (!document.body) {
    document.addEventListener("DOMContentLoaded", installEmbeddedPanel, { once: true })
    return
  }

  const height = currentEmbeddedPanelHeight()
  reservePanelSpace(height)
  const host = document.createElement("golazy-dev-panel")
  host.id = embeddedPanelID
  host.setAttribute("data-turbo-permanent", "")
  host.setAttribute("aria-label", "GoLazy development panel")
  setPanelHostHeight(host, height)

  const shadow = host.attachShadow({ mode: "open" })
  const style = document.createElement("style")
  style.textContent = `
    :host {
      --golazy-dev-panel-height: ${height}px;
      --golazy-dev-panel-resize-handle-height: ${embeddedPanelResizeHandleHeight}px;
      bottom: 0;
      display: block;
      height: calc(var(--golazy-dev-panel-height) + var(--golazy-dev-panel-resize-handle-height) + env(safe-area-inset-bottom));
      left: 0;
      position: fixed;
      right: 0;
      z-index: 2147483647;
    }
    .resize-handle {
      align-items: center;
      background: #202124;
      border-top: 1px solid #3c4043;
      cursor: ns-resize;
      display: flex;
      height: var(--golazy-dev-panel-resize-handle-height);
      justify-content: center;
      outline: none;
      touch-action: none;
      user-select: none;
      width: 100%;
    }
    .resize-handle::before {
      background: #8ab4f8;
      border-radius: 999px;
      content: "";
      display: block;
      height: 2px;
      opacity: 0.72;
      width: 44px;
    }
    .resize-handle:hover::before,
    .resize-handle:focus-visible::before,
    .resize-handle.is-resizing::before {
      opacity: 1;
    }
    iframe {
      background: #202124;
      border: 0;
      display: block;
      height: calc(100% - var(--golazy-dev-panel-resize-handle-height));
      width: 100%;
    }
  `
  const handle = document.createElement("div")
  handle.className = "resize-handle"
  handle.setAttribute("aria-label", "Resize GoLazy development panel")
  handle.setAttribute("aria-orientation", "horizontal")
  handle.setAttribute("role", "separator")
  handle.setAttribute("tabindex", "0")
  installEmbeddedPanelResize(handle, host)
  const frame = document.createElement("iframe")
  frame.src = panelPath
  frame.title = "GoLazy development panel"
  shadow.append(style, handle, frame)
  document.body.append(host)
  updateResizeHandleA11y(handle, height)
  installEmbeddedPanelViewportResize()
}

function reservePanelSpace(height) {
  const root = document.documentElement
  if (!root.dataset.golazyDevPanelPaddingBase) {
    root.dataset.golazyDevPanelPaddingBase = getComputedStyle(root).paddingBottom || "0px"
  }
  root.style.setProperty("--golazy-dev-panel-height", `${clampEmbeddedPanelHeight(height)}px`)
  root.style.setProperty("--golazy-dev-panel-resize-handle-height", `${embeddedPanelResizeHandleHeight}px`)
  root.style.paddingBottom = `calc(${root.dataset.golazyDevPanelPaddingBase} + var(--golazy-dev-panel-height) + var(--golazy-dev-panel-resize-handle-height) + env(safe-area-inset-bottom))`
}

function releasePanelSpace() {
  const root = document.documentElement
  if (!root.dataset.golazyDevPanelPaddingBase) return
  root.style.paddingBottom = root.dataset.golazyDevPanelPaddingBase
  root.style.removeProperty("--golazy-dev-panel-height")
  root.style.removeProperty("--golazy-dev-panel-resize-handle-height")
}

function installEmbeddedPanelResize(handle, host) {
  let startY = 0
  let startHeight = 0
  let dragging = false

  const move = event => {
    if (!dragging) return
    event.preventDefault()
    setEmbeddedPanelHeight(startHeight + startY - event.clientY, { host, handle })
  }
  const stop = () => {
    if (!dragging) return
    dragging = false
    handle.classList.remove("is-resizing")
    window.removeEventListener("pointermove", move)
    window.removeEventListener("pointerup", stop)
    window.removeEventListener("pointercancel", stop)
  }

  handle.addEventListener("pointerdown", event => {
    if (event.button !== undefined && event.button !== 0) return
    event.preventDefault()
    dragging = true
    startY = event.clientY
    startHeight = currentEmbeddedPanelHeight()
    handle.classList.add("is-resizing")
    handle.setPointerCapture?.(event.pointerId)
    window.addEventListener("pointermove", move)
    window.addEventListener("pointerup", stop)
    window.addEventListener("pointercancel", stop)
  })
  handle.addEventListener("keydown", event => {
    const current = currentEmbeddedPanelHeight()
    if (event.key === "Home") {
      event.preventDefault()
      setEmbeddedPanelHeight(embeddedPanelMinHeight, { host, handle })
      return
    }
    if (event.key === "End") {
      event.preventDefault()
      setEmbeddedPanelHeight(maxEmbeddedPanelHeight(), { host, handle })
      return
    }
    if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return
    event.preventDefault()
    const step = event.shiftKey ? 40 : 10
    setEmbeddedPanelHeight(current + (event.key === "ArrowUp" ? step : -step), { host, handle })
  })
}

function setEmbeddedPanelHeight(height, options = {}) {
  const next = clampEmbeddedPanelHeight(height)
  clientState.embeddedPanelHeight = next
  rememberEmbeddedPanelHeight(next)
  reservePanelSpace(next)
  setPanelHostHeight(options.host || document.getElementById(embeddedPanelID), next)
  updateResizeHandleA11y(options.handle, next)
}

function setPanelHostHeight(host, height) {
  if (!host) return
  host.style.setProperty("--golazy-dev-panel-height", `${clampEmbeddedPanelHeight(height)}px`)
  host.style.setProperty("--golazy-dev-panel-resize-handle-height", `${embeddedPanelResizeHandleHeight}px`)
}

function updateResizeHandleA11y(handle, height) {
  if (!handle) return
  handle.setAttribute("aria-valuemin", String(embeddedPanelMinHeight))
  handle.setAttribute("aria-valuemax", String(maxEmbeddedPanelHeight()))
  handle.setAttribute("aria-valuenow", String(clampEmbeddedPanelHeight(height)))
}

function installEmbeddedPanelViewportResize() {
  if (clientState.embeddedPanelViewportResizeInstalled) return
  clientState.embeddedPanelViewportResizeInstalled = true
  window.addEventListener("resize", () => setEmbeddedPanelHeight(currentEmbeddedPanelHeight()))
}

function currentEmbeddedPanelHeight() {
  if (Number.isFinite(clientState.embeddedPanelHeight)) {
    return clampEmbeddedPanelHeight(clientState.embeddedPanelHeight)
  }
  try {
    const value = Number.parseInt(window.sessionStorage.getItem(embeddedPanelHeightKey) || "", 10)
    if (Number.isFinite(value)) {
      clientState.embeddedPanelHeight = value
      return clampEmbeddedPanelHeight(value)
    }
  } catch {
    // The default height still works when sessionStorage is unavailable.
  }
  return clampEmbeddedPanelHeight(embeddedPanelDefaultHeight)
}

function rememberEmbeddedPanelHeight(height) {
  try {
    window.sessionStorage.setItem(embeddedPanelHeightKey, String(clampEmbeddedPanelHeight(height)))
  } catch {
    // Resizing should still work when sessionStorage is unavailable.
  }
}

function clampEmbeddedPanelHeight(height) {
  const value = Number.parseInt(height, 10)
  if (!Number.isFinite(value)) return embeddedPanelDefaultHeight
  return Math.min(Math.max(value, embeddedPanelMinHeight), maxEmbeddedPanelHeight())
}

function maxEmbeddedPanelHeight() {
  const viewport = window.innerHeight || embeddedPanelDefaultHeight
  return Math.max(embeddedPanelMinHeight, Math.floor(viewport * embeddedPanelMaxRatio))
}

function installEmbeddedPanelMessages() {
  if (clientState.embeddedPanelMessagesInstalled) return
  clientState.embeddedPanelMessagesInstalled = true
  window.addEventListener("message", event => {
    if (event.origin !== window.location.origin) return
    if (event.data?.type !== "golazy:devpanel:close") return
    closeEmbeddedPanel()
  })
}

function closeEmbeddedPanel() {
  rememberEmbeddedPanelClosed()
  document.getElementById(embeddedPanelID)?.remove()
  releasePanelSpace()
}

function isEmbeddedPanelClosed() {
  try {
    return window.sessionStorage.getItem(embeddedPanelClosedKey) === "true"
  } catch {
    return false
  }
}

function rememberEmbeddedPanelClosed() {
  try {
    window.sessionStorage.setItem(embeddedPanelClosedKey, "true")
  } catch {
    // Closing should still work when sessionStorage is unavailable.
  }
}

function installTurboPersistence() {
  if (clientState.turboPersistenceInstalled) return
  clientState.turboPersistenceInstalled = true
  document.addEventListener("turbo:before-render", event => {
    const host = document.getElementById(embeddedPanelID)
    const newBody = event.detail?.newBody
    if (host && newBody && !newBody.querySelector(`#${embeddedPanelID}`)) {
      newBody.append(host)
    }
  })
  document.addEventListener("turbo:render", installEmbeddedPanel)
  document.addEventListener("turbo:load", installEmbeddedPanel)
}

function installPanelClose() {
  const button = document.querySelector("[data-panel-close]")
  if (!button) return

  const embedded = window.parent && window.parent !== window
  document.documentElement.dataset.golazyEmbedded = String(embedded)
  button.hidden = !embedded
  if (!embedded) return
  if (button.dataset.golazyPanelCloseInstalled) return
  button.dataset.golazyPanelCloseInstalled = "true"

  button.addEventListener("click", () => {
    window.parent.postMessage({ type: "golazy:devpanel:close" }, window.location.origin)
  })
}

async function refreshState(event) {
  await refreshPanelState()
  refreshCache()
  refreshJobs()
}

function renderTurboStream(event) {
  if (!event?.data || event.data === "ok") return
  const turbo = window.Turbo
  if (!turbo?.renderStreamMessage) return
  turbo.renderStreamMessage(event.data)
}

async function refreshPanelState() {
  const response = await fetch("/_golazy/state", { headers: { Accept: "application/json" } })
  if (!response.ok) return
  renderState(await response.json())
}

function renderState(state) {
  currentPanelState = state || {}
  clientState.currentPanelState = currentPanelState
  const panel = panelElement()
  if (panel) {
    panel.dataset.state = state.state || ""
  }
  for (const chip of document.querySelectorAll("[data-state-chip]")) {
    chip.dataset.stateValue = state.state || ""
  }
  textAll("[data-panel-state]", state.state)
  textAll("[data-panel-message]", state.message)
  textAll("[data-panel-build]", state.build_count)
  textAll("[data-panel-duration]", state.duration || "")
  textAll("[data-panel-app-addr]", state.app_addr || "")
  textAll("[data-panel-control-addr]", state.control_plane_addr || "")
  textAll("[data-panel-output]", state.output || "")
  list("[data-panel-changes]", state.changed || [], value => {
    const item = document.createElement("li")
    const code = document.createElement("code")
    code.textContent = value
    item.append(code)
    return item
  }, "No recent changes.")
  renderServices(state.services || [], state.events || [])
}

function installServiceNavigation() {
  if (serviceNavigationInstalled) return
  serviceNavigationInstalled = true
  clientState.serviceNavigationInstalled = true
  document.addEventListener("click", event => {
    const target = event.target.closest("[data-service-select], [data-service-status]")
    if (!target) return
    const service = target.getAttribute("data-service-select") || target.getAttribute("data-service-name")
    if (!service) return
    selectService(service)
    if (!location.pathname.startsWith("/_golazy/services")) {
      visitPanelPath("/_golazy/services")
    }
  })
}

function visitPanelPath(path) {
  if (window.Turbo?.visit) {
    window.Turbo.visit(path)
    return
  }
  window.location.href = path
}

function renderServices(services, events) {
  if (!selectedService || !services.some(service => service.name === selectedService)) {
    selectedService = services[0]?.name || ""
    clientState.selectedService = selectedService
  }
  renderServiceList(services)
  renderServiceStatuses(services)
  renderServiceOutput(events)
}

function renderServiceList(services) {
  for (const node of document.querySelectorAll("[data-service-list]")) {
    node.replaceChildren()
    if (!services.length) {
      const item = document.createElement("li")
      item.className = "muted"
      item.textContent = "No services discovered."
      node.append(item)
      continue
    }
    for (const service of services) {
      const item = document.createElement("li")
      const button = serviceButton(service, "data-service-select")
      button.setAttribute("aria-selected", String(service.name === selectedService))
      item.append(button)
      node.append(item)
    }
  }
}

function renderServiceStatuses(services) {
  for (const node of document.querySelectorAll("[data-service-statuses]")) {
    node.replaceChildren()
    if (!services.length) {
      const empty = document.createElement("span")
      empty.className = "muted"
      empty.setAttribute("data-service-status-empty", "")
      empty.textContent = "No services"
      node.append(empty)
      continue
    }
    for (const service of services) {
      const button = serviceButton(service, "data-service-name")
      button.classList.add("service-status-button")
      button.setAttribute("data-service-status", "")
      button.setAttribute("aria-current", String(service.name === selectedService))
      node.append(button)
    }
  }
}

function serviceButton(service, nameAttribute) {
  const button = document.createElement("button")
  button.type = "button"
  button.setAttribute(nameAttribute, service.name || "")
  button.setAttribute("data-service-state", service.state || "stopped")
  if (service.message) {
    button.title = `${service.name}: ${service.message}`
  }
  const dot = document.createElement("span")
  dot.className = "service-dot"
  const label = document.createElement("span")
  label.textContent = service.name || ""
  button.append(dot, label)
  return button
}

function selectService(service) {
  selectedService = service || ""
  clientState.selectedService = selectedService
  for (const button of document.querySelectorAll("[data-service-select]")) {
    button.setAttribute("aria-selected", String(button.getAttribute("data-service-select") === selectedService))
  }
  for (const button of document.querySelectorAll("[data-service-status]")) {
    button.setAttribute("aria-current", String(button.getAttribute("data-service-name") === selectedService))
  }
  renderServiceOutput(currentPanelState?.events || [])
}

function renderServiceOutput(events) {
  const outputEvents = serviceOutputEvents(events, selectedService)
  textAll("[data-service-output-title]", selectedService ? `${selectedService} output` : "Select a service")
  textAll("[data-service-output-count]", `${outputEvents.length} ${outputEvents.length === 1 ? "message" : "messages"}`)
  table("[data-service-output]", outputEvents, serviceOutputRow, 3, selectedService ? "No output recorded for this service." : "Select a service to inspect output.")
}

function serviceOutputEvents(events, service) {
  if (!service) return []
  const rows = []
  for (const event of events || []) {
    if (event.type !== "output" || event.service !== service || !event.output) continue
    for (const line of String(event.output).split(/\r?\n/)) {
      if (line === "") continue
      rows.push({
        stream: event.stream || "",
        time: event.time,
        message: line,
      })
    }
  }
  return rows
}

function serviceOutputRow(event) {
  const row = document.createElement("tr")
  for (const value of [
    event.stream,
    formatDateTime(event.time),
    event.message,
  ]) {
    const cell = document.createElement("td")
    cell.textContent = value ?? ""
    row.append(cell)
  }
  return row
}

function installCacheActions() {
  if (cacheActionsInstalled) return
  cacheActionsInstalled = true
  clientState.cacheActionsInstalled = true
  document.addEventListener("click", async event => {
    const button = event.target.closest("[data-cache-action]")
    if (!button) return
      const path = button.getAttribute("data-cache-action")
      if (!path) return
      button.disabled = true
      try {
        const response = await fetch(path, {
          method: "POST",
          headers: { Accept: "application/json" },
        })
        if (response.ok) {
          renderCache(await response.json())
        } else {
          await refreshCache()
        }
      } finally {
        button.disabled = false
      }
  })
}

async function refreshCache() {
  if (!document.querySelector("[data-cache-panel]")) return
  const response = await fetch("/_golazy/cache", { headers: { Accept: "application/json" } })
  if (!response.ok) {
    textAll("[data-cache-enabled]", "Unavailable")
    textAll("[data-cache-state]", "Cache unavailable")
    for (const selector of ["[data-cache-entries]", "[data-cache-hits]", "[data-cache-misses]", "[data-cache-sets]", "[data-cache-evictions]"]) {
      textAll(selector, "0")
    }
    list("[data-cache-keys]", [], cacheKeyItem, "No keys.")
    return
  }
  renderCache(await response.json())
}

function renderCache(cache) {
  const stats = cache?.stats || {}
  const enabled = Boolean(cache?.enabled)
  textAll("[data-cache-enabled]", enabled ? "On" : "Off")
  textAll("[data-cache-state]", enabled ? "Cache enabled" : "Cache disabled")
  textAll("[data-cache-entries]", stats.entries ?? 0)
  textAll("[data-cache-hits]", stats.hits ?? 0)
  textAll("[data-cache-misses]", stats.misses ?? 0)
  textAll("[data-cache-sets]", stats.sets ?? 0)
  textAll("[data-cache-evictions]", stats.evictions ?? 0)
  list("[data-cache-keys]", cache?.keys || [], cacheKeyItem, "No keys.")
}

function installRequestMonitoringActions() {
  if (requestMonitoringInstalled) return
  requestMonitoringInstalled = true
  clientState.requestMonitoringInstalled = true
  document.addEventListener("change", async event => {
    const toggle = event.target.closest("[data-request-monitoring-toggle]")
    if (!toggle) return
    await setRequestMonitoring(toggle.checked)
  })
  document.addEventListener("click", async event => {
    const button = event.target.closest("[data-request-monitoring-button]")
    if (!button) return
    const action = button.getAttribute("data-request-monitoring-action")
    if (!action) return
    await postRequestMonitoring(action)
  })
}

async function refreshRequestMonitoring() {
  if (!document.querySelector("[data-request-monitoring-state], [data-request-monitoring-toggle], [data-request-monitoring-button]")) return
  const response = await fetch("/_golazy/request-monitoring", { headers: { Accept: "application/json" } })
  if (!response.ok) {
    renderRequestMonitoringUnavailable()
    return
  }
  renderRequestMonitoring(await response.json())
}

async function setRequestMonitoring(enabled) {
  const action = enabled ? "/_golazy/request-monitoring/on" : "/_golazy/request-monitoring/off"
  await postRequestMonitoring(action)
}

async function postRequestMonitoring(action) {
  setRequestMonitoringDisabled(true)
  try {
    const response = await fetch(action, {
      method: "POST",
      headers: { Accept: "application/json" },
    })
    if (response.ok) {
      renderRequestMonitoring(await response.json())
    } else {
      await refreshRequestMonitoring()
    }
  } catch {
    renderRequestMonitoringUnavailable()
  }
}

function renderRequestMonitoring(state) {
  const enabled = Boolean(state?.enabled)
  textAll("[data-request-monitoring-state]", enabled ? "Monitoring on" : "Monitoring off")
  textAll("[data-request-monitoring-directory]", state?.directory || ".tmp/traces")
  for (const toggle of document.querySelectorAll("[data-request-monitoring-toggle]")) {
    toggle.checked = enabled
    toggle.disabled = false
  }
  for (const button of document.querySelectorAll("[data-request-monitoring-button]")) {
    button.disabled = false
    button.setAttribute("aria-pressed", String(enabled))
    button.setAttribute("data-request-monitoring-action", enabled ? "/_golazy/request-monitoring/off" : "/_golazy/request-monitoring/on")
    button.title = enabled ? "Disable detailed request monitoring" : "Enable detailed request monitoring"
    if (button.classList.contains("toolbar-button")) {
      button.textContent = enabled ? "Disable monitoring" : "Enable monitoring"
    }
  }
}

function renderRequestMonitoringUnavailable() {
  textAll("[data-request-monitoring-state]", "Monitoring unavailable")
  for (const toggle of document.querySelectorAll("[data-request-monitoring-toggle]")) {
    toggle.checked = false
    toggle.disabled = true
  }
  setRequestMonitoringDisabled(true)
}

function setRequestMonitoringDisabled(disabled) {
  for (const control of document.querySelectorAll("[data-request-monitoring-toggle], [data-request-monitoring-button]")) {
    control.disabled = disabled
  }
}

async function refreshJobs() {
  if (!document.querySelector("[data-jobs-panel]")) return
  const response = await fetch("/_golazy/jobs", { headers: { Accept: "application/json" } })
  if (!response.ok) {
    renderJobsUnavailable()
    return
  }
  renderJobs(await response.json())
}

function renderJobsUnavailable() {
  textAll("[data-jobs-state]", "Jobs unavailable")
  textAll("[data-jobs-running]", "Unavailable")
  for (const selector of ["[data-jobs-total]", "[data-jobs-pending]", "[data-jobs-count-running]", "[data-jobs-retrying]", "[data-jobs-succeeded]", "[data-jobs-discarded]"]) {
    textAll(selector, "0")
  }
  list("[data-job-definitions]", [], jobDefinitionItem, "No job definitions.")
  table("[data-jobs-recent]", [], jobRow, 7, "No recent jobs.")
}

function renderJobs(snapshot) {
  const stats = snapshot?.stats || {}
  const byState = stats.by_state || {}
  const running = Boolean(snapshot?.running)
  textAll("[data-jobs-state]", running ? "Runner active" : "Runner stopped")
  textAll("[data-jobs-running]", running ? "Running" : "Stopped")
  textAll("[data-jobs-total]", stats.total ?? 0)
  textAll("[data-jobs-pending]", byState.pending ?? 0)
  textAll("[data-jobs-count-running]", byState.running ?? 0)
  textAll("[data-jobs-retrying]", byState.retrying ?? 0)
  textAll("[data-jobs-succeeded]", byState.succeeded ?? 0)
  textAll("[data-jobs-discarded]", byState.discarded ?? 0)
  list("[data-job-definitions]", snapshot?.definitions || [], jobDefinitionItem, "No job definitions.")
  table("[data-jobs-recent]", snapshot?.recent || [], jobRow, 7, "No recent jobs.")
}

function jobDefinitionItem(definition) {
  const item = document.createElement("li")
  const code = document.createElement("code")
  code.textContent = definition.kind || ""
  item.append(code, ` ${definition.queue || "default"} attempts ${definition.max_attempts || 0}`)
  return item
}

function jobRow(job) {
  const row = document.createElement("tr")
  for (const value of [
    job.id,
    job.kind,
    job.queue,
    job.state,
    `${job.attempt || 0}/${job.max_attempts || 0}`,
    formatDateTime(job.run_at),
    job.last_error || "",
  ]) {
    const cell = document.createElement("td")
    cell.textContent = value ?? ""
    row.append(cell)
  }
  return row
}

function cacheKeyItem(value) {
  const item = document.createElement("li")
  const code = document.createElement("code")
  code.textContent = value
  item.append(code)
  return item
}

function updateFromOutputEvent(event) {
  const payload = parseEventPayload(event)
  if (!payload) return
  appendOutput(payload)
  appendServiceOutput(payload)
}

function appendEvent(event) {
  const payload = parseEventPayload(event)
  if (!payload) return
  appendOutput(payload)
  appendServiceOutput(payload)

  appendEventItem(payload)
}

function parseEventPayload(event) {
  if (!event?.data || event.data === "ok") return null
  let payload
  try {
    payload = JSON.parse(event.data)
  } catch {
    return null
  }
  return payload
}

function appendEventItem(payload) {
  const target = document.querySelector("[data-panel-events]")
  if (!target) return
  target.querySelector(".muted")?.remove()

  const item = document.createElement("li")
  const time = document.createElement("span")
  time.textContent = eventTime(payload.time)
  const type = document.createElement("strong")
  type.textContent = payload.type || "event"
  item.append(time, " ", type, " ", eventMessage(payload))
  target.prepend(item)
  while (target.children.length > 80) {
    target.lastElementChild.remove()
  }
}

function appendOutput(payload) {
  if (payload?.type !== "output" || payload.service || !payload.output) return
  for (const node of document.querySelectorAll("[data-panel-output]")) {
    node.textContent += payload.output
    node.scrollTop = node.scrollHeight
  }
}

function appendServiceOutput(payload) {
  if (payload?.type !== "output" || !payload.service || !payload.output) return
  if (!selectedService) {
    selectedService = payload.service
    clientState.selectedService = selectedService
  }
  if (payload.service !== selectedService) return
  const rows = []
  for (const line of String(payload.output).split(/\r?\n/)) {
    if (line === "") continue
    rows.push({
      stream: payload.stream || "",
      time: payload.time,
      message: line,
    })
  }
  const target = document.querySelector("[data-service-output]")
  if (!target || !rows.length) return
  target.querySelector(".empty-cell")?.closest("tr")?.remove()
  for (const row of rows) {
    target.append(serviceOutputRow(row))
  }
  while (target.children.length > 300) {
    target.firstElementChild.remove()
  }
  textAll("[data-service-output-title]", `${selectedService} output`)
  textAll("[data-service-output-count]", `${target.children.length} ${target.children.length === 1 ? "message" : "messages"}`)
}

function eventTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  return date.toLocaleTimeString()
}

function eventMessage(payload) {
  const value = payload.message || payload.output || ""
  const trimmed = String(value).trim()
  if (trimmed.length <= 240) return trimmed
  return `${trimmed.slice(0, 237)}...`
}

function textAll(selector, value) {
  for (const node of document.querySelectorAll(selector)) {
    node.textContent = value ?? ""
  }
}

function list(selector, values, render, empty) {
  for (const node of document.querySelectorAll(selector)) {
    node.replaceChildren()
    if (!values.length) {
      const item = document.createElement("li")
      item.className = "muted"
      item.textContent = empty
      node.append(item)
      continue
    }
    for (const value of values) {
      node.append(render(value))
    }
  }
}

function table(selector, values, render, columns, empty) {
  for (const node of document.querySelectorAll(selector)) {
    node.replaceChildren()
    if (!values.length) {
      const row = document.createElement("tr")
      const cell = document.createElement("td")
      cell.colSpan = columns
      cell.className = "empty-cell"
      cell.textContent = empty
      row.append(cell)
      node.append(row)
      continue
    }
    for (const value of values) {
      node.append(render(value))
    }
  }
}

function formatDateTime(value) {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  return date.toLocaleString()
}
