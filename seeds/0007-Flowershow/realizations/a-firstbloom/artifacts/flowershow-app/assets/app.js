// Flowershow — minimal JS (HTMX handles most interactivity)

const FLOWERSHOW_MAX_PHOTO_BYTES = 20 * 1024 * 1024;
const FLOWERSHOW_MAX_PHOTO_EDGE = 1000;
const FLOWERSHOW_MAX_VIDEO_BYTES = 50 * 1024 * 1024;
const FLOWERSHOW_MAX_VIDEO_EDGE = 1920;
const FLOWERSHOW_DEFERRED_MEDIA_CONCURRENCY = 4;
const flowershowIntakeUploadStates = new WeakMap();
const flowershowIntakeAutosaveTimers = new WeakMap();
const flowershowDeferredMediaQueue = [];
const flowershowDeferredMediaActiveImages = new Set();
let flowershowDeferredMediaActive = 0;

function flowershowQueryScope(root) {
  if (root && typeof root.querySelectorAll === 'function') {
    return root;
  }
  return document;
}

function flowershowDeferredMediaSrc(img) {
  return img && img.dataset ? (img.dataset.deferredMediaSrc || '').trim() : '';
}

function flowershowQueueDeferredMediaImage(img) {
  const src = flowershowDeferredMediaSrc(img);
  if (!src || !img.isConnected) return;
  if (img.getAttribute('src') === src) {
    img.dataset.deferredMediaState = 'loaded';
    return;
  }
  if (img.dataset.deferredMediaState === 'queued' || img.dataset.deferredMediaState === 'loading' || img.dataset.deferredMediaState === 'loaded') return;
  img.dataset.deferredMediaState = 'queued';
  flowershowDeferredMediaQueue.push(img);
  flowershowPumpDeferredMediaQueue();
}

function flowershowPumpDeferredMediaQueue() {
  flowershowDeferredMediaActiveImages.forEach(function(img) {
    if (!img.isConnected) {
      flowershowDeferredMediaActiveImages.delete(img);
      flowershowDeferredMediaActive = Math.max(0, flowershowDeferredMediaActive - 1);
    }
  });
  while (flowershowDeferredMediaActive < FLOWERSHOW_DEFERRED_MEDIA_CONCURRENCY && flowershowDeferredMediaQueue.length > 0) {
    const img = flowershowDeferredMediaQueue.shift();
    const src = flowershowDeferredMediaSrc(img);
    if (!src || !img.isConnected) {
      continue;
    }
    flowershowDeferredMediaActive += 1;
    flowershowDeferredMediaActiveImages.add(img);
    img.dataset.deferredMediaState = 'loading';

    const finish = function(state) {
      img.removeEventListener('load', onLoad);
      img.removeEventListener('error', onError);
      flowershowDeferredMediaActiveImages.delete(img);
      if (img.isConnected) {
        img.dataset.deferredMediaState = state;
      }
      flowershowDeferredMediaActive -= 1;
      flowershowPumpDeferredMediaQueue();
    };
    const onLoad = function() {
      finish('loaded');
    };
    const onError = function() {
      const attempts = parseInt(img.dataset.deferredMediaAttempts || '0', 10) + 1;
      img.dataset.deferredMediaAttempts = String(attempts);
      finish('error');
      if (attempts < 3) {
        setTimeout(function() {
          if (!img.isConnected) return;
          img.dataset.deferredMediaState = '';
          flowershowQueueDeferredMediaImage(img);
        }, 1200 * attempts);
      }
    };

    img.addEventListener('load', onLoad, { once: true });
    img.addEventListener('error', onError, { once: true });
    img.src = src;
  }
}

function flowershowBindDeferredMediaImage(img) {
  if (!flowershowDeferredMediaSrc(img) || img.dataset.deferredMediaBound === 'true') return;
  img.dataset.deferredMediaBound = 'true';
  flowershowQueueDeferredMediaImage(img);
}

function flowershowActivateShowAdminTab(shell, name) {
  if (!shell || !name) return;
  shell.dataset.showAdminActiveTab = name;
  shell.querySelectorAll('[data-show-admin-nav]').forEach(function(group) {
    group.classList.toggle('is-active', group.dataset.showAdminNav === name);
  });
}

function showTab(name) {
  document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
  document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
  const tab = document.getElementById('tab-' + name);
  if (tab) tab.classList.add('active');
  // Find the clicked button
  document.querySelectorAll('.tab').forEach(el => {
    if (el.textContent.trim().toLowerCase() === name.toLowerCase() ||
        el.getAttribute('onclick')?.includes(name)) {
      el.classList.add('active');
    }
  });
  document.querySelectorAll('[data-show-admin-shell]').forEach(function(shell) {
    flowershowActivateShowAdminTab(shell, name);
  });
}

function flowershowToast(message, isError) {
  const container = document.getElementById('sse-toasts') || document.querySelector('.toast-container');
  if (!container) return;
  const toast = document.createElement('div');
  toast.className = 'toast' + (isError ? ' alert alert-error' : '');
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(() => toast.remove(), 4000);
}

function flowershowFriendlyErrorMessage(message, fallback) {
  const raw = String(message || '').replace(/<[^>]+>/g, '').replace(/\s+/g, ' ').trim();
  const lower = raw.toLowerCase();
  const looksLikeProjectionLag = lower.indexOf('context deadline exceeded') !== -1 && (
    lower.indexOf('projection') !== -1 ||
    lower.indexOf('as_flowershow_m_') !== -1 ||
    lower.indexOf('truncate flowershow') !== -1 ||
    lower.indexOf('insert class projection') !== -1 ||
    lower.indexOf('check projection rebuild') !== -1
  );
  if (looksLikeProjectionLag) {
    return 'The edit was accepted, but the live refresh is still catching up. Refresh in a moment if it does not appear.';
  }
  return raw || fallback || 'Something went wrong.';
}

function flowershowToggleRubricCriteria(select) {
  var rubricID = select.value;
  document.querySelectorAll('.criteria-group').forEach(function(el) {
    el.classList.toggle('active', el.dataset.rubricId === rubricID);
  });
}

function flowershowFilterPersonSelect(input) {
  const targetSelector = input.dataset.filterTarget;
  if (!targetSelector) return;
  const select = document.querySelector(targetSelector);
  if (!select) return;
  const query = (input.value || '').trim().toLowerCase();
  Array.from(select.options).forEach(function(option, index) {
    if (index === 0) {
      option.hidden = false;
      return;
    }
    const matches = query === '' || option.textContent.toLowerCase().includes(query);
    option.hidden = !matches;
  });
}

function flowershowBindPersonFilter(input) {
  if (input.dataset.bound === 'true') return;
  input.dataset.bound = 'true';
  input.addEventListener('input', function() {
    flowershowFilterPersonSelect(input);
  });
}

function flowershowFilterCorrections(input) {
  const targetSelector = input.dataset.filterTarget;
  if (!targetSelector) return;
  const root = document.querySelector(targetSelector);
  if (!root) return;
  const query = (input.value || '').trim().toLowerCase();
  root.querySelectorAll('[data-corrections-searchable]').forEach(function(card) {
    const haystack = (card.dataset.correctionsSearchText || '').toLowerCase();
    card.hidden = query !== '' && haystack.indexOf(query) === -1;
  });
}

function flowershowBindCorrectionsFilter(input) {
  if (!input || input.dataset.bound === 'true') return;
  input.dataset.bound = 'true';
  input.addEventListener('input', function() {
    flowershowFilterCorrections(input);
  });
}

function flowershowCloseIntakeModal(modal) {
  if (!modal) return;
  modal.querySelectorAll('[data-intake-entry-form], [data-intake-edit-form]').forEach(function(form) {
    flowershowClearAutosaveTimer(form);
    flowershowSetAutosaveStatus(form, '', false);
    flowershowResetIntakeUploadState(form);
  });
  modal.hidden = true;
  document.body.classList.remove('body-lightbox-open');
}

function flowershowSyncEntrantLookup(input) {
  if (!input) return false;
  const hidden = input
    .closest('form')
    .querySelector('[data-intake-person-id-input]');
  if (!hidden) return false;
  const listId = input.dataset.intakeEntrantList || input.getAttribute('list');
  const list = listId ? document.getElementById(listId) : null;
  const value = (input.value || '').trim();
  hidden.value = '';
  if (!list || value === '') {
    return false;
  }
  const match = Array.from(list.options).find(function(option) {
    return option.value.trim() === value;
  });
  if (!match) {
    return false;
  }
  hidden.value = match.dataset.personId || '';
  return hidden.value !== '';
}

function flowershowEntrantMatches(input) {
  const listId = input.dataset.intakeEntrantList || input.getAttribute('list');
  const list = listId ? document.getElementById(listId) : null;
  if (!list) return [];
  const query = (input.value || '').trim().toLowerCase();
  return Array.from(list.options).map(function(option) {
    return {
      label: (option.value || '').trim(),
      personID: option.dataset.personId || ''
    };
  }).filter(function(option) {
    if (option.label === '' || option.personID === '') return false;
    if (query === '') return true;
    return option.label.toLowerCase().includes(query);
  }).slice(0, 8);
}

function flowershowRenderEntrantResults(input) {
  if (!input) return;
  const formGroup = input.closest('.form-group');
  const results = formGroup ? formGroup.querySelector('[data-intake-person-results]') : null;
  if (!results) return;
  const matches = flowershowEntrantMatches(input);
  results.innerHTML = '';
  if (matches.length === 0) {
    results.hidden = true;
    return;
  }
  matches.forEach(function(match) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'intake-autocomplete-option';
    button.textContent = match.label;
    button.addEventListener('click', function() {
      input.value = match.label;
      const form = input.closest('form');
      const hidden = form ? form.querySelector('[data-intake-person-id-input]') : null;
      if (hidden) {
        hidden.value = match.personID;
      }
      results.hidden = true;
      results.innerHTML = '';
      if (form && form.dataset.intakeAutosave === 'true') {
        flowershowScheduleAutosave(form, 0);
      }
    });
    results.appendChild(button);
  });
  results.hidden = false;
}

