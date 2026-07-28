import { desktopWindow } from '../desktop-window.js'

function embeddedViewerURL(rawURL, viewer) {
  if (viewer !== 'kasmvnc') {
    return rawURL
  }

  try {
    const url = new URL(rawURL)

    url.searchParams.set('show_control_bar', 'true')
    url.searchParams.set('resize', 'remote')
    url.searchParams.set('clipboard_up', 'true')
    url.searchParams.set('clipboard_down', 'true')
    url.searchParams.set('clipboard_seamless', 'true')
    url.searchParams.set('enable_threading', 'false')
    url.searchParams.set('enable_webp', 'false')

    return url.toString()
  } catch {
    return rawURL
  }
}

export class AgentViewerDialog extends HTMLElement {
  connectedCallback() {
    if (this.initialized) {
      return
    }

    this.initialized = true
    this.innerHTML = `
      <dialog class="launcher-dialog viewer-dialog"
        data-agent-viewer-dialog>
        <section class="viewer-panel">
          <header class="viewer-heading">
            <div>
              <small class="eyebrow">LIVE AGENT</small>
              <h2 data-viewer-title>AGENT VIEW</h2>
            </div>
            <div class="viewer-heading__actions">
              <button class="secondary-button" type="button"
                data-viewer-paste hidden>PASTE CLIPBOARD</button>
              <button class="secondary-button viewer-reload" type="button"
                data-viewer-reload>RELOAD</button>
              <button class="primary-button" type="button"
                data-viewer-popout>OPEN IN WINDOW ↗</button>
              <button class="icon-button" type="button" data-viewer-close
                aria-label="Close agent view">×</button>
            </div>
          </header>
          <div class="viewer-stage">
            <div class="viewer-loading" data-viewer-loading>
              <span aria-hidden="true"></span>
              <strong>CONNECTING TO AGENT…</strong>
            </div>
            <iframe data-viewer-frame title="Agent view"
              tabindex="0"
              allow="autoplay; microphone; camera; clipboard-read;
                clipboard-write; window-management; fullscreen"></iframe>
          </div>
          <p class="form-error viewer-error" data-viewer-error hidden></p>
        </section>
      </dialog>
    `

    this.dialog = this.querySelector('dialog')
    this.frame = this.querySelector('[data-viewer-frame]')

    this.querySelector('[data-viewer-close]').addEventListener('click', () => {
      this.close()
    })
    this.querySelector('[data-viewer-reload]').addEventListener('click', () => {
      this.reload()
    })
    this.querySelector('[data-viewer-paste]').addEventListener('click', () => {
      this.pasteClipboard()
    })
    this.querySelector('[data-viewer-popout]').addEventListener('click', () => {
      if (!this.busy && this.agent) {
        this.dispatchEvent(
          new CustomEvent('pop-out-agent', {
            bubbles: true,
            composed: true,
            detail: { agent: this.agent },
          })
        )
      }
    })
    this.frame.addEventListener('load', () => {
      if (this.frame.getAttribute('src') !== 'about:blank') {
        this.querySelector('[data-viewer-loading]').hidden = true
      }
    })
    this.frame.addEventListener('pointerenter', () => {
      this.frame.focus({ preventScroll: true })
    })
    this.dialog.addEventListener('close', () => {
      this.frame.src = 'about:blank'
      this.agent = null
      this.viewer = null
    })
  }

  open(agent, viewer = 'web') {
    this.agent = agent
    this.viewer = viewer
    this.viewerURL = embeddedViewerURL(agent.url, viewer)
    this.querySelector('[data-viewer-paste]').hidden = viewer !== 'kasmvnc'
    this.querySelector('[data-viewer-title]').textContent = String(
      agent.name || 'AGENT VIEW'
    ).toUpperCase()
    this.frame.title = `${agent.name} live view`
    this.showError('')
    this.setBusy(false)
    this.load(this.viewerURL)

    if (!this.dialog.open) {
      this.dialog.showModal()
    }
  }

  close() {
    if (this.dialog?.open) {
      this.dialog.close()
    }
  }

  reload() {
    if (this.viewerURL) {
      this.load(this.viewerURL)
    }
  }

  async pasteClipboard() {
    const button = this.querySelector('[data-viewer-paste]')

    if (button.disabled || this.viewer !== 'kasmvnc') {
      return
    }

    button.disabled = true
    this.showError('')

    try {
      const text = await desktopWindow.readClipboardText()

      if (!text) {
        throw new Error('The clipboard is empty')
      }

      const origin = new URL(this.viewerURL).origin

      this.frame.contentWindow?.postMessage(
        { action: 'clipboardsnd', value: text },
        origin
      )
      button.textContent = 'PASTED ✓'
      setTimeout(() => {
        button.textContent = 'PASTE CLIPBOARD'
      }, 1200)
    } catch (error) {
      this.showError(`Could not paste: ${error.message}`)
    } finally {
      button.disabled = false
    }
  }

  load(url) {
    this.querySelector('[data-viewer-loading]').hidden = false
    this.frame.src = 'about:blank'
    requestAnimationFrame(() => {
      this.frame.src = url
    })
  }

  setBusy(busy) {
    this.busy = busy
    this.querySelectorAll('button').forEach((button) => {
      button.disabled = busy
    })
    this.querySelector('[data-viewer-popout]').textContent = busy
      ? 'OPENING…'
      : 'OPEN IN WINDOW ↗'
  }

  showError(message) {
    const error = this.querySelector('[data-viewer-error]')

    error.textContent = message
    error.hidden = !message
  }
}

customElements.define('agent-viewer-dialog', AgentViewerDialog)
