<script>
  import { repos, members as membersApi } from '../lib/api.js'
  import { auth } from '../lib/auth.svelte.js'

  let { repoName, navigate } = $props()

  let repo = $state(null)
  let memberList = $state([])
  let loading = $state(true)
  let error = $state('')
  let saveMsg = $state('')

  // Edit form
  let editForm = $state({ description: '', allowed_namespaces: '', allowed_channels: '', anonymous_access: 'none' })

  // Invite form
  let inviteForm = $state({ username: '', permission: 'read' })
  let inviteError = $state('')

  async function load() {
    loading = true
    error = ''
    try {
      const r = await repos.get(repoName)
      repo = r
      editForm = {
        description: r.description || '',
        allowed_namespaces: (r.allowed_namespaces || []).join(', '),
        allowed_channels: (r.allowed_channels || []).join(', '),
        anonymous_access: r.anonymous_access || 'none',
      }
      const m = await membersApi.list(repoName)
      memberList = m.members || []
    } catch (err) {
      error = err.message
    } finally {
      loading = false
    }
  }

  async function handleSave() {
    saveMsg = ''
    try {
      await repos.update(repoName, {
        description: editForm.description,
        allowed_namespaces: editForm.allowed_namespaces ? editForm.allowed_namespaces.split(',').map(s => s.trim()).filter(Boolean) : [],
        allowed_channels: editForm.allowed_channels ? editForm.allowed_channels.split(',').map(s => s.trim()).filter(Boolean) : [],
        anonymous_access: editForm.anonymous_access,
      })
      saveMsg = 'Saved!'
      setTimeout(() => saveMsg = '', 2000)
    } catch (err) {
      saveMsg = 'Error: ' + err.message
    }
  }

  async function handleInvite(e) {
    e.preventDefault()
    inviteError = ''
    try {
      await membersApi.invite(repoName, inviteForm)
      inviteForm = { username: '', permission: 'read' }
      const m = await membersApi.list(repoName)
      memberList = m.members || []
    } catch (err) {
      inviteError = err.message
    }
  }

  async function handleUpdateMember(username, permission) {
    try {
      await membersApi.update(repoName, username, { permission })
      const m = await membersApi.list(repoName)
      memberList = m.members || []
    } catch (err) {
      alert(err.message)
    }
  }

  async function handleRemoveMember(username) {
    if (!confirm(`Remove ${username} from ${repoName}?`)) return
    try {
      await membersApi.remove(repoName, username)
      const m = await membersApi.list(repoName)
      memberList = m.members || []
    } catch (err) {
      alert(err.message)
    }
  }

  async function handleDelete() {
    if (!confirm(`Delete repository "${repoName}"? This will remove ALL packages. Type the repo name to confirm.`)) return
    const typed = prompt(`Type "${repoName}" to confirm deletion:`)
    if (typed !== repoName) return
    try {
      await repos.delete(repoName, true)
      navigate('#/repos')
    } catch (err) {
      alert(err.message)
    }
  }

  $effect(() => { load() })
</script>

