function assetURL(source) {
  return source ? `/catalog-assets/${source}` : ''
}

const PREVIEW_REFRESH_MS = 60_000
const previewCache = new Map()

function livePreviewURL(agent, now = Date.now()) {
  const preview = agent.state === 'running' && agent.interfaces?.preview

  if (preview?.kind !== 'preview' || !preview.url) {
    return ''
  }

  try {
    const url = new URL(preview.url, globalThis.location?.href)

    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
      return ''
    }

    // Agent state is already refreshed every five seconds. A time bucket lets
    // those renders request a new thumbnail once a minute without adding a
    // second timer or forcing a new capture on every state poll.
    url.searchParams.set(
      'launcherPreview',
      Math.floor(now / PREVIEW_REFRESH_MS).toString()
    )

    return url.href
  } catch {
    return ''
  }
}

function preloadPreview(agentID, source) {
  let state = previewCache.get(agentID)

  if (!state) {
    state = {
      currentURL: '',
      pendingURL: '',
      pending: null,
    }
    previewCache.set(agentID, state)
  }

  if (state.currentURL === source) {
    return Promise.resolve(source)
  }

  if (state.pendingURL === source && state.pending) {
    return state.pending
  }

  const image = new Image()
  const loaded = new Promise((resolve, reject) => {
    image.addEventListener(
      'load',
      () => {
        const decoded =
          typeof image.decode === 'function'
            ? image.decode()
            : Promise.resolve()

        decoded.then(resolve, reject)
      },
      { once: true }
    )
    image.addEventListener(
      'error',
      () => reject(new Error('Agent preview could not be loaded')),
      { once: true }
    )
  })

  state.pendingURL = source
  state.pending = loaded
    .then(() => {
      if (state.pendingURL === source) {
        state.currentURL = source
        state.pendingURL = ''
        state.pending = null
      }

      return source
    })
    .catch((error) => {
      if (state.pendingURL === source) {
        state.pendingURL = ''
        state.pending = null
      }

      throw error
    })
  image.src = source

  return state.pending
}

function backgroundImages(...sources) {
  return sources
    .filter(Boolean)
    .map((source) => `url(${JSON.stringify(source)})`)
    .join(', ')
}

function statusFor(state) {
  if (state === 'running') {
    return 'ONLINE'
  }

  if (state === 'paused' || state === 'restarting') {
    return 'IDLE'
  }

  return 'OFFLINE'
}

function percentLabel(value) {
  if (!Number.isFinite(value)) {
    return '-'
  }

  if (value > 0 && value < 0.1) {
    return '<0.1%'
  }

  return `${value.toFixed(value < 10 ? 1 : 0)}%`
}

function formatUptime(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return '-'
  }

  const totalMinutes = Math.floor(seconds / 60)
  const days = Math.floor(totalMinutes / 1440)
  const hours = Math.floor((totalMinutes % 1440) / 60)
  const minutes = totalMinutes % 60

  if (days) {
    return `${days}d ${hours}h`
  }

  if (hours) {
    return `${hours}h ${minutes}m`
  }

  if (minutes) {
    return `${minutes}m`
  }

  return `${Math.floor(seconds)}s`
}

function emit(element, name, agent) {
  element.dispatchEvent(
    new CustomEvent(name, {
      bubbles: true,
      detail: { agent },
    })
  )
}

export class AgentCard extends HTMLElement {
  set data(value) {
    this.value = value
    this.render()
  }

  set variant(value) {
    this.cardVariant = value
    this.render()
  }

  connectedCallback() {
    this.render()
  }

