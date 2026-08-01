function assetURL(source) {
  return source ? `/catalog-assets/${source}` : ''
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

function emit(element, name, agent) {
  element.dispatchEvent(
    new CustomEvent(name, {
      bubbles: true,
      detail: { agent },
    })
  )
}

export class MarketplaceDetail extends HTMLElement {
  set data(value) {
    this.value = value
    this.render()
  }

  connectedCallback() {
    this.render()
  }

  render() {
    if (!this.isConnected || !this.value) {
      return
    }

    const { entry, instances = [] } = this.value
    const runningCount = instances.filter(
      (instance) => instance.state === 'running'
    ).length
    const screenshot = entry.media?.screenshots?.[0]
    const screenshotSource = screenshot?.source || entry.media?.cover

    this.className = 'marketplace-detail'
    this.innerHTML = `
      <section class="hero marketplace-detail__hero">
        <div class="hero__art marketplace-detail__hero-art"
          aria-hidden="true"></div>
        <div class="hero__copy">
          <small class="eyebrow">MARKETPLACE IMAGE</small>
          <h2 data-name></h2>
          <p data-description></p>
          <div class="button-row">
            <button class="primary-button" type="button"
              data-install></button>
            <button class="secondary-button" type="button"
              data-screen-link="marketplace">BACK TO MARKETPLACE</button>
          </div>
        </div>
        <div class="hero__stat">
          <small class="eyebrow">YOUR INSTALLATIONS</small>
          <strong data-install-count></strong>
          <span data-running-summary></span>
        </div>
      </section>
      <section class="panel marketplace-detail__information">
        <header class="panel-heading">
          <div>
            <small class="eyebrow">IMAGE DETAILS</small>
            <h3>WHAT IT INCLUDES</h3>
          </div>
          <div class="tag-list" data-tags></div>
        </header>
        <dl class="spec-list" data-specifications></dl>
      </section>
      <section class="panel marketplace-detail__screenshots">
        <header class="panel-heading">
          <div>
            <small class="eyebrow">SCREENSHOTS</small>
            <h3>SEE IT IN ACTION</h3>
          </div>
        </header>
        <div class="marketplace-screenshot-list" data-screenshots></div>
      </section>
      <section class="panel marketplace-detail__instances">
        <header class="panel-heading">
          <div>
            <small class="eyebrow">YOUR INSTALLATIONS</small>
            <h3 data-instance-heading></h3>
          </div>
          <button class="text-button" type="button" data-screen-link="library"
            data-manage>MANAGE ALL →</button>
        </header>
        <div class="marketplace-instance-list" data-instances></div>
      </section>
    `

    this.style.setProperty(
      '--marketplace-detail-art',
      `url("${assetURL(screenshotSource)}")`
    )
    this.querySelector('[data-name]').textContent = entry.name
    this.querySelector('[data-description]').textContent = entry.description
    this.querySelector('[data-install-count]').textContent = instances.length
    this.querySelector(
      '[data-running-summary]'
    ).textContent = `${runningCount} RUNNING LOCALLY`

    const tags = this.querySelector('[data-tags]')

    for (const tag of entry.tags || []) {
      const item = document.createElement('span')

      item.textContent = tag
      tags.append(item)
    }

    const interfaces = Object.values(entry.interfaces || {})
    const specifications = [
      [
        'EXPERIENCE',
        interfaces.some((item) => item.kind === 'kasmweb')
          ? 'STREAMED WORKSPACE'
          : interfaces.some((item) => item.kind === 'web')
          ? 'LOCAL WEB'
          : 'CONNECTED SERVICE',
      ],
      ['MEMORY', entry.memory || 'MANAGED BY LAUNCHER'],
    ]
    const specificationList = this.querySelector('[data-specifications]')

    for (const [label, value] of specifications) {
      const term = document.createElement('dt')
      const detail = document.createElement('dd')

      term.textContent = label
      detail.textContent = value
      specificationList.append(term, detail)
    }

    const screenshotList = this.querySelector('[data-screenshots]')
    const screenshots = entry.media?.screenshots?.length
      ? entry.media.screenshots
      : [
          {
            source: entry.media?.cover,
            alt: `${entry.name} desktop preview`,
          },
        ]

    for (const media of screenshots) {
      const figure = document.createElement('figure')
      const image = document.createElement('img')
      const caption = document.createElement('figcaption')

      image.src = assetURL(media.source)
      image.alt = media.alt
      caption.textContent = media.alt
      figure.append(image, caption)
      screenshotList.append(figure)
    }

    const install = this.querySelector('[data-install]')

    install.textContent = instances.length ? 'CREATE ANOTHER' : 'INSTALL'
    install.addEventListener('click', () => {
      this.dispatchEvent(
        new CustomEvent('deploy-agent', {
          bubbles: true,
          detail: { entry },
        })
      )
    })

    this.querySelector('[data-instance-heading]').textContent = instances.length
      ? `${instances.length} ${
          instances.length === 1 ? 'LOCAL AGENT' : 'LOCAL AGENTS'
        }`
      : 'NO LOCAL AGENTS YET'
    this.querySelector('[data-manage]').hidden = !instances.length

    const instanceList = this.querySelector('[data-instances]')

    if (!instances.length) {
      const empty = document.createElement('p')

      empty.className = 'marketplace-instance-list__empty'
      empty.textContent =
        'Install this image to create a private agent on this computer.'
      instanceList.append(empty)

      return
    }

    for (const instance of instances) {
      instanceList.append(this.instanceRow(instance))
    }
  }

  instanceRow(instance) {
    const status = statusFor(instance.state)
    const running = status === 'ONLINE'
    const row = document.createElement('div')

    row.className = 'marketplace-instance'
    row.innerHTML = `
      <span class="marketplace-instance__identity">
        <i aria-hidden="true" data-status-light></i>
        <span><strong data-name></strong><small data-status></small></span>
      </span>
      <span class="marketplace-instance__actions">
        <button class="secondary-button" type="button" data-primary></button>
        <button class="icon-button" type="button" data-actions
          aria-label="Agent actions" title="Agent actions">⋮</button>
      </span>
    `
    row.querySelector('[data-name]').textContent = instance.name

    const statusNode = row.querySelector('[data-status]')

    statusNode.textContent = status
    statusNode.dataset.status = status.toLowerCase()
    row.querySelector('[data-status-light]').dataset.status =
      status.toLowerCase()

    const primary = row.querySelector('[data-primary]')

    primary.textContent = running ? 'OPEN' : 'START'
    primary.addEventListener('click', () => {
      emit(this, running ? 'open-agent' : 'toggle-agent', instance)
    })
    row.querySelector('[data-actions]').addEventListener('click', () => {
      emit(this, 'agent-actions', instance)
    })

    return row
  }
}

customElements.define('marketplace-detail', MarketplaceDetail)
