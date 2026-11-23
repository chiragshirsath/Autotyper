/**
 * Auto Typer — Companion App Logic (Go Edition)
 * 
 * Connects to the laptop companion via WebSocket over ADB reverse or local Wi-Fi.
 * Supports configurable Human Jitter intensity, QWERTY proximity typos, and error rates.
 */

(function () {
    'use strict';

    // ─── Constants & Token Handling ──────────────────────────
    const RECONNECT_INTERVAL = 2500;
    const MAX_HISTORY = 10;
    const HISTORY_KEY = 'autotyper_history';
    const SETTINGS_KEY = 'autotyper_settings';
    const TOKEN_KEY = 'autotyper_pairing_token';

    function getPairingToken() {
        const urlParams = new URLSearchParams(window.location.search);
        const urlToken = urlParams.get('token');
        if (urlToken) {
            localStorage.setItem(TOKEN_KEY, urlToken);
            return urlToken;
        }
        return localStorage.getItem(TOKEN_KEY) || '';
    }

    function getWebSocketUrl() {
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = location.host;
        const token = getPairingToken();
        return `${protocol}//${host}/ws?token=${encodeURIComponent(token)}`;
    }

    // ─── DOM Elements ────────────────────────────────────────
    const $ = (id) => document.getElementById(id);

    const elements = {
        ambientGlow: $('ambient-glow'),
        connectionBadge: $('connection-badge'),
        connectionDot: $('connection-dot'),
        connectionText: $('connection-text'),
        authStatusBadge: $('auth-status-badge'),
        pairingModalOverlay: $('pairing-modal-overlay'),
        pinInput: $('pin-input'),
        pinSubmitBtn: $('pin-submit-btn'),
        textInput: $('text-input'),
        textareaWrapper: $('textarea-wrapper'),
        charCount: $('char-count'),
        clearBtn: $('clear-btn'),
        speedInput: $('speed-input'),
        speedTag: $('speed-tag'),
        speedMinus: $('speed-minus'),
        speedPlus: $('speed-plus'),
        speedPresets: $('speed-presets'),
        delayInput: $('delay-input'),
        delayMinus: $('delay-minus'),
        delayPlus: $('delay-plus'),
        estimate: $('estimate'),
        preserveFormatting: $('preserve-formatting'),
        smartIndent: $('smart-indent'),
        pasteMode: $('paste-mode'),
        jitterMode: $('jitter-mode'),
        jitterControls: $('jitter-controls'),
        jitterLevelTag: $('jitter-level-tag'),
        jitterPresetGroup: $('jitter-preset-group'),
        oopsMode: $('oops-mode'),
        oopsControls: $('oops-controls'),
        errorRateTag: $('error-rate-tag'),
        errorRateInput: $('error-rate-input'),
        errorMinus: $('error-minus'),
        errorPlus: $('error-plus'),
        errorPresetGroup: $('error-preset-group'),
        progressSection: $('progress-section'),
        progressLabel: $('progress-label'),
        progressPercent: $('progress-percent'),
        progressBarFill: $('progress-bar-fill'),
        progressBarGlow: $('progress-bar-glow'),
        progressChars: $('progress-chars'),
        progressEta: $('progress-eta'),
        punchBtn: $('punch-btn'),
        punchBtnContent: $('punch-btn-content'),
        punchBtnLoading: $('punch-btn-loading'),
        updateBtn: $('update-btn'),
        stopBtn: $('stop-btn'),
        historyList: $('history-list'),
        historyEmpty: $('history-empty'),
        clearHistoryBtn: $('clear-history-btn'),
        countdownOverlay: $('countdown-overlay'),
        countdownNumber: $('countdown-number'),
        countdownCancel: $('countdown-cancel'),
        toast: $('toast'),
        toastIcon: $('toast-icon'),
        toastMessage: $('toast-message'),
    };

    // ─── State ───────────────────────────────────────────────
    let ws = null;
    let isConnected = false;
    let isAuthorized = false;
    let isPunching = false;
    let countdownTimer = null;
    let reconnectTimer = null;
    let toastTimer = null;
    let wakeLock = null;

    let currentJitterLevel = 2; // 1=Low, 2=Med, 3=High, 4=Chaos
    let currentErrorRate = 3;    // 1-25%

    // ─── Wake Lock ───────────────────────────────────────────
    async function acquireWakeLock() {
        try {
            if ('wakeLock' in navigator) {
                wakeLock = await navigator.wakeLock.request('screen');
                wakeLock.addEventListener('release', () => { wakeLock = null; });
            }
        } catch (e) { /* ignore */ }
    }

    async function releaseWakeLock() {
        try {
            if (wakeLock) {
                await wakeLock.release();
                wakeLock = null;
            }
        } catch (e) { /* ignore */ }
    }

    // ─── Utility Functions ───────────────────────────────────
    function getSpeed() {
        const val = parseInt(elements.speedInput.value, 10);
        if (isNaN(val) || val < 10) return 10;
        if (val > 500) return 500;
        return val;
    }

    function getDelay() {
        const val = parseInt(elements.delayInput.value, 10);
        if (isNaN(val) || val < 0) return 0;
        if (val > 60) return 60;
        return val;
    }

    function wpmToDelay(wpm) {
        return Math.max(1, Math.round(12000 / wpm));
    }

    function countWords(text) {
        return text.trim() ? text.trim().split(/\s+/).length : 0;
    }

    function formatTime(seconds) {
        if (seconds < 60) return `~${Math.ceil(seconds)}s`;
        const mins = Math.floor(seconds / 60);
        const secs = Math.ceil(seconds % 60);
        return `~${mins}m ${secs}s`;
    }

    function updateJitterDisplay(level) {
        currentJitterLevel = level || currentJitterLevel;
        if (elements.jitterControls) {
            elements.jitterControls.style.display = elements.jitterMode.checked ? 'block' : 'none';
        }
        if (elements.jitterLevelTag) {
            const labels = {
                1: 'Low (25%)',
                2: 'Med (50%)',
                3: 'High (80%)',
                4: 'Chaos (120% + Pauses)'
            };
            elements.jitterLevelTag.textContent = labels[currentJitterLevel] || 'Med (50%)';
        }
        if (elements.jitterPresetGroup) {
            elements.jitterPresetGroup.querySelectorAll('.jitter-level-btn').forEach(btn => {
                btn.classList.toggle('active', parseInt(btn.dataset.level, 10) === currentJitterLevel);
            });
        }
        saveSettings();
        updateEstimate();
    }

    function updateErrorRateDisplay(rate) {
        currentErrorRate = rate !== undefined ? rate : currentErrorRate;
        if (currentErrorRate < 1) currentErrorRate = 1;
        if (currentErrorRate > 25) currentErrorRate = 25;

        if (elements.oopsControls) {
            elements.oopsControls.style.display = elements.oopsMode.checked ? 'block' : 'none';
        }
        if (elements.errorRateInput) {
            elements.errorRateInput.value = currentErrorRate;
        }
        if (elements.errorRateTag) {
            let label = 'Subtle';
            if (currentErrorRate >= 3 && currentErrorRate < 7) label = 'Normal';
            else if (currentErrorRate >= 7 && currentErrorRate < 12) label = 'High';
            else if (currentErrorRate >= 12) label = 'Chaos';
            elements.errorRateTag.textContent = `${currentErrorRate}% · ${label}`;
        }
        if (elements.errorPresetGroup) {
            elements.errorPresetGroup.querySelectorAll('.error-preset-btn').forEach(btn => {
                btn.classList.toggle('active', parseInt(btn.dataset.rate, 10) === currentErrorRate);
            });
        }
        saveSettings();
        updateEstimate();
    }

    // ─── Settings Persistence ────────────────────────────────
    function loadSettings() {
        try {
            const saved = JSON.parse(localStorage.getItem(SETTINGS_KEY) || '{}');
            if (saved.wpm) elements.speedInput.value = saved.wpm;
            if (saved.focusDelay !== undefined) elements.delayInput.value = saved.focusDelay;
            if (saved.preserveFormatting !== undefined) elements.preserveFormatting.checked = saved.preserveFormatting;
            if (saved.smartIndent !== undefined && elements.smartIndent) elements.smartIndent.checked = saved.smartIndent;
            if (saved.pasteMode !== undefined) elements.pasteMode.checked = saved.pasteMode;
            if (saved.jitterMode !== undefined) elements.jitterMode.checked = saved.jitterMode;
            if (saved.jitterLevel !== undefined) currentJitterLevel = saved.jitterLevel;
            if (saved.oopsMode !== undefined) elements.oopsMode.checked = saved.oopsMode;
            if (saved.errorRate !== undefined) currentErrorRate = saved.errorRate;
        } catch (e) { /* ignore */ }
    }

    function saveSettings() {
        try {
            localStorage.setItem(SETTINGS_KEY, JSON.stringify({
                wpm: getSpeed(),
                focusDelay: getDelay(),
                preserveFormatting: elements.preserveFormatting.checked,
                smartIndent: elements.smartIndent ? elements.smartIndent.checked : true,
                pasteMode: elements.pasteMode.checked,
                jitterMode: elements.jitterMode ? elements.jitterMode.checked : false,
                jitterLevel: currentJitterLevel,
                oopsMode: elements.oopsMode ? elements.oopsMode.checked : false,
                errorRate: currentErrorRate,
            }));
        } catch (e) { /* ignore */ }
    }

    // ─── History ─────────────────────────────────────────────
    function loadHistory() {
        try {
            return JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]');
        } catch (e) { return []; }
    }

    function saveHistory(history) {
        try {
            localStorage.setItem(HISTORY_KEY, JSON.stringify(history.slice(0, MAX_HISTORY)));
        } catch (e) { /* ignore */ }
    }

    function addToHistory(text) {
        if (!text.trim()) return;
        const history = loadHistory();
        const idx = history.findIndex(h => h.text === text);
        if (idx !== -1) history.splice(idx, 1);
        history.unshift({
            text: text,
            chars: text.length,
            time: new Date().toISOString(),
        });
        saveHistory(history);
        renderHistory();
    }

    function renderHistory() {
        const history = loadHistory();
        if (history.length === 0) {
            elements.historyEmpty.classList.remove('hidden');
            elements.historyList.querySelectorAll('.history-item').forEach(el => el.remove());
            return;
        }

        elements.historyEmpty.classList.add('hidden');
        elements.historyList.querySelectorAll('.history-item').forEach(el => el.remove());

        history.forEach((item) => {
            const el = document.createElement('div');
            el.className = 'history-item';
            el.setAttribute('role', 'button');
            el.setAttribute('tabindex', '0');

            const preview = item.text.replace(/\n/g, '↵ ').replace(/\t/g, '→ ');
            const timeAgo = getTimeAgo(new Date(item.time));

            el.innerHTML = `
                <span class="history-item-text">${escapeHtml(preview)}</span>
                <span class="history-item-meta">${item.chars} chars<br>${timeAgo}</span>
            `;

            el.addEventListener('click', () => {
                elements.textInput.value = item.text;
                updateCharCount();
                updateTextareaState();
                showToast('✓', 'Loaded from history', 'success');
            });

            elements.historyList.appendChild(el);
        });
    }

    function getTimeAgo(date) {
        const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
        if (seconds < 60) return 'just now';
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
        return `${Math.floor(seconds / 86400)}d ago`;
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // ─── WebSocket Connection ────────────────────────────────
    function connect() {
        if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
            return;
        }

        const wsUrl = getWebSocketUrl();
        try {
            ws = new WebSocket(wsUrl);
        } catch (e) {
            scheduleReconnect();
            return;
        }

        ws.onopen = () => {
            isConnected = true;
            updateConnectionUI(true);
            clearTimeout(reconnectTimer);
            showToast('⚡', 'Connected to companion', 'success');
        };

        ws.onclose = () => {
            isConnected = false;
            updateConnectionUI(false);
            if (isPunching) {
                stopPunching('Connection lost');
            }
            scheduleReconnect();
        };

        ws.onerror = () => { /* onclose will follow */ };

        ws.onmessage = (event) => {
            handleMessage(event.data);
        };
    }

    function scheduleReconnect() {
        clearTimeout(reconnectTimer);
        reconnectTimer = setTimeout(connect, RECONNECT_INTERVAL);
    }

    function updateConnectionUI(connected) {
        elements.connectionDot.classList.toggle('connected', connected);
        elements.connectionBadge.classList.toggle('connected', connected);
        elements.connectionText.textContent = connected ? 'Connected' : 'Offline';
        elements.ambientGlow.classList.toggle('connected', connected);
        updatePunchButtonState();
    }

    function updateAuthUI(authorized) {
        isAuthorized = authorized;
        if (elements.authStatusBadge) {
            elements.authStatusBadge.className = `auth-status-badge ${authorized ? 'paired' : 'unpaired'}`;
            elements.authStatusBadge.textContent = authorized ? '🔒 Paired' : '⚠️ Unpaired';
        }

        if (elements.pairingModalOverlay) {
            if (authorized) {
                elements.pairingModalOverlay.classList.add('hidden');
            } else {
                elements.pairingModalOverlay.classList.remove('hidden');
            }
        }
        updatePunchButtonState();
    }

    // ─── Message Handling ────────────────────────────────────
    function handleMessage(data) {
        let msg;
        try {
            msg = JSON.parse(data);
        } catch (e) { return; }

        switch (msg.type) {
            case 'sync_state':
                if (msg.authorized !== undefined) {
                    updateAuthUI(msg.authorized);
                }
                if (msg.state) {
                    if (msg.state.text && !elements.textInput.value.trim()) {
                        elements.textInput.value = msg.state.text;
                        updateCharCount();
                        updateTextareaState();
                    }
                    if (msg.state.wpm) {
                        elements.speedInput.value = msg.state.wpm;
                        updateSpeedDisplay();
                    }
                    if (msg.state.focus_delay_sec !== undefined) {
                        elements.delayInput.value = msg.state.focus_delay_sec;
                    }
                    if (msg.state.preserve_formatting !== undefined) {
                        elements.preserveFormatting.checked = msg.state.preserve_formatting;
                    }
                    if (msg.state.smart_indent !== undefined && elements.smartIndent) {
                        elements.smartIndent.checked = msg.state.smart_indent;
                    }
                    if (msg.state.paste_mode !== undefined) {
                        elements.pasteMode.checked = msg.state.paste_mode;
                    }
                    if (msg.state.jitter_mode !== undefined && elements.jitterMode) {
                        elements.jitterMode.checked = msg.state.jitter_mode;
                    }
                    if (msg.state.jitter_level !== undefined) {
                        currentJitterLevel = msg.state.jitter_level;
                    }
                    if (msg.state.oops_mode !== undefined && elements.oopsMode) {
                        elements.oopsMode.checked = msg.state.oops_mode;
                    }
                    if (msg.state.error_rate !== undefined) {
                        currentErrorRate = msg.state.error_rate;
                    }
                    updateJitterDisplay();
                    updateErrorRateDisplay();
                }
                break;

            case 'code_updated':
                if (msg.text !== undefined) {
                    elements.textInput.value = msg.text;
                    updateCharCount();
                    updateTextareaState();
                    flashSyncHighlight();
                    showToast('🔄', 'Code updated from device', 'success');
                }
                if (msg.wpm) {
                    elements.speedInput.value = msg.wpm;
                    updateSpeedDisplay();
                }
                if (msg.focus_delay_sec !== undefined) elements.delayInput.value = msg.focus_delay_sec;
                if (msg.preserve_formatting !== undefined) elements.preserveFormatting.checked = msg.preserve_formatting;
                if (msg.smart_indent !== undefined && elements.smartIndent) elements.smartIndent.checked = msg.smart_indent;
                if (msg.paste_mode !== undefined) elements.pasteMode.checked = msg.paste_mode;
                if (msg.jitter_mode !== undefined && elements.jitterMode) {
                    elements.jitterMode.checked = msg.jitter_mode;
                }
                if (msg.jitter_level !== undefined) {
                    currentJitterLevel = msg.jitter_level;
                }
                if (msg.oops_mode !== undefined && elements.oopsMode) {
                    elements.oopsMode.checked = msg.oops_mode;
                }
                if (msg.error_rate !== undefined) {
                    currentErrorRate = msg.error_rate;
                }
                updateJitterDisplay();
                updateErrorRateDisplay();
                break;

            case 'progress':
                updateProgress(msg.current, msg.total);
                break;

            case 'complete':
                completePunching();
                break;

            case 'error':
                stopPunching(msg.message || 'An error occurred');
                showToast('✕', msg.message || 'Error', 'error');
                if (msg.message && msg.message.toLowerCase().includes('unauthorized')) {
                    updateAuthUI(false);
                }
                break;

            case 'stopped':
                stopPunching('Stopped');
                showToast('■', 'Typing stopped', 'error');
                break;
        }
    }

    function sendMessage(msg) {
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify(msg));
            return true;
        }
        return false;
    }

    function flashSyncHighlight() {
        elements.textareaWrapper.classList.remove('synced-highlight');
        void elements.textareaWrapper.offsetWidth;
        elements.textareaWrapper.classList.add('synced-highlight');
        setTimeout(() => elements.textareaWrapper.classList.remove('synced-highlight'), 1500);
    }

    function sendUpdateCode() {
        if (!isAuthorized) {
            updateAuthUI(false);
            showToast('!', 'Pairing required to update code', 'error');
            return;
        }

        const text = elements.textInput.value;
        const wpm = getSpeed();
        const delaySec = getDelay();
        const preserve = elements.preserveFormatting.checked;
        const smart = elements.smartIndent ? elements.smartIndent.checked : true;
        const paste = elements.pasteMode.checked;
        const jitter = elements.jitterMode ? elements.jitterMode.checked : false;
        const burst = elements.oopsMode ? elements.oopsMode.checked : false;

        const sent = sendMessage({
            type: 'update_code',
            text: text,
            wpm: wpm,
            focus_delay_sec: delaySec,
            preserve_formatting: preserve,
            smart_indent: smart,
            paste_mode: paste,
            jitter_mode: jitter,
            jitter_level: currentJitterLevel,
            oops_mode: burst,
            error_rate: currentErrorRate,
        });

        if (sent) {
            elements.updateBtn.classList.add('updated-pulse');
            setTimeout(() => elements.updateBtn.classList.remove('updated-pulse'), 700);
            showToast('✓', 'Code synced to all devices', 'success');
        } else {
            showToast('✕', 'Failed to sync code', 'error');
        }
    }

    function startPunchFlow() {
        if (!isAuthorized) {
            updateAuthUI(false);
            showToast('🔒', 'Device not paired. Enter PIN from laptop screen', 'error');
            return;
        }

        const text = elements.textInput.value;
        if (!text.trim()) {
            showToast('✍️', 'Please enter or paste some text first', 'warning');
            elements.textInput.focus();
            if (elements.textareaWrapper) {
                elements.textareaWrapper.classList.add('synced-highlight');
                setTimeout(() => elements.textareaWrapper.classList.remove('synced-highlight'), 1200);
            }
            return;
        }
        if (!isConnected) {
            showToast('⚡', 'Reconnecting to Auto Typer...', 'warning');
            connect();
            return;
        }

        const delaySec = getDelay();
        if (delaySec > 0) {
            startCountdown(delaySec, () => sendPunchCommand(text));
        } else {
            sendPunchCommand(text);
        }
    }

    function sendPunchCommand(text) {
        const wpm = getSpeed();
        const delayMs = wpmToDelay(wpm);

        const success = sendMessage({
            type: 'punch',
            text: text,
            delay_ms: delayMs,
            preserve_formatting: elements.preserveFormatting.checked,
            smart_indent: elements.smartIndent ? elements.smartIndent.checked : true,
            paste_mode: elements.pasteMode.checked,
            jitter_mode: elements.jitterMode ? elements.jitterMode.checked : false,
            jitter_level: currentJitterLevel,
            oops_mode: elements.oopsMode ? elements.oopsMode.checked : false,
            error_rate: currentErrorRate,
            wpm: wpm,
        });

        if (success) {
            isPunching = true;
            addToHistory(text);
            showPunchingUI(text.length);
            elements.ambientGlow.classList.add('punching');
            acquireWakeLock();
        } else {
            showToast('!', 'Failed to send — check connection', 'error');
        }
    }

    function stopPunchingRequest() {
        sendMessage({ type: 'stop' });
    }

    function stopPunching(reason) {
        isPunching = false;
        hidePunchingUI();
        elements.ambientGlow.classList.remove('punching');
        releaseWakeLock();
    }

    function completePunching() {
        isPunching = false;
        hidePunchingUI();
        elements.ambientGlow.classList.remove('punching');
        releaseWakeLock();
        showToast('✓', 'Typing complete!', 'success');
        elements.punchBtn.style.background = 'linear-gradient(135deg, #00cec9, #55efc4)';
        setTimeout(() => elements.punchBtn.style.background = '', 1200);
    }

    function showPunchingUI(totalChars) {
        elements.progressSection.classList.remove('hidden');
        elements.punchBtnContent.classList.add('hidden');
        elements.punchBtnLoading.classList.remove('hidden');
        elements.punchBtn.disabled = true;
        elements.updateBtn.disabled = true;
        elements.stopBtn.classList.remove('hidden');
        elements.textInput.disabled = true;
        elements.textInput.style.opacity = '0.5';
        updateProgress(0, totalChars);
    }

    function hidePunchingUI() {
        elements.progressSection.classList.add('hidden');
        elements.punchBtnContent.classList.remove('hidden');
        elements.punchBtnLoading.classList.add('hidden');
        elements.stopBtn.classList.add('hidden');
        elements.textInput.disabled = false;
        elements.textInput.style.opacity = '';
        updatePunchButtonState();
    }

    function updateProgress(current, total) {
        const percent = total > 0 ? Math.round((current / total) * 100) : 0;
        elements.progressPercent.textContent = `${percent}%`;
        elements.progressBarFill.style.width = `${percent}%`;
        elements.progressBarGlow.style.left = `calc(${percent}% - 30px)`;
        elements.progressChars.textContent = `${current} / ${total} chars`;

        const wpm = getSpeed();
        const delayMs = wpmToDelay(wpm);
        const remaining = total - current;
        const etaSeconds = (remaining * delayMs) / 1000;
        elements.progressEta.textContent = formatTime(etaSeconds);

        elements.progressLabel.textContent = percent >= 100 ? 'Complete!' : 'Typing...';
    }

    function updateCharCount() {
        const text = elements.textInput.value;
        const chars = text.length;
        const words = countWords(text);
        elements.charCount.textContent = `${chars} char${chars !== 1 ? 's' : ''} · ${words} word${words !== 1 ? 's' : ''}`;
        updateEstimate();
        updatePunchButtonState();
    }

    function updateTextareaState() {
        elements.textareaWrapper.classList.toggle('has-text', elements.textInput.value.length > 0);
    }

    function updatePunchButtonState() {
        const hasText = elements.textInput.value.trim().length > 0;
        elements.punchBtn.disabled = isPunching;
        elements.punchBtn.classList.toggle('not-ready', !hasText || !isConnected || !isAuthorized);
        elements.updateBtn.disabled = isPunching;
        elements.updateBtn.classList.toggle('not-ready', !isConnected || !isAuthorized);
    }

    function updateSpeedDisplay() {
        const wpm = getSpeed();
        let label = 'Slow';
        if (wpm > 30 && wpm <= 60) label = 'Moderate';
        else if (wpm > 60 && wpm <= 120) label = 'Fast';
        else if (wpm > 120 && wpm <= 180) label = 'Very Fast';
        else if (wpm > 180) label = 'Blazing';

        elements.speedTag.textContent = `${wpm} WPM · ${label}`;

        if (elements.speedPresets) {
            elements.speedPresets.querySelectorAll('.preset-btn').forEach(btn => {
                btn.classList.toggle('active', parseInt(btn.dataset.speed, 10) === wpm);
            });
        }

        updateEstimate();
        saveSettings();
    }

    function updateEstimate() {
        const text = elements.textInput.value;
        if (!text.trim()) {
            elements.estimate.textContent = '';
            return;
        }
        if (elements.pasteMode && elements.pasteMode.checked) {
            elements.estimate.textContent = 'Estimated time: Instant (Clipboard)';
            return;
        }
        const wpm = getSpeed();
        const delayMs = wpmToDelay(wpm);
        const totalMs = text.length * delayMs;
        const seconds = totalMs / 1000;
        let estText = `Estimated time: ${formatTime(seconds)}`;
        if ((elements.jitterMode && elements.jitterMode.checked) || (elements.oopsMode && elements.oopsMode.checked)) {
            estText += ' (Approx, Jitter/Oops active)';
        }
        elements.estimate.textContent = estText;
    }

    function startCountdown(seconds, callback) {
        let remaining = seconds;
        elements.countdownOverlay.classList.remove('hidden');
        elements.countdownNumber.textContent = remaining;

        countdownTimer = setInterval(() => {
            remaining--;
            if (remaining <= 0) {
                clearInterval(countdownTimer);
                countdownTimer = null;
                elements.countdownOverlay.classList.add('hidden');
                callback();
            } else {
                elements.countdownNumber.textContent = remaining;
            }
        }, 1000);
    }

    function cancelCountdown() {
        if (countdownTimer) {
            clearInterval(countdownTimer);
            countdownTimer = null;
        }
        elements.countdownOverlay.classList.add('hidden');
    }

    function showToast(icon, message, type = '') {
        clearTimeout(toastTimer);
        elements.toastIcon.textContent = icon;
        elements.toastMessage.textContent = message;
        elements.toast.className = `toast ${type}`;
        void elements.toast.offsetWidth;
        elements.toast.classList.add('show');
        toastTimer = setTimeout(() => elements.toast.classList.remove('show'), 3000);
    }

    // ─── Event Listeners ─────────────────────────────────────
    function initEventListeners() {
        elements.textInput.addEventListener('input', () => {
            updateCharCount();
            updateTextareaState();
        });

        elements.clearBtn.addEventListener('click', (e) => {
            e.preventDefault();
            elements.textInput.value = '';
            updateCharCount();
            updateTextareaState();
            elements.textInput.focus();
        });

        elements.speedInput.addEventListener('input', updateSpeedDisplay);
        elements.speedInput.addEventListener('change', () => {
            elements.speedInput.value = getSpeed();
            updateSpeedDisplay();
        });

        elements.speedMinus.addEventListener('click', () => {
            elements.speedInput.value = Math.max(10, getSpeed() - 5);
            updateSpeedDisplay();
        });

        elements.speedPlus.addEventListener('click', () => {
            elements.speedInput.value = Math.min(500, getSpeed() + 5);
            updateSpeedDisplay();
        });

        if (elements.speedPresets) {
            elements.speedPresets.addEventListener('click', (e) => {
                const btn = e.target.closest('.preset-btn');
                if (btn && btn.dataset.speed) {
                    elements.speedInput.value = btn.dataset.speed;
                    updateSpeedDisplay();
                }
            });
        }

        elements.delayInput.addEventListener('input', saveSettings);
        elements.delayInput.addEventListener('change', () => {
            elements.delayInput.value = getDelay();
            saveSettings();
        });

        elements.delayMinus.addEventListener('click', () => {
            elements.delayInput.value = Math.max(0, getDelay() - 1);
            saveSettings();
        });

        elements.delayPlus.addEventListener('click', () => {
            elements.delayInput.value = Math.min(60, getDelay() + 1);
            saveSettings();
        });

        elements.preserveFormatting.addEventListener('change', saveSettings);
        if (elements.smartIndent) elements.smartIndent.addEventListener('change', saveSettings);
        if (elements.pasteMode) {
            elements.pasteMode.addEventListener('change', () => {
                saveSettings();
                updateEstimate();
            });
        }

        // Jitter Mode toggles & presets
        if (elements.jitterMode) {
            elements.jitterMode.addEventListener('change', () => {
                updateJitterDisplay();
            });
        }

        if (elements.jitterPresetGroup) {
            elements.jitterPresetGroup.addEventListener('click', (e) => {
                const btn = e.target.closest('.jitter-level-btn');
                if (btn && btn.dataset.level) {
                    updateJitterDisplay(parseInt(btn.dataset.level, 10));
                }
            });
        }

        // Oops Mode toggles, steppers & presets
        if (elements.oopsMode) {
            elements.oopsMode.addEventListener('change', () => {
                updateErrorRateDisplay();
            });
        }

        if (elements.errorMinus) {
            elements.errorMinus.addEventListener('click', () => {
                updateErrorRateDisplay(currentErrorRate - 1);
            });
        }

        if (elements.errorPlus) {
            elements.errorPlus.addEventListener('click', () => {
                updateErrorRateDisplay(currentErrorRate + 1);
            });
        }

        if (elements.errorRateInput) {
            elements.errorRateInput.addEventListener('change', () => {
                const val = parseInt(elements.errorRateInput.value, 10);
                if (!isNaN(val)) updateErrorRateDisplay(val);
            });
        }

        if (elements.errorPresetGroup) {
            elements.errorPresetGroup.addEventListener('click', (e) => {
                const btn = e.target.closest('.error-preset-btn');
                if (btn && btn.dataset.rate) {
                    updateErrorRateDisplay(parseInt(btn.dataset.rate, 10));
                }
            });
        }

        elements.updateBtn.addEventListener('click', sendUpdateCode);
        elements.punchBtn.addEventListener('click', startPunchFlow);
        elements.stopBtn.addEventListener('click', stopPunchingRequest);
        elements.countdownCancel.addEventListener('click', cancelCountdown);

        // PIN pairing button
        if (elements.pinSubmitBtn && elements.pinInput) {
            elements.pinSubmitBtn.addEventListener('click', () => {
                const pin = elements.pinInput.value.trim();
                if (pin.length < 6) {
                    showToast('!', 'Please enter the 6-digit PIN', 'error');
                    return;
                }
                localStorage.setItem(TOKEN_KEY, pin);
                if (ws && ws.readyState === WebSocket.OPEN) {
                    sendMessage({ type: 'auth', token: pin });
                } else {
                    connect();
                }
            });

            elements.pinInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') {
                    elements.pinSubmitBtn.click();
                }
            });
        }

        elements.clearHistoryBtn.addEventListener('click', () => {
            localStorage.removeItem(HISTORY_KEY);
            renderHistory();
            showToast('✓', 'History cleared', 'success');
        });

        document.addEventListener('visibilitychange', () => {
            if (!document.hidden && !isConnected) connect();
        });

        let lastTouchEnd = 0;
        document.addEventListener('touchend', (e) => {
            const now = Date.now();
            if (now - lastTouchEnd <= 300) e.preventDefault();
            lastTouchEnd = now;
        }, { passive: false });
    }

    function init() {
        loadSettings();
        updateSpeedDisplay();
        updateCharCount();
        updateTextareaState();
        updateJitterDisplay();
        updateErrorRateDisplay();
        renderHistory();
        initEventListeners();
        connect();

        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                e.preventDefault();
                if (!elements.punchBtn.disabled) startPunchFlow();
            } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
                e.preventDefault();
                if (!elements.updateBtn.disabled) sendUpdateCode();
            }
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