function flowershowFormatClassCount(countRaw, unitRaw) {
  const count = parseInt(countRaw || '0', 10);
  if (!Number.isFinite(count) || count <= 0) return '';
  const unit = (unitRaw || '').trim().toLowerCase();
  if (unit) {
    const label = count === 1 || unit.endsWith('s') ? unit : unit + 's';
    return count + ' ' + label;
  }
  return count + ' ' + (count === 1 ? 'specimen' : 'specimens');
}

function flowershowPopulateIntakeClassHero(modal, trigger) {
  const hero = modal.querySelector('[data-intake-class-hero]');
  if (!hero) return;
  const number = (trigger.dataset.intakeClassNumber || '').trim();
  const title = (trigger.dataset.intakeClassTitle || '').trim();
  const description = (trigger.dataset.intakeClassDescription || '').trim();
  const countLabel = flowershowFormatClassCount(trigger.dataset.intakeClassSpecimenCount, trigger.dataset.intakeClassUnit);
  const detailValues = [
    countLabel,
    (trigger.dataset.intakeClassMeasurementRule || '').trim(),
    (trigger.dataset.intakeClassNamingRequirement || '').trim(),
    (trigger.dataset.intakeClassContainerRule || '').trim(),
    (trigger.dataset.intakeClassEligibilityRule || '').trim()
  ].filter(Boolean);
  const notes = (trigger.dataset.intakeClassScheduleNotes || '').trim();

  const numberNode = hero.querySelector('[data-intake-class-number-display]');
  const titleNode = hero.querySelector('[data-intake-class-title-display]');
  const descriptionNode = hero.querySelector('[data-intake-class-description-display]');
  const detailsNode = hero.querySelector('[data-intake-class-detail-display]');
  const notesNode = hero.querySelector('[data-intake-class-notes-display]');

  if (numberNode) numberNode.textContent = number;
  if (titleNode) titleNode.textContent = title;
  if (descriptionNode) {
    descriptionNode.textContent = description;
    descriptionNode.hidden = description === '';
  }
  if (detailsNode) {
    detailsNode.innerHTML = '';
    detailValues.forEach(function(value) {
      const chip = document.createElement('span');
      chip.className = 'intake-class-detail-chip';
      chip.textContent = value;
      detailsNode.appendChild(chip);
    });
    detailsNode.hidden = detailValues.length === 0;
  }
  if (notesNode) {
    notesNode.textContent = notes;
    notesNode.hidden = notes === '';
  }
  hero.hidden = !(number || title || description || detailValues.length || notes);
}

function flowershowRefreshPlacementButtons(form) {
  const value = String(parseInt((form.querySelector('[data-intake-placement-input]') || {}).value || '0', 10) || 0);
  form.querySelectorAll('[data-intake-placement-button]').forEach(function(button) {
    const active = button.dataset.intakePlacementButton === value && value !== '0';
    button.classList.toggle('is-active', active);
    button.setAttribute('aria-pressed', active ? 'true' : 'false');
  });
  flowershowRefreshResultButtons(form);
}

function flowershowRefreshAwardState(form) {
  const specialInput = form.querySelector('[data-intake-special-status-input]');
  const awardInput = form.querySelector('[data-intake-award-input]');
  const awardGroup = form.querySelector('[data-intake-award-group]');
  const awardToggle = form.querySelector('[data-intake-award-toggle]');
  const awardSelect = form.querySelector('[data-intake-award-select]');
  const active = !!((specialInput && specialInput.value === 'true') || (awardToggle && awardToggle.dataset.forceOpen === 'true'));
  if (awardGroup) {
    awardGroup.hidden = !active;
  }
  if (awardToggle) {
    awardToggle.classList.toggle('is-active', active);
    awardToggle.setAttribute('aria-pressed', active ? 'true' : 'false');
  }
  if (awardSelect && awardInput && awardSelect.value !== awardInput.value) {
    awardSelect.value = awardInput.value;
  }
  flowershowRefreshResultButtons(form);
}

function flowershowRefreshResultButtons(form) {
  if (!form) return;
  const placementInput = form.querySelector('[data-intake-placement-input]');
  const placementValue = String(parseInt((placementInput || {}).value || '0', 10) || 0);
  const awardToggle = form.querySelector('[data-intake-award-toggle]');
  const awardActive = !!(awardToggle && awardToggle.getAttribute('aria-pressed') === 'true');
  const anyActive = placementValue !== '0' || awardActive;
  const pendingConfirm = form.dataset.intakePendingConfirm || '';

  form.querySelectorAll('[data-intake-placement-button]').forEach(function(button) {
    const active = button.dataset.intakePlacementButton === placementValue && placementValue !== '0';
    const baseLabel = button.dataset.intakeBaseLabel || button.textContent.trim();
    const confirmToken = 'placement:' + button.dataset.intakePlacementButton;
    const revokeToken = 'revoke:placement:' + button.dataset.intakePlacementButton;
    button.dataset.intakeBaseLabel = baseLabel;
    if (pendingConfirm === confirmToken) {
      button.textContent = 'Confirm ' + baseLabel;
    } else if (pendingConfirm === revokeToken) {
      button.textContent = 'Confirm remove ' + baseLabel;
    } else {
      button.textContent = baseLabel;
    }
    button.classList.toggle('is-dimmed', anyActive && !active);
  });
  if (awardToggle) {
    const awardBaseLabel = awardToggle.dataset.intakeBaseLabel || awardToggle.textContent.trim();
    awardToggle.dataset.intakeBaseLabel = awardBaseLabel;
    if (pendingConfirm === 'special:true') {
      awardToggle.textContent = 'Confirm ' + awardBaseLabel;
    } else if (pendingConfirm === 'revoke:special') {
      awardToggle.textContent = 'Confirm remove ' + awardBaseLabel;
    } else {
      awardToggle.textContent = awardBaseLabel;
    }
    awardToggle.classList.toggle('is-dimmed', anyActive && !awardActive);
  }
}

function flowershowIntakeInitials(label) {
  const text = (label || '').trim();
  if (!text) return 'this entrant';
  const parts = text.split(/\s+/).filter(Boolean);
  if (!parts.length) return 'this entrant';
  const initials = parts.slice(0, 2).map(function(part) {
    return part.charAt(0).toUpperCase();
  }).join('');
  return initials || text;
}

function flowershowIntakeClassNumber(form) {
  if (!form) return '';
  if (form.dataset.intakeClassNumber) return form.dataset.intakeClassNumber;
  const select = form.querySelector('[data-intake-existing-class-select]');
  if (select && select.selectedOptions && select.selectedOptions[0]) {
    const label = select.selectedOptions[0].textContent || '';
    return label.split(':')[0].trim();
  }
  return '';
}

function flowershowIntakeEntrantDisplay(form) {
  if (!form) return 'this entrant';
  const explicit = (form.dataset.intakeEntrantDisplay || '').trim();
  if (explicit) return explicit;
  const entrantInput = form.querySelector('[data-intake-entrant-input]');
  if (entrantInput && entrantInput.value.trim()) {
    return flowershowIntakeInitials(entrantInput.value);
  }
  return 'this entrant';
}

function flowershowIntakeConfirmMessage(form, token) {
  if (!token) return '';
  const entrant = flowershowIntakeEntrantDisplay(form);
  const classNumber = flowershowIntakeClassNumber(form);
  let resultLabel = '';
  if (token.indexOf('placement:') === 0) {
    const placement = token.split(':')[1];
    resultLabel = placement === '1' ? '1st place' : placement === '2' ? '2nd place' : placement === '3' ? '3rd place' : 'result';
  } else if (token.indexOf('revoke:placement:') === 0) {
    const placement = token.split(':')[2];
    resultLabel = placement === '1' ? 'removing 1st place' : placement === '2' ? 'removing 2nd place' : placement === '3' ? 'removing 3rd place' : 'removing the result';
  } else if (token === 'special:true') {
    resultLabel = 'special status';
  } else if (token === 'revoke:special') {
    resultLabel = 'removing special status';
  } else {
    resultLabel = 'result';
  }
  return classNumber ? ('Confirm ' + resultLabel + ' for ' + entrant + ' in Class ' + classNumber + '.') : ('Confirm ' + resultLabel + ' for ' + entrant + '.');
}

function flowershowIntakeIdleMessage(form) {
  if (!form) return '';
  return form.dataset.intakeResultIdleMessage || '';
}

function flowershowEntrantLabelForPerson(input, personID) {
  if (!input || !personID) return '';
  const listId = input.dataset.intakeEntrantList || input.getAttribute('list');
  const list = listId ? document.getElementById(listId) : null;
  if (!list) return '';
  const match = Array.from(list.options).find(function(option) {
    return (option.dataset.personId || '') === personID;
  });
  return match ? (match.value || '').trim() : '';
}

