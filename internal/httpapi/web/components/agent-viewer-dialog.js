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
              allow="clipboard-read; clipboard-write; fullscreen"></iframe>
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
    this.dialog.addEventListener('close', () => {
      this.frame.src = 'about:blank'
      this.agent = null
    })
  }

  open(agent) {
    this.agent = agent
    this.querySelector('[data-viewer-title]').textContent =
      String(agent.name || 'AGENT VIEW').toUpperCase()
    this.frame.title = `${agent.name} live view`
    this.showError('')
    this.setBusy(false)
    this.load(agent.url)

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
    if (this.agent?.url) {
      this.load(this.agent.url)
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
