// Flowershow — minimal JS (HTMX handles most interactivity)

const FLOWERSHOW_MAX_PHOTO_BYTES = 5 * 1024 * 1024;
const FLOWERSHOW_MAX_VIDEO_BYTES = 50 * 1024 * 1024;
const FLOWERSHOW_MAX_VIDEO_EDGE = 1920;
const flowershowIntakeUploadStates = new WeakMap();

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

function flowershowCloseIntakeModal(modal) {
  if (!modal) return;
  modal.querySelectorAll('[data-intake-entry-form], [data-intake-upload-form]').forEach(function(form) {
    flowershowResetIntakeUploadState(form);
  });
  modal.hidden = true;
  document.body.classList.remove('body-lightbox-open');
}

function flowershowSyncEntrantLookup(input) {
  if (!input) return false;
  const hidden = input
    .closest('[data-intake-new-panel]')
    .querySelector('[data-intake-person-id-input]');
  if (!hidden) return false;
  const listId = input.getAttribute('list');
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

function flowershowOpenIntakeModal(modal, trigger) {
  if (!modal || !trigger) return;
  const mode = trigger.dataset.intakeMode || 'new';
  const title = modal.querySelector('[data-intake-modal-title]');
  const subtitle = modal.querySelector('[data-intake-modal-subtitle]');
  const newPanel = modal.querySelector('[data-intake-new-panel]');
  const existingPanel = modal.querySelector('[data-intake-existing-panel]');
  if (!title || !subtitle || !newPanel || !existingPanel) return;

  const classLabel = trigger.dataset.intakeClassLabel || '';
  title.textContent = mode === 'existing' ? 'Update intake' : 'Add entry';
  subtitle.textContent = classLabel;

  newPanel.hidden = mode !== 'new';
  existingPanel.hidden = mode === 'new';

  if (mode === 'new') {
    const form = modal.querySelector('[data-intake-entry-form]');
    const classIDInput = modal.querySelector('[data-intake-class-id-input]');
    const classLabelInput = modal.querySelector('[data-intake-class-label-input]');
    const entrantInput = modal.querySelector('[data-intake-entrant-input]');
    const personIDInput = modal.querySelector('[data-intake-person-id-input]');
    if (form) {
      form.reset();
      form.action = trigger.dataset.intakeCreateAction || form.action;
      form.setAttribute('hx-post', trigger.dataset.intakeCreateAction || form.action);
      flowershowResetIntakeUploadState(form);
    }
    if (classIDInput) classIDInput.value = trigger.dataset.intakeClassId || '';
    if (classLabelInput) classLabelInput.value = classLabel;
    if (entrantInput) entrantInput.value = '';
    if (personIDInput) personIDInput.value = '';
  } else {
    const uploadForm = modal.querySelector('[data-intake-upload-form]');
    const entrant = modal.querySelector('[data-intake-existing-entrant]');
    const entry = modal.querySelector('[data-intake-existing-entry]');
    const classInfo = modal.querySelector('[data-intake-existing-class]');
    const entryLink = modal.querySelector('[data-intake-existing-link]');
    if (uploadForm) {
      uploadForm.reset();
      uploadForm.action = trigger.dataset.intakeUploadAction || '';
      flowershowResetIntakeUploadState(uploadForm);
    }
    if (entrant) entrant.textContent = trigger.dataset.intakeEntrant || '';
    if (entry) entry.textContent = trigger.dataset.intakeEntryName || '';
    if (classInfo) classInfo.textContent = classLabel;
    if (entryLink) entryLink.href = trigger.dataset.intakeEntryHref || '#';
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

  const entrantInput = modal.querySelector('[data-intake-entrant-input]');
  if (entrantInput) {
    entrantInput.addEventListener('input', function() {
      flowershowSyncEntrantLookup(entrantInput);
    });
    entrantInput.addEventListener('change', function() {
      flowershowSyncEntrantLookup(entrantInput);
    });
  }
  flowershowBindIntakeForm(modal.querySelector('[data-intake-entry-form]'), { isNew: true });
  flowershowBindIntakeForm(modal.querySelector('[data-intake-upload-form]'), { isNew: false });
}

function flowershowBindIntakeTrigger(button) {
  if (!button || button.dataset.bound === 'true') return;
  button.dataset.bound = 'true';
  button.addEventListener('click', function() {
    flowershowOpenIntakeModal(document.querySelector('[data-intake-modal]'), button);
  });
}

async function flowershowNormalizeImage(file) {
  const maxEdge = 2048;
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
      canvas.toBlob(resolve, 'image/jpeg', 0.86);
    });
    if (!blob) {
      throw new Error('Could not prepare photo for upload.');
    }
    const name = (file.name || 'capture')
      .replace(/\.[^.]+$/, '')
      .replace(/[^a-zA-Z0-9_-]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'capture';
    return new File([blob], name + '.jpg', { type: 'image/jpeg', lastModified: Date.now() });
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
    const empty = document.createElement('p');
    empty.className = 'intake-upload-empty';
    empty.textContent = 'No media added yet. Use Capture or Upload above.';
    queue.appendChild(empty);
    return;
  }
  state.items.forEach(function(item) {
    const card = document.createElement('article');
    card.className = 'intake-upload-card';
    if (item.status === 'uploading') card.classList.add('is-uploading');
    if (item.status === 'error') card.classList.add('is-error');

    const preview = document.createElement('div');
    preview.className = 'intake-upload-preview';

    const kind = document.createElement('div');
    kind.className = 'intake-upload-kind';
    kind.textContent = item.kind;
    preview.appendChild(kind);

    if (item.kind === 'video') {
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
    title.textContent = item.file.name;
    const meta = document.createElement('span');
    if (item.status === 'uploading') {
      meta.textContent = 'Uploading ' + Math.round(item.progress || 0) + '%';
    } else if (item.status === 'error') {
      meta.textContent = item.error || 'Upload failed';
    } else if (item.status === 'done') {
      meta.textContent = 'Uploaded';
    } else {
      meta.textContent = flowershowFormatUploadBytes(item.file.size) + ' ready';
    }
    status.appendChild(title);
    status.appendChild(meta);
    card.appendChild(status);

    const remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'intake-upload-remove';
    remove.textContent = '×';
    remove.setAttribute('aria-label', 'Remove ' + item.file.name);
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
    if (file.size > FLOWERSHOW_MAX_PHOTO_BYTES) {
      throw new Error('Photo exceeds 5 MB before optimization. Capture a smaller image.');
    }
    const normalized = await flowershowNormalizeImage(file);
    return {
      id: 'upload_' + Math.random().toString(36).slice(2, 10),
      kind: 'photo',
      file: normalized,
      progress: 0,
      status: 'ready',
      previewURL: URL.createObjectURL(normalized)
    };
  }
  if (flowershowVideoLike(file)) {
    const normalized = await flowershowNormalizeVideo(file);
    return {
      id: 'upload_' + Math.random().toString(36).slice(2, 10),
      kind: 'video',
      file: normalized,
      progress: 0,
      status: 'ready',
      previewURL: URL.createObjectURL(normalized)
    };
  }
  throw new Error('Unsupported media type. Use JPEG, PNG, MP4, WebM, or MOV.');
}

async function flowershowQueueCaptureFiles(form, files) {
  const state = flowershowGetIntakeUploadState(form);
  for (const file of Array.from(files || [])) {
    const item = await flowershowPrepareCaptureItem(file);
    state.items.push(item);
  }
  flowershowRenderIntakeUploadQueue(form);
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

function flowershowSubmitIntakeForm(form, options) {
  const isNew = !!(options && options.isNew);
  const state = flowershowGetIntakeUploadState(form);
  const submitButtons = Array.from(form.querySelectorAll('button[type="submit"]'));
  if (!isNew && state.items.length === 0) {
    flowershowToast('Capture at least one photo or video before uploading.', true);
    return;
  }
  const formData = new FormData(form);
  state.items.forEach(function(item) {
    formData.append('media', item.file, item.file.name);
  });
  state.items.forEach(function(item) {
    item.status = 'uploading';
    item.progress = 0;
  });
  state.uploading = true;
  flowershowRenderIntakeUploadQueue(form);
  submitButtons.forEach(function(button) {
    button.disabled = true;
  });
  const xhr = new XMLHttpRequest();
  xhr.open('POST', form.action);
  xhr.setRequestHeader('HX-Request', 'true');
  xhr.upload.addEventListener('progress', function(event) {
    flowershowDistributeUploadProgress(state.items, event.loaded, event.total);
    flowershowRenderIntakeUploadQueue(form);
  });
  xhr.addEventListener('load', function() {
    state.uploading = false;
    submitButtons.forEach(function(button) {
      button.disabled = false;
    });
    if (xhr.status < 200 || xhr.status >= 300) {
      state.items.forEach(function(item) {
        item.status = 'error';
        item.error = xhr.responseText || 'Upload failed';
      });
      flowershowRenderIntakeUploadQueue(form);
      flowershowToast((xhr.responseText || 'Upload failed.').replace(/<[^>]+>/g, ''), true);
      return;
    }
    state.items.forEach(function(item) {
      item.progress = 100;
      item.status = 'done';
      item.error = '';
    });
    flowershowRenderIntakeUploadQueue(form);
    const modal = form.closest('[data-intake-modal]');
    if (modal) {
      modal.hidden = true;
      document.body.classList.remove('body-lightbox-open');
    }
    flowershowResetIntakeUploadState(form);
    flowershowSwapAdminTarget(form.dataset.target || '#admin-intake-panel', xhr.responseText || '');
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
  });
  xhr.send(formData);
}

function flowershowBindIntakeCaptureInput(input) {
  if (!input || input.dataset.bound === 'true') return;
  input.dataset.bound = 'true';
  input.addEventListener('change', async function() {
    const form = input.closest('form');
    try {
      await flowershowQueueCaptureFiles(form, input.files);
    } catch (error) {
      flowershowToast(error && error.message ? error.message : 'Could not prepare media.', true);
    } finally {
      input.value = '';
    }
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
  if (!form || form.dataset.bound === 'true') return;
  form.dataset.bound = 'true';
  flowershowRenderIntakeUploadQueue(form);
  form.querySelectorAll('[data-intake-media-button]').forEach(flowershowBindIntakeCaptureButton);
  form.querySelectorAll('[data-intake-media-input]').forEach(flowershowBindIntakeCaptureInput);
  form.addEventListener('submit', function(event) {
    event.preventDefault();
    const entrantInput = form.querySelector('[data-intake-entrant-input]');
    if (options && options.isNew && entrantInput && !flowershowSyncEntrantLookup(entrantInput)) {
      flowershowToast('Choose an entrant from the full-name suggestions before saving.', true);
      return;
    }
    flowershowSubmitIntakeForm(form, options);
  });
}

async function flowershowSubmitPhotoForm(form, file) {
  const target = document.querySelector(form.dataset.target || '#admin-entries-panel');
  const button = form.querySelector('.media-add-button');
  const body = new FormData();
  body.append('media', file, file.name);
  if (button) {
    button.disabled = true;
    button.classList.add('is-uploading');
  }
  try {
    const response = await fetch(form.action, {
      method: 'POST',
      body: body,
      credentials: 'same-origin',
      headers: {
        'HX-Request': 'true'
      }
    });
    const html = await response.text();
    if (!response.ok) {
      throw new Error(html || 'Photo upload failed.');
    }
    if (target) {
      target.innerHTML = html;
      if (window.htmx) {
        window.htmx.process(target);
      }
      flowershowInit(target);
    }
  } finally {
    if (button) {
      button.disabled = false;
      button.classList.remove('is-uploading');
    }
  }
}

function flowershowBindPhotoForm(form) {
  if (form.dataset.bound === 'true') return;
  const input = form.querySelector('input[type="file"]');
  const button = form.querySelector('.media-add-button');
  if (!input || !button) return;
  form.dataset.bound = 'true';
  input.addEventListener('change', async function() {
    const file = input.files && input.files[0];
    if (!file) return;
    const lowerName = (file.name || '').toLowerCase();
    const mime = (file.type || '').toLowerCase();
    if (lowerName.endsWith('.heic') || lowerName.endsWith('.heif') || mime === 'image/heic' || mime === 'image/heif') {
      flowershowToast('HEIC/HEIF is not supported. Use JPEG, PNG, or WebP.', true);
      input.value = '';
      return;
    }
    if (!/^image\/(jpeg|png|webp)$/i.test(mime) && !/\.(jpe?g|png|webp)$/i.test(lowerName)) {
      flowershowToast('Unsupported photo type. Use JPEG, PNG, or WebP.', true);
      input.value = '';
      return;
    }
    try {
      const normalized = await flowershowNormalizeImage(file);
      const preview = URL.createObjectURL(normalized);
      button.innerHTML = '<img src="' + preview + '" alt="Selected photo" class="media-add-thumb"><span class="media-add-badge">+</span>';
      await flowershowSubmitPhotoForm(form, normalized);
    } catch (error) {
      flowershowToast(error && error.message ? error.message : 'Photo upload failed.', true);
    } finally {
      input.value = '';
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
  const scope = root || document;
  scope.querySelectorAll('[data-photo-add-form]').forEach(flowershowBindPhotoForm);
  scope.querySelectorAll('[data-copy-target]').forEach(flowershowBindCopyButton);
  scope.querySelectorAll('[data-countdown-seconds]').forEach(flowershowBindCountdownButton);
  scope.querySelectorAll('[data-agent-widget]').forEach(flowershowBindAgentWidget);
  scope.querySelectorAll('[data-person-filter-input]').forEach(flowershowBindPersonFilter);
  scope.querySelectorAll('[data-intake-modal-open]').forEach(flowershowBindIntakeTrigger);
  scope.querySelectorAll('[data-intake-modal]').forEach(flowershowBindIntakeModal);
  scope.querySelectorAll('[data-show-rotator]').forEach(flowershowBindShowRotator);
  scope.querySelectorAll('[data-nav-shell]').forEach(flowershowBindNav);
  scope.querySelectorAll('[data-media-open]').forEach(flowershowBindMediaTrigger);
  scope.querySelectorAll('[data-media-lightbox]').forEach(flowershowBindLightbox);
  scope.querySelectorAll('[data-show-admin-shell]').forEach(flowershowBindShowAdminShell);
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
  flowershowInit(evt.target);
});
