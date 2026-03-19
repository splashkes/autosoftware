// Flowershow — minimal JS (HTMX handles most interactivity)

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

function flowershowActivateAgentTab(widget, tabName) {
  if (!widget || !tabName) return;
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
    : 'access';
  flowershowActivateAgentTab(widget, defaultTab);

  widget.querySelectorAll('[data-agent-tab-trigger]').forEach(function(button) {
    button.addEventListener('click', function() {
      flowershowActivateAgentTab(widget, button.dataset.agentTabTrigger);
    });
  });
}

function flowershowInit(root) {
  const scope = root || document;
  scope.querySelectorAll('[data-photo-add-form]').forEach(flowershowBindPhotoForm);
  scope.querySelectorAll('[data-copy-target]').forEach(flowershowBindCopyButton);
  scope.querySelectorAll('[data-agent-widget]').forEach(flowershowBindAgentWidget);
  const select = document.querySelector('#scorecard-form select[name="rubric_id"]');
  if (select) flowershowToggleRubricCriteria(select);
  document.querySelectorAll('[data-agent-current-path]').forEach(function(el) {
    el.textContent = window.location.pathname;
  });
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
