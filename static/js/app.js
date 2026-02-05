class DictApp {
    constructor() {
        this.searchInput = document.getElementById('searchInput');
        this.searchBtn = document.getElementById('searchBtn');
        this.loadingSection = document.getElementById('loadingSection');
        this.resultsSection = document.getElementById('resultsSection');
        this.errorSection = document.getElementById('errorSection');
        this.resultsContainer = document.getElementById('resultsContainer');
        this.errorMessage = document.getElementById('errorMessage');
        this.feedbackSection = document.getElementById('feedbackSection');
        
        this.initEventListeners();
        this.currentTab = null;
        
        // 页面加载时检查URL并执行搜索
        this.handleInitialURL();
    }

    initEventListeners() {
        // 搜索按钮点击事件
        this.searchBtn.addEventListener('click', () => this.performSearch());

        // 输入框回车事件
        this.searchInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                this.performSearch();
            }
        });

        // 输入框实时事件（可选的搜索建议）
        this.searchInput.addEventListener('input', () => {
            this.clearResults();
            this.hideError();
        });

        // 监听浏览器前进/后退按钮
        window.addEventListener('popstate', () => {
            this.handleInitialURL();
        });

        // 复制链接点击事件（事件委托）
        this.resultsContainer.addEventListener('click', (e) => {
            if (e.target.classList.contains('copy-link')) {
                e.preventDefault();
                const textToCopy = e.target.getAttribute('data-copy-text');
                this.copyToClipboard(textToCopy);
            }
        });
    }
    
    // 处理初始URL（页面加载或前进/后退）
    handleInitialURL() {
        const path = window.location.pathname;
        const match = path.match(/^\/(person|place|both|names|places)\/(.+)$/);
        
        if (match) {
            let searchType = match[1];
            const query = decodeURIComponent(match[2]);
            
            // 标准化类型名称
            if (searchType === 'person') {
                searchType = 'names';
            } else if (searchType === 'place') {
                searchType = 'places';
            }
            
            // 设置搜索框和单选按钮
            this.searchInput.value = query;
            const radioButtons = document.getElementsByName('searchType');
            for (const radio of radioButtons) {
                if (radio.value === searchType) {
                    radio.checked = true;
                    break;
                }
            }
            
            // 执行搜索
            this.performSearch(true); // true表示不更新URL
        }
    }
    
    // 更新浏览器URL
    updateURL(query, searchType) {
        // 转换类型名称为URL友好格式
        let urlType = searchType;
        if (searchType === 'names') {
            urlType = 'person';
        } else if (searchType === 'places') {
            urlType = 'place';
        }
        
        const urlParams = new URLSearchParams(window.location.search);
        const cacheParam = urlParams.get('cache') === 'no' ? '?cache=no' : '';
        const newURL = `/${urlType}/${encodeURIComponent(query)}${cacheParam}`;
        
        // 使用pushState更新URL但不重新加载页面
        window.history.pushState({query, searchType}, '', newURL);
    }

    getSearchType() {
        const radioButtons = document.getElementsByName('searchType');
        for (const radio of radioButtons) {
            if (radio.checked) {
                return radio.value;
            }
        }
        return 'both'; // 默认值
    }

    async performSearch(skipURLUpdate = false) {
        const query = this.searchInput.value.trim();
        if (!query) {
            this.showError('请输入查询词');
            return;
        }

        const searchType = this.getSearchType();
        
        // 更新URL（除非是从URL加载触发的）
        if (!skipURLUpdate) {
            this.updateURL(query, searchType);
        }
        
        // 检查URL参数是否包含 cache=no
        const urlParams = new URLSearchParams(window.location.search);
        const noCache = urlParams.get('cache') === 'no';
        
        if (noCache) {
            console.log('Cache disabled by URL parameter');
        }
        
        this.showLoading();
        this.clearResults();
        this.hideError();

        try {
            const response = await fetch('/api/search', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    query: query,
                    type: searchType,
                    noCache: noCache
                })
            });

            const data = await response.json();
            console.log('API Response:', data);

            if (!response.ok) {
                throw new Error(data.error || '搜索失败');
            }

            if (data.success && data.data) {
                console.log('Displaying results:', data.data);
                this.displayResults(data.data, query, searchType);
            } else {
                this.showNoResults(query);
            }
        } catch (error) {
            this.showError('搜索出错: ' + error.message);
        } finally {
            this.hideLoading();
        }
    }

    displayResults(data, originalQuery, searchType) {
        this.hideLoading();
        this.hideError();

        if (searchType === 'both') {
            if (data.tabs && data.tabs.length >= 1 && data.tab_results != null) {
                this.displayTwoColumnTabResults(data, originalQuery);
            } else {
                this.displayTwoColumnResults(data, originalQuery);
            }
        } else {
            this.displaySingleColumnResults(data, originalQuery);
        }
    }

    displayTwoColumnResults(data, originalQuery) {
        const hasNames = data.names && data.names.length > 0;
        const hasPlaces = data.places && data.places.length > 0;

        if (!hasNames && !hasPlaces) {
            this.showNoResults(originalQuery);
            return;
        }

        let html = '<div class="results-columns">';
        
        // 人名列
        html += '<div class="results-column">';
        html += '<div class="column-header">人名</div>';
        if (hasNames) {
            html += this.renderResultList(data.names);
        } else {
            html += '<div class="no-results">未找到人名</div>';
        }
        html += '</div>';

        // 地名列
        html += '<div class="results-column">';
        html += '<div class="column-header">地名</div>';
        if (hasPlaces) {
            html += this.renderResultList(data.places);
        } else {
            html += '<div class="no-results">未找到地名</div>';
        }
        html += '</div>';

        html += '</div>';
        
        this.resultsContainer.innerHTML = html;
        this.showResults();
    }

    // both 模式分 tab：只为人名生成tab，地名只搜索完整查询词
    displayTwoColumnTabResults(data, originalQuery) {
        let html = '<div class="results-single">';
        if (data.full_query_no_record) {
            html += `<div class="full-query-no-record">没有找到精确匹配"${this.escapeHtml(data.full_query_no_record)}"的记录。以下是相关记录。</div>`;
        }
        html += '<div class="tab-nav">';
        for (let i = 0; i < data.tabs.length; i++) {
            const isActive = i === 0 ? 'active' : '';
            html += `<button class="tab-button ${isActive}" data-tab="${i}">${this.escapeHtml(data.tabs[i])}</button>`;
        }
        html += '</div>';

        // 地名只搜索完整查询词，不分tab
        const placesResults = data.places || [];

        for (let i = 0; i < data.tabs.length; i++) {
            const isActive = i === 0 ? 'active' : '';
            const tabName = data.tabs[i];
            const namesResults = (data.tab_results && data.tab_results[tabName]) ? data.tab_results[tabName] : [];
            html += `<div class="tab-content ${isActive}" data-tab="${i}">`;
            html += '<div class="results-columns">';
            // 人名列 - 当前tab的结果
            html += '<div class="results-column"><div class="column-header">人名</div>';
            html += namesResults.length > 0 ? this.renderResultList(namesResults) : '<div class="no-results">未找到人名</div>';
            html += '</div>';
            // 地名列 - 完整查询的结果（不分tab）
            html += '<div class="results-column"><div class="column-header">地名</div>';
            html += placesResults.length > 0 ? this.renderResultList(placesResults) : '<div class="no-results">未找到地名</div>';
            html += '</div>';
            html += '</div></div>';
        }
        html += '</div>';
        this.resultsContainer.innerHTML = html;
        this.showResults();
        this.initTabListeners();
    }

    displaySingleColumnResults(data, originalQuery) {
        const results = data.names || data.places;
        const resultType = data.names ? 'names' : 'places';
        
        if (!results || results.length === 0) {
            this.showNoResults(originalQuery);
            return;
        }

        // 如果有tabs，显示tab界面
        if (data.tabs && data.tabs.length > 1) {
            this.displayTabResults(data, originalQuery, resultType);
        } else {
            // 简单的结果列表
            let html = '<div class="results-single">';
            html += '<div class="column-header">' + (resultType === 'names' ? '人名' : '地名') + '</div>';
            html += this.renderResultList(results);
            html += '</div>';
            
            this.resultsContainer.innerHTML = html;
            this.showResults();
        }
    }

    displayTabResults(data, originalQuery, resultType) {
        console.log('displayTabResults called with:', {data, originalQuery, resultType});

        let html = '<div class="results-single">';

        // 完整查询词无记录时的提示（在 tab 栏上方）
        if (data.full_query_no_record) {
            html += `<div class="full-query-no-record">没有找到完全符合"${this.escapeHtml(data.full_query_no_record)}"的记录</div>`;
        }
        
        // Tab导航
        html += '<div class="tab-nav">';
        for (let i = 0; i < data.tabs.length; i++) {
            const isActive = i === 0 ? 'active' : '';
            html += `<button class="tab-button ${isActive}" data-tab="${i}">${this.escapeHtml(data.tabs[i])}</button>`;
        }
        html += '</div>';

        // Tab内容
        for (let i = 0; i < data.tabs.length; i++) {
            const isActive = i === 0 ? 'active' : '';
            const tabName = data.tabs[i];
            html += `<div class="tab-content ${isActive}" data-tab="${i}">`;
            
            console.log(`Tab ${i} (${tabName}):`, data.tab_results);
            
            // 使用tab_results显示对应tab的结果
            if (data.tab_results && data.tab_results[tabName]) {
                const tabResults = data.tab_results[tabName];
                console.log(`Tab ${i} results:`, tabResults);
                if (tabResults.length > 0) {
                    html += this.renderResultList(tabResults);
                } else {
                    html += '<div class="no-results">未找到相关结果</div>';
                }
            } else {
                console.log(`No tab_results for ${tabName}`);
                html += '<div class="no-results">未找到相关结果</div>';
            }
            
            html += '</div>';
        }
        
        html += '</div>';
        
        this.resultsContainer.innerHTML = html;
        this.showResults();
        this.initTabListeners();
    }

    initTabListeners() {
        const tabButtons = this.resultsContainer.querySelectorAll('.tab-button');
        const tabContents = this.resultsContainer.querySelectorAll('.tab-content');

        tabButtons.forEach(button => {
            button.addEventListener('click', () => {
                const tabId = button.dataset.tab;
                
                // 移除所有active类
                tabButtons.forEach(btn => btn.classList.remove('active'));
                tabContents.forEach(content => content.classList.remove('active'));
                
                // 添加active类到当前tab
                button.classList.add('active');
                const targetContent = this.resultsContainer.querySelector(`.tab-content[data-tab="${tabId}"]`);
                if (targetContent) {
                    targetContent.classList.add('active');
                }
            });
        });
    }

    renderResultList(results) {
        let html = '';
        results.forEach((item, index) => {
            html += '<div class="result-item">';
            html += `<div class="result-name">${this.escapeHtml(item.name)}</div>`;

            // 构建详情文本
            let detailsText = '';
            if (item.nation && item.nation.trim() !== '') {
                detailsText = `【${this.escapeHtml(item.nation)}】${this.escapeHtml(item.translation)}`;
            } else {
                detailsText = this.escapeHtml(item.translation);
            }

            html += `<div class="result-details">${detailsText}`;
            html += ` <a href="javascript:void(0)" class="copy-link" data-copy-text="${this.escapeHtml(detailsText)}">复制</a>`;
            html += '</div>';
            html += '</div>';
        });
        return html;
    }

    showLoading() {
        this.loadingSection.style.display = 'block';
        this.searchBtn.disabled = true;
        this.searchBtn.textContent = '查询中...';
    }

    hideLoading() {
        this.loadingSection.style.display = 'none';
        this.searchBtn.disabled = false;
        this.searchBtn.textContent = '查询';
    }

    showResults() {
        this.resultsSection.style.display = 'block';
        if (this.feedbackSection) {
            this.feedbackSection.style.display = 'block';
        }
    }

    hideResults() {
        this.resultsSection.style.display = 'none';
        if (this.feedbackSection) {
            this.feedbackSection.style.display = 'none';
        }
    }

    clearResults() {
        this.resultsContainer.innerHTML = '';
        this.hideResults();
    }

    showError(message) {
        this.errorMessage.textContent = message;
        this.errorSection.style.display = 'block';
        if (this.feedbackSection) {
            this.feedbackSection.style.display = 'none';
        }
    }

    hideError() {
        this.errorSection.style.display = 'none';
    }

    showNoResults(query) {
        const safeQuery = this.escapeHtml(query || '');
        this.resultsContainer.innerHTML = `<div class="no-results">未找到与"${safeQuery}"相关的结果</div>`;
        this.showResults();
        if (this.feedbackSection) {
            this.feedbackSection.style.display = 'none';
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    async copyToClipboard(text) {
        try {
            await navigator.clipboard.writeText(text);
            // 可以在这里添加复制成功的视觉反馈
            console.log('Copied to clipboard:', text);
        } catch (err) {
            console.error('Failed to copy:', err);
            // 降级方案：使用传统的复制方法
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.select();
            try {
                document.execCommand('copy');
                console.log('Copied to clipboard (fallback):', text);
            } catch (e) {
                console.error('Fallback copy failed:', e);
            }
            document.body.removeChild(textarea);
        }
    }
}

// 页面加载完成后初始化应用
document.addEventListener('DOMContentLoaded', () => {
    new DictApp();
});