function flowershowPopulateIntakeExistingMediaPreview(modal, trigger) {
  if (!modal) return;
  const preview = modal.querySelector('[data-intake-existing-media-preview]');
  if (!preview) return;
  const image = preview.querySelector('[data-intake-existing-media-image]');
  const fallback = preview.querySelector('[data-intake-existing-media-fallback]');
  const title = preview.querySelector('[data-intake-existing-media-title]');
  const meta = preview.querySelector('[data-intake-existing-media-meta]');
  const src = trigger && trigger.dataset ? (trigger.dataset.intakeThumbnailSrc || '').trim() : '';
  const mediaType = trigger && trigger.dataset ? (trigger.dataset.intakeThumbnailType || '').trim().toLowerCase() : '';
  const fileName = trigger && trigger.dataset ? (trigger.dataset.intakeThumbnailFile || '').trim() : '';
  const mediaCount = trigger && trigger.dataset ? parseInt(trigger.dataset.intakeMediaCount || '0', 10) || 0 : 0;
  if (!src) {
    preview.hidden = true;
    if (image) {
      image.hidden = true;
      image.removeAttribute('src');
    }
    if (fallback) fallback.hidden = true;
    if (title) title.textContent = '';
    if (meta) meta.textContent = '';
    return;
  }
  preview.hidden = false;
  if (title) {
    title.textContent = trigger.dataset.intakeEntryName || trigger.dataset.intakeEntrant || 'Entry media';
  }
  if (meta) {
    meta.textContent = mediaCount > 1 ? mediaCount + ' media items' : (fileName || 'Photo attached');
  }
  if (mediaType === 'video') {
    if (image) {
      image.hidden = true;
      image.removeAttribute('src');
    }
    if (fallback) {
      fallback.textContent = 'Video';
      fallback.hidden = false;
    }
    return;
  }
  if (fallback) fallback.hidden = true;
  if (image) {
    image.alt = trigger.dataset.intakeEntryName || trigger.dataset.intakeEntrant || 'Entry media';
    image.src = src;
    image.hidden = false;
  }
}

function flowershowOpenIntakeModal(modal, trigger) {
  if (!modal || !trigger) return;
  const mode = trigger.dataset.intakeMode || 'new';
  const title = modal.querySelector('[data-intake-modal-title]');
  const newPanel = modal.querySelector('[data-intake-new-panel]');
  const existingPanel = modal.querySelector('[data-intake-existing-panel]');
  if (!title || !newPanel || !existingPanel) return;

  const classLabel = trigger.dataset.intakeClassLabel || '';
  title.textContent = mode === 'existing' ? 'Update intake' : 'Add entry';
  flowershowPopulateIntakeClassHero(modal, trigger);

  newPanel.hidden = mode !== 'new';
  existingPanel.hidden = mode === 'new';

  if (mode === 'new') {
    const form = modal.querySelector('[data-intake-entry-form]');
    const classIDInput = modal.querySelector('[data-intake-class-id-input]');
    const entrantInput = modal.querySelector('[data-intake-entrant-input]');
    const personIDInput = modal.querySelector('[data-intake-person-id-input]');
    if (form) {
      form.reset();
      flowershowClearIntakeDraftEntry(form);
      flowershowSetIntakeFormAction(form, trigger.dataset.intakeCreateAction || form.action);
      flowershowResetIntakeUploadState(form);
      flowershowSetAutosaveStatus(form, '', false);
    }
    flowershowPopulateIntakeExistingMediaPreview(modal, null);
    if (classIDInput) classIDInput.value = trigger.dataset.intakeClassId || '';
    if (entrantInput) entrantInput.value = '';
    if (personIDInput) personIDInput.value = '';
    flowershowRenderEntrantResults(entrantInput);
  } else {
    flowershowPopulateIntakeExistingMediaPreview(modal, trigger);
    const editForm = modal.querySelector('[data-intake-edit-form]');
    if (editForm) {
      const placementInput = editForm.querySelector('[data-intake-placement-input]');
      const specialStatusInput = editForm.querySelector('[data-intake-special-status-input]');
      const awardInput = editForm.querySelector('[data-intake-award-input]');
      const awardSelect = editForm.querySelector('[data-intake-award-select]');
      const entrantInput = editForm.querySelector('[data-intake-entrant-input]');
      const personIDInput = editForm.querySelector('[data-intake-person-id-input]');
      const classSelect = editForm.querySelector('[data-intake-existing-class-select]');
      const nameInput = editForm.querySelector('[data-intake-existing-name-input]');
      const notesInput = editForm.querySelector('[data-intake-existing-notes-input]');
      editForm.reset();
      editForm.action = trigger.dataset.intakeUpdateAction || '';
      if (placementInput) {
        placementInput.value = trigger.dataset.intakePlacement || '0';
      }
      if (specialStatusInput) {
        specialStatusInput.value = trigger.dataset.intakeSpecialStatus === 'true' ? 'true' : 'false';
      }
      if (awardInput) {
        awardInput.value = trigger.dataset.intakeAwardId || '';
      }
      if (awardSelect) {
        awardSelect.value = trigger.dataset.intakeAwardId || '';
      }
      const selectedPersonID = trigger.dataset.intakePersonId || '';
      if (personIDInput) {
        personIDInput.value = selectedPersonID;
      }
      if (entrantInput) {
        entrantInput.value = flowershowEntrantLabelForPerson(entrantInput, selectedPersonID) || trigger.dataset.intakeEntrant || '';
        flowershowRenderEntrantResults(entrantInput);
        editForm.dataset.intakeEntrantDisplay = flowershowIntakeInitials(trigger.dataset.intakeEntrant || '');
      }
      if (classSelect) {
        classSelect.value = trigger.dataset.intakeClassId || '';
      }
      editForm.dataset.intakeClassNumber = trigger.dataset.intakeClassNumber || '';
      editForm.dataset.intakeUploadAction = trigger.dataset.intakeUploadAction || '';
      if (nameInput) {
        nameInput.value = trigger.dataset.intakeEntryName || '';
      }
      if (notesInput) {
        notesInput.value = trigger.dataset.intakeNotes || '';
      }
      const awardToggle = editForm.querySelector('[data-intake-award-toggle]');
      if (awardToggle) {
        awardToggle.dataset.forceOpen = trigger.dataset.intakeSpecialStatus === 'true' ? 'true' : '';
      }
      editForm.dataset.intakePendingConfirm = '';
      flowershowResetIntakeUploadState(editForm);
      flowershowSetAutosaveStatus(editForm, '', false);
      flowershowRefreshPlacementButtons(editForm);
      flowershowRefreshAwardState(editForm);
    }
  }

  modal.hidden = false;
  document.body.classList.add('body-lightbox-open');
}

function flowershowBindIntakeModal(modal) {
  if (!modal || modal.dataset.bound === 'true') return;
  modal.dataset.bound = 'true';

  modal.querySelectorAll('[data-intake-modal-close]').forEach(function(button) {
    button.addEventListener('click', function() {
      flowershowCloseIntakeModal(modal);
    });
  });

  document.addEventListener('keydown', function(event) {
    if (event.key === 'Escape' && !modal.hidden) {
      flowershowCloseIntakeModal(modal);
    }
  });

  modal.querySelectorAll('[data-intake-entrant-input]').forEach(flowershowBindIntakeEntrantInput);
  modal.querySelectorAll('[data-intake-existing-class-select]').forEach(function(classSelect) {
    classSelect.addEventListener('change', function() {
      const form = classSelect.closest('form');
      if (form && classSelect.selectedOptions && classSelect.selectedOptions[0]) {
        form.dataset.intakeClassNumber = (classSelect.selectedOptions[0].textContent || '').split(':')[0].trim();
        flowershowRefreshResultButtons(form);
      }
    });
  });
  flowershowBindIntakeForm(modal.querySelector('[data-intake-entry-form]'), { isNew: true });
  flowershowBindIntakeForm(modal.querySelector('[data-intake-edit-form]'), { isNew: false });
  flowershowBindIntakeResultsForm(modal.querySelector('[data-intake-results-form]'));
}

function flowershowBindIntakeTrigger(button) {
  if (!button || button.dataset.bound === 'true') return;
  button.dataset.bound = 'true';
  button.addEventListener('click', function() {
    flowershowOpenIntakeModal(document.querySelector('[data-intake-modal]'), button);
  });
}

function flowershowBindIntakeEntrantInput(entrantInput) {
  if (!entrantInput || entrantInput.dataset.intakeEntrantBound === 'true') return;
  entrantInput.dataset.intakeEntrantBound = 'true';
  entrantInput.addEventListener('input', function() {
    flowershowSyncEntrantLookup(entrantInput);
    flowershowRenderEntrantResults(entrantInput);
    const form = entrantInput.closest('form');
    if (form) {
      form.dataset.intakeEntrantDisplay = flowershowIntakeInitials(entrantInput.value);
      flowershowRefreshResultButtons(form);
    }
  });
  entrantInput.addEventListener('change', function() {
    flowershowSyncEntrantLookup(entrantInput);
    flowershowRenderEntrantResults(entrantInput);
    const form = entrantInput.closest('form');
    if (form) {
      form.dataset.intakeEntrantDisplay = flowershowIntakeInitials(entrantInput.value);
      flowershowRefreshResultButtons(form);
    }
  });
  entrantInput.addEventListener('focus', function() {
    flowershowRenderEntrantResults(entrantInput);
  });
  entrantInput.addEventListener('blur', function() {
    window.setTimeout(function() {
      const formGroup = entrantInput.closest('.form-group');
      const results = formGroup ? formGroup.querySelector('[data-intake-person-results]') : null;
      if (results) {
        results.hidden = true;
      }
    }, 120);
  });
}

async function flowershowNormalizeImage(file, maxEdge, quality) {
  const url = URL.createObjectURL(file);
  try {
    const image = await new Promise(function(resolve, reject) {
      const img = new Image();
      img.onload = function() { resolve(img); };
      img.onerror = reject;
      img.src = url;
    });
    const width = image.naturalWidth || image.width;
    const height = image.naturalHeight || image.height;
    const scale = Math.min(1, maxEdge / Math.max(width, height));
    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.round(width * scale));
    canvas.height = Math.max(1, Math.round(height * scale));
    const ctx = canvas.getContext('2d', { alpha: false });
    ctx.drawImage(image, 0, 0, canvas.width, canvas.height);
    const blob = await new Promise(function(resolve) {
      canvas.toBlob(resolve, 'image/jpeg', quality);
    });
    if (!blob) {
      throw new Error('Could not prepare photo for upload.');
    }
    const name = (file.name || 'capture')
      .replace(/\.[^.]+$/, '')
      .replace(/[^a-zA-Z0-9_-]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'capture';
    return {
      file: new File([blob], name + '.jpg', { type: 'image/jpeg', lastModified: Date.now() }),
      width: canvas.width,
      height: canvas.height
    };
  } finally {
    URL.revokeObjectURL(url);
  }
}

