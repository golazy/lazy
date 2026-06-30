import { Controller } from "@hotwired/stimulus"
import { Terminal } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"

export default class extends Controller {
  static targets = ["container", "status"]
  static values = {
    url: String,
  }

  connect() {
    this.encoder = new TextEncoder()
    this.fitAddon = new FitAddon()
    this.terminal = new Terminal({
      cursorBlink: true,
      fontFamily: 'ui-monospace, "SFMono-Regular", Consolas, monospace',
      fontSize: 12,
      scrollback: 5000,
      theme: {
        background: cssVar("--base", "#ffffff"),
        foreground: cssVar("--text", "#202124"),
        cursor: cssVar("--text", "#202124"),
        selectionBackground: cssVar("--row-selected", "#d3e3fd"),
      },
    })
    this.terminal.loadAddon(this.fitAddon)
    this.terminal.open(this.containerTarget)
    this.fit()
    this.terminal.focus()
    this.inputDisposable = this.terminal.onData((data) => {
      if (this.socket?.readyState === WebSocket.OPEN) {
        this.socket.send(this.encoder.encode(data))
      }
    })
    this.resizeObserver = new ResizeObserver(() => this.fitAndResize())
    this.resizeObserver.observe(this.containerTarget)
    this.connectSocket()
  }

  disconnect() {
    this.resizeObserver?.disconnect()
    this.inputDisposable?.dispose()
    this.socket?.close()
    this.terminal?.dispose()
  }

  connectSocket() {
    this.setStatus("Connecting")
    this.socket = new WebSocket(this.websocketURL())
    this.socket.binaryType = "arraybuffer"
    this.socket.addEventListener("open", () => {
      this.setStatus("Connected")
      this.sendResize()
    })
    this.socket.addEventListener("message", (event) => {
      if (typeof event.data === "string") {
        this.handleControl(event.data)
        return
      }
      this.terminal.write(new Uint8Array(event.data))
    })
    this.socket.addEventListener("close", () => this.setStatus("Disconnected"))
    this.socket.addEventListener("error", () => this.setStatus("Connection error"))
  }

  websocketURL() {
    const url = new URL(this.urlValue, window.location.href)
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:"
    url.searchParams.set("rows", this.terminal.rows || "24")
    url.searchParams.set("cols", this.terminal.cols || "80")
    return url.toString()
  }

  fitAndResize() {
    this.fit()
    this.sendResize()
  }

  fit() {
    try {
      this.fitAddon.fit()
    } catch (_error) {
      // xterm can throw while the panel is being removed during Turbo visits.
    }
  }

  sendResize() {
    if (this.socket?.readyState !== WebSocket.OPEN || !this.terminal) {
      return
    }
    this.socket.send(JSON.stringify({
      type: "resize",
      rows: this.terminal.rows,
      cols: this.terminal.cols,
    }))
  }

  handleControl(data) {
    let message
    try {
      message = JSON.parse(data)
    } catch (_error) {
      return
    }
    if (message.type === "status" && message.state) {
      this.setStatus(message.state)
    }
    if (message.type === "error" && message.error) {
      this.setStatus(message.error)
    }
  }

  setStatus(value) {
    if (this.hasStatusTarget) {
      this.statusTarget.textContent = value
    }
  }
}

function cssVar(name, fallback) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}
