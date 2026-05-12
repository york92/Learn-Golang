// API基础URL
const API_BASE = 'http://localhost:8080/api';

// 当前用户信息
let currentUser = null;
let authToken = null;
let currentArticleId = null;
let currentReplyTo = null;

// ===== 加载动画控制 =====
function showLoading() {
    document.getElementById('loadingOverlay').classList.add('show');
}

function hideLoading() {
    document.getElementById('loadingOverlay').classList.remove('show');
}

function setButtonLoading(btn, loading) {
    if (loading) {
        btn.classList.add('loading');
        btn.disabled = true;
    } else {
        btn.classList.remove('loading');
        btn.disabled = false;
    }
}

// 初始化应用
document.addEventListener('DOMContentLoaded', () => {
    // 检查本地存储的认证信息
    authToken = localStorage.getItem('authToken');
    const userStr = localStorage.getItem('currentUser');
    if (authToken && userStr) {
        currentUser = JSON.parse(userStr);
        console.log('Loaded user:', currentUser, 'Token:', authToken ? 'exists' : 'missing');
        updateUIForAuth();
        loadNotifications();
    }

    // 加载首页内容
    showHome();
    loadCategories();
    loadTags();
});

// ===== 页面切换 =====
function showPage(pageId) {
    document.querySelectorAll('.page').forEach(page => {
        page.style.display = 'none';
    });
    document.getElementById(pageId).style.display = 'block';
}

function showHome() {
    showPage('homePage');
    loadArticles();
}

function showLogin() {
    showPage('loginPage');
}

function showRegister() {
    showPage('registerPage');
}

function showCategories() {
    showHome();
}

function showMyArticles() {
    showHome();
    loadMyArticles();
}

function showNotifications() {
    showPage('notificationPage');
    loadNotificationList();
}

function showAdmin() {
    showPage('adminPage');
    showAdminUsers();
}

function showCreateArticle() {
    showPage('articleFormPage');
    document.getElementById('articleFormTitle').textContent = '创建文章';
    document.getElementById('articleForm').reset();
    document.getElementById('articleId').value = '';
}

function cancelArticleForm() {
    showHome();
}

// ===== 认证相关 =====
async function register(e) {
    e.preventDefault();
    
    const btn = e.target.querySelector('button[type="submit"]');
    const username = document.getElementById('registerUsername').value;
    const email = document.getElementById('registerEmail').value;
    const password = document.getElementById('registerPassword').value;

    setButtonLoading(btn, true);

    try {
        const response = await fetch(`${API_BASE}/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, email, password })
        });

        const data = await response.json();

        if (data.success) {
            showToast('注册成功，请登录', 'success');
            showLogin();
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('注册失败，请重试', 'error');
    } finally {
        setButtonLoading(btn, false);
    }
}

async function login(e) {
    e.preventDefault();

    const btn = e.target.querySelector('button[type="submit"]');
    const username = document.getElementById('loginUsername').value;
    const password = document.getElementById('loginPassword').value;

    setButtonLoading(btn, true);

    try {
        const response = await fetch(`${API_BASE}/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });

        const data = await response.json();

        if (data.success) {
            authToken = data.data.token;
            currentUser = data.data.user;

            localStorage.setItem('authToken', authToken);
            localStorage.setItem('currentUser', JSON.stringify(currentUser));

            showToast('登录成功', 'success');
            updateUIForAuth();
            showHome();
            loadNotifications();
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('登录失败，请重试', 'error');
    } finally {
        setButtonLoading(btn, false);
    }
}