function flowershowSanitizeUploadBase(name) {
  return ((name || 'capture')
    .replace(/\.[^.]+$/, '')
    .replace(/[^a-zA-Z0-9_-]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'capture');
}

function flowershowLooksLikeHeic(file) {
  const lowerName = (file.name || '').toLowerCase();
  const mime = (file.type || '').toLowerCase();
  return lowerName.endsWith('.heic') ||
    lowerName.endsWith('.heif') ||
    mime === 'image/heic' ||
    mime === 'image/heif';
}

function flowershowPhotoLike(file) {
  const lowerName = (file.name || '').toLowerCase();
  const mime = (file.type || '').toLowerCase();
  return /^image\//.test(mime) || /\.(jpe?g|png|webp|heic|heif)$/i.test(lowerName);
}

function flowershowVideoLike(file) {
  const lowerName = (file.name || '').toLowerCase();
  const mime = (file.type || '').toLowerCase();
  return /^video\//.test(mime) || /\.(mp4|webm|mov)$/i.test(lowerName);
}

async function flowershowNormalizeVideo(file) {
  if (file.size > FLOWERSHOW_MAX_VIDEO_BYTES) {
    throw new Error('Video exceeds 50 MB. Capture a shorter or smaller clip.');
  }
  const url = URL.createObjectURL(file);
  try {
    const dimensions = await new Promise(function(resolve, reject) {
      const video = document.createElement('video');
      video.preload = 'metadata';
      video.playsInline = true;
      video.onloadedmetadata = function() {
        resolve({ width: video.videoWidth || 0, height: video.videoHeight || 0 });
      };
      video.onerror = function() {
        reject(new Error('Could not read the captured video.'));
      };
      video.src = url;
    });
    if (Math.max(dimensions.width, dimensions.height) > FLOWERSHOW_MAX_VIDEO_EDGE) {
      throw new Error('Video exceeds 1920px on one edge. Capture a smaller clip.');
    }
    const lowerName = (file.name || '').toLowerCase();
    let extension = '.mp4';
    if (lowerName.endsWith('.webm') || file.type === 'video/webm') {
      extension = '.webm';
    } else if (lowerName.endsWith('.mov') || file.type === 'video/quicktime') {
      extension = '.mov';
    }
    return new File([file], flowershowSanitizeUploadBase(file.name) + extension, {
      type: file.type || 'video/mp4',
      lastModified: file.lastModified || Date.now()
    });
  } finally {
    URL.revokeObjectURL(url);
  }
}

function flowershowFormatUploadBytes(bytes) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 KB';
  if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  return Math.max(1, Math.round(bytes / 1024)) + ' KB';
}

function flowershowDescribeUploadItem(item) {
  if (!item) return '';
  if (item.status === 'preparing') return item.stage || 'Preparing media...';
  if (item.status === 'uploading') return item.stage || ('Uploading ' + Math.round(item.progress || 0) + '%');
  if (item.status === 'saving') return item.stage || 'Saving media...';
  if (item.status === 'error') return item.error || 'Upload failed';
  if (item.status === 'done') return item.stage || 'Uploaded';
  if (item.details) return item.details;
  return flowershowFormatUploadBytes(item.file && item.file.size);
}

