const panelPath = "/_golazy/"
const embeddedPanelID = "golazy-dev-panel"
const embeddedPanelHeight = "320px"
const embeddedPanelClosedKey = "golazy:devpanel:closed"

installEmbeddedPanel()
installTurboPersistence()
installEmbeddedPanelMessages()

const reloadSource = window.__lazyReloadSource || new EventSource("/__lazy/reload")
window.__lazyReloadSource = reloadSource
reloadSource.addEventListener("reload", () => {
  if (!location.pathname.startsWith("/_golazy")) {
    location.reload()
  }
})

const panel = document.querySelector("[data-panel]")
if (panel) {
  installPanelStimulus().catch(error => console.error("GoLazy panel Stimulus failed", error))
  installPanelTabs()
  installCacheActions()
  installPanelClose()
  refreshCache()

  const stateSource = new EventSource("/_golazy/events")
  stateSource.addEventListener("state", refreshState)
  stateSource.addEventListener("output", appendEvent)
  stateSource.addEventListener("file_change", refreshState)
  stateSource.addEventListener("reload", appendEvent)
  stateSource.addEventListener("manual", appendEvent)
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

async function installPanelStimulus() {
  const [{ Application }, { default: PanelResizeController }] = await Promise.all([
    import("/_golazy/assets/lazyshaft/stimulus-FNW3RRR2.js"),
    import("./controllers/panel_resize_controller.js"),
  ])
  const application = window.__golazyStimulus || Application.start()
  window.__golazyStimulus = application
  application.register("panel-resize", PanelResizeController)
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

function installPanelTabs() {
  const selected = document.querySelector("[data-tab][aria-selected='true']")
  if (selected?.dataset.tab) {
    selectPanelTab(selected.dataset.tab)
  }
  for (const button of document.querySelectorAll("[data-tab]")) {
    button.addEventListener("click", () => selectPanelTab(button.dataset.tab))
  }
}

function selectPanelTab(tab) {
  if (!tab) return
  for (const button of document.querySelectorAll("[data-tab]")) {
    button.setAttribute("aria-selected", String(button.dataset.tab === tab))
  }
  for (const view of document.querySelectorAll("[data-view]")) {
    view.classList.toggle("is-active", view.dataset.view === tab)
  }
}

async function refreshState(event) {
  appendEvent(event)
  const response = await fetch("/_golazy/state", { headers: { Accept: "application/json" } })
  if (!response.ok) return
  renderState(await response.json())
  refreshCache()
}

function renderState(state) {
  panel.dataset.state = state.state || ""
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
}

function installCacheActions() {
  for (const button of document.querySelectorAll("[data-cache-action]")) {
    button.addEventListener("click", async () => {
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

function cacheKeyItem(value) {
  const item = document.createElement("li")
  const code = document.createElement("code")
  code.textContent = value
  item.append(code)
  return item
}

function appendEvent(event) {
  if (!event?.data || event.data === "ok") return
  let payload
  try {
    payload = JSON.parse(event.data)
  } catch {
    return
  }
  appendOutput(payload)

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
  if (payload?.type !== "output" || !payload.output) return
  for (const node of document.querySelectorAll("[data-panel-output]")) {
    node.textContent += payload.output
    node.scrollTop = node.scrollHeight
  }
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
