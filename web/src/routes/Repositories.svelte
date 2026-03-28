<script>
  import { repos } from '../lib/api.js'
  import { auth } from '../lib/auth.svelte.js'

  let { navigate } = $props()

  let repoList = $state([])
  let loading = $state(true)
  let error = $state('')

  // Create form
  let showCreate = $state(false)
  let form = $state({ name: '', description: '', owner: '', allowed_namespaces: '', allowed_channels: '', anonymous_access: 'none' })
  let createError = $state('')

  async function loadRepos() {
    loading = true
    error = ''
    try {
      const data = await repos.list()
      repoList = data.repositories || []
    } catch (err) {
      error = err.message
    } finally {
      loading = false
    }
  }

  async function handleCreate(e) {
    e.preventDefault()
    createError = ''
    try {
      await repos.create({
        name: form.name,
        description: form.description,
        owner: form.owner || auth.username,
        allowed_namespaces: form.allowed_namespaces ? form.allowed_namespaces.split(',').map(s => s.trim()).filter(Boolean) : [],
        allowed_channels: form.allowed_channels ? form.allowed_channels.split(',').map(s => s.trim()).filter(Boolean) : [],
        anonymous_access: form.anonymous_access,
      })
      showCreate = false
      form = { name: '', description: '', owner: '', allowed_namespaces: '', allowed_channels: '', anonymous_access: 'none' }
      await loadRepos()
    } catch (err) {
      createError = err.message
    }
  }

  $effect(() => { loadRepos() })
</script>

<div class="space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-bold text-slate-800 dark:text-white">Repositories</h1>
    {#if auth.admin}
      <button onclick={() => showCreate = !showCreate} class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-md transition-colors cursor-pointer">
        + New Repository
      </button>
    {/if}
  </div>

  <!-- Create Form -->
  {#if showCreate}
    <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
      <h2 class="text-sm font-semibold text-slate-700 dark:text-slate-300 mb-3">Create Repository</h2>
      {#if createError}
        <div class="mb-3 text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded px-3 py-2">{createError}</div>
      {/if}
      <form onsubmit={handleCreate} class="grid grid-cols-2 gap-3">
        <div>
          <label for="cr-name" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Name *</label>
          <input id="cr-name" bind:value={form.name} required placeholder="extralib" class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        </div>
        <div>
          <label for="cr-owner" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Owner</label>
          <input id="cr-owner" bind:value={form.owner} placeholder={auth.username} class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        </div>
        <div class="col-span-2">
          <label for="cr-desc" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Description</label>
          <input id="cr-desc" bind:value={form.description} placeholder="사내 외부 라이브러리" class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        </div>
        <div>
          <label for="cr-ns" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Allowed Namespaces <span class="text-slate-400">(comma sep)</span></label>
          <input id="cr-ns" bind:value={form.allowed_namespaces} placeholder="sc" class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        </div>
        <div>
          <label for="cr-ch" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Allowed Channels <span class="text-slate-400">(comma sep)</span></label>
          <input id="cr-ch" bind:value={form.allowed_channels} placeholder="dev, release" class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        </div>
        <div>
          <label for="cr-anon" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Anonymous Access</label>
          <select id="cr-anon" bind:value={form.anonymous_access} class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500">
            <option value="none">None</option>
            <option value="read">Read</option>
          </select>
        </div>
        <div class="flex items-end">
          <div class="flex gap-2">
            <button type="submit" class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors cursor-pointer">Create</button>
            <button type="button" onclick={() => showCreate = false} class="px-3 py-1.5 bg-slate-200 dark:bg-slate-600 hover:bg-slate-300 dark:hover:bg-slate-500 text-slate-700 dark:text-slate-200 text-sm rounded transition-colors cursor-pointer">Cancel</button>
          </div>
        </div>
      </form>
    </div>
  {/if}

  <!-- Loading / Error -->
  {#if loading}
    <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">Loading...</div>
  {:else if error}
    <div class="text-sm text-red-600 dark:text-red-400 py-8 text-center">{error}</div>
  {:else if repoList.length === 0}
    <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">No repositories yet.</div>
  {:else}
    <!-- Repo list -->
    <div class="grid gap-3">
      {#each repoList as repo}
        <button
          onclick={() => navigate(`#/repo/${repo.name}`)}
          class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4 text-left hover:border-blue-300 dark:hover:border-blue-600 hover:shadow-md transition-all cursor-pointer w-full"
        >
          <div class="flex items-start justify-between">
            <div>
              <div class="flex items-center gap-2">
                <h3 class="font-semibold text-slate-800 dark:text-white">{repo.name}</h3>
                {#if repo.anonymous_access === 'read'}
                  <span class="px-1.5 py-0.5 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 text-xs rounded">public</span>
                {:else}
                  <span class="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 text-xs rounded">private</span>
                {/if}
              </div>
              {#if repo.description}
                <p class="text-sm text-slate-500 dark:text-slate-400 mt-1">{repo.description}</p>
              {/if}
            </div>
            <div class="text-right text-xs text-slate-400 dark:text-slate-500 space-y-1">
              <div>owner: {repo.owner}</div>
              <div>{repo.member_count} members</div>
            </div>
          </div>
          <div class="mt-2 flex gap-2 flex-wrap">
            {#if repo.allowed_namespaces?.length > 0}
              {#each repo.allowed_namespaces as ns}
                <span class="px-1.5 py-0.5 bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 text-xs rounded">@{ns}</span>
              {/each}
            {/if}
            {#if repo.allowed_channels?.length > 0}
              {#each repo.allowed_channels as ch}
                <span class="px-1.5 py-0.5 bg-purple-50 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400 text-xs rounded">/{ch}</span>
              {/each}
            {/if}
          </div>
          <div class="mt-2 text-xs text-slate-400 dark:text-slate-500 font-mono">
            conan remote add {repo.name} http://&lt;server&gt;/api/conan/{repo.name}
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>
