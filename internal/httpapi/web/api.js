export class LauncherAPI {
  constructor() {
    this.token =
      document.querySelector('meta[name="launcher-token"]')?.content || ''
  }

  async request(path, options = {}) {
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 10000)

    try {
      const response = await fetch(path, {
        ...options,
        signal: controller.signal,
        headers: {
          'X-Launcher-Token': this.token,
          ...(options.body ? { 'Content-Type': 'application/json' } : {}),
          ...(options.headers || {}),
        },
      })

      if (!response.ok) {
        const body = await response.json().catch(() => ({}))

        throw new Error(
          body.error || `Launcher request failed (${response.status})`
        )
      }

      return response.status === 204 ? null : response.json()
    } catch (error) {
      if (error.name === 'AbortError') {
        throw new Error('Launcher request timed out')
      }

      throw error
    } finally {
      clearTimeout(timeout)
    }
  }

  doctor() {
    return this.request('/api/doctor')
  }

  catalog() {
    return this.request('/api/catalog')
  }

  instances() {
    return this.request('/api/instances')
  }

  start(id) {
    return this.request(`/api/instances/${encodeURIComponent(id)}/start`, {
      method: 'POST',
      body: '{}',
    })
  }

  stop(id) {
    return this.request(`/api/instances/${encodeURIComponent(id)}/stop`, {
      method: 'POST',
      body: '{}',
    })
  }

  rename(id, name) {
    return this.request(`/api/instances/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify({ name }),
    })
  }

  logs(id) {
    return this.request(`/api/instances/${encodeURIComponent(id)}/logs`)
  }

  delete(id) {
    return this.request(`/api/instances/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    })
  }

  async install(catalogId, name, onProgress) {
    const response = await fetch('/api/instances/install', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Launcher-Token': this.token,
      },
      body: JSON.stringify({ catalogId, name }),
    })

    if (!response.ok) {
      const body = await response.json().catch(() => ({}))

      throw new Error(
        body.error || `Installation request failed (${response.status})`
      )
    }

    if (!response.body) {
      throw new Error('Installation progress stream is unavailable')
    }

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let installed = null

    while (true) {
      const result = await reader.read()

      buffer += decoder.decode(result.value || new Uint8Array(), {
        stream: !result.done,
      })

      const lines = buffer.split('\n')

      buffer = result.done ? '' : lines.pop()

      for (const line of lines) {
        if (!line.trim()) {
          continue
        }

        const update = JSON.parse(line)

        if (update.type === 'error') {
          throw new Error(update.error || 'Installation failed')
        }

        if (update.type === 'progress') {
          onProgress(update)
        }

        if (update.type === 'complete') {
          installed = update.instance
        }
      }

      if (result.done) {
        break
      }
    }

    if (!installed) {
      throw new Error('Installation ended before the agent was ready')
    }

    return installed
  }
}
