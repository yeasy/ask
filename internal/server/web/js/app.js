// DOM Elements

const navItems = document.querySelectorAll('.nav-item');

// State
let state = {
    view: localStorage.getItem('ask-current-view') || 'dashboard',
    skills: [],
    repos: [],
    stats: {},
    config: {},
    searchQuery: '',
    viewMode: localStorage.getItem('ask-view-mode') || 'grid',
    activeModal: null,
    skillTab: 'installed', // installed | available
    settings: {
        theme: localStorage.getItem('ask-theme') || 'dark',
        language: localStorage.getItem('ask-lang') || 'en',
        refreshInterval: parseInt(localStorage.getItem('ask-refresh-interval') || '300000') // Default 5 min
    },
    autoRefreshTimer: null
};

// Translations
const translations = {
    en: {
        dashboard: "Dashboard",
        skills: "Skills",
        repos: "Repositories",
        settings: "Settings",
        dashboard_title: "Dashboard",
        dashboard_desc: "Overview of your agent skills environment.",
        installed_skills: "INSTALLED SKILLS",
        configured_repos: "CONFIGURED REPOS",
        synced_repos: "SYNCED REPOS",
        skills_title: "Skills Manager",
        skills_desc: "Find, install, and manage agent capabilities.",
        install_skill_btn: "Install",
        refresh_btn: "Refresh",
        repos_title: "Repository Management",
        repos_desc: "Configure and sync skill sources.",
        sync_all_btn: "Sync All",
        add_repo_btn: "Add Repository",
        table_name: "Name",
        table_url: "URL",
        table_stars: "Stars",
        table_actions: "Actions",
        settings_title: "Settings",
        settings_desc: "Configure web interface preferences.",
        setting_theme: "Theme",
        setting_language: "Language",
        setting_refresh_interval: "Auto-Refresh Interval",
        modal_add_repo_title: "Add Repository",
        modal_repo_label: "Repository URL or Owner/Repo",
        btn_cancel: "Cancel",
        btn_add: "Add",
        modal_install_skill_title: "Install Skill",
        modal_skill_label: "Skill Name, URL, or Owner/Repo",
        btn_install: "Install",
        btn_close: "Close",
        settings_agents_title: "Agent Integrations",
        settings_agents_desc: "Manage which AI agents are enabled for skill installation."
    },
    zh: {
        dashboard: "仪表板",
        skills: "技能管理",
        repos: "仓库管理",
        settings: "系统设置",
        dashboard_title: "仪表板",
        dashboard_desc: "智能体技能环境概览。",
        installed_skills: "已安装技能",
        configured_repos: "配置仓库",
        synced_repos: "已同步仓库",
        skills_title: "技能管理",
        skills_desc: "发现、安装和管理智能体能力。",
        install_skill_btn: "安装",
        refresh_btn: "刷新",
        repos_title: "仓库管理",
        repos_desc: "配置并同步技能源。",
        sync_all_btn: "同步所有",
        add_repo_btn: "添加仓库",
        table_name: "名称",
        table_url: "地址",
        table_stars: "星标",
        table_actions: "操作",
        settings_title: "系统设置",
        settings_desc: "配置 Web 界面偏好。",
        setting_theme: "主题",
        setting_language: "语言",
        setting_refresh_interval: "自动刷新间隔",
        modal_add_repo_title: "添加仓库",
        modal_repo_label: "仓库地址 (URL 或 Owner/Repo)",
        btn_cancel: "取消",
        btn_add: "添加",
        modal_install_skill_title: "安装技能",
        modal_skill_label: "技能名称, URL 或 Owner/Repo",
        btn_install: "安装",
        btn_close: "关闭",
        settings_agents_title: "智能体集成",
        settings_agents_desc: "管理启用的 AI 智能体集成。"
    }
};

// Router
function navigate(view) {
    state.view = view;
    localStorage.setItem('ask-current-view', view);

    // Update Nav
    navItems.forEach(el => {
        const itemDataset = el.dataset.view || el.closest('.nav-item').dataset.view;
        if (itemDataset === view) {
            el.classList.add('active');
        } else {
            el.classList.remove('active');
        }
    });

    // Fetch data based on view
    if (view === 'skills') {
        fetchSkills();
    } else if (view === 'repos') {
        fetchRepos();
    } else if (view === 'dashboard') {
        fetchStats();
    } else if (view === 'agents' || view === 'settings') {
        fetchConfig();
    }

    render();
}

// Settings
function changeTheme(theme) {
    state.settings.theme = theme;
    localStorage.setItem('ask-theme', theme);
    document.documentElement.setAttribute('data-theme', theme);
}

function changeLanguage(lang) {
    state.settings.language = lang;
    localStorage.setItem('ask-lang', lang);
    updateTranslations();
    render();
}

function updateTranslations() {
    const t = translations[state.settings.language];
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.dataset.i18n;
        if (t[key]) el.textContent = t[key];
    });

    // Update placeholders
    if (state.settings.language === 'zh') {
        const sInput = document.getElementById('skill-search');
        if (sInput) sInput.placeholder = "搜索技能...";
    } else {
        const sInput = document.getElementById('skill-search');
        if (sInput) sInput.placeholder = "Search skills...";
    }
}

// API Calls
async function fetchStats() {
    try {
        const res = await fetch('/api/stats');
        if (!res.ok) throw new Error('Stats fetch failed');
        state.stats = await res.json();
        render(); // Re-render stats if they change
        // Note: calling render() here might conflict if user navigated away. 
        // Ideally should check state.view. But safe enough for dashboard stats.
        if (state.view === 'dashboard') renderDashboard();
    } catch (err) {
        console.error(err);
    }
}

// Save Project Root
async function saveProjectRoot() {
    const input = document.getElementById('system-project-root');
    if (!input) return;

    const newDir = input.value.trim();
    if (!newDir) {
        showToast('Project root cannot be empty', 'error');
        return;
    }

    try {
        const res = await fetch('/api/config/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ project_root: newDir })
        });

        if (res.ok) {
            showToast('Project configuration updated');
            await fetchConfig(); // Reload config
            await fetchStats(); // Reload stats if they depend on project
            await fetchRepos(); // Reload repos from new context
        } else {
            const data = await res.json();
            showToast(data.error || 'Failed to update project root', 'error');
        }
    } catch (err) {
        console.error(err);
        showToast('Error saving settings', 'error');
    }
}



async function fetchConfig() {
    try {
        const res = await fetch('/api/config');
        state.config = await res.json();
        const verEl = document.getElementById('server-version');
        if (verEl) verEl.textContent = `v${state.config.version}`;

        const sysVerEl = document.getElementById('system-version');
        if (sysVerEl) sysVerEl.textContent = `v${state.config.version}`;

        const sysRootEl = document.getElementById('system-project-root');
        if (sysRootEl) {
            sysRootEl.value = state.config.project_root || '';
        }



        // Render agents if we are in settings view or just ready for it
        if (state.view === 'settings') renderAgentSettings();
    } catch (err) {
        console.error(err);
    }
}

