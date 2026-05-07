// Admin workspace Fast / Update mode toggle and persistent collapsibles.
// Controlled via <body data-mode="fast|update"> and per-section data-section-state.
// Persistence: localStorage keys
//   as.flowershow.workspace.mode
//   as.flowershow.section.<name>

(function () {
  'use strict';

  var MODE_KEY = 'as.flowershow.workspace.mode';
  var SECTION_KEY_PREFIX = 'as.flowershow.section.';
  var DEFAULT_MODE = 'fast';
  var SECTION_NAMES = ['name_match', 'ranking', 'comment', 'photo_detail'];

  function safeGet(key) {
    try { return window.localStorage.getItem(key); } catch (e) { return null; }
  }
  function safeSet(key, value) {
    try { window.localStorage.setItem(key, value); } catch (e) { /* ignore */ }
  }
  function safeRemove(key) {
    try { window.localStorage.removeItem(key); } catch (e) { /* ignore */ }
  }

  function readMode() {
    var stored = safeGet(MODE_KEY);
    if (stored === 'fast' || stored === 'update') return stored;
    return DEFAULT_MODE;
  }

  function applyMode(mode) {
    if (!document.body) return;
    document.body.setAttribute('data-mode', mode);
    document.querySelectorAll('[data-mode-toggle]').forEach(function (root) {
      root.querySelectorAll('[data-mode-toggle-button]').forEach(function (btn) {
        var btnMode = btn.getAttribute('data-mode-toggle-button');
        var isActive = btnMode === mode;
        btn.classList.toggle('is-active', isActive);
        btn.setAttribute('aria-pressed', isActive ? 'true' : 'false');
      });
    });
  }

  function readSectionState(section) {
    var stored = safeGet(SECTION_KEY_PREFIX + section);
    if (stored === 'open' || stored === 'closed') return stored;
    return 'closed';
  }

  function applySectionState(section, state) {
    document.querySelectorAll('[data-entry-section="' + section + '"]').forEach(function (panel) {
      panel.setAttribute('data-section-state', state);
      var details = panel.tagName.toLowerCase() === 'details' ? panel : null;
      if (details) {
        if (state === 'open') details.setAttribute('open', '');
        else details.removeAttribute('open');
      }
    });
    document.querySelectorAll('[data-entry-section-toggle="' + section + '"]').forEach(function (btn) {
      btn.setAttribute('aria-expanded', state === 'open' ? 'true' : 'false');
      btn.classList.toggle('is-open', state === 'open');
    });
  }

  function bindModeToggle(root) {
    if (!root || root.dataset.modeToggleBound === 'true') return;
    root.dataset.modeToggleBound = 'true';
    root.querySelectorAll('[data-mode-toggle-button]').forEach(function (btn) {
      btn.addEventListener('click', function (event) {
        event.preventDefault();
        var mode = btn.getAttribute('data-mode-toggle-button');
        if (mode !== 'fast' && mode !== 'update') return;
        safeSet(MODE_KEY, mode);
        applyMode(mode);
      });
    });
    var resetLink = root.querySelector('[data-mode-toggle-reset]');
    if (resetLink) {
      resetLink.addEventListener('click', function (event) {
        event.preventDefault();
        safeRemove(MODE_KEY);
        SECTION_NAMES.forEach(function (n) { safeRemove(SECTION_KEY_PREFIX + n); });
        applyMode(DEFAULT_MODE);
        SECTION_NAMES.forEach(function (n) { applySectionState(n, 'closed'); });
      });
    }
  }

  function bindSectionToggle(btn) {
    if (!btn || btn.dataset.sectionToggleBound === 'true') return;
    btn.dataset.sectionToggleBound = 'true';
    btn.addEventListener('click', function (event) {
      event.preventDefault();
      var section = btn.getAttribute('data-entry-section-toggle');
      if (!section) return;
      var current = readSectionState(section);
      var next = current === 'open' ? 'closed' : 'open';
      safeSet(SECTION_KEY_PREFIX + section, next);
      applySectionState(section, next);
    });
  }

  function bindDetailsListener(panel) {
    if (!panel || panel.dataset.sectionDetailsBound === 'true') return;
    if (panel.tagName.toLowerCase() !== 'details') return;
    panel.dataset.sectionDetailsBound = 'true';
    panel.addEventListener('toggle', function () {
      var section = panel.getAttribute('data-entry-section');
      if (!section) return;
      var state = panel.open ? 'open' : 'closed';
      safeSet(SECTION_KEY_PREFIX + section, state);
      applySectionState(section, state);
    });
  }

  function refreshAll() {
    applyMode(readMode());
    SECTION_NAMES.forEach(function (n) { applySectionState(n, readSectionState(n)); });
    document.querySelectorAll('[data-mode-toggle]').forEach(bindModeToggle);
    document.querySelectorAll('[data-entry-section-toggle]').forEach(bindSectionToggle);
    document.querySelectorAll('details[data-entry-section]').forEach(bindDetailsListener);
  }

  function init() {
    refreshAll();
    document.body.addEventListener('htmx:afterSwap', refreshAll);
    document.body.addEventListener('htmx:afterSettle', refreshAll);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  window.flowershowAdminModeRefresh = refreshAll;
})();
