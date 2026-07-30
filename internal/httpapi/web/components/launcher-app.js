import { LauncherAPI } from '../api.js'
import { desktopWindow } from '../desktop-window.js'
import {
  SIDEBAR_COLUMN_WIDTH,
  SIDEBAR_METRICS,
  nativeSidebar,
} from '../native-sidebar.js'
import './agent-actions-dialog.js'
import './agent-card.js'
import './deploy-dialog.js'
import './marketplace-card.js'
import './marketplace-detail.js'
import './runtime-setup-dialog.js'

const navigation = [
  ['home', 'HOME'],
  ['agents', 'AGENTS'],
  ['marketplace', 'MARKETPLACE'],
  ['activity', 'ACTIVITY'],
]

// The native sidebar follows macOS conventions rather than the interface's own
// upper-case styling, so it carries its own title-cased labels and SF Symbols.
const nativeNavigation = [
  ['home', 'Home', 'house'],
  ['agents', 'Agents', 'square.stack.3d.up'],
  ['marketplace', 'Marketplace', 'bag'],
  ['activity', 'Activity', 'waveform.path.ecg'],
]

const nativeSidebarItems = nativeNavigation.map(([id, title, symbol]) => ({
  id,
  title,
  symbol,
}))

const screens = new Set([
  'home',
  'agents',
  'marketplace',
  'marketplace-detail',
  'activity',
  'settings',
])

const dismissedUpdateKey = 'launcher-dismissed-update'

function readDismissedUpdateVersion() {
  try {
    return globalThis.localStorage?.getItem(dismissedUpdateKey) || ''
  } catch {
    return ''
  }
}

function saveDismissedUpdateVersion(version) {
  try {
    globalThis.localStorage?.setItem(dismissedUpdateKey, version)
  } catch {
    // A private browser context may make storage unavailable.
  }
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
    return '-'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const power = Math.min(
    Math.floor(Math.log(value) / Math.log(1024)),
    units.length - 1
  )

  return `${(value / 1024 ** power).toFixed(power > 2 ? 1 : 0)} ${units[power]}`
}

function lastLogLine(logs) {
  const lines = String(logs || '')
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
  const line = lines[lines.length - 1] || ''

  return line.length > 240 ? `${line.slice(0, 237)}…` : line
}