async function toggleAgent(agentName, enabled) {
    try {
        const res = await fetch('/api/config/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ agent: agentName, enabled: enabled })
        });

        if (!res.ok) {
            const data = await res.json();
            throw new Error(data.error || 'Failed to update config');
        }

        showToast(`Agent ${agentName} ${enabled ? 'enabled' : 'disabled'}`, 'success');
        // Refresh config to ensure state is synced
        fetchConfig();
    } catch (err) {
        showToast(err.message, 'error');
        // Revert toggle visually if possible (simple way is re-fetch config)
        fetchConfig();
    }
}


function renderAgentSettings() {
    const container = document.getElementById('agent-settings-list');
    if (!container) return;

    if (!state.config.tool_targets) {
        container.innerHTML = '<div class="loader"></div>';
        return;
    }

    container.innerHTML = '';

    // Sort agents: Enabled first, then alphabetical? Or just alphabetical.
    // Let's use the order from config (likely backend sorting or default order)

    state.config.tool_targets.forEach(agent => {
        const item = document.createElement('div');
        item.className = 'agent-item';

        // Capitalize Name
        const displayName = agent.name.charAt(0).toUpperCase() + agent.name.slice(1);

        item.innerHTML = `
          <div class="agent-info">
            <span class="agent-name">${escapeHtml(displayName)}</span>
            <span class="agent-path">${escapeHtml(agent.skills_dir)}</span>
          </div>
          <label class="switch">
            <input type="checkbox" ${agent.enabled ? 'checked' : ''} onchange="toggleAgent('${escapeHtml(agent.name)}', this.checked)">
            <span class="slider"></span>
          </label>
        `;
        container.appendChild(item);
    });
}



// ... (Translations remain same, removed for brevity in this replace block, but need to ensure I don't overwrite them incorrectly) 
// Actually I should be careful not to delete translations. 
// Let's target specific blocks. 

// I will target the fetchSkills and searchSkills area.
async function fetchSkills() {
    state.skillTab = 'installed';
    updateTabs();

    const container = document.getElementById('skills-list');
    if (container.children.length === 0) {
        container.innerHTML = '<div class="loader"></div>';
    }

    try {
        // Ensure config is loaded to get full list of agents
        if (!state.config || !state.config.tool_targets) {
            await fetchConfig();
        }

        const res = await fetch('/api/skills');
        state.skills = await res.json();

        // Populate filters
        renderFilters();

        applyFilters(); // Renders the list with filters applied
    } catch (err) {
        showToast('Failed to load skills', 'error');
        console.error(err);
    }
}

function renderFilters() {
    const agentSelect = document.getElementById('filter-agent');
    const repoSelect = document.getElementById('filter-repo');

    if (!agentSelect || !repoSelect) return;

    // Collect unique agents
    const agents = new Set();
    // Repos: use configured repos + any seen in skills
    const repos = new Set();

    // Normalization Map: "owner/repo" -> "Configured Name"
    const repoAliasMap = new Map();

    if (state.repos) {
        state.repos.forEach(r => {
            repos.add(r.name);

            // Map the name itself
            repoAliasMap.set(r.name.toLowerCase(), r.name);

            // Map the URL derivatives
            if (r.url) {
                let url = r.url.toLowerCase();
                // strip https://github.com/
                url = url.replace('https://github.com/', '').replace('http://github.com/', '');
                // strip .git
                url = url.replace(/\.git$/, '');
                // strip trailing slash
                url = url.replace(/\/$/, '');

                repoAliasMap.set(url, r.name);
            }
        });
    }

    if (state.skills) {
        state.skills.forEach(skill => {
            if (skill.agents) skill.agents.forEach(a => agents.add(a));

            if (skill.repo) {
                let rName = skill.repo;
                // Try to normalize
                const lower = rName.toLowerCase();
                if (repoAliasMap.has(lower)) {
                    rName = repoAliasMap.get(lower);
                    // Update the skill object itself for consistency in list view filtering too
                    skill.repo = rName;
                }
                repos.add(rName);
            }
        });
    }

    // Show/Hide filters
    const filtersDiv = document.getElementById('skill-filters');
    if (filtersDiv) {
        filtersDiv.style.display = 'flex';
    }

    // Populate Agents
    const currentAgent = agentSelect.value;
    agentSelect.innerHTML = '<option value="">All Agents</option>';

    Array.from(agents).sort().forEach(agent => {
        const opt = document.createElement('option');
        opt.value = agent;
        opt.textContent = agent.charAt(0).toUpperCase() + agent.slice(1);
        agentSelect.appendChild(opt);
    });

    agentSelect.value = currentAgent;
    agentSelect.style.display = agents.size > 0 ? 'block' : 'none';

    // Populate Repos
    const currentRepo = repoSelect.value;
    repoSelect.innerHTML = '<option value="">All Repos</option>';
    Array.from(repos).sort().forEach(repo => {
        const opt = document.createElement('option');
        opt.value = repo;
        opt.textContent = repo;
        repoSelect.appendChild(opt);
    });
    repoSelect.value = currentRepo;
    repoSelect.style.display = repos.size > 0 ? 'block' : 'none';
}

// ... existing applyFilters ...
function applyFilters() {
    const agentFilter = document.getElementById('filter-agent') ? document.getElementById('filter-agent').value : '';
    const repoFilter = document.getElementById('filter-repo') ? document.getElementById('filter-repo').value : '';
    const query = state.searchQuery.toLowerCase();

    if (state.skillTab === 'available') {
        const searchInput = document.getElementById('skill-search');
        const q = searchInput ? searchInput.value : '';
        searchSkills(q, repoFilter); // This handles API call
        return;
    }

    let filtered = state.skills || [];

    if (agentFilter) {
        filtered = filtered.filter(s => s.agents && s.agents.includes(agentFilter));
    }

    if (repoFilter) {
        filtered = filtered.filter(s => s.repo === repoFilter);
    }

    // Also apply search if any (client-side for installed)
    if (state.skillTab === 'installed' && query) {
        filtered = filtered.filter(s =>
            s.name.toLowerCase().includes(query) ||
            (s.description && s.description.toLowerCase().includes(query))
        );
    }

    renderSkillsList(filtered);
}

// ... existing code ...

async function viewRepoSkills(repoName) {
    if (!repoName) return;

    // Manual navigation to avoid default 'Installed' tab reset in navigate('skills')
    state.view = 'skills';
    state.skillTab = 'available';
    localStorage.setItem('ask-current-view', 'skills');

    // Update Nav UI
    navItems.forEach(el => {
        const itemDataset = el.dataset.view || el.closest('.nav-item').dataset.view;
        if (itemDataset === 'skills') {
            el.classList.add('active');
        } else {
            el.classList.remove('active');
        }
    });

    render(); // Shows view-skills
    updateTabs(); // Highlights 'Available' tab

    // Populate filters (using state.repos)
    renderFilters();

    // Set filter and search
    // We set the value BEFORE search so UI looks correct, but wait, searchSkills call will do the work.
    const repoSelect = document.getElementById('filter-repo');
    if (repoSelect) repoSelect.value = repoName;

    const searchInput = document.getElementById('skill-search');
    if (searchInput) searchInput.value = '';

    await searchSkills('', repoName);
}

