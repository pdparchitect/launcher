function assetURL(source) {
  return source ? `/catalog-assets/${source}` : ''
}

export class DeployDialog extends HTMLElement {
  connectedCallback() {
    if (this.initialized) {
      return
    }

    this.initialized = true
    this.innerHTML = `
      <dialog class="launcher-dialog deploy-dialog"
        data-launcher-deploy-dialog>
        <form method="dialog" class="dialog-panel" data-form>
          <header class="dialog-heading">
            <div>
              <small class="eyebrow">NEW LOCAL AGENT</small>
              <h2>DEPLOY AGENT</h2>
            </div>
            <button class="icon-button" data-close type="button"
              aria-label="Close deployment dialog">×</button>
          </header>
          <div class="deploy-dialog__cover" data-cover></div>
          <div class="deploy-dialog__identity">
            <span class="deploy-dialog__icon" data-icon></span>
            <span>
              <strong data-name></strong>
              <small data-publisher></small>
            </span>
          </div>
          <p class="dialog-copy" data-description></p>
          <label class="field">
            <span>AGENT NAME</span>
            <input name="agentName" data-agent-name maxlength="64"
              autocomplete="off" required>
          </label>
          <div class="install-progress" data-progress hidden>
            <div class="install-progress__heading">
              <strong data-progress-stage>PREPARING</strong>
              <span data-progress-value>5%</span>
            </div>
            <div class="progress-track"><i data-progress-bar></i></div>
            <p data-progress-message>Preparing the installation…</p>
          </div>
          <p class="form-error" data-error hidden></p>
          <footer class="dialog-footer">
            <button class="secondary-button" data-cancel type="button">
              CANCEL
            </button>
            <button class="primary-button" type="submit"
              data-submit>INSTALL & START</button>
          </footer>
        </form>
      </dialog>
    `
    this.dialog = this.querySelector('dialog')
    this.form = this.querySelector('[data-form]')
    this.form.addEventListener('submit', (event) => {
      event.preventDefault()

      if (!this.entry || this.busy || !this.form.reportValidity()) {
        return
      }

      this.dispatchEvent(
        new CustomEvent('install-agent', {
          bubbles: true,
          detail: {
            entry: this.entry,
            name: this.querySelector('[data-agent-name]').value.trim(),
          },
        })
      )
    })
    this.dialog.addEventListener('cancel', (event) => {
      if (this.busy) {
        event.preventDefault()
      }
    })
    this.querySelectorAll('[data-close], [data-cancel]').forEach((button) => {
      button.addEventListener('click', (event) => {
        if (this.busy) {
          event.preventDefault()
        }

        if (!this.busy) {
          this.close()
        }
      })
    })
  }

  open(entry) {
    this.entry = entry
    this.setBusy(false)
    this.showError('')
    this.querySelector('[data-progress]').hidden = true
    this.querySelector('[data-cover]').style.backgroundImage = `url("${assetURL(
      entry.media?.cover
    )}")`
    this.querySelector('[data-icon]').style.backgroundImage = `url("${assetURL(
      entry.media?.icon
    )}")`
    this.querySelector('[data-name]').textContent = entry.name
    this.querySelector('[data-publisher]').textContent = `BY ${
      entry.publisher || 'INDEPENDENT'
    }`
    this.querySelector('[data-description]').textContent = entry.description

    const name = this.querySelector('[data-agent-name]')

    name.value = entry.name

    if (!this.dialog.open) {
      this.dialog.showModal()
    }

    requestAnimationFrame(() => name.select())
  }

  close() {
    if (this.dialog?.open) {
      this.dialog.close()
    }
  }

  setBusy(busy) {
    this.busy = busy
    this.querySelector('[data-submit]').disabled = busy
    this.querySelector('[data-cancel]').disabled = busy
    this.querySelector('[data-close]').disabled = busy
    this.querySelector('[data-agent-name]').disabled = busy
    this.querySelector('[data-submit]').textContent = busy
      ? 'INSTALLING…'
      : 'INSTALL & START'
  }

  setProgress(update) {
    const progress = this.querySelector('[data-progress]')

    progress.hidden = false

    const values = {
      preparing: 8,
      pulling: 48,
      creating: 72,
      starting: 90,
      ready: 100,
    }
    const value = values[update.stage] || 24

    this.querySelector('[data-progress-stage]').textContent = String(
      update.stage || 'INSTALLING'
    ).toUpperCase()
    this.querySelector('[data-progress-value]').textContent = `${value}%`
    this.querySelector('[data-progress-bar]').style.width = `${value}%`
    this.querySelector('[data-progress-message]').textContent =
      update.message || 'Installing the agent…'
  }

  showError(message) {
    const error = this.querySelector('[data-error]')

    error.textContent = message
    error.hidden = !message
  }
}

customElements.define('deploy-dialog', DeployDialog)
