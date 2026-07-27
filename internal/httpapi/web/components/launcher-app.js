import { LauncherAPI } from '../api.js'
import { desktopWindow } from '../desktop-window.js'
import './agent-actions-dialog.js'
import './agent-card.js'
import './deploy-dialog.js'
import './marketplace-card.js'

const navigation = [
  ['home', 'HOME'],
  ['agents', 'AGENTS'],
  ['marketplace', 'MARKETPLACE'],
  ['activity', 'ACTIVITY'],
]

const pageTitles = {
  home: ['AGENT LAUNCHER', 'LOCAL AGENT ORCHESTRATOR'],
  agents: ['MY AGENTS', 'MANAGE YOUR AGENTS'],
  marketplace: ['MARKETPLACE', 'DISCOVER LOCAL AGENTS'],
  activity: ['ACTIVITY', 'WHAT YOUR LAUNCHER HAS BEEN DOING'],
  settings: ['SETTINGS', 'LAUNCHER PREFERENCES'],
}

function statusFor(agent) {
  if (agent.state === 'running') {
    return 'online'
  }

  if (agent.state === 'paused' || agent.state === 'restarting') {
    return 'idle'
  }

  return 'offline'
}

function formatTime(value) {
  return new Intl.DateTimeFormat(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(value)
}

function formatBytes(value) {
  if (!Number.isFinite(value) || value <= 0) {
    return '—'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const power = Math.min(
    Math.floor(Math.log(value) / Math.log(1024)),
    units.length - 1
  )

  return `${(value / 1024 ** power).toFixed(power > 2 ? 1 : 0)} ${units[power]}`
}

function catalogueEntry(catalog, agent) {
  return catalog.find((item) => item.id === agent.catalogId)
}

export class LauncherApp extends HTMLElement {
  constructor() {
    super()
    this.api = new LauncherAPI()
    this.screen = 'home'
    this.agents = []
    this.catalog = []
    this.doctorReport = null
    this.activity = []
    this.query = ''
    this.filter = 'all'
    this.page = 1
    this.agentRefreshPending = false
  }

  connectedCallback() {
    if (this.initialized) {
      return
    }

    this.initialized = true
    this.renderShell()
    this.bindEvents()
    this.refresh()
    this.refreshTimer = setInterval(() => this.refreshAgents(), 5000)
  }

  disconnectedCallback() {
    clearInterval(this.refreshTimer)
  }

  renderShell() {
    this.innerHTML = `
      <div class="launcher-shell">
        <aside class="sidebar">
          <a class="brand" href="#" data-screen-link="home"
            aria-label="Agent Launcher home">
            <img src="/assets/logo.png" alt="" draggable="false">
          </a>
          <nav class="main-navigation" aria-label="Primary navigation"
            data-navigation></nav>
          <div class="sidebar__footer">
            <span class="runtime-light"></span>
            <span><strong>LOCAL RUNTIME</strong><small data-runtime>CHECKING…</small></span>
          </div>
        </aside>
        <main class="main-panel">
          <header class="topbar">
            <div>
              <h1 data-page-title>AGENT LAUNCHER</h1>
              <p data-page-subtitle>LOCAL AGENT ORCHESTRATOR</p>
            </div>
            <div class="window-controls" aria-label="Window controls">
              <button type="button" data-window-action="minimise"
                aria-label="Minimise window" title="Minimise">
                —
              </button>
              <button type="button" data-window-action="maximise"
                aria-label="Maximise or restore window" title="Maximise or restore">
                ◆
              </button>
              <button type="button" data-window-action="close"
                aria-label="Close Agent Launcher" title="Close">
                ×
              </button>
            </div>
          </header>
          <div class="content">
            <section class="screen" data-screen="home"></section>
            <section class="screen" data-screen="agents" hidden></section>
            <section class="screen" data-screen="marketplace" hidden></section>
            <section class="screen" data-screen="activity" hidden></section>
            <section class="screen" data-screen="settings" hidden></section>
          </div>
        </main>
      </div>
      <deploy-dialog></deploy-dialog>
      <agent-actions-dialog></agent-actions-dialog>
      <div class="toast" role="status" aria-live="polite" data-toast hidden></div>
    `

    const nav = this.querySelector('[data-navigation]')

    for (const [id, label] of navigation) {
      const link = document.createElement('button')

      link.type = 'button'
      link.dataset.screenLink = id
      link.innerHTML = `
        <span class="nav-marker" aria-hidden="true"></span>
        <span class="nav-label"></span>
      `
      link.querySelector('.nav-label').textContent = label
      nav.append(link)
    }

    this.renderSettings()
    this.updateNavigation()
  }

  bindEvents() {
    this.querySelector('.topbar').addEventListener('dblclick', (event) => {
      if (!event.target.closest('[data-window-action]')) {
        desktopWindow.toggleMaximise()
      }
    })
    this.querySelector('.window-controls').addEventListener('click', (event) => {
      const button = event.target.closest('[data-window-action]')

      if (!button) {
        return
      }

      switch (button.dataset.windowAction) {
      case 'maximise':
        desktopWindow.toggleMaximise()
        break
      case 'minimise':
        desktopWindow.minimise()
        break
      case 'close':
        desktopWindow.close()
        break
      }
    })
    this.addEventListener('click', (event) => {
      const link = event.target.closest('[data-screen-link]')

      if (link) {
        event.preventDefault()
        this.setScreen(link.dataset.screenLink)
      }
    })
    this.addEventListener('toggle-agent', (event) => {
      this.toggleAgent(event.detail.agent)
    })
    this.addEventListener('open-agent', (event) => {
      this.openAgent(event.detail.agent)
    })
    this.addEventListener('agent-actions', (event) => {
      this.querySelector('agent-actions-dialog').open(event.detail.agent)
    })
    this.addEventListener('deploy-agent', (event) => {
      this.querySelector('deploy-dialog').open(event.detail.entry)
    })
    this.addEventListener('install-agent', (event) => {
      this.installAgent(event.detail)
    })
    this.addEventListener('rename-agent', (event) => {
      this.renameAgent(event.detail)
    })
    this.addEventListener('load-agent-logs', (event) => {
      this.loadAgentLogs(event.detail.agent)
    })
    this.addEventListener('delete-agent', (event) => {
      this.deleteAgent(event.detail.agent)
    })
  }

  async refresh() {
    try {
      const [doctor, catalog, instances] = await Promise.all([
        this.api.doctor(),
        this.api.catalog(),
        this.api.instances(),
      ])

      this.doctorReport = doctor.report || null
      this.catalog = catalog.catalog || []
      this.agents = instances.instances || []
      this.render()
    } catch (error) {
      console.error('Launcher startup failed:', error)
      this.showToast(error.message, true)
      this.render()
    }
  }

  async refreshAgents() {
    if (this.agentRefreshPending) {
      return
    }

    this.agentRefreshPending = true

    try {
      const result = await this.api.instances()

      this.agents = result.instances || []
      this.renderScreen()
    } catch (error) {
      console.warn('Agent refresh failed:', error)
    } finally {
      this.agentRefreshPending = false
    }
  }

  render() {
    this.querySelector('[data-runtime]').textContent = this.doctorReport
      ? `${this.doctorReport.runtime} ${this.doctorReport.version}`
      : 'UNAVAILABLE'
    this.renderScreen()
  }

  setScreen(screen) {
    if (!pageTitles[screen]) {
      return
    }

    this.screen = screen

    if (screen !== 'agents') {
      this.page = 1
    }

    this.updateNavigation()
    this.renderScreen()
  }

  updateNavigation() {
    this.querySelectorAll('[data-screen-link]').forEach((link) => {
      const active = link.dataset.screenLink === this.screen

      link.classList.toggle('is-active', active)

      if (active) {
        link.setAttribute('aria-current', 'page')
      } else {
        link.removeAttribute('aria-current')
      }
    })
    this.querySelectorAll('[data-screen]').forEach((screen) => {
      screen.hidden = screen.dataset.screen !== this.screen
    })

    const [title, subtitle] = pageTitles[this.screen]

    this.querySelector('[data-page-title]').textContent = title
    this.querySelector('[data-page-subtitle]').textContent = subtitle
  }

  renderScreen() {
    this.updateNavigation()

    if (this.screen === 'home') {
      this.renderHome()
    }

    if (this.screen === 'agents') {
      this.renderAgents()
    }

    if (this.screen === 'marketplace') {
      this.renderMarketplace()
    }

    if (this.screen === 'activity') {
      this.renderActivity()
    }

    if (this.screen === 'settings') {
      this.updateSettings()
    }
  }

  renderHome() {
    const screen = this.querySelector('[data-screen="home"]')

    screen.innerHTML = `
      <section class="hero">
        <div class="hero__art"></div>
        <div class="hero__copy">
          <small class="eyebrow">YOUR LOCAL AGENT WORKSPACE</small>
          <h2>Your Agents.<br><mark>Your Rules.</mark></h2>
          <p>
            Deploy specialized AI agents to automate, assist, and accelerate
            your workflow—all on this computer.
          </p>
          <div class="button-row">
            <button class="primary-button" data-screen-link="marketplace">
              DEPLOY NEW AGENT <span>＋</span>
            </button>
            <button class="secondary-button" data-screen-link="marketplace">
              BROWSE MARKETPLACE
            </button>
          </div>
        </div>
        <div class="hero__stat">
          <small class="eyebrow">ACTIVE AGENTS</small>
          <strong data-home-online></strong>
          <span>RUNNING LOCALLY</span>
        </div>
      </section>
      <div class="dashboard-grid">
        <section class="panel panel--agents">
          <header class="panel-heading">
            <div><small class="eyebrow">YOUR AGENTS</small>
              <h3>READY WHEN YOU ARE</h3></div>
            <button class="text-button" data-screen-link="agents">
              VIEW ALL →
            </button>
          </header>
          <div class="home-agent-grid" data-home-agents></div>
        </section>
        <aside class="dashboard-stack">
          <section class="panel">
            <small class="eyebrow">QUICK ACTIONS</small>
            <button class="quick-action" data-screen-link="marketplace">
              <span>＋</span><strong>DEPLOY AGENT</strong><i>→</i>
            </button>
            <button class="quick-action" data-screen-link="agents">
              <span>◈</span><strong>MANAGE AGENTS</strong><i>→</i>
            </button>
          </section>
          <section class="panel">
            <small class="eyebrow">RECENT ACTIVITY</small>
            <div class="mini-activity" data-mini-activity></div>
            <button class="text-button text-button--wide"
              data-screen-link="activity">VIEW ALL ACTIVITY →</button>
          </section>
        </aside>
      </div>
      <section class="panel system-overview">
        <small class="eyebrow">SYSTEM OVERVIEW</small>
        <div class="overview-stats" data-overview></div>
      </section>
    `

    const online = this.agents.filter(
      (agent) => statusFor(agent) === 'online'
    ).length

    screen.querySelector('[data-home-online]').textContent = online

    const agentGrid = screen.querySelector('[data-home-agents]')

    if (!this.agents.length) {
      agentGrid.append(
        this.emptyState(
          'NO AGENTS DEPLOYED',
          'Visit the Marketplace to create your first local agent.',
          'OPEN MARKETPLACE',
          'marketplace'
        )
      )
    } else {
      for (const agent of this.agents.slice(0, 2)) {
        agentGrid.append(this.agentCard(agent, 'home'))
      }
    }

    this.renderActivityList(
      screen.querySelector('[data-mini-activity]'),
      this.activity.slice(0, 3),
      true
    )

    const overview = screen.querySelector('[data-overview]')
    const overviewItems = [
      ['INSTALLED AGENTS', this.agents.length, 'MANAGED BY LAUNCHER'],
      ['RUNNING AGENTS', online, 'LOCAL CONTAINERS'],
      [
        'RUNTIME',
        this.doctorReport?.runtime?.toUpperCase() || '—',
        this.doctorReport?.version || 'NOT DETECTED',
      ],
    ]

    for (const [label, value, detail] of overviewItems) {
      const item = document.createElement('div')

      item.innerHTML = '<small></small><strong></strong><span></span>'
      item.querySelector('small').textContent = label
      item.querySelector('strong').textContent = value
      item.querySelector('span').textContent = detail
      overview.append(item)
    }
  }

  renderAgents() {
    const screen = this.querySelector('[data-screen="agents"]')

    screen.innerHTML = `
      <div class="agents-layout">
        <section class="agents-main">
          <div class="agent-toolbar">
            <label class="search-box">
              <span aria-hidden="true">⌕</span>
              <input type="search" placeholder="Search agents…"
                aria-label="Search agents" data-search>
            </label>
            <div class="filter-list" data-filters></div>
          </div>
          <div class="agent-list" data-agent-list></div>
          <footer class="pagination-footer">
            <span data-result-count></span>
            <nav class="pagination" aria-label="Agent pages"
              data-pagination></nav>
          </footer>
        </section>
        <aside class="agents-sidebar">
          <section class="panel">
            <small class="eyebrow">AGENTS OVERVIEW</small>
            <strong class="large-number" data-total></strong>
            <span class="large-number__label">TOTAL AGENTS</span>
            <div class="status-counts" data-status-counts></div>
          </section>
          <section class="panel">
            <small class="eyebrow">RESOURCE USAGE</small>
            <div class="resource-list" data-resources></div>
          </section>
        </aside>
      </div>
    `

    const search = screen.querySelector('[data-search]')

    search.value = this.query
    search.addEventListener('input', (event) => {
      this.query = event.target.value
      this.page = 1
      this.renderAgents()
      this.querySelector('[data-search]').focus()
    })

    const filters = [
      ['all', 'ALL STATUS'],
      ['online', 'ONLINE'],
      ['idle', 'IDLE'],
      ['offline', 'OFFLINE'],
    ]
    const filterList = screen.querySelector('[data-filters]')

    for (const [id, label] of filters) {
      const button = document.createElement('button')

      button.type = 'button'
      button.textContent = label
      button.classList.toggle('is-active', this.filter === id)
      button.addEventListener('click', () => {
        this.filter = id
        this.page = 1
        this.renderAgents()
      })
      filterList.append(button)
    }

    const needle = this.query.trim().toLowerCase()
    const matchingAgents = this.agents.filter((agent) => {
      const statusMatches =
        this.filter === 'all' || statusFor(agent) === this.filter
      const entry = catalogueEntry(this.catalog, agent)
      const text = `${agent.name} ${entry?.name || ''}`.toLowerCase()

      return statusMatches && (!needle || text.includes(needle))
    })
    const pageSize = 6
    const pageCount = Math.max(1, Math.ceil(matchingAgents.length / pageSize))

    this.page = Math.min(this.page, pageCount)

    const pages = Array.from({ length: pageCount }, (_, index) => index + 1)
    const pageStart = (this.page - 1) * pageSize
    const visibleAgents = matchingAgents.slice(pageStart, pageStart + pageSize)
    const list = screen.querySelector('[data-agent-list]')

    if (!visibleAgents.length) {
      list.append(
        this.emptyState(
          'NO MATCHING AGENTS',
          'Try another search or deploy a new agent.',
          'BROWSE MARKETPLACE',
          'marketplace'
        )
      )
    } else {
      for (const agent of visibleAgents) {
        list.append(this.agentCard(agent, 'row'))
      }
    }

    screen.querySelector(
      '[data-result-count]'
    ).textContent = `SHOWING ${visibleAgents.length} OF ${matchingAgents.length} AGENTS`

    const pagination = screen.querySelector('[data-pagination]')

    for (const page of pages) {
      const button = document.createElement('button')

      button.type = 'button'
      button.textContent = page
      button.classList.toggle('is-active', page === this.page)
      button.setAttribute('aria-label', `Page ${page}`)

      if (page === this.page) {
        button.setAttribute('aria-current', 'page')
      }

      button.addEventListener('click', () => {
        this.page = page
        this.renderAgents()
      })
      pagination.append(button)
    }

    this.renderAgentSummary(screen)
  }

  renderAgentSummary(screen) {
    const counts = {
      online: this.agents.filter((agent) => statusFor(agent) === 'online')
        .length,
      idle: this.agents.filter((agent) => statusFor(agent) === 'idle').length,
      offline: this.agents.filter((agent) => statusFor(agent) === 'offline')
        .length,
    }

    screen.querySelector('[data-total]').textContent = this.agents.length

    const countList = screen.querySelector('[data-status-counts]')

    for (const status of ['online', 'idle', 'offline']) {
      const item = document.createElement('span')

      item.innerHTML = '<strong></strong><small></small>'
      item.querySelector('strong').textContent = counts[status]
      item.querySelector('small').textContent = status.toUpperCase()
      countList.append(item)
    }

    const running = this.agents.filter((agent) => agent.state === 'running')
    const cpu = running.reduce(
      (sum, agent) => sum + (agent.metrics?.cpuPercent || 0),
      0
    )
    const memory = running.reduce(
      (sum, agent) => sum + (agent.metrics?.memoryUsageBytes || 0),
      0
    )
    const resources = [
      ['TOTAL CPU', running.length ? `${cpu.toFixed(1)}%` : '—', cpu],
      ['MEMORY IN USE', formatBytes(memory), running.length ? 62 : 0],
      [
        'RUNTIME',
        this.doctorReport?.runtime?.toUpperCase() || '—',
        this.doctorReport ? 100 : 0,
      ],
    ]
    const resourceList = screen.querySelector('[data-resources]')

    for (const [label, value, percent] of resources) {
      const item = document.createElement('div')

      item.innerHTML = `
        <span><strong></strong><b></b></span>
        <div class="progress-track"><i></i></div>
      `
      item.querySelector('strong').textContent = label
      item.querySelector('b').textContent = value
      item.querySelector('i').style.width = `${Math.min(
        100,
        Math.max(0, percent)
      )}%`
      resourceList.append(item)
    }
  }

  renderMarketplace() {
    const screen = this.querySelector('[data-screen="marketplace"]')

    screen.innerHTML = `
      <section class="marketplace-hero">
        <div>
          <small class="eyebrow">BUILT-IN CATALOGUE</small>
          <h2>FIND YOUR<br><mark>NEXT AGENT.</mark></h2>
          <p>
            Pick an agent, give it a name, and Launcher handles the machinery.
          </p>
        </div>
        <span>${this.catalog.length} AGENTS AVAILABLE</span>
      </section>
      <div class="marketplace-grid" data-marketplace-grid></div>
    `

    const grid = screen.querySelector('[data-marketplace-grid]')

    if (!this.catalog.length) {
      grid.append(
        this.emptyState(
          'CATALOGUE UNAVAILABLE',
          'Launcher could not load its built-in catalogue.'
        )
      )

      return
    }

    for (const entry of this.catalog) {
      const card = document.createElement('marketplace-card')

      card.data = {
        entry,
        installed: this.agents.some((agent) => agent.catalogId === entry.id),
      }
      grid.append(card)
    }
  }

  renderActivity() {
    const screen = this.querySelector('[data-screen="activity"]')

    screen.innerHTML = `
      <section class="activity-heading">
        <div>
          <small class="eyebrow">LAUNCHER EVENT LOG</small>
          <h2>RECENT ACTIVITY</h2>
        </div>
        <span>THIS SESSION</span>
      </section>
      <section class="panel activity-panel">
        <div class="activity-list" data-activity-list></div>
      </section>
    `
    this.renderActivityList(
      screen.querySelector('[data-activity-list]'),
      this.activity
    )
  }

  renderSettings() {
    const screen = this.querySelector('[data-screen="settings"]')

    screen.innerHTML = `
      <section class="settings-heading">
        <small class="eyebrow">EXPERIMENTAL</small>
        <h2>LAUNCHER SETTINGS</h2>
        <p>
          These controls are implemented for the future desktop shell.
          Settings are intentionally hidden from the main navigation for now.
        </p>
      </section>
      <section class="panel settings-list">
        <label>
          <span><strong>START WITH THE SYSTEM</strong>
            <small>Open Launcher when you sign in.</small></span>
          <input type="checkbox" disabled>
        </label>
        <label>
          <span><strong>OPEN AGENT AFTER START</strong>
            <small>Show its desktop when startup completes.</small></span>
          <input type="checkbox" disabled>
        </label>
        <div>
          <span><strong>DATA DIRECTORY</strong>
            <small data-settings-root>Checking…</small></span>
        </div>
      </section>
    `
  }

  updateSettings() {
    const root = this.querySelector('[data-settings-root]')

    if (root) {
      root.textContent = this.doctorReport?.dataRoot || 'Unavailable'
    }
  }

  agentCard(agent, variant) {
    const card = document.createElement('agent-card')

    card.variant = variant
    card.data = {
      agent,
      entry: catalogueEntry(this.catalog, agent),
    }

    return card
  }

  emptyState(title, copy, action, screen) {
    const empty = document.createElement('div')

    empty.className = 'empty-state'
    empty.innerHTML = `
      <span aria-hidden="true">◇</span>
      <strong></strong>
      <p></p>
    `
    empty.querySelector('strong').textContent = title
    empty.querySelector('p').textContent = copy

    if (action && screen) {
      const button = document.createElement('button')

      button.type = 'button'
      button.className = 'primary-button primary-button--outline'
      button.dataset.screenLink = screen
      button.textContent = action
      empty.append(button)
    }

    return empty
  }

  renderActivityList(container, events, compact = false) {
    if (!events.length) {
      const empty = document.createElement('p')

      empty.className = 'activity-empty'
      empty.textContent = 'Actions you take in Launcher will appear here.'
      container.append(empty)

      return
    }

    for (const activity of events) {
      const item = document.createElement('div')

      item.className = compact
        ? 'activity-item activity-item--compact'
        : 'activity-item activity-item--full'
      item.innerHTML = compact
        ? `
          <i></i>
          <span><strong></strong><time></time></span>
        `
        : `
          <time></time>
          <i></i>
          <span><strong></strong><small></small></span>
          <b data-activity-agent></b>
        `
      item.querySelector('i').dataset.type = activity.type
      item.querySelector('strong').textContent = activity.message
      item.querySelector('time').textContent = formatTime(activity.time)
      if (!compact) {
        item.querySelector('small').textContent = activity.detail
        item.querySelector('[data-activity-agent]').textContent =
          activity.agent
      }
      container.append(item)
    }
  }

  async toggleAgent(agent) {
    const shouldStop = agent.state === 'running'

    try {
      if (shouldStop) {
        await this.api.stop(agent.id)
      } else {
        await this.api.start(agent.id)
      }

      this.recordActivity(
        shouldStop ? 'stop' : 'start',
        `${shouldStop ? 'Stopped' : 'Started'} ${agent.name}`,
        agent.name,
        `Agent ${shouldStop ? 'stopped' : 'started'} successfully`
      )
      await this.refreshAgents()
      this.showToast(`${agent.name} ${shouldStop ? 'stopped' : 'started'}`)
    } catch (error) {
      this.showToast(error.message, true)
    }
  }

  openAgent(agent) {
    if (agent.state !== 'running') {
      this.showToast(`Start ${agent.name} before opening it`, true)

      return
    }

    window.open(agent.url, '_blank', 'noopener')
    this.recordActivity(
      'open',
      `Opened ${agent.name}`,
      agent.name,
      'Agent desktop opened in a new tab'
    )
  }

  async installAgent({ entry, name }) {
    const dialog = this.querySelector('deploy-dialog')

    dialog.setBusy(true)
    dialog.showError('')

    try {
      const agent = await this.api.install(entry.id, name, (progress) => {
        dialog.setProgress(progress)
      })

      dialog.setProgress({
        stage: 'ready',
        message: `${agent.name} is ready`,
      })
      this.recordActivity(
        'install',
        `Installed ${agent.name}`,
        agent.name,
        `${entry.name} installed and started`
      )
      await this.refreshAgents()
      setTimeout(() => dialog.close(), 500)
      this.showToast(`${agent.name} is ready`)
    } catch (error) {
      dialog.showError(error.message)
    } finally {
      dialog.setBusy(false)
    }
  }

  async renameAgent({ agent, name }) {
    const dialog = this.querySelector('agent-actions-dialog')

    dialog.setBusy(true)
    dialog.showError('rename', '')

    try {
      await this.api.rename(agent.id, name)
      this.recordActivity(
        'rename',
        `Renamed ${agent.name}`,
        name,
        `Agent is now called ${name}`
      )
      dialog.close()
      await this.refreshAgents()
      this.showToast(`Agent renamed to ${name}`)
    } catch (error) {
      dialog.showError('rename', error.message)
    } finally {
      dialog.setBusy(false)
    }
  }

  async loadAgentLogs(agent) {
    const dialog = this.querySelector('agent-actions-dialog')

    dialog.setLogs('Loading logs…')
    dialog.showError('logs', '')

    try {
      const result = await this.api.logs(agent.id)

      dialog.setLogs(result.logs)
    } catch (error) {
      dialog.setLogs('')
      dialog.showError('logs', error.message)
    }
  }

  async deleteAgent(agent) {
    const dialog = this.querySelector('agent-actions-dialog')

    dialog.setBusy(true)
    dialog.showError('delete', '')

    try {
      await this.api.delete(agent.id)
      this.recordActivity(
        'delete',
        `Deleted ${agent.name}`,
        agent.name,
        'Agent and managed files removed'
      )
      dialog.close()
      await this.refreshAgents()
      this.showToast(`${agent.name} deleted`)
    } catch (error) {
      dialog.showError('delete', error.message)
    } finally {
      dialog.setBusy(false)
    }
  }

  recordActivity(type, message, agent, detail) {
    this.activity.unshift({
      type,
      message,
      agent,
      detail,
      time: new Date(),
    })
    this.activity = this.activity.slice(0, 50)

    if (this.screen === 'activity' || this.screen === 'home') {
      this.renderScreen()
    }
  }

  showToast(message, error = false) {
    const toast = this.querySelector('[data-toast]')

    toast.textContent = message
    toast.classList.toggle('toast--error', error)
    toast.hidden = false
    clearTimeout(this.toastTimer)
    this.toastTimer = setTimeout(() => {
      toast.hidden = true
    }, 4200)
  }
}

customElements.define('launcher-app', LauncherApp)