async function logout() {
    showLoading();
    
    try {
        await fetch(`${API_BASE}/logout`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${authToken}` }
        });
    } catch (error) {
        console.error('Logout error:', error);
    }

    authToken = null;
    currentUser = null;
    localStorage.removeItem('authToken');
    localStorage.removeItem('currentUser');

    updateUIForAuth();
    showToast('已退出登录', 'success');
    showHome();
    hideLoading();
}

function updateUIForAuth() {
    const authButtons = document.getElementById('authButtons');
    const userMenu = document.getElementById('userMenu');
    const createBtn = document.getElementById('createArticleBtn');
    const myArticlesLink = document.getElementById('myArticlesLink');
    const adminLink = document.getElementById('adminLink');
    const commentForm = document.getElementById('commentForm');
    const loginPrompt = document.getElementById('loginPrompt');

    if (currentUser) {
        console.log('Current user role:', currentUser.role);
        authButtons.style.display = 'none';
        userMenu.style.display = 'flex';
        document.getElementById('welcomeUser').textContent = `欢迎, ${currentUser.username}`;

        if (currentUser.role === 'admin' || currentUser.role === 'author') {
            createBtn.style.display = 'inline-flex';
            myArticlesLink.style.display = 'block';
        }

        if (currentUser.role === 'admin') {
            adminLink.style.display = 'block';
        }

        if (commentForm) commentForm.style.display = 'block';
        if (loginPrompt) loginPrompt.style.display = 'none';
    } else {
        authButtons.style.display = 'flex';
        userMenu.style.display = 'none';
        createBtn.style.display = 'none';
        myArticlesLink.style.display = 'none';
        adminLink.style.display = 'none';

        if (commentForm) commentForm.style.display = 'none';
        if (loginPrompt) loginPrompt.style.display = 'block';
    }
}

// ===== 文章相关 =====
async function loadArticles(params = {}) {
    showLoading();
    
    try {
        const queryParams = new URLSearchParams(params);
        const response = await fetch(`${API_BASE}/articles?${queryParams}`);
        const data = await response.json();

        if (data.success) {
            displayArticles(data.data || []);
        }
    } catch (error) {
        showToast('加载文章失败', 'error');
    } finally {
        hideLoading();
    }
}

async function loadMyArticles() {
    if (!currentUser) return;

    showLoading();
    
    try {
        const response = await fetch(`${API_BASE}/articles`);
        const data = await response.json();

        if (data.success) {
            const myArticles = (data.data || []).filter(a => a.author_id === currentUser.id);
            displayArticles(myArticles);
        }
    } catch (error) {
        showToast('加载文章失败', 'error');
    } finally {
        hideLoading();
    }
}

function displayArticles(articles) {
    const articleList = document.getElementById('articleList');

    if (articles.length === 0) {
        articleList.innerHTML = `
            <div class="card">
                <div class="empty-state">
                    <div class="empty-state-icon">📭</div>
                    <div class="empty-state-text">暂无文章</div>
                </div>
            </div>
        `;
        return;
    }

    articleList.innerHTML = articles.map(article => `
        <div class="article-card" onclick="viewArticle(${article.id})">
            <h3>${escapeHtml(article.title)}</h3>
            <div class="article-meta">
                <span>👤 ${escapeHtml(article.author_name)}</span>
                <span>📅 ${formatDate(article.created_at)}</span>
                <span>👁️ ${article.views}</span>
                <span>❤️ ${article.likes}</span>
                ${article.category ? `<span>📂 ${escapeHtml(article.category)}</span>` : ''}
            </div>
            ${article.tags && article.tags.length > 0 ? `
                <div class="article-tags">
                    ${article.tags.map(tag => `<span class="tag">${escapeHtml(tag)}</span>`).join('')}
                </div>
            ` : ''}
            <div class="article-content">${escapeHtml(article.content)}</div>
        </div>
    `).join('');
}

async function viewArticle(id) {
    currentArticleId = id;
    showLoading();

    try {
        const response = await fetch(`${API_BASE}/articles/${id}`);
        const data = await response.json();

        if (data.success) {
            displayArticleDetail(data.data);
            loadComments(id);
            showPage('articleDetailPage');
            updateUIForAuth();
        }
    } catch (error) {
        showToast('加载文章失败', 'error');
    } finally {
        hideLoading();
    }
}

function displayArticleDetail(article) {
    const detail = document.getElementById('articleDetail');

    const canEdit = currentUser && (currentUser.role === 'admin' || currentUser.id === article.author_id);

    detail.innerHTML = `
        <h1>${escapeHtml(article.title)}</h1>
        <div class="article-meta">
            <span>👤 ${escapeHtml(article.author_name)}</span>
            <span>📅 ${formatDate(article.created_at)}</span>
            <span>👁️ ${article.views}</span>
            <span>❤️ ${article.likes}</span>
            ${article.category ? `<span>📂 ${escapeHtml(article.category)}</span>` : ''}
        </div>
        ${article.tags && article.tags.length > 0 ? `
            <div class="article-tags">
                ${article.tags.map(tag => `<span class="tag">${escapeHtml(tag)}</span>`).join('')}
            </div>
        ` : ''}
        ${article.cover_image ? `<img src="${article.cover_image}" alt="封面">` : ''}
        <div class="article-content" style="white-space: pre-wrap;">${escapeHtml(article.content)}</div>
        <div class="article-actions">
            ${currentUser ? `<button onclick="likeArticle(${article.id})" class="btn btn-primary btn-sm">❤️ 点赞</button>` : ''}
            ${canEdit ? `
                <button onclick="editArticle(${article.id})" class="btn btn-secondary btn-sm">✏️ 编辑</button>
                <button onclick="deleteArticle(${article.id})" class="btn btn-danger btn-sm">🗑️ 删除</button>
            ` : ''}
        </div>
    `;
}

async function submitArticle(e) {
    e.preventDefault();

    const btn = e.target.querySelector('button[type="submit"]');
    const id = document.getElementById('articleId').value;
    const title = document.getElementById('articleTitle').value;
    const content = document.getElementById('articleContent').value;
    const category = document.getElementById('articleCategory').value;
    const cover_image = document.getElementById('articleCover').value;
    const tagsStr = document.getElementById('articleTags').value;
    const tags = tagsStr ? tagsStr.split(',').map(t => t.trim()).filter(t => t) : [];

    const method = id ? 'PUT' : 'POST';
    const url = id ? `${API_BASE}/articles/${id}` : `${API_BASE}/articles`;

    setButtonLoading(btn, true);

    try {
        const response = await fetch(url, {
            method,
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${authToken}`
            },
            body: JSON.stringify({ title, content, category, cover_image, tags })
        });

        const data = await response.json();

        if (data.success) {
            showToast(id ? '更新成功' : '发布成功', 'success');
            showHome();
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('操作失败，请重试', 'error');
    } finally {
        setButtonLoading(btn, false);
    }
}