{#if loading}
  <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">Loading...</div>
{:else if error}
  <div class="text-sm text-red-600 dark:text-red-400 py-8 text-center">{error}</div>
{:else if repo}
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <button onclick={() => navigate('#/repos')} class="text-sm text-slate-500 dark:text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer">&larr; Back</button>
        <h1 class="text-xl font-bold text-slate-800 dark:text-white">{repoName}</h1>
      </div>
      <div class="flex gap-2">
        <button onclick={() => navigate(`#/repo/${repoName}/packages`)} class="px-3 py-1.5 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-sm text-slate-700 dark:text-slate-200 rounded transition-colors cursor-pointer">
          Browse Packages
        </button>
        {#if auth.admin}
          <button onclick={handleDelete} class="px-3 py-1.5 bg-red-100 dark:bg-red-900/30 hover:bg-red-200 dark:hover:bg-red-900/50 text-red-700 dark:text-red-400 text-sm rounded transition-colors cursor-pointer">
            Delete
          </button>
        {/if}
      </div>
    </div>

    <!-- Connection info -->
    <div class="bg-slate-100 dark:bg-slate-800 rounded-lg p-3 font-mono text-sm text-slate-600 dark:text-slate-300">
      conan remote add {repoName} http://{window.location.hostname}:{window.location.port || '9300'}/api/conan/{repoName}
    </div>

    <!-- Settings -->
    <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
      <h2 class="text-sm font-semibold text-slate-700 dark:text-slate-300 mb-3">Settings</h2>
      <div class="grid grid-cols-2 gap-3">
        <div class="col-span-2">
          <label for="ed-desc" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Description</label>
          <input id="ed-desc" bind:value={editForm.description} class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        </div>
        <div>
          <label for="ed-ns" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Allowed Namespaces <span class="text-slate-400">(comma sep)</span></label>
          <input id="ed-ns" bind:value={editForm.allowed_namespaces} placeholder="sc" class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        </div>
        <div>
          <label for="ed-ch" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Allowed Channels <span class="text-slate-400">(comma sep)</span></label>
          <input id="ed-ch" bind:value={editForm.allowed_channels} placeholder="dev, release" class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        </div>
        <div>
          <label for="ed-anon" class="block text-xs text-slate-500 dark:text-slate-400 mb-1">Anonymous Access</label>
          <select id="ed-anon" bind:value={editForm.anonymous_access} class="w-full px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500">
            <option value="none">None</option>
            <option value="read">Read</option>
          </select>
        </div>
        <div class="flex items-end">
          <div class="flex items-center gap-2">
            <button onclick={handleSave} class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded transition-colors cursor-pointer">Save</button>
            {#if saveMsg}
              <span class="text-xs {saveMsg.startsWith('Error') ? 'text-red-500' : 'text-green-500'}">{saveMsg}</span>
            {/if}
          </div>
        </div>
      </div>
    </div>

    <!-- Members -->
    <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
      <h2 class="text-sm font-semibold text-slate-700 dark:text-slate-300 mb-3">Members</h2>

      <!-- Invite -->
      <form onsubmit={handleInvite} class="flex gap-2 mb-4">
        <input bind:value={inviteForm.username} required placeholder="username" class="flex-1 px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500" />
        <select bind:value={inviteForm.permission} class="px-2 py-1.5 border border-slate-300 dark:border-slate-600 rounded text-sm bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none focus:ring-1 focus:ring-blue-500">
          <option value="read">read</option>
          <option value="write">write</option>
          <option value="delete">delete</option>
          <option value="owner">owner</option>
        </select>
        <button type="submit" class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white text-sm rounded transition-colors cursor-pointer">Invite</button>
      </form>
      {#if inviteError}
        <div class="mb-3 text-sm text-red-600 dark:text-red-400">{inviteError}</div>
      {/if}

      <!-- Member list -->
      <div class="divide-y divide-slate-100 dark:divide-slate-700">
        {#each memberList as member}
          <div class="flex items-center justify-between py-2">
            <div class="flex items-center gap-2">
              <span class="text-sm text-slate-800 dark:text-white font-medium">{member.username}</span>
              {#if member.is_owner}
                <span class="px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 text-xs rounded">owner</span>
              {/if}
            </div>
            {#if !member.is_owner}
              <div class="flex items-center gap-2">
                <select
                  value={member.permission}
                  onchange={(e) => handleUpdateMember(member.username, e.target.value)}
                  class="px-2 py-1 border border-slate-300 dark:border-slate-600 rounded text-xs bg-white dark:bg-slate-700 text-slate-900 dark:text-white outline-none"
                >
                  <option value="read">read</option>
                  <option value="write">write</option>
                  <option value="delete">delete</option>
                  <option value="owner">owner</option>
                </select>
                <button onclick={() => handleRemoveMember(member.username)} class="text-red-500 hover:text-red-700 dark:hover:text-red-400 text-xs cursor-pointer">remove</button>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}
