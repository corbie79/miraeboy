<script>
  import { packages } from '../lib/api.js'

  let { repoName, navigate } = $props()

  let query = $state('*')
  let results = $state([])
  let loading = $state(false)
  let error = $state('')
  let searched = $state(false)

  async function handleSearch(e) {
    e?.preventDefault()
    loading = true
    error = ''
    searched = true
    try {
      const data = await packages.search(repoName, query)
      results = data.results || []
    } catch (err) {
      error = err.message
    } finally {
      loading = false
    }
  }

  // Auto-search on mount
  $effect(() => { handleSearch() })

  function parseRef(ref) {
    // "zlib/1.3.1@sc/dev" → { name: "zlib", version: "1.3.1", namespace: "sc", channel: "dev" }
    const atIdx = ref.indexOf('@')
    const nameVer = atIdx >= 0 ? ref.slice(0, atIdx) : ref
    const nsChannel = atIdx >= 0 ? ref.slice(atIdx + 1) : ''
    const [name, version] = nameVer.split('/')
    const [namespace, channel] = nsChannel ? nsChannel.split('/') : ['', '']
    return { name, version, namespace, channel }
  }
</script>

<div class="space-y-4">
  <div class="flex items-center gap-3">
    <button onclick={() => navigate(`#/repo/${repoName}`)} class="text-sm text-slate-500 dark:text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer">&larr; Back</button>
    <h1 class="text-xl font-bold text-slate-800 dark:text-white">{repoName} / Packages</h1>
  </div>

  <!-- Search -->
  <form onsubmit={handleSearch} class="flex gap-2">
    <input
      bind:value={query}
      placeholder="Search packages (e.g. zlib*, *)"
      class="flex-1 px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-sm outline-none focus:ring-2 focus:ring-blue-500"
    />
    <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-md transition-colors cursor-pointer">Search</button>
  </form>

  <!-- Results -->
  {#if loading}
    <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">Searching...</div>
  {:else if error}
    <div class="text-sm text-red-600 dark:text-red-400 py-8 text-center">{error}</div>
  {:else if searched && results.length === 0}
    <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">No packages found.</div>
  {:else}
    <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 dark:border-slate-700 text-left">
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Package</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Version</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Namespace</th>
            <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Channel</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
          {#each results as ref}
            {@const pkg = parseRef(ref)}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/50">
              <td class="px-4 py-2 font-medium text-slate-800 dark:text-white">{pkg.name}</td>
              <td class="px-4 py-2 text-slate-600 dark:text-slate-300">{pkg.version}</td>
              <td class="px-4 py-2">
                {#if pkg.namespace}
                  <span class="px-1.5 py-0.5 bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 text-xs rounded">@{pkg.namespace}</span>
                {/if}
              </td>
              <td class="px-4 py-2">
                {#if pkg.channel}
                  <span class="px-1.5 py-0.5 bg-purple-50 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400 text-xs rounded">{pkg.channel}</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="text-xs text-slate-400 dark:text-slate-500 text-right">{results.length} package(s)</div>
  {/if}
</div>
