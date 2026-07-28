export class AgentActionsDialog extends HTMLElement {
  connectedCallback() {
    if (this.initialized) {
      return
    }

    this.initialized = true

    this.innerHTML = `
      <dialog class="launcher-dialog actions-dialog"
        data-launcher-actions-dialog>
        <section class="dialog-panel">
          <header class="dialog-heading">
            <div>
              <small class="eyebrow">AGENT CONTROLS</small>
              <h2 data-title>AGENT ACTIONS</h2>
            </div>
            <button class="icon-button" type="button" data-close
              aria-label="Close agent actions">×</button>
          </header>
          <div data-menu>
            <p class="dialog-copy">
              Manage this agent without exposing container details.
            </p>
            <div class="action-menu">
              <button type="button" data-action="update"
                data-update-action hidden>
                <span>↑</span><strong>UPDATE AGENT</strong><i>›</i>
              </button>
              <button type="button" data-action="rename">
                <span>✎</span><strong>RENAME AGENT</strong><i>›</i>
              </button>
              <button type="button" data-action="logs">
                <span>⌘</span><strong>VIEW LOGS</strong><i>›</i>
              </button>
              <button type="button" data-open-files>
                <span>▰</span><strong>OPEN LOCAL FILES</strong><i>›</i>
              </button>
              <button type="button" data-action="delete"
                class="action-menu__danger">
                <span>×</span><strong>DELETE AGENT</strong><i>›</i>
              </button>
            </div>
          </div>
          <form data-rename hidden>
            <label class="field">
              <span>NEW AGENT NAME</span>
              <input name="name" data-rename-name maxlength="64"
                autocomplete="off" required>
            </label>
            <p class="form-error" data-rename-error hidden></p>
            <footer class="dialog-footer">
              <button class="secondary-button" type="button"
                data-back>BACK</button>
              <button class="primary-button" type="submit"
                data-rename-submit>SAVE NAME</button>
            </footer>
          </form>
          <div data-logs hidden>
            <pre class="agent-logs" data-log-output>Loading logs…</pre>
            <p class="form-error" data-logs-error hidden></p>
            <footer class="dialog-footer">
              <button class="secondary-button" type="button"
                data-back>BACK</button>
              <button class="primary-button primary-button--outline"
                type="button" data-refresh-logs>REFRESH</button>
            </footer>
          </div>
          <div data-update hidden>
            <div class="update-notice">
              <strong>UPDATE THIS AGENT?</strong>
              <p>
                Launcher will replace only the runtime container. Its
                workspace, credentials, name, and port will be preserved.
              </p>
              <dl>
                <div><dt>CURRENT</dt><dd data-current-image></dd></div>
                <div><dt>AVAILABLE</dt><dd data-available-image></dd></div>
              </dl>
            </div>
            <div class="install-progress update-progress"
              data-update-progress hidden>
              <div class="install-progress__heading">
                <strong data-update-progress-stage>PREPARING</strong>
                <span data-update-progress-value>5%</span>
              </div>
              <div class="progress-track">
                <i data-update-progress-bar></i>
              </div>
              <p data-update-progress-message>Preparing the update…</p>
              <details class="install-log" data-update-log>
                <summary>
                  <span>UPDATE LOG</span>
                  <span data-update-log-count>0 ENTRIES</span>
                </summary>
                <pre data-update-log-output></pre>
              </details>
            </div>
            <p class="form-error" data-update-error hidden></p>
            <footer class="dialog-footer">
              <button class="secondary-button" type="button"
                data-back>BACK</button>
              <button class="primary-button" type="button"
                data-confirm-update>UPDATE AGENT</button>
            </footer>
          </div>
          <div data-delete hidden>
            <div class="danger-notice">
              <strong>DELETE THIS AGENT?</strong>
              <p>
                Its container and Launcher-managed files will be removed.
                This cannot be undone.
              </p>
            </div>
            <p class="form-error" data-delete-error hidden></p>
            <footer class="dialog-footer">
              <button class="secondary-button" type="button"
                data-back>KEEP AGENT</button>
              <button class="danger-button" type="button"
                data-confirm-delete>DELETE PERMANENTLY</button>
            </footer>
          </div>
        </section>
      </dialog>
    `

    this.dialog = this.querySelector('dialog')

    this.querySelector('[data-close]').addEventListener('click', () => {
      if (!this.busy) {
        this.close()
      }
    })

    this.dialog.addEventListener('cancel', (event) => {
      if (this.busy) {
        event.preventDefault()
      }
    })

    this.querySelectorAll('[data-action]').forEach((button) => {
      button.addEventListener('click', () => {
        this.setMode(button.dataset.action)
      })
    })

    this.querySelectorAll('[data-back]').forEach((button) => {
      button.addEventListener('click', () => this.setMode('menu'))
    })

    this.querySelector('[data-rename]').addEventListener('submit', (event) => {
      event.preventDefault()

      if (this.busy || !event.currentTarget.reportValidity()) {
        return
      }

      this.dispatch('rename-agent', {
        name: this.querySelector('[data-rename-name]').value.trim(),
      })
    })

    this.querySelector('[data-refresh-logs]').addEventListener('click', () => {
      if (!this.busy) {
        this.dispatch('load-agent-logs')
      }
    })

    this.querySelector('[data-open-files]').addEventListener('click', () => {
      if (!this.busy) {
        this.dispatch('open-agent-files')
      }
    })

    this.querySelector('[data-confirm-update]').addEventListener(
      'click',
      () => {
        if (!this.busy) {
          this.dispatch('update-agent')
        }
      }
    )

    this.querySelector('[data-confirm-delete]').addEventListener(
      'click',
      () => {
        if (!this.busy) {
          this.dispatch('delete-agent')
        }
      }
    )
  }

  open(agent, mode = 'menu') {
    this.agent = agent

    this.setBusy(false)
    this.querySelector('[data-update-action]').hidden = !agent.updateAvailable
    this.querySelector('[data-rename-name]').value = agent.name
    this.querySelector('[data-current-image]').textContent = agent.image
    this.querySelector('[data-available-image]').textContent =
      agent.availableImage || 'No update available'
    this.clearErrors()
    this.setMode(
      mode === 'update' && agent.updateAvailable ? 'update' : 'menu'
    )

    if (!this.dialog.open) {
      this.dialog.showModal()
    }
  }

  close() {
    if (this.dialog?.open) {
      this.dialog.close()
    }
  }

  setMode(mode) {
    this.mode = mode
    this.querySelector('[data-menu]').hidden = mode !== 'menu'
    this.querySelector('[data-rename]').hidden = mode !== 'rename'
    this.querySelector('[data-logs]').hidden = mode !== 'logs'
    this.querySelector('[data-update]').hidden = mode !== 'update'
    this.querySelector('[data-delete]').hidden = mode !== 'delete'

    const titles = {
      menu: this.agent?.name || 'AGENT ACTIONS',
      rename: 'RENAME AGENT',
      logs: 'AGENT LOGS',
      update: 'UPDATE AGENT',
      delete: 'DELETE AGENT',
    }

    this.querySelector('[data-title]').textContent = String(
      titles[mode]
    ).toUpperCase()
    this.clearErrors()

    if (mode === 'rename') {
      requestAnimationFrame(() => {
        this.querySelector('[data-rename-name]').select()
      })
    }

    if (mode === 'logs') {
      this.dispatch('load-agent-logs')
    }

    if (mode === 'update') {
      this.resetUpdateProgress()
    }
  }

  setBusy(busy) {
    this.busy = busy
    this.querySelectorAll('button, input').forEach((control) => {
      control.disabled = busy
    })
    this.querySelector('[data-rename-submit]').textContent = busy
      ? 'SAVING…'
      : 'SAVE NAME'
    this.querySelector('[data-confirm-delete]').textContent = busy
      ? 'DELETING…'
      : 'DELETE PERMANENTLY'
    this.querySelector('[data-confirm-update]').textContent = busy
      ? 'UPDATING…'
      : 'UPDATE AGENT'
  }

  showError(mode, message) {
    const error = this.querySelector(`[data-${mode}-error]`)

    if (!error) {
      return
    }

    error.textContent = message
    error.hidden = !message

    if (mode === 'update' && message) {
      this.appendUpdateLog('error', message)
      this.querySelector('[data-update-log]').open = true
    }
  }

  setLogs(logs) {
    this.querySelector('[data-log-output]').textContent =
      logs || 'No recent output.'
  }

  clearErrors() {
    this.querySelectorAll('.form-error').forEach((error) => {
      error.textContent = ''
      error.hidden = true
    })
  }

  resetUpdateProgress() {
    this.updateLogLines = []
    this.lastUpdateLogLine = ''
    this.querySelector('[data-update-progress]').hidden = true
    this.querySelector('[data-update-log]').open = false
    this.querySelector('[data-update-log-output]').textContent = ''
    this.updateUpdateLogCount()
  }

  setUpdateProgress(update) {
    const progress = this.querySelector('[data-update-progress]')
    const values = {
      preparing: 8,
      stopping: 62,
      replacing: 78,
      starting: 92,
      ready: 100,
    }
    const indeterminate =
      update.stage === 'pulling' || update.stage === 'restoring'
    const value = values[update.stage] || 28
    const track = progress.querySelector('.progress-track')

    progress.hidden = false
    track.classList.toggle('progress-track--indeterminate', indeterminate)
    this.querySelector('[data-update-progress-stage]').textContent = String(
      update.stage || 'UPDATING'
    ).toUpperCase()
    this.querySelector('[data-update-progress-value]').textContent =
      update.stage === 'pulling'
        ? 'DOWNLOADING'
        : update.stage === 'restoring'
          ? 'RECOVERING'
          : `${value}%`
    this.querySelector('[data-update-progress-bar]').style.width =
      indeterminate ? '32%' : `${value}%`
    this.querySelector('[data-update-progress-message]').textContent =
      update.message || 'Updating the agent…'
    this.appendUpdateLog(update.stage, update.message)
  }

  appendUpdateLog(stage, message) {
    const cleanMessage = String(message || '').trim()

    if (!cleanMessage) {
      return
    }

    const line = `[${String(stage || 'updating').toUpperCase()}] ${cleanMessage}`

    if (line === this.lastUpdateLogLine) {
      return
    }

    this.lastUpdateLogLine = line
    this.updateLogLines ??= []
    this.updateLogLines.push(line)

    if (this.updateLogLines.length > 300) {
      this.updateLogLines.shift()
    }

    const output = this.querySelector('[data-update-log-output]')

    output.textContent = this.updateLogLines.join('\n')
    output.scrollTop = output.scrollHeight
    this.updateUpdateLogCount()
  }

  updateUpdateLogCount() {
    const count = this.updateLogLines?.length || 0

    this.querySelector('[data-update-log-count]').textContent =
      `${count} ${count === 1 ? 'ENTRY' : 'ENTRIES'}`
  }

  dispatch(name, detail = {}) {
    this.dispatchEvent(
      new CustomEvent(name, {
        bubbles: true,
        detail: { agent: this.agent, ...detail },
      })
    )
  }
}

customElements.define('agent-actions-dialog', AgentActionsDialog)