  render() {
    if (!this.isConnected || !this.value) {
      return
    }

    const { agent, entry } = this.value
    const variant = this.cardVariant || 'row'
    const status = statusFor(agent.state)
    const running = status === 'ONLINE'
    const metrics = running ? agent.metrics : null
    const cpu = Number.isFinite(metrics?.cpuPercent) ? metrics.cpuPercent : null
    const memory = Number.isFinite(metrics?.memoryPercent)
      ? metrics.memoryPercent
      : null
    const screenshot = entry?.media?.screenshots?.[0]
    const screenshotURL = assetURL(screenshot?.source || entry?.media?.cover)
    const description =
      entry?.description || 'Local AI agent managed by Launcher.'
    const tags = entry?.tags || ['LOCAL', 'AGENT']

    this.className = `agent-card agent-card--${variant}`
    this.innerHTML =
      variant === 'home'
        ? `
        <div class="agent-card__heading">
          <div class="agent-card__identity">
            <span class="agent-card__icon" data-icon></span>
            <span>
              <strong data-name></strong>
              <small class="agent-status" data-status></small>
            </span>
          </div>
          <span class="agent-card__controls" data-controls></span>
        </div>
        <button class="agent-preview agent-preview--home" data-open
          aria-label="Open agent desktop"><span data-preview></span></button>
        <p class="agent-description" data-description></p>
        <div class="tag-list" data-tags></div>
      `
        : `
        <button class="agent-preview agent-preview--row" data-open
          aria-label="Open agent desktop"><span data-preview></span></button>
        <div class="agent-card__details">
          <div class="agent-card__title">
            <span class="agent-card__icon" data-icon></span>
            <strong data-name></strong>
            <small class="status-pill" data-status></small>
          </div>
          <p class="agent-description" data-description></p>
          <div class="tag-list" data-tags></div>
        </div>
        <div class="agent-metrics">
          <div class="metric">
            <span>CPU</span><strong data-cpu></strong>
            <i><b data-cpu-bar></b></i>
          </div>
          <div class="metric">
            <span>MEMORY</span><strong data-memory></strong>
            <i><b data-memory-bar></b></i>
          </div>
          <div class="metric metric--inline">
            <span>UPTIME</span><strong data-uptime></strong>
          </div>
        </div>
        <span class="agent-card__controls agent-card__controls--column"
          data-controls></span>
      `

    this.querySelectorAll('[data-name]').forEach((node) => {
      node.textContent = agent.name
    })
    this.querySelectorAll('[data-status]').forEach((node) => {
      node.textContent = status
      node.dataset.status = status.toLowerCase()
    })
    this.querySelector('[data-description]').textContent = description
    this.querySelector('[data-icon]').style.backgroundImage = `url("${assetURL(
      entry?.media?.icon
    )}")`

    const preview = this.querySelector('[data-preview]')
    const requestedPreview = livePreviewURL(agent)
    const cachedPreview = requestedPreview
      ? previewCache.get(agent.id)?.currentURL || ''
      : ''

    this.showPreview(
      preview,
      cachedPreview,
      screenshotURL,
      cachedPreview
        ? `Live preview of ${agent.name}`
        : screenshot?.alt || entry?.name || agent.name
    )

    if (requestedPreview && requestedPreview !== cachedPreview) {
      const agentID = agent.id

      preloadPreview(agentID, requestedPreview)
        .then((loadedPreview) => {
          if (
            !this.isConnected ||
            this.value?.agent.id !== agentID ||
            this.value.agent.state !== 'running' ||
            livePreviewURL(this.value.agent) !== loadedPreview ||
            this.querySelector('[data-preview]') !== preview
          ) {
            return
          }

          this.showPreview(
            preview,
            loadedPreview,
            screenshotURL,
            `Live preview of ${agent.name}`
          )
        })
        .catch(() => {
          // Keep the last successfully decoded preview, or the catalogue
          // screenshot when no live preview has loaded yet.
        })
    }

    const tagList = this.querySelector('[data-tags]')

    for (const tag of tags) {
      const item = document.createElement('span')

      item.textContent = tag
      tagList.append(item)
    }

    const controls = this.querySelector('[data-controls]')
    const toggle = this.controlButton(
      running ? '❚❚' : '▶',
      agent.pendingAction === 'stop' ? 'Stopping agent' : 'Toggle agent',
      () => {
        emit(this, 'toggle-agent', agent)
      }
    )

    toggle.disabled = Boolean(agent.pendingAction)

    if (agent.updateAvailable) {
      controls.append(
        this.controlButton('↑', 'Update agent', () => {
          emit(this, 'show-agent-update', agent)
        })
      )
    }

    controls.append(
      toggle,
      this.controlButton('↗', 'Open desktop', () => {
        emit(this, 'open-agent', agent)
      }),
      this.controlButton('⋮', 'Agent actions', () => {
        emit(this, 'agent-actions', agent)
      })
    )
    this.querySelector('[data-open]').addEventListener('click', () => {
      emit(this, 'open-agent', agent)
    })

    if (variant === 'row') {
      this.querySelector('[data-cpu]').textContent = percentLabel(cpu)
      this.querySelector('[data-memory]').textContent = percentLabel(memory)
      this.querySelector('[data-uptime]').textContent = running
        ? formatUptime(metrics?.uptimeSeconds)
        : '-'
      this.setBar('[data-cpu-bar]', cpu)
      this.setBar('[data-memory-bar]', memory)
    }
  }

  controlButton(label, title, onClick) {
    const button = document.createElement('button')

    button.type = 'button'
    button.className = 'icon-button'
    button.textContent = label
    button.title = title
    button.setAttribute('aria-label', title)
    button.addEventListener('click', onClick)

    return button
  }

  showPreview(element, liveURL, fallbackURL, label) {
    element.style.backgroundImage = backgroundImages(liveURL, fallbackURL)
    element.dataset.live = liveURL ? 'true' : 'false'
    element.setAttribute('aria-label', label)
  }

  setBar(selector, value) {
    const width = Number.isFinite(value) ? Math.min(100, Math.max(2, value)) : 0

    this.querySelector(selector).style.width = `${width}%`
  }
}

customElements.define('agent-card', AgentCard)
