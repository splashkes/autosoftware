// Class splits UI — handles split-select multi-select activation, batched
// commands to class_splits.create + entries.move_to_split, and per-entry
// "Move to" affordance. Talks to the JSON command API at
//   /v1/commands/0007-Flowershow/class_splits.create
//   /v1/commands/0007-Flowershow/class_splits.delete
//   /v1/commands/0007-Flowershow/entries.move_to_split

(function () {
  'use strict';

  var COMMAND_BASE = '/v1/commands/0007-Flowershow/';

  function basePath() {
    return (window.flowershowBasePath || (document.body && document.body.getAttribute('data-base-path')) || '').replace(/\/$/, '');
  }

  function commandURL(name) {
    return basePath() + COMMAND_BASE + name;
  }

  function postJSON(url, body) {
    return fetch(url, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
      body: JSON.stringify(body || {})
    }).then(function (resp) {
      if (!resp.ok) {
        return resp.text().then(function (txt) {
          throw new Error(resp.status + ': ' + (txt || resp.statusText));
        });
      }
      return resp.json().catch(function () { return {}; });
    });
  }

  function activeClassRoot(classID) {
    return document.querySelector('[data-splittable-class="' + classID + '"]');
  }

  function setSelectMode(root, on) {
    if (!root) return;
    root.classList.toggle('split-select-active', !!on);
    if (!on) {
      root.querySelectorAll('[data-split-checkbox]').forEach(function (cb) { cb.checked = false; });
    }
  }

  function selectedEntryIDs(root) {
    var ids = [];
    root.querySelectorAll('[data-split-checkbox]').forEach(function (cb) {
      if (cb.checked) {
        var id = cb.getAttribute('data-entry-id');
        if (id) ids.push(id);
      }
    });
    return ids;
  }

  function refreshCounts(root) {
    var bar = root.querySelector('[data-split-action-bar]');
    if (!bar) return;
    var count = selectedEntryIDs(root).length;
    var label = bar.querySelector('[data-split-count-label]');
    if (label) label.textContent = String(count);
    var submitBtn = bar.querySelector('[data-split-create-button]');
    if (submitBtn) submitBtn.disabled = count === 0;
  }

  function handleSplitButtonClick(button) {
    var classID = button.getAttribute('data-class-id');
    if (!classID) return;
    var root = activeClassRoot(classID);
    if (!root) return;
    var on = !root.classList.contains('split-select-active');
    setSelectMode(root, on);
    refreshCounts(root);
  }

  function handleCancelClick(button) {
    var classID = button.getAttribute('data-class-id');
    var root = activeClassRoot(classID);
    setSelectMode(root, false);
    refreshCounts(root);
  }

  function handleCreateSplit(button) {
    var classID = button.getAttribute('data-class-id');
    var root = activeClassRoot(classID);
    if (!root) return;
    var entryIDs = selectedEntryIDs(root);
    if (!entryIDs.length) return;
    var labelInput = root.querySelector('[data-split-label-input]');
    var label = labelInput ? (labelInput.value || '').trim() : '';

    button.disabled = true;
    postJSON(commandURL('class_splits.create'), { class_id: classID, label: label })
      .then(function (resp) {
        var splitID = (resp && (resp.id || (resp.split && resp.split.id))) || '';
        if (!splitID) {
          throw new Error('class_splits.create did not return id');
        }
        var chain = Promise.resolve();
        entryIDs.forEach(function (entryID) {
          chain = chain.then(function () {
            return postJSON(commandURL('entries.move_to_split'), { entry_id: entryID, split_id: splitID });
          });
        });
        return chain;
      })
      .then(function () { window.location.reload(); })
      .catch(function (err) {
        button.disabled = false;
        if (window.flowershowToast) {
          window.flowershowToast('Split failed: ' + err.message, true);
        } else {
          alert('Split failed: ' + err.message);
        }
      });
  }

  function handleDeleteSplit(button) {
    var splitID = button.getAttribute('data-split-id');
    if (!splitID) return;
    if (!window.confirm('Delete this split? Entries will return to the class without a split assignment.')) {
      return;
    }
    button.disabled = true;
    postJSON(commandURL('class_splits.delete'), { id: splitID })
      .then(function () { window.location.reload(); })
      .catch(function (err) {
        button.disabled = false;
        if (window.flowershowToast) {
          window.flowershowToast('Delete split failed: ' + err.message, true);
        } else {
          alert('Delete split failed: ' + err.message);
        }
      });
  }

  function handleMoveSelect(select) {
    var entryID = select.getAttribute('data-entry-id');
    var splitID = select.value || '';
    if (!entryID) return;
    var prev = select.getAttribute('data-prev-value') || '';
    if (splitID === prev) return;
    select.disabled = true;
    postJSON(commandURL('entries.move_to_split'), { entry_id: entryID, split_id: splitID })
      .then(function () { window.location.reload(); })
      .catch(function (err) {
        select.disabled = false;
        select.value = prev;
        if (window.flowershowToast) {
          window.flowershowToast('Move failed: ' + err.message, true);
        } else {
          alert('Move failed: ' + err.message);
        }
      });
  }

  function bindRoot(root) {
    if (!root || root.dataset.splitsBound === 'true') return;
    root.dataset.splitsBound = 'true';

    root.querySelectorAll('[data-split-checkbox]').forEach(function (cb) {
      cb.addEventListener('change', function () { refreshCounts(root); });
    });
    var labelInput = root.querySelector('[data-split-label-input]');
    if (labelInput) labelInput.addEventListener('input', function () { refreshCounts(root); });
  }

  function bindAll() {
    document.querySelectorAll('[data-split-button]').forEach(function (b) {
      if (b.dataset.splitButtonBound === 'true') return;
      b.dataset.splitButtonBound = 'true';
      b.addEventListener('click', function (e) { e.preventDefault(); handleSplitButtonClick(b); });
    });
    document.querySelectorAll('[data-split-cancel-button]').forEach(function (b) {
      if (b.dataset.splitCancelBound === 'true') return;
      b.dataset.splitCancelBound = 'true';
      b.addEventListener('click', function (e) { e.preventDefault(); handleCancelClick(b); });
    });
    document.querySelectorAll('[data-split-create-button]').forEach(function (b) {
      if (b.dataset.splitCreateBound === 'true') return;
      b.dataset.splitCreateBound = 'true';
      b.addEventListener('click', function (e) { e.preventDefault(); handleCreateSplit(b); });
    });
    document.querySelectorAll('[data-split-delete-button]').forEach(function (b) {
      if (b.dataset.splitDeleteBound === 'true') return;
      b.dataset.splitDeleteBound = 'true';
      b.addEventListener('click', function (e) { e.preventDefault(); handleDeleteSplit(b); });
    });
    document.querySelectorAll('[data-split-move-select]').forEach(function (s) {
      if (s.dataset.splitMoveBound === 'true') return;
      s.dataset.splitMoveBound = 'true';
      s.setAttribute('data-prev-value', s.value || '');
      s.addEventListener('change', function () { handleMoveSelect(s); });
    });
    document.querySelectorAll('[data-splittable-class]').forEach(bindRoot);
  }

  function init() {
    bindAll();
    document.body.addEventListener('htmx:afterSwap', bindAll);
    document.body.addEventListener('htmx:afterSettle', bindAll);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  window.flowershowSplitsRefresh = bindAll;
})();
