export class RuntimeSetupDialog extends HTMLElement {
  connectedCallback() {
    if (this.initialized) {
      return
    }

    this.initialized = true
    this.innerHTML = `
      <dialog class="launcher-dialog runtime-setup-dialog"
        data-runtime-setup-dialog>
        <section class="dialog-panel">
          <header class="dialog-heading">
            <div>
              <small class="eyebrow">LOCAL RUNTIME</small>
              <h2>SET UP CONTAINERS</h2>
            </div>
            <button class="icon-button" type="button" data-close
              aria-label="Close runtime setup">×</button>
          </header>
          <div class="runtime-setup__status">
            <span aria-hidden="true" data-status-icon>!</span>
            <div>
              <small data-status-label>REQUIRED</small>
              <strong data-runtime-name>CONTAINER RUNTIME</strong>
              <p data-summary></p>
            </div>
          </div>
          <ol class="runtime-setup__steps" data-steps></ol>
          <p class="runtime-setup__guidance" data-guidance hidden></p>
          <p class="form-error" data-error hidden></p>
          <footer class="dialog-footer">
            <button class="text-button" type="button" data-close>
              DO THIS LATER
            </button>
            <button class="secondary-button" type="button" data-recheck>
              CHECK AGAIN
            </button>
            <button class="primary-button" type="button" data-primary>
              OPEN INSTALLATION PAGE
            </button>
          </footer>
        </section>
      </dialog>
    `
    this.dialog = this.querySelector('dialog')
    this.querySelectorAll('[data-close]').forEach((button) => {
      button.addEventListener('click', () => {
        if (!this.busy) {
          this.close()
        }
      })
    })
    this.dialog.addEventListener('cancel', (event) => {
      if (this.busy) {
        event.preventDefault()
      }
    })
    this.querySelector('[data-recheck]').addEventListener('click', () => {
      if (!this.busy) {
        this.dispatch('check-runtime')
      }
    })
    this.querySelector('[data-primary]').addEventListener('click', () => {
      if (this.busy) {
        return
      }

      this.dispatch(
        this.setup?.state === 'stopped'
          ? 'start-runtime'
          : 'open-runtime-installer'
      )
    })
  }

  open(setup) {
    this.setup = setup || {
      state: 'error',
      runtime: 'Container runtime',
      message: 'Launcher could not detect a working container runtime.',
    }
    this.setBusy(false)
    this.showError('')
    this.renderState()

    if (!this.dialog.open) {
      this.dialog.showModal()
    }
  }

  close() {
    if (this.dialog?.open) {
      this.dialog.close()
    }
  }

  renderState() {
    const missing = this.setup.state === 'missing'
    const stopped = this.setup.state === 'stopped'
    const runtime = this.setup.runtime || 'Container runtime'
    const apple = runtime.toLowerCase().includes('apple')
    const steps = missing
      ? apple
        ? [
            'Open the official Apple Container release page.',
            'Download the signed installer package ending in installer-signed.pkg.',
            'Open the package and approve the macOS administrator prompt.',
            'Return to Launcher and select Check Again.',
          ]
        : [
            'Open the official Docker installation page.',
            'Install Docker for this computer.',
            'Start Docker and wait until its engine is ready.',
            'Return to Launcher and select Check Again.',
          ]
      : stopped
      ? [
          'Apple Container is installed, but its background service is stopped.',
          'Select Start Runtime below.',
          'Approve the recommended Linux kernel download if macOS asks.',
          'Launcher will verify the service before enabling agents.',
        ]
      : [
          'Review the runtime error below.',
          'Correct the runtime installation or service problem.',
          'Select Check Again to verify the runtime.',
        ]

    this.querySelector('[data-status-label]').textContent = missing
      ? 'INSTALLATION REQUIRED'
      : stopped
      ? 'SERVICE STOPPED'
      : 'RUNTIME UNAVAILABLE'
    this.querySelector('[data-runtime-name]').textContent =
      runtime.toUpperCase()
    this.querySelector('[data-summary]').textContent = missing
      ? `${runtime} is required before Launcher can install or run local agents.`
      : stopped
      ? `${runtime} is installed and ready to be started.`
      : this.setup.message

    const list = this.querySelector('[data-steps]')

    list.replaceChildren()

    for (const instruction of steps) {
      const item = document.createElement('li')

      item.textContent = instruction
      list.append(item)
    }

    const guidance = this.querySelector('[data-guidance]')

    guidance.textContent = this.setup.guidance || ''
    guidance.hidden = !this.setup.guidance

    const primary = this.querySelector('[data-primary]')

    primary.hidden = !missing && !stopped
    primary.textContent = stopped ? 'START RUNTIME' : 'OPEN INSTALLATION PAGE'
    this.querySelector('[data-status-icon]').textContent = stopped ? '▶' : '!'
  }

  setBusy(busy, label = '') {
    this.busy = busy
    this.querySelectorAll('button').forEach((button) => {
      button.disabled = busy
    })

    const primary = this.querySelector('[data-primary]')

    if (busy && label) {
      primary.textContent = label
    } else if (this.setup) {
      primary.textContent =
        this.setup.state === 'stopped'
          ? 'START RUNTIME'
          : 'OPEN INSTALLATION PAGE'
    }
  }

  showError(message) {
    const error = this.querySelector('[data-error]')

    error.textContent = message
    error.hidden = !message
  }

  dispatch(name) {
    this.dispatchEvent(
      new CustomEvent(name, {
        bubbles: true,
        detail: { setup: this.setup },
      })
    )
  }
}

customElements.define('runtime-setup-dialog', RuntimeSetupDialog)
