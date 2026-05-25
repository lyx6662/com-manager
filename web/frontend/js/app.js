// 全局配置
const API_BASE = '/api/v1';
let authToken = localStorage.getItem('token') || '';

// API 请求封装
async function api(method, path, data = null) {
    const config = {
        method: method,
        url: API_BASE + path,
        headers: {
            'Content-Type': 'application/json',
            'Authorization': authToken ? `Bearer ${authToken}` : ''
        }
    };

    if (data && (method === 'POST' || method === 'PUT')) {
        config.data = data;
    }

    try {
        const response = await axios(config);
        return response.data;
    } catch (error) {
        if (error.response && error.response.status === 401) {
            showLogin();
            return null;
        }
        throw error;
    }
}

// 页面加载
async function loadPage(page) {
    const content = document.getElementById('content');
    content.innerHTML = '<div class="text-center py-5"><div class="loading"></div><p class="mt-3">加载中...</p></div>';

    try {
        const response = await fetch(`/pages/${page}.html`);
        if (response.ok) {
            content.innerHTML = await response.text();
            // 执行页面特定的初始化脚本
            if (typeof window[`init${capitalize(page)}Page`] === 'function') {
                window[`init${capitalize(page)}Page`]();
            }
        } else {
            content.innerHTML = '<div class="alert alert-warning">页面加载失败</div>';
        }
    } catch (error) {
        content.innerHTML = '<div class="alert alert-danger">网络错误</div>';
    }

    // 更新导航状态
    document.querySelectorAll('.sidebar .nav-link').forEach(link => {
        link.classList.remove('active');
    });
    event.target.closest('.nav-link').classList.add('active');
}

// 首字母大写
function capitalize(str) {
    return str.charAt(0).toUpperCase() + str.slice(1);
}

// 显示登录
function showLogin() {
    const modal = new bootstrap.Modal(document.getElementById('loginModal'));
    modal.show();
}

// 登录
document.getElementById('loginForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;
    const errorDiv = document.getElementById('loginError');

    try {
        const result = await api('POST', '/auth/login', { username, password });
        if (result && result.code === 0) {
            authToken = result.data.token;
            localStorage.setItem('token', authToken);
            bootstrap.Modal.getInstance(document.getElementById('loginModal')).hide();
            errorDiv.classList.add('d-none');
        } else {
            errorDiv.textContent = result.message || '登录失败';
            errorDiv.classList.remove('d-none');
        }
    } catch (error) {
        errorDiv.textContent = '网络错误';
        errorDiv.classList.remove('d-none');
    }
});

// 登出
function logout() {
    authToken = '';
    localStorage.removeItem('token');
    showLogin();
}

// 侧边栏切换
function toggleSidebar() {
    document.getElementById('sidebar').classList.toggle('show');
}

// 显示提示消息
function showToast(message, type = 'success') {
    const toastContainer = document.querySelector('.toast-container') || createToastContainer();
    const toast = document.createElement('div');
    toast.className = `toast show align-items-center text-white bg-${type} border-0`;
    toast.innerHTML = `
        <div class="d-flex">
            <div class="toast-body">${message}</div>
            <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button>
        </div>
    `;
    toastContainer.appendChild(toast);
    setTimeout(() => toast.remove(), 3000);
}

function createToastContainer() {
    const container = document.createElement('div');
    container.className = 'toast-container';
    document.body.appendChild(container);
    return container;
}

// 确认对话框
function confirmAction(message) {
    return new Promise(resolve => {
        if (confirm(message)) {
            resolve(true);
        } else {
            resolve(false);
        }
    });
}

// 格式化时间
function formatTime(timestamp) {
    if (!timestamp) return '-';
    return new Date(timestamp).toLocaleString('zh-CN');
}

// 格式化状态
function formatStatus(online) {
    return online ?
        '<span class="status-online"><i class="bi bi-circle-fill"></i> 在线</span>' :
        '<span class="status-offline"><i class="bi bi-circle-fill"></i> 离线</span>';
}

// 页面初始化
document.addEventListener('DOMContentLoaded', () => {
    if (!authToken) {
        showLogin();
    }
});
