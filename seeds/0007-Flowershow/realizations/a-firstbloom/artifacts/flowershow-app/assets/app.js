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
    media.alt = label;
  }
  stage.appendChild(media);
  lightbox.hidden = false;
  document.body.classList.add('body-lightbox-open');
}

function flowershowCloseLightbox(lightbox) {
  if (!lightbox) return;
  lightbox.hidden = true;
  const stage = lightbox.querySelector('[data-media-lightbox-stage]');
  if (stage) {
    stage.innerHTML = '';
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
  document.addEventListener('keydown', function(event) {
    if (event.key === 'Escape' && !lightbox.hidden) {
      flowershowCloseLightbox(lightbox);
    }
  });
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
  scope.querySelectorAll('[data-show-rotator]').forEach(flowershowBindShowRotator);
  scope.querySelectorAll('[data-nav-shell]').forEach(flowershowBindNav);
  scope.querySelectorAll('[data-media-open]').forEach(flowershowBindMediaTrigger);
  scope.querySelectorAll('[data-media-lightbox]').forEach(flowershowBindLightbox);
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
