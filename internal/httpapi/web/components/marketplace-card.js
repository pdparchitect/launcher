function assetURL(source) {
  return source ? `/catalog-assets/${source}` : ''
}

export class MarketplaceCard extends HTMLElement {
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
    const installedCount = instances.length
    const runningCount = instances.filter(
      (instance) => instance.state === 'running'
    ).length

    this.className = 'marketplace-card'
    this.innerHTML = `
      <button class="marketplace-card__cover" type="button" data-view>
        <span data-cover></span>
        <strong data-title-overlay></strong>
      </button>
      <div class="marketplace-card__body">
        <div class="marketplace-card__heading">
          <span class="marketplace-card__identity">
            <span class="marketplace-card__icon" data-icon></span>
            <strong data-name></strong>
          </span>
          <small>LOCAL</small>
        </div>
        <p data-description></p>
        <div class="tag-list" data-tags></div>
        <div class="marketplace-card__footer">
          <small>
            <span data-installs></span>
            <span data-running></span>
          </small>
          <span class="marketplace-card__actions">
            <button class="text-button" data-view>VIEW DETAILS →</button>
            <button class="primary-button" data-install></button>
          </span>
        </div>
      </div>
    `
    this.querySelector('[data-cover]').style.backgroundImage = `url("${assetURL(
      entry.media?.cover
    )}")`
    this.querySelector('[data-icon]').style.backgroundImage = `url("${assetURL(
      entry.media?.icon
    )}")`
    this.querySelector('[data-title-overlay]').textContent =
      entry.name.toUpperCase()
    this.querySelector('[data-name]').textContent = entry.name
    this.querySelector('[data-description]').textContent = entry.description
    this.querySelector('[data-installs]').textContent = `${installedCount} ${
      installedCount === 1 ? 'install' : 'installs'
    }`
    this.querySelector('[data-running]').textContent = runningCount
      ? ` · ${runningCount} running`
      : ''

    const tags = this.querySelector('[data-tags]')

    for (const tag of entry.tags || []) {
      const item = document.createElement('span')

      item.textContent = tag
      tags.append(item)
    }

    const install = this.querySelector('[data-install]')

    install.textContent = installedCount ? 'CREATE ANOTHER' : 'INSTALL'

    if (installedCount) {
      install.classList.add('primary-button--outline')
    }

    install.addEventListener('click', () => {
      this.dispatchEvent(
        new CustomEvent('deploy-agent', {
          bubbles: true,
          detail: { entry },
        })
      )
    })

    this.querySelectorAll('[data-view]').forEach((button) => {
      button.setAttribute('aria-label', `View ${entry.name} details`)
      button.addEventListener('click', () => {
        this.dispatchEvent(
          new CustomEvent('view-marketplace-entry', {
            bubbles: true,
            detail: { entry },
          })
        )
      })
    })
  }
}

customElements.define('marketplace-card', MarketplaceCard)
