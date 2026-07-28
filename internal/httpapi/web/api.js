export class LauncherAPI {
  constructor() {
    this.token =
      document.querySelector('meta[name="launcher-token"]')?.content || ''
  }

  async request(path, options = {}) {
    const { timeoutMs = 10000, ...requestOptions } = options
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), timeoutMs)

    try {
      const response = await fetch(path, {
        ...requestOptions,
        signal: controller.signal,
        headers: {
          'X-Launcher-Token': this.token,
          ...(requestOptions.body
            ? { 'Content-Type': 'application/json' }
            : {}),
          ...(requestOptions.headers || {}),
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

  startRuntime() {
    return this.request('/api/runtime/start', {
      method: 'POST',
      body: '{}',
      timeoutMs: 5 * 60 * 1000,
    })
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

  update(id, onProgress) {
    return this.progressRequest(
      `/api/instances/${encodeURIComponent(id)}/update`,
      {},
      onProgress,
      'Update'
    )
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

  openFiles(id) {
    return this.request(`/api/instances/${encodeURIComponent(id)}/files`, {
      method: 'POST',
      body: '{}',
    })
  }

  delete(id) {
    return this.request(`/api/instances/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    })
  }

  async install(catalogId, name, onProgress) {
    return this.progressRequest(
      '/api/instances/install',
      { catalogId, name },
      onProgress,
      'Installation'
    )
  }

  async progressRequest(path, body, onProgress, operation) {
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 15 * 60 * 1000)

    try {
      const response = await fetch(path, {
        method: 'POST',
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          'X-Launcher-Token': this.token,
        },
        body: JSON.stringify(body),
      })

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}))

        throw new Error(
          errorBody.error ||
            `${operation} request failed (${response.status})`
        )
      }

      if (!response.body) {
        throw new Error(`${operation} progress stream is unavailable`)
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let completed = null

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
            throw new Error(update.error || `${operation} failed`)
          }

          if (update.type === 'progress') {
            onProgress?.(update)
          }

          if (update.type === 'complete') {
            completed = update.instance
          }
        }

        if (result.done) {
          break
        }
      }

      if (!completed) {
        throw new Error(
          `${operation} ended before the agent was ready`
        )
      }

      return completed
    } catch (error) {
      if (error.name === 'AbortError') {
        throw new Error(`${operation} request timed out`)
      }

      throw error
    } finally {
      clearTimeout(timeout)
    }
  }
}
