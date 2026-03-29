<script>
  import { users } from '../lib/api.js'
  import { auth } from '../lib/auth.svelte.js'

  let list = $state([])
  let loading = $state(false)
  let error = $state('')

  // Create form
  let newUsername = $state('')
  let newPassword = $state('')
  let newAdmin = $state(false)
  let createError = $state('')
  let creating = $state(false)

  // Per-row edit state: { [username]: { password, saving, error } }
  let editState = $state({})

  async function loadUsers() {
    loading = true
    error = ''
    try {
      const data = await users.list()
      list = data.users || data || []
    } catch (err) {
      error = err.message
    } finally {
      loading = false
    }
  }

  $effect(() => { loadUsers() })

  function initEdit(username) {
    if (!editState[username]) {
      editState[username] = { password: '', saving: false, error: '' }
    }
  }

  async function savePassword(username) {
    initEdit(username)
    const pw = editState[username].password
    if (!pw) return
    editState[username].saving = true
    editState[username].error = ''
    try {
      await users.update(username, { password: pw })
      editState[username].password = ''
    } catch (err) {
      editState[username].error = err.message
    } finally {
      editState[username].saving = false
    }
  }

  async function toggleAdmin(user) {
    try {
      await users.update(user.username, { admin: !user.admin })
      await loadUsers()
    } catch (err) {
      error = err.message
    }
  }

  async function deleteUser(username) {
    if (!confirm(`Delete user "${username}"?`)) return
    try {
      await users.delete(username)
      await loadUsers()
    } catch (err) {
      error = err.message
    }
  }

  async function handleCreate(e) {
    e.preventDefault()
    if (!newUsername || !newPassword) return
    creating = true
    createError = ''
    try {
      await users.create({ username: newUsername, password: newPassword, admin: newAdmin })
      newUsername = ''
      newPassword = ''
      newAdmin = false
      await loadUsers()
    } catch (err) {
      createError = err.message
    } finally {
      creating = false
    }
  }

  function formatDate(ts) {
    if (!ts) return '—'
    return new Date(ts).toLocaleDateString()
  }
</script>

{#if !auth.admin}
  <div class="py-16 text-center text-slate-500 dark:text-slate-400">Admin access required.</div>
{:else}
  <div class="space-y-6">
    <h1 class="text-xl font-bold text-slate-800 dark:text-white">User Management</h1>

    <!-- Create user form -->
    <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 p-4">
      <h2 class="text-sm font-semibold text-slate-700 dark:text-slate-200 mb-3">Create User</h2>
      <form onsubmit={handleCreate} class="flex flex-wrap gap-2 items-end">
        <div class="flex flex-col gap-1">
          <label for="new-username" class="text-xs text-slate-500 dark:text-slate-400">Username</label>
          <input
            id="new-username"
            bind:value={newUsername}
            placeholder="username"
            required
            class="px-3 py-1.5 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-sm outline-none focus:ring-2 focus:ring-blue-500 w-40"
          />
        </div>
        <div class="flex flex-col gap-1">
          <label for="new-password" class="text-xs text-slate-500 dark:text-slate-400">Password</label>
          <input
            id="new-password"
            bind:value={newPassword}
            type="password"
            placeholder="password"
            required
            class="px-3 py-1.5 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-sm outline-none focus:ring-2 focus:ring-blue-500 w-40"
          />
        </div>
        <label class="flex items-center gap-1.5 text-sm text-slate-700 dark:text-slate-300 pb-1 cursor-pointer">
          <input type="checkbox" bind:checked={newAdmin} class="accent-amber-500" />
          Admin
        </label>
        <button type="submit" disabled={creating} class="px-4 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded-md transition-colors cursor-pointer">
          {creating ? 'Creating…' : 'Create'}
        </button>
        {#if createError}
          <span class="text-xs text-red-600 dark:text-red-400">{createError}</span>
        {/if}
      </form>
    </div>

    <!-- User list -->
    {#if loading}
      <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">Loading users…</div>
    {:else if error}
      <div class="text-sm text-red-600 dark:text-red-400 py-8 text-center">{error}</div>
    {:else if list.length === 0}
      <div class="text-sm text-slate-500 dark:text-slate-400 py-8 text-center">No users found.</div>
    {:else}
      <div class="bg-white dark:bg-slate-800 rounded-lg border border-slate-200 dark:border-slate-700 overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200 dark:border-slate-700 text-left">
              <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Username</th>
              <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Source</th>
              <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Created</th>
              <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Change Password</th>
              <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium">Admin</th>
              <th class="px-4 py-2 text-slate-500 dark:text-slate-400 font-medium"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
            {#each list as user}
              {@const isSelf = user.username === auth.username}
              {@const es = editState[user.username] || { password: '', saving: false, error: '' }}
              <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/50">
                <td class="px-4 py-2 font-medium text-slate-800 dark:text-white">
                  {user.username}
                  {#if user.admin}
                    <span class="ml-1 px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 text-xs rounded font-medium">admin</span>
                  {/if}
                </td>
                <td class="px-4 py-2 text-slate-500 dark:text-slate-400">{user.source || 'local'}</td>
                <td class="px-4 py-2 text-slate-500 dark:text-slate-400">{formatDate(user.created_at)}</td>
                <td class="px-4 py-2">
                  <div class="flex gap-1 items-center">
                    <input
                      type="password"
                      placeholder="new password"
                      value={es.password}
                      oninput={(e) => { initEdit(user.username); editState[user.username].password = e.target.value }}
                      class="px-2 py-1 border border-slate-300 dark:border-slate-600 rounded bg-white dark:bg-slate-700 text-slate-900 dark:text-white text-xs outline-none focus:ring-1 focus:ring-blue-500 w-32"
                    />
                    <button
                      onclick={() => savePassword(user.username)}
                      disabled={es.saving || !es.password}
                      class="px-2 py-1 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 disabled:opacity-40 text-slate-700 dark:text-slate-300 text-xs rounded transition-colors cursor-pointer"
                    >
                      {es.saving ? '…' : 'Save'}
                    </button>
                    {#if es.error}
                      <span class="text-xs text-red-500">{es.error}</span>
                    {/if}
                  </div>
                </td>
                <td class="px-4 py-2">
                  <button
                    onclick={() => toggleAdmin(user)}
                    disabled={isSelf}
                    title={isSelf ? "Cannot change your own admin status" : "Toggle admin"}
                    class="px-2 py-0.5 text-xs rounded border transition-colors cursor-pointer
                      {user.admin
                        ? 'bg-amber-100 dark:bg-amber-900/50 border-amber-300 dark:border-amber-700 text-amber-700 dark:text-amber-300 hover:bg-amber-200 dark:hover:bg-amber-900'
                        : 'bg-slate-100 dark:bg-slate-700 border-slate-300 dark:border-slate-600 text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-600'}
                      disabled:opacity-40"
                  >
                    {user.admin ? 'Admin' : 'User'}
                  </button>
                </td>
                <td class="px-4 py-2">
                  {#if !isSelf}
                    <button
                      onclick={() => deleteUser(user.username)}
                      class="text-xs text-red-500 hover:text-red-700 dark:hover:text-red-400 transition-colors cursor-pointer"
                    >
                      Delete
                    </button>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
{/if}
