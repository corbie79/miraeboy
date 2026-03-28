<script>
  import { login } from '../lib/api.js'
  import { refreshAuth } from '../lib/auth.svelte.js'

  let { onSuccess } = $props()

  let username = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)

  async function handleLogin(e) {
    e.preventDefault()
    error = ''
    loading = true
    try {
      await login(username, password)
      refreshAuth()
      onSuccess?.()
    } catch (err) {
      error = err.message
    } finally {
      loading = false
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center bg-slate-50 dark:bg-slate-900">
  <div class="w-full max-w-sm">
    <div class="bg-white dark:bg-slate-800 rounded-lg shadow-lg p-8 border border-slate-200 dark:border-slate-700">
      <div class="text-center mb-8">
        <h1 class="text-2xl font-bold text-slate-800 dark:text-white">Conan Repository</h1>
        <p class="text-sm text-slate-500 dark:text-slate-400 mt-1">Package Server Management</p>
      </div>

      <form onsubmit={handleLogin} class="space-y-4">
        {#if error}
          <div class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm rounded-md px-3 py-2">
            {error}
          </div>
        {/if}

        <div>
          <label for="username" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Username</label>
          <input
            id="username"
            type="text"
            bind:value={username}
            class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors text-sm"
            placeholder="admin"
            required
          />
        </div>

        <div>
          <label for="password" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Password</label>
          <input
            id="password"
            type="password"
            bind:value={password}
            class="w-full px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors text-sm"
            placeholder="••••••••"
            required
          />
        </div>

        <button
          type="submit"
          disabled={loading}
          class="w-full py-2 px-4 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white rounded-md text-sm font-medium transition-colors cursor-pointer"
        >
          {loading ? 'Logging in...' : 'Login'}
        </button>
      </form>
    </div>
  </div>
</div>