function refreshSkills() {
    // If in available tab, search again with current query
    if (state.skillTab === 'available') {
        const searchInput = document.getElementById('skill-search');
        searchSkills(searchInput ? searchInput.value : '');
    } else {
        // Installed tab
        fetchSkills();
    }
}

async function searchSkills(query, repoFilter = '') {
    if (state.skillTab !== 'available' && (query || repoFilter)) {
        // Switch to available if searching
        switchSkillTab('available', false);
    }

    const listEl = document.getElementById('skills-list');
    listEl.innerHTML = '<div class="loader"></div>';

    // If query is empty and we are in available tab, this will trigger default search
    try {
        let url = `/api/skills/search?q=${encodeURIComponent(query)}`;
        if (repoFilter) {
            url += `&repo=${encodeURIComponent(repoFilter)}`;
        }

        const res = await fetch(url);
        const results = await res.json();
        renderSearchResults(results);
    } catch (err) {
        showToast('Search failed', 'error');
        listEl.innerHTML = '';
    }
}

function switchSkillTab(tab, triggerSearch = true) {
    state.skillTab = tab;
    updateTabs();

    // Clear search input if switching to installed
    const searchInput = document.getElementById('skill-search');
    if (searchInput && tab === 'installed') {
        searchInput.value = '';
    }

    if (tab === 'installed') {
        fetchSkills();
    } else {
        // Available
        if (triggerSearch) {
            searchSkills(searchInput ? searchInput.value : '');
        }
    }
}

function updateTabs() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        if (btn.dataset.tab === state.skillTab) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });

    // Update install button visibility - only relevant for installed view or manual install
    // Keeping it simple for now
}

async function fetchRepos() {
    const listEl = document.getElementById('repos-list').querySelector('tbody');
    if (listEl.children.length === 0) {
        listEl.innerHTML = '<tr><td colspan="4" style="text-align:center;"><div class="loader" style="margin:1rem auto"></div></td></tr>';
    }

    try {
        const res = await fetch('/api/repos');
        if (!res.ok) throw new Error('Failed to load repos');
        state.repos = await res.json();
        renderReposList();
    } catch (err) {
        showToast('Failed to load repos', 'error');
        listEl.innerHTML = '<tr><td colspan="4" class="empty-state">Failed to load</td></tr>';
    }
}

// Replaces direct installSkill with a modal opener
async function openInstallModal(name) {
    // Ensure we have config to list agents
    if (!state.config || !state.config.tool_targets) {
        await fetchConfig();
    }

    const nameInput = document.getElementById('install-skill-name');
    if (nameInput) nameInput.value = name || '';

    const agentSelect = document.getElementById('install-skill-agent');
    if (agentSelect) {
        agentSelect.innerHTML = '';

        let firstEnabled = null;
        let defaultOption = document.createElement('option');
        defaultOption.value = "";
        defaultOption.textContent = "Default (Auto Detect)";
        agentSelect.appendChild(defaultOption);

        if (state.config.tool_targets) {
            state.config.tool_targets.forEach(agent => {
                if (agent.enabled) { // Only show enabled agents
                    const opt = document.createElement('option');
                    opt.value = agent.name;
                    // Show path hint if available? agent.skills_dir
                    const pathHint = agent.skills_dir ? ` (${agent.skills_dir})` : '';
                    opt.textContent = `${agent.name}${pathHint}`;
                    agentSelect.appendChild(opt);

                    if (!firstEnabled) firstEnabled = agent.name;
                }
            });

            // Default option "" is already selected by default (first option)
        }
    }

    openModal('install-skill-modal');
}

async function performInstall() {
    const nameInput = document.getElementById('install-skill-name');
    const agentSelect = document.getElementById('install-skill-agent');

    const name = nameInput ? nameInput.value : '';
    const agent = agentSelect ? agentSelect.value : '';

    if (!name) {
        showToast('Please enter a skill name', 'error');
        return;
    }

    // Close modal immediately or wait? BETTER to close, show toast.
    closeModal('install-skill-modal');

    // Legacy `installSkill` logic adapted
    showToast(`Installing ${name}${agent ? ' to ' + agent : ''}...`, 'info');
    try {
        const body = { name };
        if (agent) body.agent = agent;

        const res = await fetch('/api/skills/install', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        const data = await res.json();
        if (data.status === 'success') {
            showToast('Skill installed successfully', 'success');
            navigate('skills');
            // Force refresh of installed skills
            fetchSkills();
        } else {
            throw new Error(data.error);
        }
    } catch (err) {
        showToast(err.message || 'Installation failed', 'error');
    }
}

// Kept as alias if needed, but redirects to openInstallModal for consistency
// Or if called programmatically without UI, it might fail? 
// The UI buttons now call openInstallModal or performInstall.
// Renaming old installSkill to openInstallModal where used.


async function uninstallSkill(name) {
    const confirmed = await showConfirm(
        'Uninstall Skill',
        `Are you sure you want to uninstall "${name}"? This action cannot be undone.`
    );
    if (!confirmed) return;

    try {
        const res = await fetch('/api/skills/uninstall', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name })
        });
        const data = await res.json();
        if (data.status === 'success') {
            showToast('Skill uninstalled', 'success');
            fetchSkills();
        } else {
            throw new Error(data.error);
        }
    } catch (err) {
        showToast(err.message || 'Uninstall failed', 'error');
    }
}

async function addRepo(url) {
    if (!url) return;
    try {
        const res = await fetch('/api/repos/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url, sync: true })
        });
        const data = await res.json();
        if (data.status === 'success') {
            showToast('Repository added successfully', 'success');
            closeModal('add-repo-modal');
            fetchRepos();
        } else {
            throw new Error(data.error);
        }
    } catch (err) {
        showToast(err.message || 'Failed to add repo', 'error');
    }
}

async function removeRepo(name) {
    const confirmed = await showConfirm(
        'Remove Repository',
        `Are you sure you want to remove "${name}" from your configured repositories?`
    );
    if (!confirmed) return;

    try {
        const res = await fetch('/api/repos/remove', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name })
        });
        const data = await res.json();
        if (data.status === 'success') {
            showToast('Repository removed', 'success');
            fetchRepos();
        } else {
            throw new Error(data.error);
        }
    } catch (err) {
        showToast(err.message || 'Failed to remove repo', 'error');
    }
}

async function syncRepos() {
    // Legacy sync all
    syncRepo('');
}

