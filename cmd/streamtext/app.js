(function () {
    var editor = document.getElementById('editor');
    var editorShell = document.getElementById('editorShell');
    var preview = document.getElementById('preview');
    var toggleBtn = document.getElementById('toggleBtn');
    var editModeBtn = document.getElementById('editModeBtn');
    var previewModeBtn = document.getElementById('previewModeBtn');
    var formatToolbar = document.getElementById('formatToolbar');
    var saveBarMobile = document.getElementById('saveBarMobile');
    var autosaveBtn = document.getElementById('autosaveBtn');
    var refreshTriggers = document.querySelectorAll('[data-refresh-trigger]');
    var saveHintDesktop = document.getElementById('saveHintDesktop');
    var saveHintMobile = document.getElementById('saveHintMobile');
    var dot = document.getElementById('dot');
    var statusText = document.getElementById('statusText');
    var toast = document.getElementById('toast');
    var showingPreview = false;

    var desktopMq = window.matchMedia('(min-width: 769px)');
    var autosaveDelayMs = 800;
    var autosaveTimer = null;
    var hintTimer = null;
    var autosaveStorageKey = 'streamtext_autosave';
    var autosaveEnabled = localStorage.getItem(autosaveStorageKey) === '1';
    var indentUnit = '  ';

    if (typeof window.__INITIAL_DATA__ === 'string') {
        editor.value = window.__INITIAL_DATA__;
    }

    /** Mobile: native undo on textarea is unreliable; keep snapshot stack. Desktop uses execCommand. */
    var undoHistory = [];
    var redoHistory = [];
    var undoMax = 150;
    var undoRestoring = false;

    function editorSnapshot() {
        return {
            v: editor.value,
            ss: editor.selectionStart,
            se: editor.selectionEnd,
        };
    }

    function pushUndoHistory() {
        if (isDesktop() || undoRestoring) {
            return;
        }
        var snap = editorSnapshot();
        var last = undoHistory[undoHistory.length - 1];
        if (last && last.v === snap.v && last.ss === snap.ss && last.se === snap.se) {
            return;
        }
        undoHistory.push(snap);
        if (undoHistory.length > undoMax) {
            undoHistory.shift();
        }
        redoHistory.length = 0;
    }

    function resetUndoHistory() {
        if (isDesktop()) {
            return;
        }
        undoHistory.length = 0;
        redoHistory.length = 0;
        undoHistory.push(editorSnapshot());
    }

    function undoMobile() {
        if (undoHistory.length < 2) {
            return;
        }
        redoHistory.push(undoHistory.pop());
        var prev = undoHistory[undoHistory.length - 1];
        undoRestoring = true;
        editor.value = prev.v;
        editor.selectionStart = prev.ss;
        editor.selectionEnd = prev.se;
        undoRestoring = false;
        scheduleAutosave();
        scheduleCaretScroll();
    }

    function redoMobile() {
        if (redoHistory.length === 0) {
            return;
        }
        var snap = redoHistory.pop();
        undoRestoring = true;
        editor.value = snap.v;
        editor.selectionStart = snap.ss;
        editor.selectionEnd = snap.se;
        undoRestoring = false;
        undoHistory.push(snap);
        scheduleAutosave();
        scheduleCaretScroll();
    }

    function isDesktop() {
        return desktopMq.matches;
    }

    function setSaveHint(msg) {
        saveHintDesktop.textContent = msg;
        saveHintMobile.textContent = msg;
    }

    var caretScrollQueued = false;
    var caretMeasureEl = null;

    /**
     * Wrap-aware caret position via mirror; only changes scrollTop when the caret
     * would be clipped (avoids resetting scroll on every keystroke / selection tick).
     */
    function scrollCaretIntoViewMobile() {
        if (document.activeElement !== editor) {
            return;
        }
        if (editor.selectionStart !== editor.selectionEnd) {
            return;
        }
        var pos = editor.selectionStart;
        var style = getComputedStyle(editor);
        if (!caretMeasureEl) {
            caretMeasureEl = document.createElement('div');
            document.body.appendChild(caretMeasureEl);
        }
        var m = caretMeasureEl;
        var w = editor.clientWidth;
        if (w < 8) {
            return;
        }
        m.style.cssText =
            'position:absolute;left:-99999px;top:0;visibility:hidden;overflow:hidden;' +
            'box-sizing:border-box;margin:0;border:none;' +
            'width:' +
            w +
            'px;' +
            'padding:' +
            style.paddingTop +
            ' ' +
            style.paddingRight +
            ' 0 ' +
            style.paddingLeft +
            ';' +
            'font:' +
            style.font +
            ';' +
            'line-height:' +
            style.lineHeight +
            ';' +
            'letter-spacing:' +
            style.letterSpacing +
            ';' +
            'white-space:pre-wrap;word-wrap:break-word;word-break:break-word;';
        if (!style.font || style.font.length < 10) {
            m.style.fontSize = style.fontSize;
            m.style.fontFamily = style.fontFamily;
            m.style.fontWeight = style.fontWeight;
            m.style.fontStyle = style.fontStyle;
        }
        m.textContent = editor.value.slice(0, pos);
        var caretBottom = m.scrollHeight;
        var innerH = editor.clientHeight;
        if (innerH < 8) {
            return;
        }
        var lh = parseFloat(style.lineHeight);
        if (!(lh > 0)) {
            var fs = parseFloat(style.fontSize);
            lh = (fs > 0 ? fs : 16) * 1.25;
        }
        var caretTop = Math.max(0, caretBottom - lh);
        var st = editor.scrollTop;
        var maxScroll = Math.max(0, editor.scrollHeight - innerH);
        var marginTop = Math.min(innerH * 0.08, 48);
        var marginBottom = Math.min(innerH * 0.12, 72);
        var visTop = st + marginTop;
        var visBottom = st + innerH - marginBottom;
        var clippedAbove = caretTop < visTop;
        var clippedBelow = caretBottom > visBottom;
        if (!clippedAbove && !clippedBelow) {
            return;
        }
        var next = st;
        if (clippedBelow && (!clippedAbove || caretBottom - visBottom > visTop - caretTop)) {
            next = caretBottom - innerH + marginBottom;
        } else if (clippedAbove) {
            next = caretTop - marginTop;
        }
        editor.scrollTop = Math.max(0, Math.min(maxScroll, next));
    }

    function scheduleCaretScroll() {
        if (isDesktop()) {
            return;
        }
        if (caretScrollQueued) {
            return;
        }
        caretScrollQueued = true;
        requestAnimationFrame(function () {
            caretScrollQueued = false;
            scrollCaretIntoViewMobile();
        });
    }

    /** Lift fixed save + format bar with the OS keyboard (Visual Viewport API). */
    var mobileDockOverlapApplied = -1;
    var mobileDockRaf = 0;

    function syncMobileDockKeyboard() {
        if (mobileDockRaf) {
            cancelAnimationFrame(mobileDockRaf);
        }
        mobileDockRaf = requestAnimationFrame(function () {
            mobileDockRaf = 0;
            if (isDesktop() || !window.visualViewport) {
                if (mobileDockOverlapApplied !== 0) {
                    mobileDockOverlapApplied = 0;
                    formatToolbar.style.transform = '';
                    saveBarMobile.style.transform = '';
                }
                return;
            }
            var vv = window.visualViewport;
            var overlap = window.innerHeight - vv.height - vv.offsetTop;
            if (overlap < 0) {
                overlap = 0;
            }
            overlap = Math.round(overlap);
            if (overlap === mobileDockOverlapApplied) {
                return;
            }
            mobileDockOverlapApplied = overlap;
            var tr = overlap > 0 ? 'translateY(-' + overlap + 'px)' : '';
            formatToolbar.style.transform = tr;
            saveBarMobile.style.transform = tr;
        });
    }

    /** Keep textarea focused so the virtual keyboard stays up (mobile). */
    function preventChromeFromStealingFocus(ev) {
        if (isDesktop()) {
            return;
        }
        if (ev.type === 'mousedown' && ev.button !== 0) {
            return;
        }
        ev.preventDefault();
    }

    saveBarMobile.addEventListener('mousedown', preventChromeFromStealingFocus, true);
    saveBarMobile.addEventListener('pointerdown', preventChromeFromStealingFocus, true);
    formatToolbar.addEventListener('mousedown', preventChromeFromStealingFocus, true);
    formatToolbar.addEventListener('pointerdown', preventChromeFromStealingFocus, true);

    function getLineCol(v, abs) {
        var ls = v.lastIndexOf('\n', abs - 1) + 1;
        var li = (v.slice(0, abs).match(/\n/g) || []).length;
        var col = abs - ls;
        return { li: li, col: col };
    }

    function posFromLineCol(v, li, col) {
        var parts = v.split('\n');
        if (parts.length === 0) {
            return 0;
        }
        if (li < 0) {
            li = 0;
        }
        if (li >= parts.length) {
            li = parts.length - 1;
        }
        var p = 0;
        for (var i = 0; i < li; i++) {
            p += parts[i].length + 1;
        }
        return p + Math.min(col, parts[li].length);
    }

    /** Inclusive line block [ls, le) for indent / outdent / line prefixes. */
    function getAffectedLineRange(start, end) {
        var v = editor.value;
        var from = Math.min(start, end);
        var to = Math.max(start, end);
        var ls = v.lastIndexOf('\n', from - 1) + 1;
        var le;
        if (to > from) {
            le = v.indexOf('\n', to - 1);
            if (le < 0) {
                le = v.length;
            } else {
                le += 1;
            }
        } else {
            le = v.indexOf('\n', from);
            if (le < 0) {
                le = v.length;
            } else {
                le += 1;
            }
        }
        return { ls: ls, le: le };
    }

    function syncAutosaveUI() {
        autosaveBtn.classList.toggle('active', autosaveEnabled);
        autosaveBtn.setAttribute('aria-pressed', autosaveEnabled ? 'true' : 'false');
        autosaveBtn.textContent = autosaveEnabled ? 'Autosave on' : 'Autosave';
    }
    syncAutosaveUI();

    function toggleAutosave() {
        autosaveEnabled = !autosaveEnabled;
        localStorage.setItem(autosaveStorageKey, autosaveEnabled ? '1' : '0');
        syncAutosaveUI();
        if (!autosaveEnabled && autosaveTimer) {
            clearTimeout(autosaveTimer);
            autosaveTimer = null;
        }
    }
    window.toggleAutosave = toggleAutosave;

    var refreshInFlight = false;

    function refreshFromServer() {
        if (refreshInFlight) {
            return;
        }
        refreshInFlight = true;
        refreshTriggers.forEach(function (btn) {
            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');
        });
        fetch('/content', { cache: 'no-store' })
            .then(function (r) {
                if (!r.ok) {
                    return r.text().then(function (t) {
                        throw new Error(t.trim() || 'HTTP ' + r.status);
                    });
                }
                return r.text();
            })
            .then(function (text) {
                editor.value = text;
                if (showingPreview && typeof marked !== 'undefined') {
                    preview.innerHTML = marked.parse(text, { breaks: true, gfm: true });
                }
                resetUndoHistory();
                if (!isDesktop()) {
                    scheduleCaretScroll();
                }
                setSaveHint('');
                showToast('Reloaded');
            })
            .catch(function (err) {
                setSaveHint(err.message || 'Reload failed');
                showToast('Reload failed');
            })
            .finally(function () {
                refreshInFlight = false;
                refreshTriggers.forEach(function (btn) {
                    btn.disabled = false;
                    btn.removeAttribute('aria-busy');
                });
            });
    }
    window.refreshFromServer = refreshFromServer;

    function scheduleAutosave() {
        if (!autosaveEnabled) {
            return;
        }
        if (autosaveTimer) {
            clearTimeout(autosaveTimer);
        }
        autosaveTimer = setTimeout(function () {
            autosaveTimer = null;
            saveToServer({ silent: true });
        }, autosaveDelayMs);
    }

    editor.addEventListener('input', function () {
        scheduleAutosave();
        if (!isDesktop()) {
            pushUndoHistory();
        }
        scheduleCaretScroll();
    });
    editor.addEventListener('keyup', function (e) {
        if (isDesktop()) {
            return;
        }
        var k = e.key;
        if (
            k === 'ArrowUp' ||
            k === 'ArrowDown' ||
            k === 'ArrowLeft' ||
            k === 'ArrowRight' ||
            k === 'PageUp' ||
            k === 'PageDown' ||
            k === 'Home' ||
            k === 'End'
        ) {
            scheduleCaretScroll();
        }
    });
    editor.addEventListener('click', function () {
        scheduleCaretScroll();
    });
    editor.addEventListener('focus', function () {
        if (!isDesktop()) {
            scheduleCaretScroll();
        }
    });

    function syncModeButtons() {
        toggleBtn.textContent = showingPreview ? 'Edit' : 'Preview';
        toggleBtn.classList.toggle('active', showingPreview);
        editModeBtn.classList.toggle('active', !showingPreview);
        previewModeBtn.classList.toggle('active', showingPreview);
        editModeBtn.setAttribute('aria-pressed', (!showingPreview).toString());
        previewModeBtn.setAttribute('aria-pressed', showingPreview.toString());
    }

    function setMode(isPreview) {
        showingPreview = isPreview;
        document.body.classList.toggle('is-preview', showingPreview);
        if (showingPreview) {
            preview.innerHTML = marked.parse(editor.value, { breaks: true, gfm: true });
            preview.classList.add('visible');
            editorShell.style.display = 'none';
        } else {
            preview.classList.remove('visible');
            editorShell.style.display = '';
            editor.focus();
        }
        syncModeButtons();
        syncMobileDockKeyboard();
    }

    function togglePreview() {
        setMode(!showingPreview);
    }
    window.togglePreview = togglePreview;

    function setEditorMode() {
        setMode(false);
    }
    window.setEditorMode = setEditorMode;

    function setPreviewMode() {
        setMode(true);
    }
    window.setPreviewMode = setPreviewMode;

    function showToast(msg) {
        toast.textContent = msg;
        toast.classList.add('show');
        setTimeout(function () {
            toast.classList.remove('show');
        }, 2000);
    }

    function refocusEditor(ss, se) {
        requestAnimationFrame(function () {
            editor.focus();
            if (typeof ss === 'number' && typeof se === 'number') {
                var max = editor.value.length;
                editor.selectionStart = Math.min(ss, max);
                editor.selectionEnd = Math.min(se, max);
            }
            if (!isDesktop()) {
                scheduleCaretScroll();
            }
        });
    }

    function saveToServer(opts) {
        opts = opts || {};
        var silent = !!opts.silent;
        var ss = editor.selectionStart;
        var se = editor.selectionEnd;
        var refocusMobile = !silent && !isDesktop();
        fetch('/save', {
            method: 'POST',
            headers: { 'Content-Type': 'text/plain' },
            body: editor.value,
        })
            .then(function (r) {
                if (r.ok) {
                    if (silent) {
                        clearTimeout(hintTimer);
                        setSaveHint('Autosaved');
                        hintTimer = setTimeout(function () {
                            setSaveHint('');
                        }, 2500);
                    } else {
                        setSaveHint('');
                        showToast('Saved');
                        if (refocusMobile) {
                            refocusEditor(ss, se);
                        }
                    }
                } else {
                    showToast('Save failed');
                    if (refocusMobile) {
                        refocusEditor(ss, se);
                    }
                }
            })
            .catch(function () {
                showToast('Save failed');
                if (refocusMobile) {
                    refocusEditor(ss, se);
                }
            });
    }
    window.saveToServer = saveToServer;

    document.addEventListener(
        'keydown',
        function (e) {
            if (!isDesktop()) {
                return;
            }
            if (!e.ctrlKey && !e.metaKey) {
                return;
            }
            if (e.key !== 's' && e.key !== 'S') {
                return;
            }
            e.preventDefault();
            saveToServer();
        },
        true
    );

    function insertAtCursor(text) {
        var start = editor.selectionStart;
        var end = editor.selectionEnd;
        var v = editor.value;
        editor.value = v.slice(0, start) + text + v.slice(end);
        editor.selectionStart = editor.selectionEnd = start + text.length;
        scheduleAutosave();
        if (!isDesktop()) {
            pushUndoHistory();
            scheduleCaretScroll();
        }
    }

    function insertFenceBlock() {
        var start = editor.selectionStart;
        var v = editor.value;
        var prefix = start > 0 && v.charAt(start - 1) !== '\n' ? '\n' : '';
        var ins = prefix + '```\n\n```';
        editor.value = v.slice(0, start) + ins + v.slice(start);
        editor.selectionStart = editor.selectionEnd = start + prefix.length + 4;
        scheduleAutosave();
        if (!isDesktop()) {
            pushUndoHistory();
            scheduleCaretScroll();
        }
    }

    function wrapSelection(before, after) {
        var start = editor.selectionStart;
        var end = editor.selectionEnd;
        var v = editor.value;
        var sel = v.slice(start, end);
        editor.value = v.slice(0, start) + before + sel + after + v.slice(end);
        if (sel.length) {
            editor.selectionStart = start + before.length;
            editor.selectionEnd = start + before.length + sel.length;
        } else {
            editor.selectionStart = editor.selectionEnd = start + before.length;
        }
        scheduleAutosave();
        if (!isDesktop()) {
            pushUndoHistory();
            refocusEditor(editor.selectionStart, editor.selectionEnd);
        }
    }

    function prefixSelectedLines(prefix) {
        var start = editor.selectionStart;
        var end = editor.selectionEnd;
        var v = editor.value;
        var cStart = getLineCol(v, start);
        var cEnd = getLineCol(v, end);
        var pl = prefix.length;
        var r = getAffectedLineRange(start, end);
        var ls = r.ls;
        var le = r.le;
        var block = v.slice(ls, le);
        var lines = block.split('\n');
        for (var i = 0; i < lines.length; i++) {
            lines[i] = prefix + lines[i];
        }
        var nb = lines.join('\n');
        editor.value = v.slice(0, ls) + nb + v.slice(le);
        var v2 = editor.value;
        var firstLi = (v.slice(0, ls).match(/\n/g) || []).length;
        var lastLi = firstLi + lines.length - 1;
        var ns = cStart.li >= firstLi && cStart.li <= lastLi ? cStart.col + pl : cStart.col;
        var ne = cEnd.li >= firstLi && cEnd.li <= lastLi ? cEnd.col + pl : cEnd.col;
        editor.selectionStart = posFromLineCol(v2, cStart.li, ns);
        editor.selectionEnd = posFromLineCol(v2, cEnd.li, ne);
        scheduleAutosave();
        if (!isDesktop()) {
            pushUndoHistory();
            refocusEditor(editor.selectionStart, editor.selectionEnd);
        }
    }

    function indentSelectedLines() {
        var start = editor.selectionStart;
        var end = editor.selectionEnd;
        var v = editor.value;
        var cStart = getLineCol(v, start);
        var cEnd = getLineCol(v, end);
        var r = getAffectedLineRange(start, end);
        var ls = r.ls;
        var le = r.le;
        var block = v.slice(ls, le);
        var lines = block.split('\n');
        var d = indentUnit.length;
        var firstLi = (v.slice(0, ls).match(/\n/g) || []).length;
        var lastLi = firstLi + lines.length - 1;
        for (var i = 0; i < lines.length; i++) {
            lines[i] = indentUnit + lines[i];
        }
        var nb = lines.join('\n');
        editor.value = v.slice(0, ls) + nb + v.slice(le);
        var v2 = editor.value;
        var ns = cStart.li >= firstLi && cStart.li <= lastLi ? cStart.col + d : cStart.col;
        var ne = cEnd.li >= firstLi && cEnd.li <= lastLi ? cEnd.col + d : cEnd.col;
        editor.selectionStart = posFromLineCol(v2, cStart.li, ns);
        editor.selectionEnd = posFromLineCol(v2, cEnd.li, ne);
        scheduleAutosave();
        if (!isDesktop()) {
            pushUndoHistory();
            refocusEditor(editor.selectionStart, editor.selectionEnd);
        }
    }

    function outdentWidth(line) {
        if (!line.length) {
            return 0;
        }
        if (line.charCodeAt(0) === 9) {
            return 1;
        }
        if (line.startsWith(indentUnit)) {
            return indentUnit.length;
        }
        if (line.charAt(0) === ' ') {
            return 1;
        }
        return 0;
    }

    function outdentSelectedLines() {
        var start = editor.selectionStart;
        var end = editor.selectionEnd;
        var v = editor.value;
        var cStart = getLineCol(v, start);
        var cEnd = getLineCol(v, end);
        var linesBefore = v.split('\n');
        var rs = outdentWidth(linesBefore[cStart.li] || '');
        var re = outdentWidth(linesBefore[cEnd.li] || '');
        var r = getAffectedLineRange(start, end);
        var ls = r.ls;
        var le = r.le;
        var block = v.slice(ls, le);
        var lines = block.split('\n');
        for (var i = 0; i < lines.length; i++) {
            var line = lines[i];
            var rm = 0;
            if (line.charCodeAt(0) === 9) {
                rm = 1;
            } else if (line.startsWith(indentUnit)) {
                rm = indentUnit.length;
            } else if (line.charAt(0) === ' ') {
                rm = 1;
            }
            lines[i] = line.slice(rm);
        }
        var nb = lines.join('\n');
        editor.value = v.slice(0, ls) + nb + v.slice(le);
        var v2 = editor.value;
        editor.selectionStart = posFromLineCol(v2, cStart.li, Math.max(0, cStart.col - rs));
        editor.selectionEnd = posFromLineCol(v2, cEnd.li, Math.max(0, cEnd.col - re));
        scheduleAutosave();
        if (!isDesktop()) {
            pushUndoHistory();
            refocusEditor(editor.selectionStart, editor.selectionEnd);
        }
    }

    function handleEnterList(e) {
        var pos = editor.selectionStart;
        var selEnd = editor.selectionEnd;
        if (pos !== selEnd) {
            return false;
        }
        var v = editor.value;
        var lineStart = v.lastIndexOf('\n', pos - 1) + 1;
        var lineEnd = v.indexOf('\n', pos);
        if (lineEnd < 0) {
            lineEnd = v.length;
        }
        if (pos !== lineEnd) {
            return false;
        }

        var line = v.slice(lineStart, lineEnd);
        var emptyUl = line.match(/^(\s*)([-*+])\s*$/);
        if (emptyUl) {
            e.preventDefault();
            var ind = emptyUl[1];
            editor.value = v.slice(0, lineStart) + ind + v.slice(lineEnd);
            editor.selectionStart = editor.selectionEnd = lineStart + ind.length;
            scheduleAutosave();
            if (!isDesktop()) {
                pushUndoHistory();
            }
            return true;
        }
        var emptyOl = line.match(/^(\s*)(\d+)\.\s*$/);
        if (emptyOl) {
            e.preventDefault();
            var ind2 = emptyOl[1];
            editor.value = v.slice(0, lineStart) + ind2 + v.slice(lineEnd);
            editor.selectionStart = editor.selectionEnd = lineStart + ind2.length;
            scheduleAutosave();
            if (!isDesktop()) {
                pushUndoHistory();
            }
            return true;
        }

        var ul = line.match(/^(\s*)([-*+])(\s+)/);
        if (ul) {
            e.preventDefault();
            var ins = '\n' + ul[1] + ul[2] + ul[3];
            editor.value = v.slice(0, pos) + ins + v.slice(pos);
            editor.selectionStart = editor.selectionEnd = pos + ins.length;
            scheduleAutosave();
            if (!isDesktop()) {
                pushUndoHistory();
            }
            return true;
        }
        var ol = line.match(/^(\s*)(\d+)\.(\s+)/);
        if (ol) {
            e.preventDefault();
            var n = parseInt(ol[2], 10) + 1;
            var ins2 = '\n' + ol[1] + n + '.' + ol[3];
            editor.value = v.slice(0, pos) + ins2 + v.slice(pos);
            editor.selectionStart = editor.selectionEnd = pos + ins2.length;
            scheduleAutosave();
            if (!isDesktop()) {
                pushUndoHistory();
            }
            return true;
        }
        return false;
    }

    function applyFormat(kind) {
        if (showingPreview) {
            return;
        }
        switch (kind) {
            case 'undo':
                editor.focus();
                if (isDesktop()) {
                    try {
                        document.execCommand('undo');
                    } catch (err) {
                        void err;
                    }
                    scheduleAutosave();
                } else {
                    undoMobile();
                }
                break;
            case 'redo':
                editor.focus();
                if (isDesktop()) {
                    try {
                        document.execCommand('redo');
                    } catch (err) {
                        void err;
                    }
                    scheduleAutosave();
                } else {
                    redoMobile();
                }
                break;
            case 'bold':
                wrapSelection('**', '**');
                break;
            case 'italic':
                wrapSelection('*', '*');
                break;
            case 'code':
                wrapSelection('`', '`');
                break;
            case 'h1':
                prefixSelectedLines('# ');
                break;
            case 'h2':
                prefixSelectedLines('## ');
                break;
            case 'h3':
                prefixSelectedLines('### ');
                break;
            case 'bullet':
                prefixSelectedLines('- ');
                break;
            case 'ordered':
                prefixSelectedLines('1. ');
                break;
            case 'quote':
                prefixSelectedLines('> ');
                break;
            case 'indent':
                indentSelectedLines();
                break;
            case 'outdent':
                outdentSelectedLines();
                break;
            case 'link':
                wrapSelection('[ ] ', '');
                break;
            case 'strike':
                wrapSelection('~~', '~~');
                break;
            case 'hr':
                if (editor.selectionStart === 0) {
                    insertAtCursor('---\n');
                } else {
                    insertAtCursor('\n---\n');
                }
                break;
            case 'codeblock':
                insertFenceBlock();
                break;
            default:
                break;
        }
        editor.focus();
        if (!isDesktop()) {
            scheduleCaretScroll();
        }
    }

    formatToolbar.addEventListener('click', function (e) {
        var t = e.target.closest('[data-fmt]');
        if (!t || !formatToolbar.contains(t)) {
            return;
        }
        applyFormat(t.getAttribute('data-fmt'));
    });

    editor.addEventListener('keydown', function (e) {
        if (showingPreview) {
            return;
        }
        if (!isDesktop() && (e.metaKey || e.ctrlKey) && !e.altKey) {
            var k = e.key.toLowerCase();
            if (k === 'z') {
                e.preventDefault();
                if (e.shiftKey) {
                    redoMobile();
                } else {
                    undoMobile();
                }
                return;
            }
            if (k === 'y' && e.ctrlKey) {
                e.preventDefault();
                redoMobile();
                return;
            }
        }
        if (e.key === 'Tab') {
            e.preventDefault();
            if (e.shiftKey) {
                outdentSelectedLines();
            } else {
                indentSelectedLines();
            }
            return;
        }
        if (e.key === 'Enter') {
            if (handleEnterList(e)) {
                return;
            }
        }
        if (e.ctrlKey || e.metaKey) {
            if (e.key === 'b' || e.key === 'B') {
                e.preventDefault();
                wrapSelection('**', '**');
            } else if (e.key === 'i' || e.key === 'I') {
                e.preventDefault();
                wrapSelection('*', '*');
            }
        }
    });

    var scheme = location.protocol === 'https:' ? 'wss:' : 'ws:';
    var conn = new WebSocket(scheme + '//' + location.host + '/ws');

    conn.onopen = function () {
        dot.classList.remove('offline');
        statusText.textContent = 'live';
    };
    conn.onclose = function () {
        dot.classList.add('offline');
        statusText.textContent = 'disconnected';
    };
    conn.onmessage = function (evt) {
        editor.value = evt.data;
        if (showingPreview) {
            preview.innerHTML = marked.parse(evt.data, { breaks: true, gfm: true });
        } else if (!isDesktop()) {
            resetUndoHistory();
            scheduleCaretScroll();
        }
    };

    resetUndoHistory();
    syncModeButtons();

    if (window.visualViewport) {
        window.visualViewport.addEventListener('resize', syncMobileDockKeyboard, { passive: true });
        window.visualViewport.addEventListener('scroll', syncMobileDockKeyboard, { passive: true });
    }
    window.addEventListener('resize', syncMobileDockKeyboard, { passive: true });
    if (typeof desktopMq.addEventListener === 'function') {
        desktopMq.addEventListener('change', syncMobileDockKeyboard);
    } else if (desktopMq.addListener) {
        desktopMq.addListener(syncMobileDockKeyboard);
    }
    editor.addEventListener('focus', syncMobileDockKeyboard);
    editor.addEventListener('blur', function () {
        requestAnimationFrame(syncMobileDockKeyboard);
    });
    syncMobileDockKeyboard();
})();