function flowershowCreateUploadItem(file) {
  const lowerName = (file && file.name ? file.name : '').toLowerCase();
  const mime = (file && file.type ? file.type : '').toLowerCase();
  const kind = flowershowVideoLike(file) ? 'video' : 'photo';
  let previewURL = '';
  if ((kind === 'photo' && (/^image\//.test(mime) || /\.(jpe?g|png|webp|heic|heif)$/i.test(lowerName))) || (kind === 'video' && (/^video\//.test(mime) || /\.(mp4|webm|mov)$/i.test(lowerName)))) {
    previewURL = URL.createObjectURL(file);
  }
  return {
    id: 'upload_' + Math.random().toString(36).slice(2, 10),
    kind: kind,
    file: file,
    originalName: file && file.name ? file.name : (kind === 'video' ? 'capture.mov' : 'capture.jpg'),
    progress: 4,
    status: 'preparing',
    stage: kind === 'video' ? 'Checking video…' : 'Preparing photo…',
    details: '',
    previewURL: previewURL
  };
}

function flowershowGetIntakeUploadState(form) {
  let state = flowershowIntakeUploadStates.get(form);
  if (state) return state;
  state = {
    items: [],
    queue: form ? form.querySelector('[data-intake-upload-queue]') : null,
    uploading: false
  };
  flowershowIntakeUploadStates.set(form, state);
  return state;
}

function flowershowRenderIntakeUploadQueue(form) {
  const state = flowershowGetIntakeUploadState(form);
  const queue = state.queue;
  if (!queue) return;
  queue.innerHTML = '';
  if (state.items.length === 0) {
    queue.hidden = true;
    return;
  }
  queue.hidden = false;
  state.items.forEach(function(item) {
    const card = document.createElement('article');
    card.className = 'intake-upload-card';
    if (item.status === 'preparing' || item.status === 'uploading' || item.status === 'saving') card.classList.add('is-uploading');
    if (item.status === 'error') card.classList.add('is-error');

    const preview = document.createElement('div');
    preview.className = 'intake-upload-preview';

    const kind = document.createElement('div');
    kind.className = 'intake-upload-kind';
    kind.textContent = item.kind;
    preview.appendChild(kind);

    if (!item.previewURL) {
      const fallback = document.createElement('div');
      fallback.className = 'intake-upload-fallback';
      fallback.textContent = item.kind === 'video' ? 'Video' : 'Photo';
      preview.appendChild(fallback);
    } else if (item.kind === 'video') {
      const video = document.createElement('video');
      video.src = item.previewURL;
      video.muted = true;
      video.playsInline = true;
      video.loop = true;
      video.autoplay = true;
      preview.appendChild(video);
    } else {
      const image = document.createElement('img');
      image.src = item.previewURL;
      image.alt = '';
      preview.appendChild(image);
    }

    const progress = document.createElement('div');
    progress.className = 'intake-upload-progress';
    const bar = document.createElement('div');
    bar.className = 'intake-upload-progress-bar';
    bar.style.width = String(Math.max(0, Math.min(100, item.progress || 0))) + '%';
    progress.appendChild(bar);
    preview.appendChild(progress);
    card.appendChild(preview);

    const status = document.createElement('div');
    status.className = 'intake-upload-status';
    const title = document.createElement('strong');
    title.textContent = item.originalName || (item.file && item.file.name) || 'media';
    const meta = document.createElement('span');
    meta.textContent = flowershowDescribeUploadItem(item);
    status.appendChild(title);
    status.appendChild(meta);
    card.appendChild(status);

    const remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'intake-upload-remove';
    remove.textContent = '×';
    remove.setAttribute('aria-label', 'Remove ' + (item.originalName || (item.file && item.file.name) || 'media'));
    remove.addEventListener('click', function() {
      if (state.uploading) return;
      URL.revokeObjectURL(item.previewURL);
      state.items = state.items.filter(function(candidate) {
        return candidate.id !== item.id;
      });
      flowershowRenderIntakeUploadQueue(form);
    });
    card.appendChild(remove);

    queue.appendChild(card);
  });
}

function flowershowResetIntakeUploadState(form) {
  if (!form) return;
  const state = flowershowGetIntakeUploadState(form);
  state.items.forEach(function(item) {
    if (item.previewURL) {
      URL.revokeObjectURL(item.previewURL);
    }
  });
  state.items = [];
  state.uploading = false;
  form.querySelectorAll('[data-intake-media-input]').forEach(function(input) {
    input.value = '';
  });
  flowershowRenderIntakeUploadQueue(form);
}

async function flowershowPrepareCaptureItem(file) {
  if (flowershowLooksLikeHeic(file)) {
    throw new Error('HEIC/HEIF is not supported. Capture JPEG or PNG instead.');
  }
  if (flowershowPhotoLike(file)) {
    try {
      const primary = await flowershowNormalizeImage(file, FLOWERSHOW_MAX_PHOTO_EDGE, 0.86);
      let normalized = primary;
      if (primary.file.size > FLOWERSHOW_MAX_PHOTO_BYTES) {
        normalized = await flowershowNormalizeImage(file, FLOWERSHOW_MAX_PHOTO_EDGE, 0.78);
      }
      return {
        id: 'upload_' + Math.random().toString(36).slice(2, 10),
        kind: 'photo',
        file: normalized.file,
        originalName: file.name,
        progress: 0,
        status: 'ready',
        stage: '',
        details: flowershowFormatUploadBytes(normalized.file.size) + ' · JPEG · ' + normalized.width + '×' + normalized.height,
        previewURL: URL.createObjectURL(normalized.file)
      };
    } catch (error) {
      const lowerName = (file.name || '').toLowerCase();
      const mime = (file.type || '').toLowerCase();
      const fallbackAllowed = /^image\/(jpeg|png|webp)$/i.test(mime) || /\.(jpe?g|png|webp)$/i.test(lowerName);
      if (!fallbackAllowed) {
        throw error;
      }
      return {
        id: 'upload_' + Math.random().toString(36).slice(2, 10),
        kind: 'photo',
        file: new File([file], flowershowSanitizeUploadBase(file.name) + (lowerName.endsWith('.png') ? '.png' : lowerName.endsWith('.webp') ? '.webp' : '.jpg'), {
          type: file.type || 'image/jpeg',
          lastModified: file.lastModified || Date.now()
        }),
        originalName: file.name,
        progress: 0,
        status: 'ready',
        stage: '',
        details: flowershowFormatUploadBytes(file.size) + ' · original file',
        previewURL: URL.createObjectURL(file)
      };
    }
  }
  if (flowershowVideoLike(file)) {
    const normalized = await flowershowNormalizeVideo(file);
    return {
      id: 'upload_' + Math.random().toString(36).slice(2, 10),
      kind: 'video',
      file: normalized,
      originalName: file.name,
      progress: 0,
      status: 'ready',
      stage: '',
      details: flowershowFormatUploadBytes(normalized.size) + ' · video',
      previewURL: URL.createObjectURL(normalized)
    };
  }
  throw new Error('Unsupported media type. Use JPEG, PNG, MP4, WebM, or MOV.');
}

async function flowershowQueueCaptureFiles(form, files) {
  const state = flowershowGetIntakeUploadState(form);
  let firstError = '';
  for (const file of Array.from(files || [])) {
    const item = flowershowCreateUploadItem(file);
    state.items.push(item);
    flowershowRenderIntakeUploadQueue(form);
    try {
      const prepared = await flowershowPrepareCaptureItem(file);
      if (item.previewURL && prepared.previewURL && item.previewURL !== prepared.previewURL) {
        URL.revokeObjectURL(item.previewURL);
      }
      item.kind = prepared.kind;
      item.file = prepared.file;
      item.originalName = prepared.originalName || item.originalName;
      item.progress = prepared.progress || 0;
      item.status = prepared.status || 'ready';
      item.stage = prepared.stage || '';
      item.details = prepared.details || '';
      item.previewURL = prepared.previewURL || item.previewURL;
    } catch (error) {
      item.progress = 100;
      item.status = 'error';
      item.error = error && error.message ? error.message : 'Could not prepare media.';
      item.stage = '';
      if (!firstError) {
        firstError = item.error;
      }
    }
    flowershowRenderIntakeUploadQueue(form);
  }
  return { firstError: firstError };
}

function flowershowDistributeUploadProgress(items, loaded, total) {
  if (!items.length) return;
  if (!Number.isFinite(total) || total <= 0) {
    items.forEach(function(item) {
      item.progress = 100;
    });
    return;
  }
  let remaining = loaded;
  items.forEach(function(item) {
    const size = Math.max(1, item.file.size || 1);
    const itemLoaded = Math.max(0, Math.min(size, remaining));
    item.progress = Math.max(0, Math.min(100, Math.round((itemLoaded / size) * 100)));
    remaining -= itemLoaded;
  });
}

function flowershowSwapAdminTarget(targetSelector, html) {
  const target = document.querySelector(targetSelector || '#admin-intake-panel');
  if (!target) return;
  target.innerHTML = html;
  if (window.htmx) {
    window.htmx.process(target);
  }
  flowershowInit(target);
}

function flowershowEntryClassValue(form) {
  if (!form) return '';
  const classInput = form.querySelector('[name="class_id"]');
  return classInput ? (classInput.value || '').trim() : '';
}

function flowershowSetIntakeFormAction(form, action) {
  if (!form || !action) return;
  form.action = action;
  form.setAttribute('action', action);
  form.setAttribute('hx-post', action);
}

function flowershowEnsureHiddenFormValue(form, name, value) {
  if (!form || !name) return;
  let input = form.querySelector('input[type="hidden"][name="' + name + '"]');
  if (!input) {
    input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    form.appendChild(input);
  }
  input.value = value || '';
}

function flowershowMarkIntakeDraftEntry(form, payload) {
  if (!form || !payload) return;
  const entryID = payload.entry_id || '';
  const updateURL = payload.update_url || '';
  if (!entryID || !updateURL) return;
  form.dataset.intakeDraftEntryId = entryID;
  form.dataset.intakeCreatedEntryId = entryID;
  form.setAttribute('data-intake-draft-entry', entryID);
  flowershowSetIntakeFormAction(form, updateURL);
  flowershowEnsureHiddenFormValue(form, 'section', 'intake');
}

function flowershowClearIntakeDraftEntry(form) {
  if (!form) return;
  delete form.dataset.intakeDraftEntryId;
  delete form.dataset.intakeCreatedEntryId;
  form.removeAttribute('data-intake-draft-entry');
}

function flowershowSubmitIntakeForm(form, options) {
  const closeModal = !options || options.closeModal !== false;
  const onSuccess = options && typeof options.onSuccess === 'function' ? options.onSuccess : null;
  const onError = options && typeof options.onError === 'function' ? options.onError : null;
  const skipSwap = !!(options && options.skipSwap);
  const jsonResponse = !!(options && options.jsonResponse);
  const silent = !!(options && options.silent);
  const state = flowershowGetIntakeUploadState(form);
  const submitButtons = Array.from(form.querySelectorAll('button[type="submit"]'));
  const formData = new FormData(form);
  if (jsonResponse) {
    formData.set('response', 'json');
  }
  if (silent) {
    formData.set('_silent', '1');
  }
  state.items.forEach(function(item) {
    formData.append('media', item.file, item.file.name);
  });
  state.items.forEach(function(item) {
    item.status = 'uploading';
    item.progress = 0;
    item.stage = 'Uploading…';
  });
  state.uploading = true;
  flowershowRenderIntakeUploadQueue(form);
  submitButtons.forEach(function(button) {
    button.disabled = true;
  });
  const xhr = new XMLHttpRequest();
  xhr.open('POST', form.action);
  xhr.setRequestHeader('HX-Request', 'true');
  if (silent) {
    xhr.setRequestHeader('X-Flowershow-Silent', 'true');
  }
  if (jsonResponse) {
    xhr.setRequestHeader('Accept', 'application/json');
  }
  xhr.upload.addEventListener('progress', function(event) {
    flowershowDistributeUploadProgress(state.items, event.loaded, event.total);
    state.items.forEach(function(item) {
      item.stage = 'Uploading ' + Math.round(item.progress || 0) + '%';
    });
    flowershowRenderIntakeUploadQueue(form);
  });
  xhr.upload.addEventListener('load', function() {
    state.items.forEach(function(item) {
      item.progress = 100;
      item.status = 'saving';
      item.stage = 'Saving media…';
    });
    flowershowRenderIntakeUploadQueue(form);
  });
  xhr.addEventListener('load', function() {
    state.uploading = false;
    submitButtons.forEach(function(button) {
      button.disabled = false;
    });
    if (xhr.status < 200 || xhr.status >= 300) {
      const message = flowershowFriendlyErrorMessage(xhr.responseText, 'Upload failed.');
      state.items.forEach(function(item) {
        item.status = 'error';
        item.error = message;
      });
      flowershowRenderIntakeUploadQueue(form);
      flowershowToast(message, true);
      if (onError) {
        onError(new Error(message));
      }
      return;
    }
    let responsePayload = null;
    if (jsonResponse) {
      try {
        responsePayload = xhr.responseText ? JSON.parse(xhr.responseText) : {};
      } catch (error) {
        const message = 'Entry saved, but the server returned an unexpected response.';
        state.items.forEach(function(item) {
          item.status = 'error';
          item.error = message;
        });
        flowershowRenderIntakeUploadQueue(form);
        flowershowToast(message, true);
        if (onError) {
          onError(new Error(message));
        }
        return;
      }
      flowershowMarkIntakeDraftEntry(form, responsePayload);
    }
    state.items.forEach(function(item) {
      item.progress = 100;
      item.status = 'done';
      item.error = '';
      item.stage = 'Saved';
    });
    flowershowRenderIntakeUploadQueue(form);
    if (closeModal) {
      const modal = form.closest('[data-intake-modal]');
      if (modal) {
        flowershowCloseIntakeModal(modal);
      }
    }
    flowershowResetIntakeUploadState(form);
    if (!skipSwap && !jsonResponse) {
      flowershowSwapAdminTarget(form.dataset.target || '#admin-intake-panel', xhr.responseText || '');
    }
    if (onSuccess) {
      onSuccess(responsePayload);
    }
    document.body.dispatchEvent(new CustomEvent('flowershow:media-ready'));
  });
  xhr.addEventListener('error', function() {
    state.uploading = false;
    submitButtons.forEach(function(button) {
      button.disabled = false;
    });
    state.items.forEach(function(item) {
      item.status = 'error';
      item.error = 'Network error while uploading';
    });
    flowershowRenderIntakeUploadQueue(form);
    flowershowToast('Upload failed. Check the connection and try again.', true);
    if (onError) {
      onError(new Error('Upload failed. Check the connection and try again.'));
    }
  });
  xhr.send(formData);
}

function flowershowBindIntakeCaptureInput(input) {
  if (!input || input.dataset.bound === 'true') return;
  input.dataset.bound = 'true';
  input.addEventListener('change', async function() {
    const form = input.closest('form');
    try {
      const result = await flowershowQueueCaptureFiles(form, input.files);
      if (result && result.firstError) {
        flowershowToast(result.firstError, true);
        if (form && form.dataset.intakeAutosave === 'true') {
          flowershowSetAutosaveStatus(form, result.firstError, true);
        }
      }
      const state = form ? flowershowGetIntakeUploadState(form) : null;
      const hasReadyItems = !!(state && state.items.some(function(item) {
        return item.status === 'ready';
      }));
      if (form && form.hasAttribute('data-intake-edit-form') && hasReadyItems) {
        await flowershowSubmitQueuedMediaForm(form, {
          action: form.dataset.intakeUploadAction || '',
          target: form.dataset.target || '#admin-intake-panel',
          section: 'intake'
        });
        flowershowSetAutosaveStatus(form, 'Media saved.', false);
      } else if (form && form.dataset.intakeAutosave === 'true' && hasReadyItems) {
        await flowershowSubmitAutosaveForm(form, { keepMessage: true, closeModal: false });
      } else if (form && form.hasAttribute('data-corrections-media-form') && hasReadyItems) {
        await flowershowSubmitQueuedMediaForm(form);
      } else if (form && form.hasAttribute('data-intake-entry-form') && hasReadyItems) {
        if (!flowershowEntryClassValue(form)) {
          flowershowToast('Choose a class first, then media will upload immediately.', true);
          return;
        }
        const entrantInput = form.querySelector('[data-intake-entrant-input]');
        flowershowSyncEntrantLookup(entrantInput);
        await new Promise(function(resolve, reject) {
          flowershowSubmitIntakeForm(form, {
            closeModal: false,
            skipSwap: true,
            jsonResponse: true,
            silent: true,
            onSuccess: function(payload) {
              flowershowMarkIntakeDraftEntry(form, payload);
              resolve();
            },
            onError: reject
          });
        });
      }
    } catch (error) {
      const message = flowershowFriendlyErrorMessage(error && error.message, 'Could not prepare media.');
      flowershowToast(message, true);
      if (form && form.dataset.intakeAutosave === 'true') {
        flowershowSetAutosaveStatus(form, message, true);
      }
    } finally {
      input.value = '';
    }
  });
}

function flowershowSubmitQueuedMediaForm(form, options) {
  const state = flowershowGetIntakeUploadState(form);
  const items = state.items.filter(function(item) {
    return item && item.status === 'ready' && item.file;
  });
  if (state.uploading || items.length === 0) {
    return Promise.resolve();
  }
  const buttons = Array.from(form.querySelectorAll('[data-intake-media-button]'));
  const action = options && options.action ? String(options.action) : form.action;
  if (!action) {
    return Promise.reject(new Error('Media upload URL is missing.'));
  }
  const formData = new FormData();
  const section = options && options.section ? String(options.section) : '';
  if (section) {
    formData.set('section', section);
  } else {
    const sectionInput = form.querySelector('[name="section"]');
    if (sectionInput && sectionInput.value) {
      formData.set('section', sectionInput.value);
    }
  }
  items.forEach(function(item) {
    formData.append('media', item.file, item.file.name);
    item.status = 'uploading';
    item.progress = 0;
    item.stage = 'Uploading…';
  });
  state.uploading = true;
  flowershowRenderIntakeUploadQueue(form);
  buttons.forEach(function(button) {
    button.disabled = true;
  });
  return new Promise(function(resolve, reject) {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', action);
    xhr.setRequestHeader('HX-Request', 'true');
    xhr.upload.addEventListener('progress', function(event) {
      flowershowDistributeUploadProgress(items, event.loaded, event.total);
      items.forEach(function(item) {
        item.stage = 'Uploading ' + Math.round(item.progress || 0) + '%';
      });
      flowershowRenderIntakeUploadQueue(form);
    });
    xhr.upload.addEventListener('load', function() {
      items.forEach(function(item) {
        item.progress = 100;
        item.status = 'saving';
        item.stage = 'Saving media…';
      });
      flowershowRenderIntakeUploadQueue(form);
    });
    xhr.addEventListener('load', function() {
      state.uploading = false;
      buttons.forEach(function(button) {
        button.disabled = false;
      });
      if (xhr.status < 200 || xhr.status >= 300) {
        const message = flowershowFriendlyErrorMessage(xhr.responseText, 'Media upload failed.');
        items.forEach(function(item) {
          item.status = 'error';
          item.error = message;
        });
        flowershowRenderIntakeUploadQueue(form);
        flowershowToast(message, true);
        reject(new Error(message));
        return;
      }
      items.forEach(function(item) {
        item.progress = 100;
        item.status = 'done';
        item.error = '';
        item.stage = 'Saved';
      });
      flowershowRenderIntakeUploadQueue(form);
      flowershowResetIntakeUploadState(form);
      const target = options && options.target ? String(options.target) : (form.dataset.target || '#admin-floor-panel');
      flowershowSwapAdminTarget(target, xhr.responseText || '');
      document.body.dispatchEvent(new CustomEvent('flowershow:media-ready'));
      resolve();
    });
    xhr.addEventListener('error', function() {
      state.uploading = false;
      buttons.forEach(function(button) {
        button.disabled = false;
      });
      items.forEach(function(item) {
        item.status = 'error';
        item.error = 'Network error while uploading';
      });
      flowershowRenderIntakeUploadQueue(form);
      flowershowToast('Upload failed. Check the connection and try again.', true);
      reject(new Error('Upload failed. Check the connection and try again.'));
    });
    xhr.send(formData);
  });
}

function flowershowSetAutosaveStatus(form, message, isError) {
  if (!form) return;
  const node = form.querySelector('[data-intake-autosave-status]');
  if (!node) return;
  node.textContent = message || '';
  node.classList.toggle('is-error', !!(isError && message));
  node.classList.toggle('is-success', !!(!isError && message));
}

function flowershowClearAutosaveTimer(form) {
  const timer = flowershowIntakeAutosaveTimers.get(form);
  if (timer) {
    clearTimeout(timer);
    flowershowIntakeAutosaveTimers.delete(form);
  }
}

async function flowershowSubmitAutosaveForm(form, options) {
  const closeModal = !!(options && options.closeModal);
  const keepMessage = !!(options && options.keepMessage);
  const silent = !options || options.silent !== false;
  const successMessage = options && options.successMessage ? String(options.successMessage) : '';
  const entrantInput = form.querySelector('[data-intake-entrant-input]');
  flowershowSyncEntrantLookup(entrantInput);
  flowershowClearAutosaveTimer(form);
  flowershowSetAutosaveStatus(form, 'Saving…', false);
  try {
    await new Promise(function(resolve, reject) {
      const modal = form.closest('[data-intake-modal]');
      flowershowSubmitIntakeForm(form, {
        closeModal: closeModal,
        skipSwap: !!modal,
        silent: silent,
        onSuccess: resolve,
        onError: reject
      });
    });
    flowershowSetAutosaveStatus(form, keepMessage ? 'Saved.' : '', false);
    if (successMessage) {
      flowershowToast(successMessage, false);
    }
  } catch (error) {
    const message = flowershowFriendlyErrorMessage(error && error.message, 'Could not save.');
    flowershowSetAutosaveStatus(form, message, true);
    flowershowToast(message, true);
  }
}

function flowershowScheduleAutosave(form, delay) {
  if (!form || form.dataset.intakeAutosave !== 'true') return;
  flowershowClearAutosaveTimer(form);
  const timeout = window.setTimeout(function() {
    flowershowSubmitAutosaveForm(form, { keepMessage: true });
  }, Math.max(0, delay || 0));
  flowershowIntakeAutosaveTimers.set(form, timeout);
}

function flowershowSubmitConfirmedResultEdit(form) {
  flowershowSubmitAutosaveForm(form, {
    keepMessage: true,
    closeModal: true,
    successMessage: 'Saved.'
  });
}

function flowershowBindIntakeCaptureButton(button) {
  if (!button || button.dataset.bound === 'true') return;
  button.dataset.bound = 'true';
  button.addEventListener('click', function() {
    const form = button.closest('form');
    const kind = button.dataset.intakeMediaButton || 'capture';
    const input = form && form.querySelector('[data-intake-media-input="' + kind + '"]');
    if (input) {
      input.click();
    }
  });
}

function flowershowBindIntakeForm(form, options) {
  if (!form || form.dataset.intakeFormBound === 'true') return;
  form.dataset.intakeFormBound = 'true';
  flowershowRenderIntakeUploadQueue(form);
  form.querySelectorAll('[data-intake-entrant-input]').forEach(flowershowBindIntakeEntrantInput);
  form.querySelectorAll('[data-intake-media-button]').forEach(flowershowBindIntakeCaptureButton);
  form.querySelectorAll('[data-intake-media-input]').forEach(flowershowBindIntakeCaptureInput);
  form.addEventListener('submit', function(event) {
    event.preventDefault();
    if (flowershowGetIntakeUploadState(form).uploading) {
      flowershowToast('Wait for the current media upload to finish before saving.', true);
      return;
    }
    const entrantInput = form.querySelector('[data-intake-entrant-input]');
    flowershowSyncEntrantLookup(entrantInput);
    if (form.hasAttribute('data-intake-entry-form') && !flowershowEntryClassValue(form)) {
      flowershowToast('Choose a class before saving.', true);
      return;
    }
    flowershowSubmitIntakeForm(form, options);
  });
  if (form.dataset.intakeAutosave === 'true') {
    form.querySelectorAll('input[type="text"], textarea').forEach(function(input) {
      input.addEventListener('input', function() {
        flowershowScheduleAutosave(form, 450);
      });
      input.addEventListener('change', function() {
        flowershowScheduleAutosave(form, 0);
      });
    });
    form.querySelectorAll('select').forEach(function(select) {
      select.addEventListener('change', function() {
        flowershowScheduleAutosave(form, 0);
      });
    });
  }
}

function flowershowBindCorrectionsMediaForm(form) {
  if (!form || form.dataset.correctionsMediaBound === 'true') return;
  form.dataset.correctionsMediaBound = 'true';
  flowershowRenderIntakeUploadQueue(form);
  form.querySelectorAll('[data-intake-media-button]').forEach(flowershowBindIntakeCaptureButton);
  form.querySelectorAll('[data-intake-media-input]').forEach(flowershowBindIntakeCaptureInput);
}

function flowershowBindIntakeResultsForm(form) {
  if (!form || form.dataset.intakeResultsBound === 'true') return;
  form.dataset.intakeResultsBound = 'true';
  const placementInput = form.querySelector('[data-intake-placement-input]');
  const specialInput = form.querySelector('[data-intake-special-status-input]');
  const awardInput = form.querySelector('[data-intake-award-input]');
  const awardToggle = form.querySelector('[data-intake-award-toggle]');
  const awardSelect = form.querySelector('[data-intake-award-select]');
  const resultHelp = form.querySelector('[data-intake-result-help]');

  function setPendingConfirm(token) {
    form.dataset.intakePendingConfirm = token || '';
    if (resultHelp) {
      if (token) {
        resultHelp.textContent = flowershowIntakeConfirmMessage(form, token);
      } else {
        resultHelp.textContent = flowershowIntakeIdleMessage(form);
      }
    }
    flowershowRefreshResultButtons(form);
  }

  form.querySelectorAll('[data-intake-placement-button]').forEach(function(button) {
    button.addEventListener('click', function() {
      if (!placementInput) return;
      const next = button.dataset.intakePlacementButton || '0';
      const current = String(parseInt(placementInput.value || '0', 10) || 0);
      const isSameSelection = current === next;
      const token = 'placement:' + next;
      const revokeToken = 'revoke:placement:' + next;
      const isConfirm = form.dataset.intakePendingConfirm === token;
      const isRevokeConfirm = form.dataset.intakePendingConfirm === revokeToken;
      if (isConfirm) {
        placementInput.value = next;
        flowershowRefreshPlacementButtons(form);
        flowershowRefreshAwardState(form);
        setPendingConfirm('');
        if (form.dataset.intakeAutosave === 'true') {
          flowershowSubmitConfirmedResultEdit(form);
        } else {
          form.requestSubmit();
        }
        return;
      }
      if (isSameSelection) {
        if (isRevokeConfirm) {
          placementInput.value = '0';
          flowershowRefreshPlacementButtons(form);
          flowershowRefreshAwardState(form);
          setPendingConfirm('');
          if (form.dataset.intakeAutosave === 'true') {
            flowershowSubmitConfirmedResultEdit(form);
          } else {
            form.requestSubmit();
          }
          return;
        }
        flowershowRefreshPlacementButtons(form);
        flowershowRefreshAwardState(form);
        setPendingConfirm(revokeToken);
        return;
      }
      placementInput.value = next;
      flowershowRefreshPlacementButtons(form);
      flowershowRefreshAwardState(form);
      setPendingConfirm(token);
    });
  });
  if (awardToggle) {
    awardToggle.addEventListener('click', function() {
      const token = 'special:true';
      const revokeToken = 'revoke:special';
      const nextActive = awardToggle.getAttribute('aria-pressed') !== 'true';
      const isConfirm = nextActive && form.dataset.intakePendingConfirm === token;
      const isRevokeConfirm = !nextActive && form.dataset.intakePendingConfirm === revokeToken;
      if (form.dataset.intakePendingConfirm === token) {
        awardToggle.dataset.forceOpen = 'true';
        if (specialInput) {
          specialInput.value = 'true';
        }
        flowershowRefreshAwardState(form);
        setPendingConfirm('');
        if (form.dataset.intakeAutosave === 'true') {
          flowershowSubmitConfirmedResultEdit(form);
        } else {
          form.requestSubmit();
        }
        return;
      }
      if (!nextActive) {
        if (isRevokeConfirm) {
          awardToggle.dataset.forceOpen = '';
          if (specialInput) {
            specialInput.value = 'false';
          }
          if (awardInput) awardInput.value = '';
          if (awardSelect) awardSelect.value = '';
          flowershowRefreshAwardState(form);
          setPendingConfirm('');
          if (form.dataset.intakeAutosave === 'true') {
            flowershowSubmitConfirmedResultEdit(form);
          } else {
            form.requestSubmit();
          }
          return;
        }
        flowershowRefreshAwardState(form);
        setPendingConfirm(revokeToken);
        return;
      }
      awardToggle.dataset.forceOpen = nextActive ? 'true' : '';
      if (specialInput) {
        specialInput.value = nextActive ? 'true' : 'false';
      }
      flowershowRefreshAwardState(form);
      if (nextActive && awardSelect) {
        awardSelect.focus();
      }
      if (nextActive) {
        setPendingConfirm(token);
      }
    });
  }
  if (awardSelect) {
    awardSelect.addEventListener('change', function() {
      if (awardInput) awardInput.value = awardSelect.value || '';
      if (specialInput && awardSelect.value) {
        specialInput.value = 'true';
      }
      if (awardToggle && specialInput) {
        awardToggle.dataset.forceOpen = specialInput.value === 'true' ? 'true' : '';
      }
      flowershowRefreshAwardState(form);
      setPendingConfirm('');
      if (form.dataset.intakeAutosave === 'true') {
        flowershowSubmitConfirmedResultEdit(form);
      } else {
        form.requestSubmit();
      }
    });
  }
  flowershowRefreshPlacementButtons(form);
  flowershowRefreshAwardState(form);

  form.addEventListener('submit', async function(event) {
    event.preventDefault();
    try {
      const formData = new FormData(form);
      formData.set('_silent', '1');
      const response = await fetch(form.action, {
        method: 'POST',
        body: formData,
        credentials: 'same-origin',
        headers: { 'HX-Request': 'true', 'X-Flowershow-Silent': 'true' }
      });
      const html = await response.text();
      if (!response.ok) {
        throw new Error(flowershowFriendlyErrorMessage(html, 'Could not save result.'));
      }
      setPendingConfirm('');
      const modal = form.closest('[data-intake-modal]');
      if (modal) {
        flowershowCloseIntakeModal(modal);
      }
      flowershowSetAutosaveStatus(form, form.dataset.intakeAutosave === 'true' ? 'Saved.' : '', false);
      flowershowToast('Saved.', false);
      flowershowSwapAdminTarget(form.dataset.target || '#admin-intake-panel', html || '');
    } catch (error) {
      setPendingConfirm('');
      const message = flowershowFriendlyErrorMessage(error && error.message, 'Could not save result.');
      flowershowSetAutosaveStatus(form, message, true);
      flowershowToast(message, true);
    }
  });
}

async function flowershowCopyTarget(button) {
  const targetSelector = button.dataset.copyTarget;
  if (!targetSelector) return;
  const target = document.querySelector(targetSelector);
  if (!target) return;
  const text = typeof target.value === 'string' ? target.value : (target.textContent || '').trim();
  if (!text) return;

  if (navigator.clipboard && navigator.clipboard.writeText) {
    await navigator.clipboard.writeText(text);
  } else if (target.select) {
    target.select();
    document.execCommand('copy');
  }

  const label = button.querySelector('[data-copy-label]');
  const original = button.dataset.copyOriginal || (label ? label.textContent : button.textContent);
  if (!button.dataset.copyOriginal) {
    button.dataset.copyOriginal = original || 'Copy';
  }
  if (label) {
    label.textContent = button.dataset.copyFeedback || 'Copied';
  } else {
    button.textContent = button.dataset.copyFeedback || 'Copied';
  }
  button.classList.add('is-copied');
  window.setTimeout(function() {
    if (label) {
      label.textContent = button.dataset.copyOriginal || 'Copy';
    } else {
      button.textContent = button.dataset.copyOriginal || 'Copy';
    }
    button.classList.remove('is-copied');
  }, 1800);
}

function flowershowBindCopyButton(button) {
  if (button.dataset.bound === 'true') return;
  button.dataset.bound = 'true';
  button.addEventListener('click', async function() {
    try {
      await flowershowCopyTarget(button);
    } catch (error) {
      flowershowToast('Could not copy the token. Copy it manually from the field.', true);
    }
  });
}

function flowershowBindCountdownButton(button) {
  if (button.dataset.bound === 'true') return;
  button.dataset.bound = 'true';
  var remaining = parseInt(button.dataset.countdownSeconds || '0', 10);
  if (!Number.isFinite(remaining) || remaining <= 0) {
    button.disabled = false;
    return;
  }

  var readyLabel = button.dataset.countdownReadyLabel || 'Request another code';
  button.disabled = true;

  function renderCountdown(seconds) {
    if (seconds <= 0) {
      button.disabled = false;
      button.textContent = readyLabel;
      delete button.dataset.countdownSeconds;
      return;
    }
    button.textContent = 'You can request another code in ' + seconds + 's';
  }

  renderCountdown(remaining);
  var timer = window.setInterval(function() {
    remaining -= 1;
    renderCountdown(remaining);
    if (remaining <= 0) {
      window.clearInterval(timer);
    }
  }, 1000);
}

function flowershowActivateAgentTab(widget, tabName) {
  if (!widget || !tabName) return;
  widget.setAttribute('data-agent-active-tab', tabName);
  widget.querySelectorAll('[data-agent-tab-trigger]').forEach(function(button) {
    const active = button.dataset.agentTabTrigger === tabName;
    button.classList.toggle('is-active', active);
    button.setAttribute('aria-selected', active ? 'true' : 'false');
  });
  widget.querySelectorAll('[data-agent-tab-panel]').forEach(function(panel) {
    const active = panel.dataset.agentTabPanel === tabName;
    panel.classList.toggle('is-active', active);
    panel.hidden = !active;
  });
}

function flowershowBindAgentWidget(widget) {
  if (!widget || widget.dataset.bound === 'true') return;
  widget.dataset.bound = 'true';

  const activeTrigger = widget.querySelector('[data-agent-tab-trigger].is-active');
  const defaultTab = activeTrigger && activeTrigger.dataset
    ? activeTrigger.dataset.agentTabTrigger
    : 'summary';
  flowershowActivateAgentTab(widget, defaultTab);

  widget.querySelectorAll('[data-agent-tab-trigger]').forEach(function(button) {
    button.addEventListener('click', function() {
      flowershowActivateAgentTab(widget, button.dataset.agentTabTrigger);
    });
  });
}

function flowershowBindShowRotator(container) {
  if (!container || container.dataset.bound === 'true') return;
  const frames = Array.from(container.querySelectorAll('.show-visual-frame'));
  if (frames.length < 2) return;
  container.dataset.bound = 'true';
  let index = frames.findIndex(function(frame) {
    return frame.classList.contains('is-active');
  });
  if (index < 0) index = 0;
  window.setInterval(function() {
    frames[index].classList.remove('is-active');
    index = (index + 1) % frames.length;
    frames[index].classList.add('is-active');
  }, 5000);
}

function flowershowToggleNav(shell, forceOpen) {
  if (!shell) return;
  const currentlyOpen = shell.getAttribute('data-nav-open') === 'true';
  const nextOpen = typeof forceOpen === 'boolean' ? forceOpen : !currentlyOpen;
  shell.setAttribute('data-nav-open', nextOpen ? 'true' : 'false');
  const toggle = shell.querySelector('[data-nav-toggle]');
  if (toggle) {
    toggle.setAttribute('aria-expanded', nextOpen ? 'true' : 'false');
  }
}

function flowershowBindNav(shell) {
  if (!shell || shell.dataset.bound === 'true') return;
  shell.dataset.bound = 'true';
  const toggle = shell.querySelector('[data-nav-toggle]');
  if (!toggle) return;
  flowershowToggleNav(shell, false);
  toggle.addEventListener('click', function() {
    flowershowToggleNav(shell);
  });
  shell.querySelectorAll('.nav-menu a').forEach(function(link) {
    link.addEventListener('click', function() {
      flowershowToggleNav(shell, false);
    });
  });
  window.addEventListener('resize', function() {
    if (window.innerWidth > 640) {
      flowershowToggleNav(shell, false);
    }
  });
}

function flowershowOpenLightbox(lightbox, trigger) {
  if (!lightbox || !trigger) return;
  const stage = lightbox.querySelector('[data-media-lightbox-stage]');
  if (!stage) return;
  const type = trigger.dataset.mediaType || 'image';
  const src = trigger.dataset.mediaSrc || '';
  const label = trigger.dataset.mediaLabel || 'Entry media';
  if (!src) return;

  stage.innerHTML = '';
  let media;
  if (type === 'video') {
    media = document.createElement('video');
    media.src = src;
    media.controls = true;
    media.autoplay = true;
    media.playsInline = true;
  } else {
    media = document.createElement('img');
    media.src = src;
    media.alt = '';
  }
  stage.appendChild(media);
  const gallery = Array.from(document.querySelectorAll('[data-media-open]'));
  const index = gallery.indexOf(trigger);
  lightbox.dataset.mediaIndex = index >= 0 ? String(index) : '';
  const meta = lightbox.querySelector('[data-media-lightbox-meta]');
  if (meta) {
    const fields = {
      '[data-media-lightbox-entry]': trigger.dataset.mediaEntry || label,
      '[data-media-lightbox-entrant]': trigger.dataset.mediaEntrant || '',
      '[data-media-lightbox-class]': trigger.dataset.mediaClass || '',
      '[data-media-lightbox-class-detail]': trigger.dataset.mediaClassDetail || '',
      '[data-media-lightbox-show]': trigger.dataset.mediaShow || ''
    };
    let hasContent = false;
    Object.keys(fields).forEach(function(selector) {
      const element = meta.querySelector(selector);
      if (!element) return;
      const value = (fields[selector] || '').trim();
      element.textContent = value;
      element.hidden = value === '';
      if (value !== '') {
        hasContent = true;
      }
    });
    meta.hidden = !hasContent;
  }
  lightbox.querySelectorAll('[data-media-prev], [data-media-next]').forEach(function(button) {
    button.hidden = gallery.length < 2;
  });
  lightbox.hidden = false;
  document.body.classList.add('body-lightbox-open');
}

function flowershowStepLightbox(lightbox, delta) {
  if (!lightbox) return;
  const gallery = Array.from(document.querySelectorAll('[data-media-open]'));
  if (gallery.length < 2) return;
  const currentIndex = parseInt(lightbox.dataset.mediaIndex || '-1', 10);
  const safeIndex = Number.isFinite(currentIndex) && currentIndex >= 0 ? currentIndex : 0;
  const nextIndex = (safeIndex + delta + gallery.length) % gallery.length;
  flowershowOpenLightbox(lightbox, gallery[nextIndex]);
}

function flowershowCloseLightbox(lightbox) {
  if (!lightbox) return;
  lightbox.hidden = true;
  const stage = lightbox.querySelector('[data-media-lightbox-stage]');
  if (stage) {
    stage.innerHTML = '';
  }
  const meta = lightbox.querySelector('[data-media-lightbox-meta]');
  if (meta) {
    meta.hidden = true;
    meta.querySelectorAll('[data-media-lightbox-entry],[data-media-lightbox-entrant],[data-media-lightbox-class],[data-media-lightbox-class-detail],[data-media-lightbox-show]').forEach(function(element) {
      element.textContent = '';
      element.hidden = true;
    });
  }
  document.body.classList.remove('body-lightbox-open');
}

function flowershowBindLightbox(lightbox) {
  if (!lightbox || lightbox.dataset.bound === 'true') return;
  lightbox.dataset.bound = 'true';
  lightbox.querySelectorAll('[data-media-close]').forEach(function(button) {
    button.addEventListener('click', function() {
      flowershowCloseLightbox(lightbox);
    });
  });
  lightbox.querySelectorAll('[data-media-prev]').forEach(function(button) {
    button.addEventListener('click', function() {
      flowershowStepLightbox(lightbox, -1);
    });
  });
  lightbox.querySelectorAll('[data-media-next]').forEach(function(button) {
    button.addEventListener('click', function() {
      flowershowStepLightbox(lightbox, 1);
    });
  });
  document.addEventListener('keydown', function(event) {
    if (event.key === 'Escape' && !lightbox.hidden) {
      flowershowCloseLightbox(lightbox);
      return;
    }
    if (lightbox.hidden) {
      return;
    }
    if (event.key === 'ArrowLeft') {
      flowershowStepLightbox(lightbox, -1);
    } else if (event.key === 'ArrowRight') {
      flowershowStepLightbox(lightbox, 1);
    }
  });
}

function flowershowBindShowAdminShell(shell) {
  if (!shell) return;
  const activeTab = shell.dataset.showAdminActiveTab || 'setup';
  flowershowActivateShowAdminTab(shell, activeTab);
}

function flowershowBindMediaTrigger(button) {
  if (!button || button.dataset.bound === 'true') return;
  button.dataset.bound = 'true';
  button.addEventListener('click', function() {
    const lightbox = document.querySelector('[data-media-lightbox]');
    flowershowOpenLightbox(lightbox, button);
  });
}

function flowershowSuppressDuplicateAgentWidgets() {
  const kernelWidgets = document.querySelectorAll('.agent-access-widget[data-agent-widget-source="kernel"]');
  if (kernelWidgets.length === 0) return;
  document.querySelectorAll('.agent-access-widget:not([data-agent-widget-source="kernel"])').forEach(function(widget) {
    const shell = widget.closest('.agent-access-shell');
    if (shell) {
      shell.remove();
      return;
    }
    widget.remove();
  });
}

function flowershowInit(root) {
  const scope = flowershowQueryScope(root);
  scope.querySelectorAll('[data-copy-target]').forEach(flowershowBindCopyButton);
  scope.querySelectorAll('[data-countdown-seconds]').forEach(flowershowBindCountdownButton);
  scope.querySelectorAll('[data-agent-widget]').forEach(flowershowBindAgentWidget);
  scope.querySelectorAll('[data-person-filter-input]').forEach(flowershowBindPersonFilter);
  scope.querySelectorAll('[data-corrections-filter-input]').forEach(flowershowBindCorrectionsFilter);
  scope.querySelectorAll('[data-intake-modal-open]').forEach(flowershowBindIntakeTrigger);
  scope.querySelectorAll('[data-intake-modal]').forEach(flowershowBindIntakeModal);
  scope.querySelectorAll('[data-intake-entry-form]').forEach(function(form) {
    flowershowBindIntakeForm(form, { isNew: true });
  });
  scope.querySelectorAll('[data-intake-results-form]').forEach(flowershowBindIntakeResultsForm);
  scope.querySelectorAll('[data-corrections-media-form]').forEach(flowershowBindCorrectionsMediaForm);
  scope.querySelectorAll('[data-show-rotator]').forEach(flowershowBindShowRotator);
  scope.querySelectorAll('[data-nav-shell]').forEach(flowershowBindNav);
  scope.querySelectorAll('[data-media-open]').forEach(flowershowBindMediaTrigger);
  scope.querySelectorAll('[data-media-lightbox]').forEach(flowershowBindLightbox);
  scope.querySelectorAll('[data-show-admin-shell]').forEach(flowershowBindShowAdminShell);
  scope.querySelectorAll('img[data-deferred-media-src]').forEach(flowershowBindDeferredMediaImage);
  const select = document.querySelector('#scorecard-form select[name="rubric_id"]');
  if (select) flowershowToggleRubricCriteria(select);
  document.querySelectorAll('[data-agent-current-path]').forEach(function(el) {
    el.textContent = window.location.pathname;
  });
  flowershowSuppressDuplicateAgentWidgets();
}

// Auto-remove toasts after 4 seconds
document.addEventListener('htmx:afterSettle', function(evt) {
  const toasts = document.querySelectorAll('.toast');
  toasts.forEach(t => {
    setTimeout(() => t.remove(), 4000);
  });
});

document.addEventListener('DOMContentLoaded', function() {
  flowershowInit(document);
});

document.addEventListener('htmx:afterSwap', function(evt) {
  flowershowInit(evt && evt.target);
});
