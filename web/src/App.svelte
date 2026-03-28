<script>
  import './app.css'
  import { auth, logout } from './lib/auth.svelte.js'
  import Login from './routes/Login.svelte'
  import Repositories from './routes/Repositories.svelte'
  import RepoDetail from './routes/RepoDetail.svelte'
  import Packages from './routes/Packages.svelte'

  let currentRoute = $state(window.location.hash || '#/')

  function navigate(hash) {
    window.location.hash = hash
    currentRoute = hash
  }

  $effect(() => {
    const handler = () => { currentRoute = window.location.hash || '#/' }
    window.addEventListener('hashchange', handler)
    return () => window.removeEventListener('hashchange', handler)
  })

  function handleLogout() {
    logout()
    navigate('#/')
  }

  // Parse route
  let routeInfo = $derived.by(() => {
    if (currentRoute.startsWith('#/repo/')) {
      const parts = currentRoute.slice(7).split('/')
      const repoName = parts[0]
      if (parts[1] === 'packages') return { page: 'packages', repo: repoName }
      return { page: 'detail', repo: repoName }
    }
    if (currentRoute === '#/repos') return { page: 'repos' }
    return { page: 'login' }
  })
</script>

{#if !auth.loggedIn}
  <Login onSuccess={() => navigate('#/repos')} />
{:else}
  <div class="min-h-screen bg-slate-50 dark:bg-slate-900">
    <!-- Nav -->
    <nav class="bg-white dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700 shadow-sm">
      <div class="max-w-6xl mx-auto px-4 h-14 flex items-center justify-between">
        <div class="flex items-center gap-6">
          <button onclick={() => navigate('#/repos')} class="text-lg font-bold text-slate-800 dark:text-white hover:text-blue-600 dark:hover:text-blue-400 transition-colors cursor-pointer">
            Conan Repository
          </button>
          <button onclick={() => navigate('#/repos')} class="text-sm text-slate-600 dark:text-slate-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors cursor-pointer">
            Repositories
          </button>
        </div>
        <div class="flex items-center gap-4">
          <span class="text-sm text-slate-500 dark:text-slate-400">
            {auth.username}
            {#if auth.admin}
              <span class="ml-1 px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 text-xs rounded font-medium">admin</span>
            {/if}
          </span>
          <button onclick={handleLogout} class="text-sm text-red-500 hover:text-red-700 dark:hover:text-red-400 transition-colors cursor-pointer">
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
      {:else}
        <Repositories {navigate} />
      {/if}
    </main>
  </div>
{/if}