async function syncRepo(name) {
    const label = name ? name : 'all repositories';
    showToast(`Syncing ${label}...`, 'info');
    try {
        const body = name ? { name: name } : {};
        const res = await fetch('/api/repos/sync', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        const data = await res.json();
        if (data.status === 'success') {
            showToast(`${label} synced`, 'success');
            fetchRepos();
        } else {
            throw new Error(data.error);
        }
    } catch (err) {
        showToast(err.message || 'Sync failed', 'error');
    }
}


async function viewSkillGuide(name) {
    const contentEl = document.getElementById('guide-content');
    const titleEl = document.getElementById('guide-modal-title');
    titleEl.textContent = `Info: ${name}`;
    contentEl.textContent = 'Loading...';
    openModal('skill-guide-modal');

    try {
        const res = await fetch(`/api/skills/readme?name=${encodeURIComponent(name)}`);
        if (!res.ok) throw new Error('Failed to load guide');
        const data = await res.json();
        // Simple markdown cleanup for display since we don't have a renderer
        // Just displaying as pre-wrapped text is often enough for simple guides
        contentEl.textContent = data.content;
    } catch (err) {
        contentEl.innerHTML = `<div class="error-message">Failed to load guide: ${escapeHtml(err.message)}. <br>Make sure SKILL.md exists in the skill directory.</div>`;
    }
}

// Render Functions
function render() {
    // Hide all views
    document.querySelectorAll('.view-section').forEach(el => el.style.display = 'none');

    // Show current view
    const viewEl = document.getElementById(`view-${state.view}`);
    if (viewEl) {
        viewEl.style.display = 'block';
        // View specific updates
        if (state.view === 'dashboard') renderDashboard();
        if (state.view === 'skills') {
            toggleView(state.viewMode, true);
        }
        if (state.view === 'settings' || state.view === 'agents') {
            renderAgentSettings();
        }
    }

    // Apply Settings UI State
    const themeSelect = document.getElementById('theme-select');
    if (themeSelect) themeSelect.value = state.settings.theme;

    const langSelect = document.getElementById('lang-select');
    if (langSelect) langSelect.value = state.settings.language;

    // Update texts
    updateTranslations();
}

function renderDashboard() {
    const s = state.stats;
    document.getElementById('stat-skills').textContent = s.installed_skills !== undefined ? s.installed_skills : '-';
    document.getElementById('stat-repos').textContent = s.configured_repos !== undefined ? s.configured_repos : '-';
    document.getElementById('stat-synced').textContent = s.synced_repos !== undefined ? s.synced_repos : '-';

    // Render recent skills
    renderRecentSkills();
}

function renderRecentSkills() {
    const container = document.getElementById('recent-skills-list');
    if (!container) return;

    // Fetch skills if not already loaded
    if (!state.skills || state.skills.length === 0) {
        fetch('/api/skills')
            .then(res => res.json())
            .then(skills => {
                state.skills = skills;
                displayRecentSkills(container, skills);
            })
            .catch(() => {
                container.innerHTML = '<div class="empty-state-inline">No skills installed yet</div>';
            });
    } else {
        displayRecentSkills(container, state.skills);
    }
}

function displayRecentSkills(container, skills) {
    container.innerHTML = '';

    if (!skills || skills.length === 0) {
        container.innerHTML = '<div class="empty-state-inline">No skills installed yet. <a href="#" onclick="navigate(\'skills\'); switchSkillTab(\'available\'); return false;" style="color:var(--accent-color)">Browse available skills</a></div>';
        return;
    }

    // Show up to 4 recent skills
    const recentSkills = skills.slice(0, 4);

    recentSkills.forEach(skill => {
        const iconUrl = getIcon(skill);
        const agentText = skill.agents && skill.agents.length > 0 ? skill.agents.join(', ') : 'No agent';

        const card = document.createElement('div');
        card.className = 'recent-skill-card';
        card.onclick = () => { navigate('skills'); };
        card.innerHTML = `
            <img src="${escapeHtml(iconUrl)}" class="recent-skill-icon" onerror="this.src='data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>📦</text></svg>'">
            <div>
                <div class="recent-skill-name">${escapeHtml(skill.name)}</div>
                <div class="recent-skill-agents">${escapeHtml(agentText)}</div>
            </div>
        `;
        container.appendChild(card);
    });
}

function renderSkillsList(skills) {
    const container = document.getElementById('skills-list');
    container.innerHTML = '';

    // Apply View Mode
    if (state.viewMode === 'list') {
        container.classList.add('list-view');
    } else {
        container.classList.remove('list-view');
    }

    if (!skills || skills.length === 0) {
        const isInstalled = state.skillTab === 'installed';
        container.innerHTML = `
      <div class="empty-state">
        <div class="empty-state-icon">${isInstalled ? '📦' : '🔍'}</div>
        <div class="empty-state-title">${isInstalled ? 'No Skills Installed' : 'No Results Found'}</div>
        <div class="empty-state-text">
          ${isInstalled
                ? 'Get started by browsing and installing skills from the community.'
                : 'Try adjusting your search or filter to find what you\'re looking for.'}
        </div>
        <div class="empty-state-actions">
          ${isInstalled
                ? `<button class="btn btn-primary" onclick="searchSkills('mcp')">Browse Skills</button>`
                : `<button class="btn btn-secondary" onclick="clearSearch()">Clear Search</button>`}
        </div>
      </div>
    `;
        return;
    }

    skills.forEach(skill => {
        const iconUrl = getIcon(skill);
        const safeName = escapeHtml(skill.name);
        const safeDesc = escapeHtml(skill.description || 'No description available');

        // Badges HTML
        let badgesHtml = '';
        if (skill.repo) {
            badgesHtml += `<span class="skill-version" style="background-color:var(--bg-hover); color:var(--text-secondary); border:1px solid var(--border-color)">${escapeHtml(skill.repo)}</span>`;
        }
        if (skill.agents && skill.agents.length > 0) {
            skill.agents.forEach(agent => {
                badgesHtml += `<span class="skill-version" style="background-color:var(--accent-dim); color:var(--accent-color)">${escapeHtml(agent)}</span>`;
            });
        }
        if (skill.version) {
            badgesHtml += `<span class="skill-version">v${escapeHtml(skill.version)}</span>`;
        }


        const card = document.createElement('div');
        card.className = 'skill-card';
        card.innerHTML = `
      <div class="skill-header">
        <div class="skill-title-group">
            <img src="${escapeHtml(iconUrl)}" class="skill-icon" onerror="this.src='data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>📦</text></svg>'">
            <div>
                <div class="skill-name">${safeName}</div>
                <div class="skill-meta" style="flex-wrap:wrap; gap:0.3rem;">
                ${badgesHtml}
                </div>
            </div>
        </div>
      </div>
      <div class="skill-description">${safeDesc}</div>
      <div class="skill-actions">
        <button class="btn btn-danger" onclick="uninstallSkill('${safeName}')">
          Uninstall
        </button>
        <button class="btn btn-secondary" onclick="viewSkillGuide('${safeName}')">Info</button>
      </div>
    `;
        container.appendChild(card);
    });
}

function renderSearchResults(results) {
    const container = document.getElementById('skills-list');
    container.innerHTML = '';

    if (results.length === 0) {
        container.innerHTML = `
      <div class="empty-state">
        <div class="empty-state-icon">🔍</div>
        <div class="empty-state-title">No Skills Found</div>
        <div class="empty-state-text">We couldn't find any skills matching your search. Try a different keyword.</div>
        <div class="empty-state-actions">
          <button class="btn btn-secondary" onclick="clearSearch()">Clear Search</button>
        </div>
      </div>
    `;
        return;
    }

    results.forEach(item => {
        // Check if installed
        const isInstalled = state.skills.some(s => s.name === item.name);
        const iconUrl = getIcon(item);
        const safeName = escapeHtml(item.name);
        const safeDesc = escapeHtml(item.description || '');
        const safeFullName = escapeHtml(item.full_name);
        const safeUrl = escapeHtml(item.url); // Though usually URL is safe-ish if protocol checked? escapeHtml makes it safe for attribute.

        const card = document.createElement('div');
        card.className = 'skill-card';
        card.innerHTML = `
      <div class="skill-header">
        <div class="skill-title-group">
            <img src="${escapeHtml(iconUrl)}" class="skill-icon" onerror="this.src='data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>📦</text></svg>'">
            <div>
                <div class="skill-name">${safeName}</div>
                <div class="skill-meta">
                <span style="color:var(--warning-color)">★ ${item.stars}</span>
                </div>
            </div>
        </div>
      </div>
      <div class="skill-description">${safeDesc}</div>
        <div class="skill-actions">
        ${isInstalled ?
                `<button class="btn btn-secondary" disabled>Installed</button>` :
                `<button class="btn btn-primary" onclick="openInstallModal('${safeFullName}')">Install</button>`
            }
        <a href="${safeUrl}" target="_blank" class="btn btn-secondary">View</a>
      </div>
    `;
        container.appendChild(card);
    });
}

function renderReposList() {
    const listEl = document.getElementById('repos-list');
    if (!listEl) return;
    const tbody = listEl.querySelector('tbody');
    tbody.innerHTML = '';

    if (!state.repos || state.repos.length === 0) {
        tbody.innerHTML = `
          <tr>
            <td colspan="4">
              <div class="empty-state">
                <div class="empty-state-icon">📁</div>
                <div class="empty-state-title">No Repositories</div>
                <div class="empty-state-text">Add a GitHub repository to browse and install skills.</div>
                <div class="empty-state-actions">
                  <button class="btn btn-primary" onclick="openModal('add-repo-modal')">Add Repository</button>
                </div>
              </div>
            </td>
          </tr>`;
        return;
    }

    state.repos.forEach(repo => {
        const iconUrl = getIcon(repo);
        const tr = document.createElement('tr');
        tr.innerHTML = `
      <td>
        <div style="display:flex; align-items:center; gap:0.75rem;">
            <img src="${escapeHtml(iconUrl)}" class="repo-icon"
                 style="width:24px; height:24px; border-radius:4px;"
                 onerror="this.src='data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>📦</text></svg>'">
            <strong>${escapeHtml(repo.name)}</strong>
        </div>
      </td>
      <td class="url-cell">
        <div style="display:flex; align-items:center; gap:0.5rem;">
            <a href="${repo.url.startsWith('http') ? escapeHtml(repo.url) : 'https://github.com/' + escapeHtml(repo.url)}" target="_blank" class="repo-link" title="${escapeHtml(repo.url)}">
                ${escapeHtml(repo.url)} ↗
            </a>
        </div>
      </td>
      <td>${repo.stars !== undefined ? repo.stars : '-'}</td>
      <td>
        <div class="repo-actions" style="display:flex; gap:0.5rem;">
            <button class="btn btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;"
                    onclick="syncRepo('${escapeHtml(repo.name)}')" title="Sync this repository">Sync</button>
            <button class="btn btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;"
                    onclick="viewRepoSkills('${escapeHtml(repo.name)}')" title="View skills in this repository">Skills</button>
            <button class="btn btn-danger" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;"
                    onclick="removeRepo('${escapeHtml(repo.name)}')" title="Remove repository">Remove</button>
        </div>
      </td>
    `;
        tbody.appendChild(tr);
    });
}

async function viewRepoSkills(repoName) {
    if (!repoName) return;
    navigate('skills');

    // Switch to Available tab which uses search
    // We pass repoName as filter
    state.skillTab = 'available';
    updateTabs();

    const searchInput = document.getElementById('skill-search');
    if (searchInput) searchInput.value = ''; // Clear text search

    await searchSkills('', repoName);
}

// ... syncRepos function update below ...

// Settings Actions
async function clearCache() {
    try {
        const response = await fetch('/api/cache/clear', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });
        if (response.ok) {
            showToast('Cache cleared successfully', 'success');
        } else {
            showToast('Failed to clear cache', 'error');
        }
    } catch (err) {
        showToast('Error clearing cache', 'error');
    }
}

