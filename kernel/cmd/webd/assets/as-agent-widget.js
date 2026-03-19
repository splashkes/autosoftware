(function() {
  function activateAgentTab(widget, tabName) {
    if (!widget || !tabName) return;
    widget.querySelectorAll('[data-agent-tab-trigger]').forEach(function(button) {
      var active = button.getAttribute('data-agent-tab-trigger') === tabName;
      button.classList.toggle('is-active', active);
      button.setAttribute('aria-selected', active ? 'true' : 'false');
    });
    widget.querySelectorAll('[data-agent-tab-panel]').forEach(function(panel) {
      var active = panel.getAttribute('data-agent-tab-panel') === tabName;
      panel.classList.toggle('is-active', active);
      panel.hidden = !active;
    });
  }

  function dedupeKernelWidgets() {
    var kernelWidgets = Array.prototype.slice.call(
      document.querySelectorAll('[data-agent-widget][data-agent-widget-source="kernel"]')
    );
    if (kernelWidgets.length === 0) return;
    document.querySelectorAll('[data-agent-widget]:not([data-agent-widget-source="kernel"])').forEach(function(widget) {
      widget.remove();
    });
  }

  function bindAgentWidget(widget) {
    if (!widget || widget.dataset.bound === 'true') return;
    widget.dataset.bound = 'true';
    var activeTrigger = widget.querySelector('[data-agent-tab-trigger].is-active');
    var defaultTab = activeTrigger ? activeTrigger.getAttribute('data-agent-tab-trigger') : 'access';
    activateAgentTab(widget, defaultTab);
    widget.querySelectorAll('[data-agent-tab-trigger]').forEach(function(button) {
      button.addEventListener('click', function() {
        activateAgentTab(widget, button.getAttribute('data-agent-tab-trigger'));
      });
    });
  }

  function initAgentWidgets(scope) {
    dedupeKernelWidgets();
    (scope || document).querySelectorAll('[data-agent-widget]').forEach(bindAgentWidget);
    document.querySelectorAll('[data-agent-current-path]').forEach(function(el) {
      el.textContent = window.location.pathname;
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function() {
      initAgentWidgets(document);
    });
  } else {
    initAgentWidgets(document);
  }

  document.addEventListener('htmx:afterSwap', function(evt) {
    initAgentWidgets(evt.target);
  });
})();
