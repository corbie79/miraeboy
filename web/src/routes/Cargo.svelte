<script>
  import { cargo } from '../lib/api.js'
  import { auth } from '../lib/auth.svelte.js'

  let { repoName, navigate } = $props()

  let query = $state('')
  let results = $state([])
  let loading = $state(false)
  let error = $state('')
  let searched = $state(false)

  // Per-crate action state
  let actionState = $state({})

  // Determine if current user can write to this repo
  let canWrite = $derived(
    auth.admin ||
    (auth.groups && (auth.groups[repoName] === 'write' || auth.groups[repoName] === 'admin' || auth.groups[repoName] === 'owner'))
  )

  // Connection info
  let host = $derived(window.location.host)

  async function handleSearch(e) {
    e?.preventDefault()
    loading = true
    error = ''
    searched = true
    try {
      const data = await cargo.search(repoName, query)
      results = data.crates || []
    } catch (err) {
      error = err.message
    } finally {
      loading = false
    }
  }

  $effect(() => { handleSearch() })

  async function toggleYank(crate) {
    const key = `${crate.name}@${crate.max_version}`
    actionState[key] = true
    try {
      if (crate.yanked) {
        await cargo.unyank(repoName, crate.name, crate.max_version)
      } else {
        await cargo.yank(repoName, crate.name, crate.max_version)
      }
      await handleSearch()
    } catch (err) {
      error = err.message
    } finally {
      actionState[key] = false
    }
  }
</script>

<div class="space-y-4">
  <div class="flex items-center gap-3">
    <button onclick={() => navigate(`#/repo/${repoName}`)} class="text-sm text-slate-500 dark:text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer">&larr; Back</button>
    <h1 class="text-xl font-bold text-slate-800 dark:text-white">{repoName} / Cargo Crates</h1>
  </div>

  <!-- Connection info -->
  <div class="bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700 rounded-lg px-4 py-3 text-sm">
    <span class="font-medium text-slate-700 dark:text-slate-200">Registry URL: </span>
    <code class="text-blue-700 dark:text-blue-300 font-mono select-all">sparse+http://{host}/cargo/{repoName}/</code>
  </div>

  <!-- Search -->
  <form onsubmit={handleSearch} class="flex gap-2">
    <input
      bind:value={query}
      placeholder="Search crates…"
      class="flex-1 px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-sm outline-none focus:ring-2 focus:ring-blue-500"
    />
    <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-md transition-colors cursor-pointer">Search</button>
  </form>

  <!-- Results -->
  {#if loading}
    <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">Searching…</div>
  {:else if error}
    <div class="text-sm text-red-600 dark:text-red-400 py-8 text-center">{error}</div>
  {:else if searched && results.length === 0}
    <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">No crates found.</div>
  {:else if results.length > 0}
    <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 dark:border-slate-700 text-left">
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Crate</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Version</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Description</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Status</th>
            {#if canWrite}
              <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium"></th>
            {/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
          {#each results as crate}
            {@const key = `${crate.name}@${crate.max_version}`}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/50">
              <td class="px-4 py-2 font-medium text-slate-800 dark:text-white">{crate.name}</td>
              <td class="px-4 py-2 text-slate-600 dark:text-slate-300 font-mono text-xs">{crate.max_version}</td>
              <td class="px-4 py-2 text-slate-500 dark:text-slate-400 max-w-xs truncate">{crate.description || '—'}</td>
              <td class="px-4 py-2">
                {#if crate.yanked}
                  <span class="px-1.5 py-0.5 bg-red-100 dark:bg-red-900/40 text-red-600 dark:text-red-400 text-xs rounded font-medium">yanked</span>
                {:else}
                  <span class="px-1.5 py-0.5 bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-400 text-xs rounded font-medium">ok</span>
                {/if}
              </td>
              {#if canWrite}
                <td class="px-4 py-2">
                  <button
                    onclick={() => toggleYank(crate)}
                    disabled={!!actionState[key]}
                    class="text-xs px-2 py-1 rounded border transition-colors cursor-pointer disabled:opacity-40
                      {crate.yanked
                        ? 'bg-green-50 dark:bg-green-900/20 border-green-300 dark:border-green-700 text-green-700 dark:text-green-400 hover:bg-green-100 dark:hover:bg-green-900/40'
                        : 'bg-red-50 dark:bg-red-900/20 border-red-300 dark:border-red-700 text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/40'}"
                  >
                    {actionState[key] ? '…' : (crate.yanked ? 'Unyank' : 'Yank')}
                  </button>
                </td>
              {/if}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="text-xs text-slate-400 dark:text-slate-500 text-right">{results.length} crate(s)</div>
  {/if}
</div>