async function resetWebPreferences() {
    const confirmed = await showConfirm(
        'Reset Preferences',
        'Are you sure you want to reset all web preferences? This will reload the page and reset your theme, language, and view settings.'
    );
    if (confirmed) {
        localStorage.clear();
        window.location.reload();
    }
}

// UI Helpers
function escapeHtml(text) {
    if (!text) return '';
    return String(text)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function getIcon(item) {
    if (!item) return 'data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>📦</text></svg>';

    // Emoji Mapping based on keywords
    const getEmoji = (name) => {
        const lower = name.toLowerCase();
        if (lower.includes('research') || lower.includes('search')) return '🔍';
        if (lower.includes('brainstorm')) return '💡';
        if (lower.includes('test')) return '🧪';
        if (lower.includes('debug')) return '🐛';
        if (lower.includes('git')) return '🐙';
        if (lower.includes('doc') || lower.includes('pdf')) return '📝';
        if (lower.includes('web') || lower.includes('browser') || lower.includes('scraper')) return '🌐';
        if (lower.includes('memory') || lower.includes('remember')) return '🧠';
        if (lower.includes('tool') || lower.includes('util')) return '🛠️';
        if (lower.includes('art') || lower.includes('draw') || lower.includes('image')) return '🎨';
        if (lower.includes('code') || lower.includes('dev')) return '💻';
        if (lower.includes('plan') || lower.includes('manage')) return '📅';
        if (lower.includes('review')) return '👀';
        if (lower.includes('sql') || lower.includes('db') || lower.includes('data')) return '💾';
        if (lower.includes('file') || lower.includes('fs')) return '📁';
        if (lower.includes('weather')) return '🌤️';
        if (lower.includes('time') || lower.includes('date')) return '⏰';
        if (lower.includes('chart') || lower.includes('graph')) return '📊';
        if (lower.includes('mail') || lower.includes('post')) return '✉️';
        if (lower.includes('user') || lower.includes('profile')) return '👤';
        if (lower.includes('auth') || lower.includes('login')) return '🔐';
        if (lower.includes('music') || lower.includes('audio')) return '🎵';
        if (lower.includes('video') || lower.includes('watch')) return '📹';
        if (lower.includes('game') || lower.includes('play')) return '🎮';
        if (lower.includes('learn') || lower.includes('guide')) return '📚';
        if (lower.includes('translate')) return '🗣️';
        if (lower.includes('calc') || lower.includes('math')) return '🧮';
        return '📦';
    };

    // Metric 1: Keyword-based Emoji Icon (Local/Installed skills usually don't have nice GitHub avatars)
    // We prioritize this for skills to give them distinct visual identity
    // But ONLY if we don't have other signals like a URL or full_name that might give us a real avatar
    if ((!item.repo || item.repo === "") && (!item.url || item.url === "") && (!item.full_name || item.full_name === "") && item.name) {
        const emoji = getEmoji(item.name);
        return `data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>${emoji}</text></svg>`;
    }

    // Metric 2: Explicit Icon URL (backend provided, sanitized)
    if (item.icon_url && (item.icon_url.startsWith('https://') || item.icon_url.startsWith('data:'))) return item.icon_url;

    // Metric 3: Repo Owner Avatar (highest quality for GitHub repos)
    const validOwner = /^[a-zA-Z0-9_-]+$/;
    // Check item.repo (e.g. "owner/repo")
    if (item.repo && item.repo.includes('/')) {
        const [owner] = item.repo.split('/');
        if (validOwner.test(owner)) return `https://github.com/${owner}.png?size=64`;
    }

    // Check item.full_name (from GitHub search results)
    if (item.full_name && item.full_name.includes('/')) {
        const [owner] = item.full_name.split('/');
        if (validOwner.test(owner)) return `https://github.com/${owner}.png?size=64`;
    }

    // Check URL if it's a GitHub URL
    if (item.url && item.url.includes('github.com/')) {
        const parts = item.url.split('github.com/');
        if (parts.length > 1) {
            const path = parts[1];
            const [owner] = path.split('/');
            if (owner && validOwner.test(owner)) return `https://github.com/${owner}.png?size=64`;
        }
    }

    // Metric 4: Fallback to Emoji
    const emoji = getEmoji(item.name || "");
    return `data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>${emoji}</text></svg>`;
}

function toggleView(mode, skipRender = false) {
    state.viewMode = mode;
    localStorage.setItem('ask-view-mode', mode);

    // Update buttons
    const gridBtn = document.getElementById('view-grid');
    const listBtn = document.getElementById('view-list');
    if (gridBtn && listBtn) {
        if (mode === 'grid') {
            gridBtn.classList.add('active');
            listBtn.classList.remove('active');
        } else {
            gridBtn.classList.remove('active');
            listBtn.classList.add('active');
        }
    }

    if (state.view === 'skills') {
        const container = document.getElementById('skills-list');
        if (container) {
            if (mode === 'list') container.classList.add('list-view');
            else container.classList.remove('list-view');
        }
    }
}

// -- SkillsLM: Scan & Import --

async function openScanModal() {
    const pathInput = document.getElementById('scan-path');
    if (pathInput && !pathInput.value) {
        // Try to pre-fill with a default? or leave empty
        // Maybe ~/.ask/skills if we knew the home dir?
    }
    document.getElementById('scan-results-area').style.display = 'none';
    document.getElementById('scan-results-list').innerHTML = '';
    document.getElementById('btn-import-selected').disabled = true;

    openModal('scan-skill-modal');
}

let scannedSkills = [];

async function performScan() {
    const pathInput = document.getElementById('scan-path');
    const path = pathInput.value.trim();
    if (!path) {
        showToast('Please enter a directory path', 'error');
        return;
    }

    const btn = document.getElementById('scan-btn');
    btn.disabled = true;
    btn.textContent = 'Scanning...';

    const listEl = document.getElementById('scan-results-list');
    listEl.innerHTML = '<div style="padding:1rem; text-align:center;">Scanning...</div>';
    document.getElementById('scan-results-area').style.display = 'block';

    try {
        const res = await fetch('/api/skills/scan', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path })
        });

        if (!res.ok) throw new Error("Scan failed");

        scannedSkills = await res.json();
        renderScanResults(scannedSkills);

    } catch (err) {
        showToast(err.message, 'error');
        listEl.innerHTML = `<div style="padding:1rem; color:var(--danger-color)">Error: ${escapeHtml(err.message)}</div>`;
    } finally {
        btn.disabled = false;
        btn.textContent = 'Scan';
    }
}