async function editArticle(id) {
    showLoading();
    
    try {
        const response = await fetch(`${API_BASE}/articles/${id}`);
        const data = await response.json();

        if (data.success) {
            const article = data.data;
            showPage('articleFormPage');
            document.getElementById('articleFormTitle').textContent = '编辑文章';
            document.getElementById('articleId').value = article.id;
            document.getElementById('articleTitle').value = article.title;
            document.getElementById('articleContent').value = article.content;
            document.getElementById('articleCategory').value = article.category || '';
            document.getElementById('articleCover').value = article.cover_image || '';
            document.getElementById('articleTags').value = article.tags ? article.tags.join(', ') : '';
        }
    } catch (error) {
        showToast('加载文章失败', 'error');
    } finally {
        hideLoading();
    }
}

async function deleteArticle(id) {
    if (!confirm('确定要删除这篇文章吗？')) return;

    showLoading();

    try {
        const response = await fetch(`${API_BASE}/articles/${id}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            showToast('删除成功', 'success');
            showHome();
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('删除失败，请重试', 'error');
    } finally {
        hideLoading();
    }
}

async function likeArticle(id) {
    try {
        const response = await fetch(`${API_BASE}/articles/${id}/like`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            showToast('操作成功', 'success');
            viewArticle(id);
        }
    } catch (error) {
        showToast('操作失败', 'error');
    }
}

// ===== 评论相关 =====
async function loadComments(articleId) {
    try {
        const response = await fetch(`${API_BASE}/articles/${articleId}/comments`);
        const data = await response.json();

        if (data.success) {
            displayComments(data.data || []);
        }
    } catch (error) {
        console.error('Load comments error:', error);
    }
}

function displayComments(comments) {
    const commentList = document.getElementById('commentList');

    if (comments.length === 0) {
        commentList.innerHTML = '<p class="text-muted" style="text-align:center;padding:1rem;">暂无评论，快来发表第一条评论吧！</p>';
        return;
    }

    commentList.innerHTML = comments.map(comment => renderComment(comment)).join('');
}

function renderComment(comment) {
    const canDelete = currentUser && (currentUser.role === 'admin' || currentUser.id === comment.user_id);

    return `
        <div class="comment">
            <div class="comment-header">
                <div class="comment-author">
                    <div class="comment-avatar">${comment.username.charAt(0).toUpperCase()}</div>
                    <span class="comment-username">${escapeHtml(comment.username)}</span>
                </div>
                <span class="comment-date">${formatDate(comment.created_at)}</span>
            </div>
            <div class="comment-content">${escapeHtml(comment.content)}</div>
            <div class="comment-actions">
                ${currentUser ? `
                    <button onclick="likeComment(${comment.id})">❤️ ${comment.likes}</button>
                    <button onclick="replyToComment(${comment.id})">💬 回复</button>
                ` : `<span style="font-size:0.75rem;color:var(--gray-500);">❤️ ${comment.likes}</span>`}
                ${canDelete ? `<button onclick="deleteComment(${comment.id})">🗑️ 删除</button>` : ''}
            </div>
            ${comment.replies && comment.replies.length > 0 ? `
                <div class="comment-replies">
                    ${comment.replies.map(reply => renderComment(reply)).join('')}
                </div>
            ` : ''}
        </div>
    `;
}

async function submitComment() {
    const content = document.getElementById('commentContent').value.trim();
    const btn = document.querySelector('#commentForm button');

    if (!content) {
        showToast('请输入评论内容', 'error');
        return;
    }

    setButtonLoading(btn, true);

    try {
        const response = await fetch(`${API_BASE}/articles/${currentArticleId}/comments`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${authToken}`
            },
            body: JSON.stringify({
                content,
                parent_id: currentReplyTo
            })
        });

        const data = await response.json();

        if (data.success) {
            showToast('评论成功', 'success');
            document.getElementById('commentContent').value = '';
            currentReplyTo = null;
            loadComments(currentArticleId);
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('评论失败，请重试', 'error');
    } finally {
        setButtonLoading(btn, false);
    }
}

