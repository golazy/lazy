const panelPath = "/_golazy/"
const panelAssetPath = "/_golazy/assets/golazy-mark.svg"

installPanelActivator()

const reloadSource = window.__lazyReloadSource || new EventSource("/__lazy/reload")
window.__lazyReloadSource = reloadSource
reloadSource.addEventListener("reload", () => {
  if (!location.pathname.startsWith("/_golazy")) {
    location.reload()
  }
})

const panel = document.querySelector("[data-panel]")
if (panel) {
  const stateSource = new EventSource("/_golazy/events")
  stateSource.addEventListener("state", refreshState)
  stateSource.addEventListener("output", appendEvent)
  stateSource.addEventListener("file_change", refreshState)
  stateSource.addEventListener("reload", appendEvent)
  stateSource.addEventListener("manual", appendEvent)
  installCacheActions()
  refreshCache()
}

function installPanelActivator() {
  if (location.pathname.startsWith("/_golazy")) return
  if (document.querySelector("golazy-dev-panel")) return

  const host = document.createElement("golazy-dev-panel")
  const shadow = host.attachShadow({ mode: "open" })
  const style = document.createElement("style")
  style.textContent = `
    :host {
      bottom: max(1rem, env(safe-area-inset-bottom));
      display: block;
      position: fixed;
      right: max(1rem, env(safe-area-inset-right));
      z-index: 2147483647;
    }
    a {
      align-items: center;
      background: #151719;
      border: 2px solid #ffffff;
      border-radius: 999px;
      box-shadow: 0 14px 34px rgba(15, 23, 42, 0.28), 0 3px 10px rgba(15, 23, 42, 0.18);
      display: inline-flex;
      height: 3.75rem;
      justify-content: center;
      outline: none;
      transition: transform 120ms ease, box-shadow 120ms ease;
      width: 3.75rem;
    }
    a:hover {
      box-shadow: 0 18px 40px rgba(15, 23, 42, 0.34), 0 4px 14px rgba(15, 23, 42, 0.2);
      transform: translateY(-1px);
    }
    a:focus-visible {
      box-shadow: 0 0 0 4px #fddd00, 0 18px 40px rgba(15, 23, 42, 0.34);
    }
    img {
      display: block;
      height: 2.35rem;
      width: 2.35rem;
    }
  `
  const link = document.createElement("a")
  link.href = panelPath
  link.setAttribute("aria-label", "Open GoLazy development panel")
  link.title = "Open GoLazy development panel"
  const img = document.createElement("img")
  img.src = panelAssetPath
  img.alt = ""
  img.decoding = "async"
  link.append(img)
  shadow.append(style, link)
  document.documentElement.append(host)
}

async function refreshState(event) {
  appendEvent(event)
  const response = await fetch("/_golazy/state", { headers: { Accept: "application/json" } })
  if (!response.ok) return
  const state = await response.json()
  text("[data-panel-state]", state.state)
  text("[data-panel-message]", state.message)
  text("[data-panel-build]", state.build_count)
  text("[data-panel-duration]", state.duration || "")
  text("[data-panel-app-addr]", state.app_addr || "")
  text("[data-panel-control-addr]", state.control_plane_addr || "")
  text("[data-panel-output]", state.output || "")
  list("[data-panel-changes]", state.changed || [], value => {
    const item = document.createElement("li")
    const code = document.createElement("code")
    code.textContent = value
    item.append(code)
    return item
  }, "No recent changes.")
  refreshCache()
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
    text("[data-cache-enabled]", "Unavailable")
    for (const selector of ["[data-cache-entries]", "[data-cache-hits]", "[data-cache-misses]", "[data-cache-sets]", "[data-cache-evictions]"]) {
      text(selector, "0")
    }
    list("[data-cache-keys]", [], cacheKeyItem, "No keys.")
    return
  }
  renderCache(await response.json())
}

function renderCache(cache) {
  const stats = cache?.stats || {}
  text("[data-cache-enabled]", cache?.enabled ? "On" : "Off")
  text("[data-cache-entries]", stats.entries ?? 0)
  text("[data-cache-hits]", stats.hits ?? 0)
  text("[data-cache-misses]", stats.misses ?? 0)
  text("[data-cache-sets]", stats.sets ?? 0)
  text("[data-cache-evictions]", stats.evictions ?? 0)
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
  const target = document.querySelector("[data-panel-events]")
  if (!target || !event.data || event.data === "ok") return
  let payload
  try {
    payload = JSON.parse(event.data)
  } catch {
    return
  }
  const item = document.createElement("li")
  const time = document.createElement("span")
  time.textContent = new Date(payload.time).toLocaleTimeString()
  const type = document.createElement("strong")
  type.textContent = payload.type
  item.append(time, " ", type, " ", payload.message || payload.output || "")
  target.prepend(item)
  while (target.children.length > 80) {
    target.lastElementChild.remove()
  }
}

function text(selector, value) {
  const node = document.querySelector(selector)
  if (node) node.textContent = value ?? ""
}

function list(selector, values, render, empty) {
  const node = document.querySelector(selector)
  if (!node) return
  node.replaceChildren()
  if (!values.length) {
    const item = document.createElement("li")
    item.className = "muted"
    item.textContent = empty
    node.append(item)
    return
  }
  for (const value of values) {
    node.append(render(value))
  }
}