function renderScanResults(skills) {
    const listEl = document.getElementById('scan-results-list');
    listEl.innerHTML = '';

    if (!skills || skills.length === 0) {
        listEl.innerHTML = '<div style="padding:1rem; text-align:center; color:var(--text-secondary)">No skills found (look for SKILL.md)</div>';
        return;
    }

    skills.forEach((skill, idx) => {
        const item = document.createElement('div');
        item.className = 'scan-result-item';

        // Name from meta or folder name
        let name = skill.meta ? skill.meta.name : '';
        if (!name) {
            // fallback to basename
            // We assume skill.path is full path
            const parts = skill.path.split(/[/\\]/);
            name = parts[parts.length - 1];
        }

        item.innerHTML = `
            <div class="scan-result-check">
                <input type="checkbox" class="scan-check" data-idx="${idx}" onchange="updateImportButton()">
            </div>
            <div class="scan-result-info">
                <div class="scan-result-name">${escapeHtml(name)}</div>
                <div class="scan-result-path">${escapeHtml(skill.path)}</div>
                <div style="font-size:0.75rem; color:var(--text-muted)">${skill.meta ? escapeHtml(skill.meta.description || 'No description') : 'No metadata'}</div>
            </div>
        `;
        listEl.appendChild(item);
    });
}

function selectAllScanResults(checked) {
    // Toggle all logic... actually let's just create a toggle
    const checks = document.querySelectorAll('.scan-check');
    let allChecked = true;
    // Check if any is unchecked
    checks.forEach(c => { if (!c.checked) allChecked = false; });

    // logic: if all checked, uncheck all. else check all.
    // The param `checked` from button click implies "select all" usually means check all.
    // Let's implement simpler: click select all -> check all.
    checks.forEach(c => c.checked = true);
    updateImportButton();
}

function updateImportButton() {
    const anyChecked = document.querySelector('.scan-check:checked');
    document.getElementById('btn-import-selected').disabled = !anyChecked;
}

async function importSelectedSkills() {
    const checks = document.querySelectorAll('.scan-check:checked');
    if (checks.length === 0) return;

    const btn = document.getElementById('btn-import-selected');
    btn.disabled = true;
    btn.textContent = `Importing (${checks.length})...`;

    let successCount = 0;

    for (const check of checks) {
        const idx = parseInt(check.dataset.idx);
        const skill = scannedSkills[idx];

        try {
            const res = await fetch('/api/skills/import', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ src_path: skill.path })
            });
            if (res.ok) successCount++;
        } catch (err) {
            console.error("Import failed for", skill.path, err);
        }
    }

    showToast(`Imported ${successCount} skills`, 'success');
    btn.textContent = 'Import Selected';
    closeModal('scan-skill-modal');
    fetchSkills(); // Refresh list
}

