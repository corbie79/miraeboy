<script>
  import { builds } from '../lib/api.js'
  import { auth } from '../lib/auth.svelte.js'

  let { navigate } = $props()

  let list = $state([])
  let loading = $state(false)
  let error = $state('')
  let expandedId = $state(null)

  // Trigger form
  let triggerRepo = $state('')
  let triggerGitUrl = $state('')
  let triggerRef = $state('')
  let triggerPlatforms = $state('')
  let triggering = $state(false)
  let triggerError = $state('')

  let refreshTimer = $state(null)

  let hasActive = $derived(list.some(b => b.status === 'pending' || b.status === 'running'))

  async function loadBuilds() {
    if (loading) return
    loading = true
    error = ''
    try {
      const data = await builds.list()
      list = data.builds || data || []
    } catch (err) {
      error = err.message
    } finally {
      loading = false
    }
  }

  $effect(() => {
    loadBuilds()
  })

  // Auto-refresh when builds are active
  $effect(() => {
    if (hasActive) {
      const id = setInterval(loadBuilds, 5000)
      return () => clearInterval(id)
    }
  })

  async function handleTrigger(e) {
    e.preventDefault()
    triggering = true
    triggerError = ''
    try {
      const platforms = triggerPlatforms.split(',').map(s => s.trim()).filter(Boolean)
      await builds.trigger({
        repo: triggerRepo,
        git_url: triggerGitUrl,
        ref: triggerRef,
        platforms,
      })
      triggerRepo = ''
      triggerGitUrl = ''
      triggerRef = ''
      triggerPlatforms = ''
      await loadBuilds()
    } catch (err) {
      triggerError = err.message
    } finally {
      triggering = false
    }
  }

  function toggleExpand(id) {
    expandedId = expandedId === id ? null : id
  }

  function statusColor(status) {
    switch (status) {
      case 'pending':  return 'bg-yellow-100 dark:bg-yellow-900/40 text-yellow-700 dark:text-yellow-300'
      case 'running':  return 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300'
      case 'done':     return 'bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300'
      case 'failed':   return 'bg-red-100 dark:bg-red-900/40 text-red-600 dark:text-red-400'
      default:         return 'bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300'
    }
  }

  function formatDate(ts) {
    if (!ts) return '—'
    return new Date(ts).toLocaleString()
  }
</script>

<div class="space-y-6">
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-bold text-slate-800 dark:text-white">Builds</h1>
    {#if hasActive}
      <span class="text-xs text-blue-500 dark:text-blue-400 animate-pulse">Auto-refreshing…</span>
    {/if}
  </div>

  <!-- Trigger form (admin only) -->
  {#if auth.admin}
    <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
      <h2 class="text-sm font-semibold text-slate-700 dark:text-slate-200 mb-3">Trigger Build</h2>
      <form onsubmit={handleTrigger} class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <div class="flex flex-col gap-1">
          <label class="text-xs text-slate-500 dark:text-slate-400">Repo name</label>
          <input
            bind:value={triggerRepo}
            placeholder="my-repo"
            required
            class="px-3 py-1.5 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-sm outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-xs text-slate-500 dark:text-slate-400">Git URL</label>
          <input
            bind:value={triggerGitUrl}
            placeholder="https://github.com/org/repo.git"
            required
            class="px-3 py-1.5 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-sm outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-xs text-slate-500 dark:text-slate-400">Ref (branch/tag/commit)</label>
          <input
            bind:value={triggerRef}
            placeholder="main"
            required
            class="px-3 py-1.5 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-sm outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-xs text-slate-500 dark:text-slate-400">Platforms (comma-separated)</label>
          <input
            bind:value={triggerPlatforms}
            placeholder="linux/amd64, linux/arm64"
            class="px-3 py-1.5 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-sm outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div class="sm:col-span-2 flex items-center gap-3">
          <button type="submit" disabled={triggering} class="px-4 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded-md transition-colors cursor-pointer">
            {triggering ? 'Triggering…' : 'Trigger Build'}
          </button>
          {#if triggerError}
            <span class="text-xs text-red-600 dark:text-red-400">{triggerError}</span>
          {/if}
        </div>
      </form>
    </div>
  {/if}

  <!-- Build list -->
  {#if loading && list.length === 0}
    <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">Loading builds…</div>
  {:else if error}
    <div class="text-sm text-red-600 dark:text-red-400 py-8 text-center">{error}</div>
  {:else if list.length === 0}
    <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">No builds yet.</div>
  {:else}
    <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 dark:border-slate-700 text-left">
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">ID</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Status</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Repo</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Ref</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Platforms</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Created</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
          {#each list as build}
            <tr
              onclick={() => toggleExpand(build.id)}
              class="hover:bg-slate-50 dark:hover:bg-slate-700/50 cursor-pointer"
            >
              <td class="px-4 py-2 font-mono text-xs text-slate-500 dark:text-slate-400">{String(build.id).slice(0, 8)}</td>
              <td class="px-4 py-2">
                <span class="px-1.5 py-0.5 rounded text-xs font-medium {statusColor(build.status)}">{build.status}</span>
              </td>
              <td class="px-4 py-2 text-slate-800 dark:text-white font-medium">{build.repo || '—'}</td>
              <td class="px-4 py-2 font-mono text-xs text-slate-600 dark:text-slate-300">{build.ref || '—'}</td>
              <td class="px-4 py-2 text-slate-500 dark:text-slate-400 text-xs">
                {#if build.platforms?.length}
                  {build.platforms.join(', ')}
                {:else}
                  —
                {/if}
              </td>
              <td class="px-4 py-2 text-slate-500 dark:text-slate-400 text-xs">{formatDate(build.created_at)}</td>
            </tr>
            {#if expandedId === build.id}
              <tr class="bg-slate-50 dark:bg-slate-900/50">
                <td colspan="6" class="px-4 py-3">
                  <div class="space-y-2 text-xs text-slate-700 dark:text-slate-300">
                    <div><span class="font-medium">Git URL:</span> {build.git_url || '—'}</div>
                    <div><span class="font-medium">Finished:</span> {formatDate(build.finished_at)}</div>
                    {#if build.artifacts?.length}
                      <div>
                        <span class="font-medium">Artifacts:</span>
                        <ul class="mt-1 ml-4 list-disc space-y-0.5">
                          {#each build.artifacts as artifact}
                            <li>
                              <a href={artifact.url || '#'} class="text-blue-600 dark:text-blue-400 hover:underline">{artifact.name || artifact}</a>
                            </li>
                          {/each}
                        </ul>
                      </div>
                    {:else}
                      <div class="text-slate-400 dark:text-slate-500">No artifacts.</div>
                    {/if}
                    <div class="mt-2">
                      <span class="font-medium">Logs:</span>
                      <pre class="mt-1 p-2 bg-slate-100 dark:bg-slate-800 rounded text-slate-600 dark:text-slate-300 text-xs overflow-x-auto">{build.logs || '(no logs available)'}</pre>
                    </div>
                  </div>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
