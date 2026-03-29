<script>
  import './app.css'
  import { auth, logout, refreshAuth } from './lib/auth.svelte.js'
  import { setToken, builds } from './lib/api.js'
  import Login from './routes/Login.svelte'
  import Repositories from './routes/Repositories.svelte'
  import RepoDetail from './routes/RepoDetail.svelte'
  import Packages from './routes/Packages.svelte'
  import Cargo from './routes/Cargo.svelte'
  import Users from './routes/Users.svelte'
  import Builds from './routes/Builds.svelte'

  let currentRoute = $state(window.location.hash || '#/')
  let buildsEnabled = $state(false)
  let tokenExpired = $state(false)

  function navigate(hash) {
    window.location.hash = hash
    currentRoute = hash
  }

  // Handle hash change
  $effect(() => {
    const handler = () => { currentRoute = window.location.hash || '#/' }
    window.addEventListener('hashchange', handler)
    return () => window.removeEventListener('hashchange', handler)
  })

  // On mount: handle OIDC callback, check token expiry, probe builds
  $effect(() => {
    const hash = window.location.hash

    // OIDC callback: #/auth/callback?token=...
    // The token may be in the query part of the fragment
    if (hash.startsWith('#/auth/callback')) {
      const queryStr = hash.includes('?') ? hash.slice(hash.indexOf('?') + 1) : ''
      const params = new URLSearchParams(queryStr)
      const tok = params.get('token')
      if (tok) {
        setToken(tok)
        refreshAuth()
      }
      navigate('#/repos')
      return
    }

    // Check token expiry (already handled by auth.svelte.js — if token is expired, loggedIn will be false)
    if (!auth.loggedIn && localStorage.getItem('token')) {
      tokenExpired = true
    }

    // Probe builds endpoint
    if (auth.loggedIn) {
      builds.list().then(() => {
        buildsEnabled = true
      }).catch(() => {
        buildsEnabled = false
      })
    }
  })

  function handleLogout() {
    logout()
    tokenExpired = false
    navigate('#/')
  }

  // Parse route
  let routeInfo = $derived.by(() => {
    const h = currentRoute

    if (h.startsWith('#/auth/callback')) {
      return { page: 'callback' }
    }
    if (h.startsWith('#/repo/')) {
      const rest = h.slice(7) // after "#/repo/"
      const slashIdx = rest.indexOf('/')
      if (slashIdx === -1) {
        return { page: 'detail', repo: rest }
      }
      const repoName = rest.slice(0, slashIdx)
      const sub = rest.slice(slashIdx + 1)
      if (sub === 'packages') return { page: 'packages', repo: repoName }
      if (sub === 'cargo')    return { page: 'cargo',    repo: repoName }
      return { page: 'detail', repo: repoName }
    }
    if (h === '#/repos')  return { page: 'repos' }
    if (h === '#/users')  return { page: 'users' }
    if (h === '#/builds') return { page: 'builds' }
    if (h === '#/')       return { page: auth.loggedIn ? 'repos' : 'login' }
    return { page: auth.loggedIn ? 'repos' : 'login' }
  })
</script>

{#if !auth.loggedIn}
  <div class="min-h-screen bg-slate-50 dark:bg-slate-900 flex flex-col items-center justify-center">
    {#if tokenExpired}
      <div class="mb-4 px-4 py-3 bg-amber-50 dark:bg-amber-900/30 border border-amber-200 dark:border-amber-700 text-amber-700 dark:text-amber-300 text-sm rounded-lg">
        Your session has expired. Please log in again.
      </div>
    {/if}
    <Login onSuccess={() => { tokenExpired = false; navigate('#/repos') }} />
  </div>
{:else}
  <div class="min-h-screen bg-slate-50 dark:bg-slate-900">
    <!-- Nav -->
    <nav class="bg-white dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700 shadow-sm">
      <div class="max-w-6xl mx-auto px-4 h-14 flex items-center justify-between">
        <div class="flex items-center gap-6">
          <button
            onclick={() => navigate('#/repos')}
            class="text-lg font-bold text-slate-800 dark:text-white hover:text-blue-600 dark:hover:text-blue-400 transition-colors cursor-pointer"
          >
            miraeboy
          </button>
          <button
            onclick={() => navigate('#/repos')}
            class="text-sm text-slate-600 dark:text-slate-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors cursor-pointer"
          >
            Repositories
          </button>
          {#if buildsEnabled}
            <button
              onclick={() => navigate('#/builds')}
              class="text-sm text-slate-600 dark:text-slate-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors cursor-pointer"
            >
              Builds
            </button>
          {/if}
          {#if auth.admin}
            <button
              onclick={() => navigate('#/users')}
              class="text-sm text-slate-600 dark:text-slate-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors cursor-pointer"
            >
              Users
            </button>
          {/if}
        </div>
        <div class="flex items-center gap-4">
          <span class="text-sm text-slate-500 dark:text-slate-400">
            {auth.username}
            {#if auth.admin}
              <span class="ml-1 px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 text-xs rounded font-medium">admin</span>
            {/if}
          </span>
          <button
            onclick={handleLogout}
            class="text-sm text-red-500 hover:text-red-700 dark:hover:text-red-400 transition-colors cursor-pointer"
          >
            Logout
          </button>
        </div>
      </div>
    </nav>

    <!-- Content -->
    <main class="max-w-6xl mx-auto px-4 py-6">
      {#if routeInfo.page === 'repos'}
        <Repositories {navigate} />
      {:else if routeInfo.page === 'detail'}
        <RepoDetail repoName={routeInfo.repo} {navigate} />
      {:else if routeInfo.page === 'packages'}
        <Packages repoName={routeInfo.repo} {navigate} />
      {:else if routeInfo.page === 'cargo'}
        <Cargo repoName={routeInfo.repo} {navigate} />
      {:else if routeInfo.page === 'users'}
        <Users {navigate} />
      {:else if routeInfo.page === 'builds'}
        <Builds {navigate} />
      {:else if routeInfo.page === 'callback'}
        <div class="py-16 text-center text-slate-500 dark:text-slate-400">Completing login…</div>
      {:else}
        <Repositories {navigate} />
      {/if}
    </main>
  </div>
{/if}
