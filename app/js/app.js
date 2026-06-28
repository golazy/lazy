// golazy:turbo
// golazy:stimulus

const panelPath = "/_golazy/"
const embeddedPanelID = "golazy-dev-panel"
const embeddedPanelHeight = "320px"
const embeddedPanelClosedKey = "golazy:devpanel:closed"
let selectedService = ""
let currentPanelState = null
let stateSource = null
let serviceNavigationInstalled = false
let cacheActionsInstalled = false

installEmbeddedPanel()
installTurboPersistence()
installEmbeddedPanelMessages()
bootPanelPage()
document.addEventListener("turbo:load", bootPanelPage)

const reloadSource = window.__lazyReloadSource || new EventSource("/__lazy/reload")
window.__lazyReloadSource = reloadSource
reloadSource.addEventListener("reload", () => {
  if (!location.pathname.startsWith("/_golazy")) {
    location.reload()
  }
})

function bootPanelPage() {
  if (!panelElement()) return
  installServiceNavigation()
  installCacheActions()
  installPanelClose()
  refreshPanelState()
  refreshCache()
  refreshJobs()
  startPanelEvents()
}

function panelElement() {
  return document.querySelector("[data-panel]")
}

function startPanelEvents() {
  if (stateSource) return
  stateSource = new EventSource("/_golazy/events")
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

  if (document.getElementById(embeddedPanelID)) return
  if (!document.body) {
    document.addEventListener("DOMContentLoaded", installEmbeddedPanel, { once: true })
    return
  }

  reservePanelSpace()
  const host = document.createElement("golazy-dev-panel")
  host.id = embeddedPanelID
  host.setAttribute("data-turbo-permanent", "")
  host.setAttribute("aria-label", "GoLazy development panel")

  const shadow = host.attachShadow({ mode: "open" })
  const style = document.createElement("style")
  style.textContent = `
    :host {
      --golazy-dev-panel-height: ${embeddedPanelHeight};
      bottom: 0;
      display: block;
      height: calc(var(--golazy-dev-panel-height) + env(safe-area-inset-bottom));
      left: 0;
      position: fixed;
      right: 0;
      z-index: 2147483647;
    }
    iframe {
      background: #202124;
      border: 0;
      border-top: 1px solid #3c4043;
      display: block;
      height: 100%;
      width: 100%;
    }
  `
  const frame = document.createElement("iframe")
  frame.src = panelPath
  frame.title = "GoLazy development panel"
  shadow.append(style, frame)
  document.body.append(host)
}

function reservePanelSpace() {
  const root = document.documentElement
  if (!root.dataset.golazyDevPanelPaddingBase) {
    root.dataset.golazyDevPanelPaddingBase = getComputedStyle(root).paddingBottom || "0px"
  }
  root.style.setProperty("--golazy-dev-panel-height", embeddedPanelHeight)
  root.style.paddingBottom = `calc(${root.dataset.golazyDevPanelPaddingBase} + var(--golazy-dev-panel-height) + env(safe-area-inset-bottom))`
}

function releasePanelSpace() {
  const root = document.documentElement
  if (!root.dataset.golazyDevPanelPaddingBase) return
  root.style.paddingBottom = root.dataset.golazyDevPanelPaddingBase
  root.style.removeProperty("--golazy-dev-panel-height")
}

function installEmbeddedPanelMessages() {
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