function replyToComment(commentId) {
    currentReplyTo = commentId;
    document.getElementById('commentContent').focus();
    showToast('正在回复评论', 'success');
}

async function deleteComment(id) {
    if (!confirm('确定要删除这条评论吗？')) return;

    showLoading();

    try {
        const response = await fetch(`${API_BASE}/comments/${id}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            showToast('删除成功', 'success');
            loadComments(currentArticleId);
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('删除失败，请重试', 'error');
    } finally {
        hideLoading();
    }
}

async function likeComment(id) {
    try {
        const response = await fetch(`${API_BASE}/comments/${id}/like`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            loadComments(currentArticleId);
        }
    } catch (error) {
        showToast('操作失败', 'error');
    }
}

// ===== 分类和标签 =====
async function loadCategories() {
    try {
        const response = await fetch(`${API_BASE}/categories`);
        const data = await response.json();

        if (data.success) {
            const select = document.getElementById('categoryFilter');
            select.innerHTML = '<option value="">全部分类</option>' +
                (data.data || []).map(cat => `<option value="${escapeHtml(cat)}">${escapeHtml(cat)}</option>`).join('');
        }
    } catch (error) {
        console.error('Load categories error:', error);
    }
}

async function loadTags() {
    try {
        const response = await fetch(`${API_BASE}/tags`);
        const data = await response.json();

        if (data.success) {
            const select = document.getElementById('tagFilter');
            select.innerHTML = '<option value="">全部标签</option>' +
                (data.data || []).map(tag => `<option value="${escapeHtml(tag.name)}">${escapeHtml(tag.name)}</option>`).join('');
        }
    } catch (error) {
        console.error('Load tags error:', error);
    }
}

function filterArticles() {
    const category = document.getElementById('categoryFilter').value;
    const tag = document.getElementById('tagFilter').value;

    const params = {};
    if (category) params.category = category;
    if (tag) params.tag = tag;

    loadArticles(params);
}

// ===== 搜索 =====
function handleSearch(e) {
    if (e.key === 'Enter') {
        performSearch();
    }
}

async function performSearch() {
    const keyword = document.getElementById('searchInput').value.trim();

    if (!keyword) {
        showToast('请输入搜索关键词', 'error');
        return;
    }

    showLoading();

    try {
        const response = await fetch(`${API_BASE}/search?q=${encodeURIComponent(keyword)}`);
        const data = await response.json();

        if (data.success) {
            displayArticles(data.data || []);
        }
    } catch (error) {
        showToast('搜索失败', 'error');
    } finally {
        hideLoading();
    }
}

// ===== 通知 =====
async function loadNotifications() {
    if (!currentUser || !authToken) return;

    try {
        const response = await fetch(`${API_BASE}/notifications`, {
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            const unreadCount = (data.data || []).filter(n => !n.is_read).length;
            const badge = document.getElementById('notifBadge');
            if (unreadCount > 0) {
                badge.textContent = unreadCount;
                badge.style.display = 'inline';
            } else {
                badge.style.display = 'none';
            }
        }
    } catch (error) {
        console.error('Load notifications error:', error);
    }
}

async function loadNotificationList() {
    showLoading();
    
    try {
        const response = await fetch(`${API_BASE}/notifications`, {
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            displayNotifications(data.data || []);
        }
    } catch (error) {
        showToast('加载通知失败', 'error');
    } finally {
        hideLoading();
    }
}

function displayNotifications(notifications) {
    const list = document.getElementById('notificationList');

    if (notifications.length === 0) {
        list.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">🔔</div>
                <div class="empty-state-text">暂无通知</div>
            </div>
        `;
        return;
    }

    list.innerHTML = notifications.map(notif => `
        <div class="notification ${notif.is_read ? '' : 'unread'}" onclick="markNotificationRead(${notif.id})">
            <div class="notification-content">${escapeHtml(notif.content)}</div>
            <div class="notification-date">${formatDate(notif.created_at)}</div>
        </div>
    `).join('');
}

async function markNotificationRead(id) {
    try {
        await fetch(`${API_BASE}/notifications/${id}/read`, {
            method: 'PUT',
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        loadNotificationList();
        loadNotifications();
    } catch (error) {
        console.error('Mark read error:', error);
    }
}

// ===== 管理功能 =====
async function showAdminUsers() {
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    event.target.classList.add('active');

    showLoading();

    try {
        const response = await fetch(`${API_BASE}/admin/users`, {
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            displayAdminUsers(data.data || []);
        }
    } catch (error) {
        showToast('加载用户列表失败', 'error');
    } finally {
        hideLoading();
    }
}

function displayAdminUsers(users) {
    const content = document.getElementById('adminContent');

    if (users.length === 0) {
        content.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">👥</div>
                <div class="empty-state-text">暂无用户</div>
            </div>
        `;
        return;
    }

    content.innerHTML = `
        <table class="user-table">
            <thead>
                <tr>
                    <th>ID</th>
                    <th>用户名</th>
                    <th>邮箱</th>
                    <th>角色</th>
                    <th>注册时间</th>
                    <th>操作</th>
                </tr>
            </thead>
            <tbody>
                ${users.map(user => `
                    <tr>
                        <td>${user.id}</td>
                        <td>${escapeHtml(user.username)}</td>
                        <td>${escapeHtml(user.email)}</td>
                        <td>
                            <select onchange="changeUserRole(${user.id}, this.value)" class="form-select form-select-sm" style="width:auto;">
                                <option value="reader" ${user.role === 'reader' ? 'selected' : ''}>读者</option>
                                <option value="author" ${user.role === 'author' ? 'selected' : ''}>作者</option>
                                <option value="admin" ${user.role === 'admin' ? 'selected' : ''}>管理员</option>
                            </select>
                        </td>
                        <td>${formatDate(user.created_at)}</td>
                        <td>
                            <button onclick="deleteUser(${user.id})" class="btn btn-danger btn-sm">删除</button>
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;
}

async function changeUserRole(userId, role) {
    showLoading();
    
    try {
        const response = await fetch(`${API_BASE}/admin/users/${userId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${authToken}`
            },
            body: JSON.stringify({ role })
        });

        const data = await response.json();

        if (data.success) {
            showToast('更新成功', 'success');
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('更新失败', 'error');
    } finally {
        hideLoading();
    }
}

async function deleteUser(userId) {
    if (!confirm('确定要删除此用户吗？')) return;
    
    showLoading();
    
    try {
        const response = await fetch(`${API_BASE}/admin/users/${userId}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            showToast('删除成功', 'success');
            showAdminUsers();
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('删除失败', 'error');
    } finally {
        hideLoading();
    }
}

async function showAdminComments() {
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    event.target.classList.add('active');

    showLoading();

    try {
        const response = await fetch(`${API_BASE}/admin/comments/pending`, {
            headers: { 'Authorization': `Bearer ${authToken}` }
        });

        const data = await response.json();

        if (data.success) {
            displayAdminComments(data.data || []);
        }
    } catch (error) {
        showToast('加载评论列表失败', 'error');
    } finally {
        hideLoading();
    }
}

function displayAdminComments(comments) {
    const content = document.getElementById('adminContent');

    if (comments.length === 0) {
        content.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">💬</div>
                <div class="empty-state-text">暂无待审核评论</div>
            </div>
        `;
        return;
    }

    content.innerHTML = `
        <table class="comment-table">
            <thead>
                <tr>
                    <th>ID</th>
                    <th>用户</th>
                    <th>内容</th>
                    <th>时间</th>
                    <th>操作</th>
                </tr>
            </thead>
            <tbody>
                ${comments.map(comment => `
                    <tr>
                        <td>${comment.id}</td>
                        <td>${escapeHtml(comment.username)}</td>
                        <td>${escapeHtml(comment.content)}</td>
                        <td>${formatDate(comment.created_at)}</td>
                        <td>
                            <button onclick="approveComment(${comment.id}, 'approved')" class="btn btn-primary btn-sm">通过</button>
                            <button onclick="approveComment(${comment.id}, 'rejected')" class="btn btn-danger btn-sm">拒绝</button>
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;
}

async function approveComment(commentId, status) {
    showLoading();
    
    try {
        const response = await fetch(`${API_BASE}/admin/comments/${commentId}/approve`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${authToken}`
            },
            body: JSON.stringify({ status })
        });

        const data = await response.json();

        if (data.success) {
            showToast('操作成功', 'success');
            showAdminComments();
        } else {
            showToast(data.message, 'error');
        }
    } catch (error) {
        showToast('操作失败', 'error');
    } finally {
        hideLoading();
    }
}

// ===== 工具函数 =====
function showToast(message, type = 'success') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast show ${type}`;

    setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

function formatDate(dateString) {
    const date = new Date(dateString);
    const now = new Date();
    const diff = now - date;
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);

    if (minutes < 1) return '刚刚';
    if (minutes < 60) return `${minutes}分钟前`;
    if (hours < 24) return `${hours}小时前`;
    if (days < 7) return `${days}天前`;

    return date.toLocaleDateString('zh-CN');
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