// -- SkillsLM: Detail View & Files --

async function openDetailModal(name) {
    // Replaces viewSkillGuide
    const titleEl = document.getElementById('detail-modal-title');
    titleEl.textContent = `Skill: ${name}`;
    titleEl.dataset.skill = name; // Store for tab switching

    // Reset tabs
    document.querySelectorAll('.tab-btn[data-detail-tab]').forEach(b => {
        b.classList.remove('active');
        if (b.dataset.detailTab === 'info') b.classList.add('active');
    });
    document.getElementById('detail-tab-info').style.display = 'block';
    document.getElementById('detail-tab-files').style.display = 'none';

    // Reset Content
    document.getElementById('detail-content').textContent = 'Loading...';
    document.getElementById('file-tree-root').innerHTML = 'Loading...';
    document.getElementById('file-content-code').textContent = '';

    openModal('skill-detail-modal');

    // Fetch Info (README)
    try {
        const res = await fetch(`/api/skills/readme?name=${encodeURIComponent(name)}`);
        const data = await res.json();
        const contentEl = document.getElementById('detail-content');
        if (data.status === 'success') {
            // Simple naive markdown link converter for display
            let html = escapeHtml(data.content || '')
                .replace(/# (.*)/g, '<h1>$1</h1>')
                .replace(/## (.*)/g, '<h2>$1</h2>')
                .replace(/### (.*)/g, '<h3>$1</h3>')
                .replace(/\*\*(.*)\*\*/g, '<strong>$1</strong>')
                .replace(/`([^`]*)`/g, '<code>$1</code>')
                .replace(/\n/g, '<br>');
            contentEl.innerHTML = html;
        } else {
            contentEl.textContent = 'No README information available.';
        }
    } catch (err) {
        document.getElementById('detail-content').textContent = 'Failed to load info.';
    }

    // Render Sync Badges
    renderSyncBadges(name);
}

// Alias for old calls in HTML
const viewSkillGuide = openDetailModal;

function switchDetailTab(tab) {
    document.querySelectorAll('.tab-btn[data-detail-tab]').forEach(b => {
        if (b.dataset.detailTab === tab) b.classList.add('active');
        else b.classList.remove('active');
    });

    if (tab === 'info') {
        document.getElementById('detail-tab-info').style.display = 'block';
        document.getElementById('detail-tab-files').style.display = 'none';
    } else {
        document.getElementById('detail-tab-info').style.display = 'none';
        document.getElementById('detail-tab-files').style.display = 'flex';
        // Load files if first time
        const skillName = document.getElementById('detail-modal-title').dataset.skill;
        fetchSkillFiles(skillName);
    }
}

async function fetchSkillFiles(skillName) {
    const rootEl = document.getElementById('file-tree-root');
    // Prevent reload if already loaded? slightly tricky to detect, just reload for now

    try {
        const res = await fetch(`/api/skills/files?skill=${encodeURIComponent(skillName)}`);
        if (!res.ok) throw new Error("Failed to load files");

        const rootNode = await res.json();
        rootEl.innerHTML = '';
        renderFileNode(rootNode, rootEl, skillName);

    } catch (err) {
        rootEl.innerHTML = `<div style="color:var(--error-color); padding:1rem">Failed to load files: ${escapeHtml(err.message)}</div>`;
    }
}

function renderFileNode(node, container, skillName) {
    const el = document.createElement('div');
    const isDir = node.type === 'dir';
    const icon = isDir ? '📁' : '📄';

    el.innerHTML = `
        <div class="file-tree-item" data-skill="${escapeHtml(skillName)}" data-path="${escapeHtml(node.path)}" data-isdir="${isDir}">
            <span class="file-icon">${icon}</span>
            <span>${escapeHtml(node.name)}</span>
        </div>
    `;
    el.querySelector('.file-tree-item').addEventListener('click', function() {
        handleFileClick(this, skillName, node.path, isDir);
    });
    container.appendChild(el);

    if (isDir && node.children) {
        const childrenContainer = document.createElement('div');
        childrenContainer.className = 'file-tree-children';
        // Open by default? maybe only root
        childrenContainer.style.display = 'block';

        node.children.forEach(child => {
            renderFileNode(child, childrenContainer, skillName);
        });
        el.appendChild(childrenContainer);
    }
}

async function handleFileClick(el, skillName, path, isDir) {
    // Highlight
    document.querySelectorAll('.file-tree-item').forEach(e => e.classList.remove('active'));
    el.querySelector('.file-tree-item').classList.add('active');

    if (!isDir) {
        // Fetch Content
        const codeEl = document.getElementById('file-content-code');
        codeEl.textContent = 'Loading...';
        document.getElementById('file-content-header').textContent = path;

        try {
            const res = await fetch(`/api/skills/files?skill=${encodeURIComponent(skillName)}&mode=content&path=${encodeURIComponent(path)}`);
            const data = await res.json();
            codeEl.textContent = data.content || '(Empty)';
        } catch (err) {
            codeEl.textContent = 'Error loading file content.';
        }
    } else {
        // Toggle dir (if we implemented closing)
    }
}

function renderSyncBadges(skillName) {
    const container = document.getElementById('detail-sync-status');
    if (!container) return;
    container.innerHTML = '';

    // Find the skill object to see current agents
    const skill = state.skills.find(s => s.name === skillName);
    const installedAgents = skill ? (skill.agents || []) : [];

    // List all configured agents
    const allAgents = state.config.tool_targets || [];

    allAgents.forEach(agent => {
        if (!agent.enabled) return;

        const isInstalled = installedAgents.includes(agent.name);
        const badge = document.createElement('div');
        badge.className = `sync-badge ${isInstalled ? 'active' : ''}`;
        badge.innerHTML = `
            <span>${escapeHtml(agent.name)}</span>
            <span>${isInstalled ? '✓' : '○'}</span>
        `;
        // Make clickable to toggle sync?
        badge.style.cursor = 'pointer';
        badge.onclick = () => toggleSkillSync(skillName, agent.name, !isInstalled);

        container.appendChild(badge);
    });
}

async function toggleSkillSync(skillName, agentName, install) {
    showToast(`${install ? 'Installing' : 'Uninstalling'} for ${agentName}...`, 'info');

    // We reuse install/uninstall endpoints but with agent param
    try {
        const endpoint = install ? '/api/skills/install' : '/api/skills/uninstall';
        // Note: uninstall API in server.go currently does --all, it doesn't support specific agent yet?
        // Let's check server.go handleSkillUninstall
        // It calls `skill uninstall --all req.Name`. It does NOT accept agent param.
        // So for now, we can only INSTALL per agent. Uninstalling per agent needs backend support.
        // If install=false, we might fail or remove global?

        if (!install) {
            showToast("Uninstall per agent not supported yet (Uninstalls everywhere)", "warning");
            // Fallback to uninstall all?
            return;
        }

        const res = await fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: skillName, agent: agentName })
        });

        if (res.ok) {
            showToast(`Synced ${skillName} to ${agentName}`, 'success');
            // Update state
            fetchSkills().then(() => renderSyncBadges(skillName));
        } else {
            throw new Error('Sync failed');
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `<span>${escapeHtml(message)}</span>`;

    container.appendChild(toast);

    setTimeout(() => {
        toast.style.opacity = '0';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

function openModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.add('active');
        state.activeModal = id;
    }
}

function closeModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.remove('active');
        state.activeModal = null;
    }
}

// Clear search input
function clearSearch() {
    const searchInput = document.getElementById('skill-search');
    const searchClearBtn = document.getElementById('search-clear');
    if (searchInput) {
        searchInput.value = '';
        state.searchQuery = '';
        if (searchClearBtn) searchClearBtn.style.display = 'none';
        fetchSkills();
    }
}

// Custom confirmation modal
let confirmResolve = null;

function showConfirm(title, message) {
    return new Promise((resolve) => {
        confirmResolve = resolve;
        document.getElementById('confirm-title').textContent = title;
        document.getElementById('confirm-message').textContent = message;
        openModal('confirm-modal');
    });
}

function closeConfirmModal(result) {
    closeModal('confirm-modal');
    if (confirmResolve) {
        confirmResolve(result);
        confirmResolve = null;
    }
}


// Init
document.addEventListener('DOMContentLoaded', () => {
    // Apply initial theme
    changeTheme(state.settings.theme);

    // Nav Click
    navItems.forEach(item => {
        item.addEventListener('click', (e) => {
            e.preventDefault();
            // Handle span inside a
            const view = item.dataset.view || item.closest('.nav-item').dataset.view;
            navigate(view);
        });
    });

    // Search
    const searchInput = document.getElementById('skill-search');
    const searchClearBtn = document.getElementById('search-clear');
    let debounce;
    if (searchInput) {
        searchInput.addEventListener('input', (e) => {
            const query = e.target.value;
            // Show/hide clear button
            if (searchClearBtn) {
                searchClearBtn.style.display = query.length > 0 ? 'flex' : 'none';
            }
            clearTimeout(debounce);
            debounce = setTimeout(() => {
                state.searchQuery = query;
                if (state.view !== 'skills') navigate('skills');
                if (query) {
                    searchSkills(query);
                } else {
                    fetchSkills();
                }
            }, 500);
        });
    }

    // Initial Load
    fetchConfig();
    fetchStats();
    navigate(state.view);

    // Apply Language
    changeLanguage(state.settings.language);

    // Initialize Refresh Interval UI
    const refreshSelect = document.getElementById('refresh-interval-select');
    if (refreshSelect) {
        refreshSelect.value = state.settings.refreshInterval;
    }
    setupAutoRefresh();

    // Close modal on overlay click
    document.querySelectorAll('.modal-overlay').forEach(overlay => {
        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) {
                // Special handling for confirm modal to ensure promise resolves
                if (overlay.id === 'confirm-modal') {
                    closeConfirmModal(false);
                } else {
                    overlay.classList.remove('active');
                }
            }
        });
    });
});

// Refresh Logic
async function refreshDashboard() {
    const btn = document.querySelector('#view-dashboard button[title*="Refresh"] svg');
    if (btn) btn.classList.add('spin-anim'); // Add simple CSS animation if defined, or just use visual cues

    await Promise.all([
        fetchStats(),
        // fetchSkills() // fetchStats updates stats, skills list is loaded by fetchSkills.
        // Although fetchStats doesn't return recent skills. fetchSkills does. 
        // renderDashboard calls fetchSkills internally? No. 
        // renderDashboard uses data from state.stats. 
        // We might want to re-fetch recent skills specifically?
        // Let's just re-fetch config and stats.
        fetchConfig()
    ]);

    // Recent skills are populated by fetchSkills() usually? 
    // Actually renderDashboard logic: 
    // It calls `displayRecentSkills(state.skills)` 
    // So we need to update state.skills.
    await fetchSkills();

    if (btn) setTimeout(() => btn.classList.remove('spin-anim'), 500);
    showToast('Dashboard Refreshed', 'success');
}

async function refreshRepos() {
    const btn = document.querySelector('#view-repos button[title*="Refresh"] svg');
    if (btn) btn.classList.add('spin-anim');
    await fetchRepos(); // This should re-read config/cache
    // Also re-render list
    renderReposList();
    if (btn) setTimeout(() => btn.classList.remove('spin-anim'), 500);
    showToast('Repositories Refreshed', 'success');
}

async function refreshAgents() {
    const btn = document.querySelector('#view-agents button[title*="Refresh"] svg');
    if (btn) btn.classList.add('spin-anim');
    await fetchConfig();
    renderAgentSettings();
    if (btn) setTimeout(() => btn.classList.remove('spin-anim'), 500);
    showToast('Agents Refreshed', 'success');
}

// Reuse existing refreshSkills but add toast
const originalRefreshSkills = window.refreshSkills || null;
// Wait, refreshSkills is already defined in app.js? (Searched for it, saw it in button onclick).
// It was likely defined as: function refreshSkills() { fetchSkills(); }
// I should verify. I'll define it or overwrite it if needed.
// Actually, I don't see `refreshSkills` function definition in my `view_file` output (lines 1150-1342).
// It must be earlier.
// I'll make sure it's available or redefine it properly.

window.refreshSkills = async function () {
    const btn = document.querySelector('#refresh-btn svg');
    if (btn) btn.classList.add('spin-anim');
    await fetchSkills();
    if (btn) setTimeout(() => btn.classList.remove('spin-anim'), 500);
    showToast('Skills Refreshed', 'success');
};


function changeRefreshInterval(val) {
    const interval = parseInt(val);
    state.settings.refreshInterval = interval;
    localStorage.setItem('ask-refresh-interval', interval);
    setupAutoRefresh();
    showToast(`Auto-Refresh set to ${interval ? (interval / 60000) + 'm' : 'Off'}`);
}

function setupAutoRefresh() {
    if (state.autoRefreshTimer) {
        clearInterval(state.autoRefreshTimer);
        state.autoRefreshTimer = null;
    }

    if (state.settings.refreshInterval > 0) {
        state.autoRefreshTimer = setInterval(() => {
            // Refresh based on active view
            switch (state.view) {
                case 'dashboard':
                    refreshDashboard(); // This is silent refresh (maybe suppress toast?)
                    // modify manual refresh to show toast, auto maybe not?
                    // For now let it show toast or suppress it. The user didn't specify.
                    // Usually auto-refresh is silent.
                    break;
                case 'skills':
                    fetchSkills();
                    break;
                case 'repos':
                    fetchRepos();
                    renderReposList();
                    break;
                case 'agents':
                    fetchConfig();
                    renderAgentSettings();
                    break;
            }
        }, state.settings.refreshInterval);
    }
}