// Browser preview override, for `make web`: ?chrome=macos renders the layout
// the packaged macOS build gets — native-style sidebar panel and hero artwork
// bleeding underneath it.
function macOSChromePreviewRequested() {
  try {
    return (
      new URLSearchParams(globalThis.location?.search || '').get('chrome') ===
      'macos'
    )
  } catch {
    return false
  }
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
    this.catalogLoading = true
    this.catalogError = ''
    this.doctorReport = null
    this.runtimeSetup = null
    this.launcherStatus = null
    this.dismissedUpdateVersion = readDismissedUpdateVersion()
    this.activity = []
    this.marketplaceEntryID = null
    this.query = ''
    this.filter = 'all'
    this.page = 1
    this.agentRefreshPending = false
    this.startWatches = new Map()
  }

  connectedCallback() {
    if (this.initialized) {
      return
    }

    this.initialized = true
    this.applyPlatformChrome()
    this.renderShell()
    this.bindEvents()
    this.setUpNativeSidebar()
    this.render()
    this.refresh()
    this.refreshLauncherUpdate()
    this.refreshTimer = setInterval(() => this.refreshAgents(), 5000)
    this.updateRefreshTimer = setInterval(
      () => this.refreshLauncherUpdate(),
      30000
    )
  }

  disconnectedCallback() {
    clearInterval(this.refreshTimer)
    clearInterval(this.updateRefreshTimer)
  }

  renderShell() {
    this.innerHTML = `
      <div class="launcher-shell">
        <aside class="sidebar">
          <nav class="main-navigation" aria-label="Primary navigation"
            data-navigation></nav>
          <button class="sidebar__footer" type="button" data-runtime-setup
            title="Container runtime status">
            <span class="runtime-light"></span>
            <span><strong>LOCAL RUNTIME</strong><small data-runtime>CHECKING…</small></span>
          </button>
        </aside>
        <main class="main-panel">
          <div class="native-window-drag-region" aria-hidden="true"></div>
          <aside class="launcher-update-banner" data-launcher-update
            aria-label="Launcher update" hidden>
            <span class="launcher-update-banner__light"
              aria-hidden="true"></span>
            <span class="launcher-update-banner__copy">
              <small>LAUNCHER UPDATE AVAILABLE</small>
              <strong>
                <span data-launcher-current-version></span>
                <span aria-hidden="true">→</span>
                <span data-launcher-latest-version></span>
              </strong>
            </span>
            <button class="primary-button launcher-update-banner__action"
              type="button" data-open-launcher-update>
              VIEW RELEASE
            </button>
            <button class="text-button launcher-update-banner__dismiss"
              type="button" data-dismiss-launcher-update>
              REMIND ME LATER
            </button>
          </aside>
          <div class="content">
            <section class="screen" data-screen="home"></section>
            <section class="screen" data-screen="agents" hidden></section>
            <section class="screen" data-screen="marketplace" hidden></section>
            <section class="screen" data-screen="marketplace-detail"
              hidden></section>
            <section class="screen" data-screen="activity" hidden></section>
            <section class="screen" data-screen="settings" hidden></section>
          </div>
        </main>
      </div>
      <deploy-dialog></deploy-dialog>
      <agent-actions-dialog></agent-actions-dialog>
      <runtime-setup-dialog></runtime-setup-dialog>
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
    this.addEventListener('show-agent-update', (event) => {
      this.querySelector('agent-actions-dialog').open(
        event.detail.agent,
        'update'
      )
    })
    this.addEventListener('deploy-agent', (event) => {
      if (!this.doctorReport) {
        this.openRuntimeSetup()
        this.showToast(
          'Set up the local runtime before installing agents',
          true
        )

        return
      }

      this.querySelector('deploy-dialog').open(event.detail.entry)
    })
    this.addEventListener('view-marketplace-entry', (event) => {
      this.showMarketplaceEntry(event.detail.entry)
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
    this.addEventListener('open-agent-files', (event) => {
      this.openAgentFiles(event.detail.agent)
    })
    this.addEventListener('update-agent', (event) => {
      this.updateAgent(event.detail.agent)
    })
    this.addEventListener('delete-agent', (event) => {
      this.deleteAgent(event.detail.agent)
    })
    this.addEventListener('open-runtime-installer', (event) => {
      const dialog = this.querySelector('runtime-setup-dialog')
      const installURL = event.detail.setup?.installUrl

      if (!installURL || !desktopWindow.openExternal(installURL)) {
        dialog.showError('Could not open the official installation page.')
      }
    })
    this.addEventListener('check-runtime', () => {
      this.checkRuntime()
    })
    this.addEventListener('start-runtime', () => {
      this.startRuntime()
    })
    this.querySelector('[data-runtime-setup]').addEventListener('click', () => {
      if (this.doctorReport) {
        this.showToast(
          `${this.doctorReport.runtime} ${this.doctorReport.version} is ready`
        )

        return
      }

      this.openRuntimeSetup()
    })
    this.querySelector('[data-open-launcher-update]').addEventListener(
      'click',
      () => this.openLauncherUpdate()
    )
    this.querySelector('[data-dismiss-launcher-update]').addEventListener(
      'click',
      () => {
        this.dismissedUpdateVersion = this.launcherStatus?.latestVersion || ''
        saveDismissedUpdateVersion(this.dismissedUpdateVersion)
        this.renderUpdateBanner()
      }
    )
  }

  async refresh() {
    this.catalogLoading = true
    this.catalogError = ''
    this.renderScreen()

    try {
      const [doctor, catalog, instances] = await Promise.all([
        this.api.doctor(),
        this.api.catalog(),
        this.api.instances(),
      ])

      this.doctorReport = doctor.ready ? doctor.report : null
      this.runtimeSetup = doctor.ready ? null : doctor.setup
      this.catalog = catalog.catalog || []
      this.agents = instances.instances || []
      this.catalogLoading = false
      this.render()

      if (!doctor.ready) {
        this.openRuntimeSetup()
      }
    } catch (error) {
      console.error('Launcher startup failed:', error)
      this.catalogLoading = false
      this.catalogError = error.message
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
      await this.checkStartWatches()
      this.renderScreen()
    } catch (error) {
      console.warn('Agent refresh failed:', error)
    } finally {
      this.agentRefreshPending = false
    }
  }

  async refreshLauncherUpdate() {
    try {
      this.launcherStatus = await this.api.launcher()
      this.renderUpdateBanner()

      if (this.screen === 'home') {
        this.renderHome()
      }
    } catch (error) {
      console.warn('Launcher update check failed:', error)
    }
  }

  renderUpdateBanner() {
    const banner = this.querySelector('[data-launcher-update]')
    const status = this.launcherStatus
    const visible =
      status?.updateAvailable &&
      status.releaseUrl &&
      status.latestVersion !== this.dismissedUpdateVersion

    banner.hidden = !visible
    if (!visible) {
      return
    }

    banner.querySelector(
      '[data-launcher-current-version]'
    ).textContent = `v${status.currentVersion}`
    banner.querySelector(
      '[data-launcher-latest-version]'
    ).textContent = `v${status.latestVersion}`
  }

  openLauncherUpdate() {
    const releaseURL = this.launcherStatus?.releaseUrl

    if (!releaseURL || !desktopWindow.openExternal(releaseURL)) {
      this.showToast('Could not open the Launcher release page', true)
    }
  }

  render() {
    const runtimeControl = this.querySelector('[data-runtime-setup]')

    runtimeControl.classList.toggle(
      'sidebar__footer--unavailable',
      !this.doctorReport
    )
    this.querySelector('[data-runtime]').textContent = this.doctorReport
      ? `${this.doctorReport.runtime} ${this.doctorReport.version}`
      : 'UNAVAILABLE'
    this.renderScreen()
  }

  openRuntimeSetup() {
    this.querySelector('runtime-setup-dialog').open(this.runtimeSetup)
  }

  async checkRuntime() {
    const dialog = this.querySelector('runtime-setup-dialog')

    dialog.setBusy(true, 'CHECKING…')
    dialog.showError('')

    try {
      const result = await this.api.doctor()

      this.doctorReport = result.ready ? result.report : null
      this.runtimeSetup = result.ready ? null : result.setup
      this.render()

      if (result.ready) {
        dialog.close()
        this.showToast(
          `${result.report.runtime} ${result.report.version} is ready`
        )
      } else {
        dialog.open(result.setup)
        dialog.showError(
          result.setup?.state === 'missing'
            ? 'The installer has not been detected yet. Complete it, then check again.'
            : result.setup?.state === 'stopped'
            ? ''
            : result.error || 'The runtime is not ready yet.'
        )
      }
    } catch (error) {
      dialog.showError(error.message)
    } finally {
      dialog.setBusy(false)
    }
  }

  async startRuntime() {
    const dialog = this.querySelector('runtime-setup-dialog')

    dialog.setBusy(true, 'STARTING…')
    dialog.showError('')

    try {
      const result = await this.api.startRuntime()

      this.doctorReport = result.report
      this.runtimeSetup = null
      this.render()
      dialog.close()
      this.showToast(
        `${result.report.runtime} ${result.report.version} is ready`
      )
    } catch (error) {
      dialog.showError(error.message)
    } finally {
      dialog.setBusy(false)
    }
  }

  // macOS needs additional page layout because its SwiftUI shell places the
  // webview beneath a native sidebar and unified toolbar. Other platforms use
  // ordinary native window decorations without changing the page layout.
  applyPlatformChrome() {
    if (macOSChromePreviewRequested()) {
      this.applyMacosChrome(true)

      return
    }

    if (!desktopWindow.isDesktop()) {
      return
    }

    const platform =
      globalThis.navigator?.platform || globalThis.navigator?.userAgent || ''

    this.applyMacosChrome(/mac/i.test(platform))
  }

  applyMacosChrome(enabled) {
    document.documentElement.classList.toggle('is-macos-desktop', enabled)

    // The page clips itself to the window's shape so overlay scrollbars stop
    // short of the rounded corners instead of running into them.
    document.documentElement.style.setProperty(
      '--window-radius',
      `${SIDEBAR_METRICS.windowRadius}px`
    )
  }

  // In preview there is no native panel, so the HTML sidebar is restyled to
  // stand in for it rather than hidden — otherwise there is no navigation.
  applyNativeSidebarLayout(preview) {
    const shell = this.querySelector('.launcher-shell')

    if (!shell) {
      return
    }

    // The webview fills the whole window in both layouts and draws underneath
    // the sidebar, so the interface always reserves the space itself. Preview
    // mirrors the constants; the packaged app is told the measured insets as
    // the native column is resized or collapsed.
    this.applyNativeInsets(
      preview
        ? { sidebar: SIDEBAR_COLUMN_WIDTH, titlebar: 0 }
        : nativeSidebar.insets
    )
    shell.style.setProperty('--panel-inset', `${SIDEBAR_METRICS.inset}px`)
    shell.style.setProperty('--panel-width', `${SIDEBAR_METRICS.width}px`)
    shell.style.setProperty('--panel-radius', `${SIDEBAR_METRICS.radius}px`)
    shell.style.setProperty(
      '--panel-top-inset',
      `${SIDEBAR_METRICS.topInset}px`
    )
    shell.classList.add('has-native-sidebar')
    shell.classList.toggle('is-sidebar-preview', preview)
    document.documentElement.classList.toggle('is-swiftui-host', !preview)

    if (!preview) {
      return
    }

    // Show the labels the native panel shows, not the interface's own
    // upper-case ones, so the preview is not a hybrid of the two.
    for (const item of nativeSidebarItems) {
      const label = this.querySelector(
        `[data-screen-link="${item.id}"] .nav-label`
      )

      if (label) {
        label.textContent = item.title
      }
    }
  }

  // Set on the document rather than the shell: the dialogs are siblings of the
  // shell, and they have to clear the sidebar too.
  applyNativeInsets({ sidebar, titlebar }) {
    const style = document.documentElement.style

    style.setProperty('--sidebar-width', `${sidebar}px`)

    // Zero means SwiftUI has not measured the window yet. The stylesheet's own
    // value stands in until it has, rather than the page jumping to the top.
    if (titlebar > 0) {
      style.setProperty('--content-top', `${titlebar}px`)
    }
  }

  setUpNativeSidebar() {
    if (macOSChromePreviewRequested()) {
      this.applyNativeSidebarLayout(true)

      return
    }

    if (!nativeSidebar.available()) {
      return
    }

    if (this.nativeSidebarSetup) {
      return
    }
    this.nativeSidebarSetup = true

    nativeSidebar.onSelect((id) => this.setScreen(id))

    // Resizing or collapsing the native column moves the page's left edge.
    nativeSidebar.onInsets((insets) => this.applyNativeInsets(insets))

    // Only swap out the HTML navigation once the native sidebar confirms it
    // exists, so an unpatched build is left with a working interface.
    nativeSidebar.onReady(() => this.applyNativeSidebarLayout(false))

    nativeSidebar.configure(nativeSidebarItems, this.screen)
  }

  setScreen(screen) {
    if (!screens.has(screen)) {
      return
    }

    this.screen = screen

    if (screen !== 'agents') {
      this.page = 1
    }

    const navigationScreen =
      screen === 'marketplace-detail' ? 'marketplace' : screen

    if (nativeSidebar.ready) {
      nativeSidebar.select(nativeSidebarItems, navigationScreen)
    }

    this.updateNavigation()
    this.renderScreen()
    this.scrollToTop()
  }

  // A new screen starts at its own top: where the last one had been scrolled to
  // means nothing here. Both scrollers are reset because the document owns
  // scrolling under the native macOS shell and .main-panel owns it everywhere
  // else - whichever is not in use simply sits at zero already.
  scrollToTop() {
    this.querySelector('.main-panel')?.scrollTo({ top: 0 })
    globalThis.scrollTo({ top: 0 })
  }

  updateNavigation() {
    this.querySelectorAll('[data-screen-link]').forEach((link) => {
      const active =
        link.dataset.screenLink === this.screen ||
        (this.screen === 'marketplace-detail' &&
          link.dataset.screenLink === 'marketplace')

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

    if (this.screen === 'marketplace-detail') {
      this.renderMarketplaceDetail()
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
            your workflow-all on this computer.
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
    const launcherVersion = this.launcherStatus?.currentVersion || '-'
    const launcherVersionLabel =
      launcherVersion === 'dev'
        ? 'DEV'
        : launcherVersion === '-'
        ? '-'
        : `V${launcherVersion}`
    let launcherVersionDetail = 'UPDATE STATUS UNKNOWN'

    if (launcherVersion === 'dev') {
      launcherVersionDetail = 'DEVELOPMENT BUILD'
    } else if (this.launcherStatus?.updateAvailable) {
      launcherVersionDetail = `V${this.launcherStatus.latestVersion} AVAILABLE`
    } else if (this.launcherStatus?.checking) {
      launcherVersionDetail = 'CHECKING FOR UPDATES'
    } else if (this.launcherStatus?.checkedAt) {
      launcherVersionDetail = 'UP TO DATE'
    }

    const overviewItems = [
      ['INSTALLED AGENTS', this.agents.length, 'MANAGED BY LAUNCHER'],
      ['RUNNING AGENTS', online, 'LOCAL CONTAINERS'],
      [
        'RUNTIME',
        this.doctorReport?.runtime?.toUpperCase() || '-',
        this.doctorReport?.version || 'NOT DETECTED',
      ],
      ['LAUNCHER', launcherVersionLabel, launcherVersionDetail],
    ]

    for (const [label, value, detail] of overviewItems) {
      const item = document.createElement('div')

      item.classList.toggle(
        'overview-stat--update',
        label === 'LAUNCHER' && this.launcherStatus?.updateAvailable
      )
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
      ['TOTAL CPU', running.length ? `${cpu.toFixed(1)}%` : '-', cpu],
      ['MEMORY IN USE', formatBytes(memory), running.length ? 62 : 0],
      [
        'RUNTIME',
        this.doctorReport?.runtime?.toUpperCase() || '-',
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
        <span>${
          this.catalogLoading
            ? 'LOADING AGENTS…'
            : `${this.catalog.length} AGENTS AVAILABLE`
        }</span>
      </section>
      <div class="marketplace-grid" data-marketplace-grid></div>
    `

    const grid = screen.querySelector('[data-marketplace-grid]')

    if (this.catalogLoading) {
      grid.append(
        this.loadingState(
          'LOADING MARKETPLACE',
          'Fetching application publishers and their latest releases.'
        )
      )

      return
    }

    if (!this.catalog.length) {
      grid.append(
        this.emptyState(
          'CATALOGUE UNAVAILABLE',
          this.catalogError ||
            'Launcher could not load any application publisher.'
        )
      )

      return
    }

    for (const entry of this.catalog) {
      const card = document.createElement('marketplace-card')

      card.data = {
        entry,
        instances: this.agents.filter((agent) => agent.catalogId === entry.id),
      }
      grid.append(card)
    }
  }

  showMarketplaceEntry(entry) {
    this.marketplaceEntryID = entry.id
    this.setScreen('marketplace-detail')
  }

  renderMarketplaceDetail() {
    const screen = this.querySelector('[data-screen="marketplace-detail"]')
    const entry = this.catalog.find(
      (catalogEntry) => catalogEntry.id === this.marketplaceEntryID
    )

    screen.innerHTML = ''

    if (!entry) {
      screen.append(
        this.emptyState(
          'IMAGE UNAVAILABLE',
          'This application image is no longer available.',
          'BACK TO MARKETPLACE',
          'marketplace'
        )
      )

      return
    }

    const detail = document.createElement('marketplace-detail')

    detail.data = {
      entry,
      instances: this.agents.filter((agent) => agent.catalogId === entry.id),
    }
    screen.append(detail)
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

  loadingState(title, copy) {
    const loading = this.emptyState(title, copy)

    loading.classList.add('loading-state')
    loading.setAttribute('role', 'status')
    loading.setAttribute('aria-live', 'polite')

    return loading
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
        item.querySelector('[data-activity-agent]').textContent = activity.agent
      }

      container.append(item)
    }
  }

  async toggleAgent(agent) {
    const shouldStop = agent.state === 'running'

    try {
      if (shouldStop) {
        this.startWatches.delete(agent.id)
        await this.api.stop(agent.id)
        await this.refreshAgents()
        this.recordActivity(
          'stop',
          `Stopped ${agent.name}`,
          agent.name,
          'Agent stopped successfully'
        )
        this.showToast(`${agent.name} stopped`)

        return
      }

      await this.api.start(agent.id)

      const now = Date.now()

      this.startWatches.set(agent.id, {
        name: agent.name,
        reportAfter: now + 750,
        expiresAt: now + 15000,
        confirmed: false,
      })
      setTimeout(() => this.refreshAgents(), 750)
      await this.refreshAgents()
    } catch (error) {
      this.showToast(error.message, true)
    }
  }

  async checkStartWatches() {
    for (const [id, watch] of this.startWatches) {
      const agent = this.agents.find((current) => current.id === id)

      if (agent?.state === 'running') {
        if (!watch.confirmed) {
          watch.confirmed = true
          this.recordActivity(
            'start',
            `Started ${watch.name}`,
            watch.name,
            'Agent started successfully'
          )
          this.showToast(`${watch.name} started`)
        }

        if (Date.now() >= watch.expiresAt) {
          this.startWatches.delete(id)
        }

        continue
      }

      if (Date.now() < watch.reportAfter) {
        continue
      }

      if (
        agent &&
        (agent.state === 'restarting' || agent.state === 'paused') &&
        Date.now() < watch.expiresAt
      ) {
        continue
      }

      this.startWatches.delete(id)

      let detail = ''

      try {
        const result = await this.api.logs(id)

        detail = lastLogLine(result.logs)
      } catch (error) {
        console.warn('Failed to load startup logs:', error)
      }

      const message = `${watch.name} failed to start${
        detail ? `: ${detail}` : ''
      }`

      this.recordActivity(
        'error',
        `Failed to start ${watch.name}`,
        watch.name,
        detail || 'The container stopped during startup'
      )
      this.showToast(message, true)
    }
  }

  async openAgent(agent) {
    if (agent.state !== 'running') {
      this.showToast(`Start ${agent.name} before opening it`, true)

      return
    }

    if (!desktopWindow.isDesktop()) {
      if (!desktopWindow.openExternal(agent.url)) {
        this.showToast(`Could not open ${agent.name} in a browser window`, true)

        return
      }

      this.recordActivity(
        'open',
        `Opened ${agent.name} in a browser`,
        agent.name,
        'Desktop windows require the packaged Launcher'
      )
      this.showToast(`${agent.name} opened in a browser window`)

      return
    }

    try {
      await this.api.openViewer(agent.id)
      this.recordActivity(
        'open',
        `Opened ${agent.name} in a window`,
        agent.name,
        'Independent framed agent window opened'
      )
      this.showToast(`${agent.name} opened in its own window`)
    } catch (error) {
      this.showToast(`Could not open ${agent.name}: ${error.message}`, true)
    }
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

  async openAgentFiles(agent) {
    const dialog = this.querySelector('agent-actions-dialog')

    dialog.setBusy(true)

    try {
      await this.api.openFiles(agent.id)
      this.recordActivity(
        'open',
        `Opened ${agent.name} files`,
        agent.name,
        'Opened Launcher-managed local files'
      )
      dialog.close()
      this.showToast(`${agent.name} local files opened`)
    } catch (error) {
      this.showToast(error.message, true)
    } finally {
      dialog.setBusy(false)
    }
  }

  async updateAgent(agent) {
    const dialog = this.querySelector('agent-actions-dialog')

    dialog.setBusy(true)
    dialog.showError('update', '')

    try {
      const updated = await this.api.update(agent.id, (progress) => {
        dialog.setUpdateProgress(progress)
      })

      dialog.setUpdateProgress({
        stage: 'ready',
        message: `${agent.name} is up to date`,
      })
      this.recordActivity(
        'update',
        `Updated ${agent.name}`,
        agent.name,
        `${agent.image} → ${updated.image}`
      )
      await this.refreshAgents()
      setTimeout(() => dialog.close(), 500)
      this.showToast(`${agent.name} updated`)
    } catch (error) {
      dialog.showError('update', error.message)
      await this.refreshAgents()
    } finally {
      dialog.setBusy(false)
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
