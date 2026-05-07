// Sequential photo intake — retry queue + optimistic gallery.
// Lives only on /admin/shows/{showID}/intake/photos.
(function () {
  'use strict';

  function init() {
    var shell = document.querySelector('[data-intake-shell]');
    if (!shell) return;

    var captureInput = shell.querySelector('[data-intake-capture-input]');
    var grid = shell.querySelector('[data-intake-grid]');
    var emptyMsg = shell.querySelector('[data-intake-empty]');
    var tray = shell.querySelector('[data-intake-tray]');
    var trayUploaded = shell.querySelector('[data-intake-tray-uploaded]');
    var trayUploading = shell.querySelector('[data-intake-tray-uploading]');
    var trayFailed = shell.querySelector('[data-intake-tray-failed]');
    var retryButton = shell.querySelector('[data-intake-retry]');
    var nextCta = shell.querySelector('[data-intake-next-cta]');

    var anonURL = shell.dataset.intakeAnonUrl;
    var classID = shell.dataset.intakeClassId;

    var queue = []; // { id, file, status, mediaURL, entryID, error }
    var idCounter = 0;
    function nextID() {
      idCounter += 1;
      return 'q' + idCounter;
    }

    function counts() {
      var c = { uploaded: 0, uploading: 0, failed: 0 };
      for (var i = 0; i < queue.length; i++) {
        var t = queue[i];
        if (t.status === 'ok') c.uploaded++;
        else if (t.status === 'failed') c.failed++;
        else c.uploading++;
      }
      return c;
    }

    function renderTray() {
      var c = counts();
      if (queue.length === 0) {
        if (tray) tray.hidden = true;
        return;
      }
      if (tray) tray.hidden = false;
      if (trayUploaded) trayUploaded.textContent = c.uploaded + ' uploaded';
      if (trayUploading) trayUploading.textContent = c.uploading + ' uploading';
      if (trayFailed) trayFailed.textContent = c.failed + ' failed';
      if (retryButton) retryButton.hidden = c.failed === 0;
    }

    function showNextCta() {
      if (nextCta && nextCta.hidden) nextCta.hidden = false;
    }

    function setEmptyVisibility() {
      if (!emptyMsg) return;
      if (queue.length === 0) emptyMsg.hidden = false;
      else emptyMsg.hidden = true;
    }

    function renderTask(task) {
      // Find or create the tile in the grid.
      var tile = grid.querySelector('[data-intake-tile="' + task.id + '"]');
      if (!tile) {
        tile = document.createElement('div');
        tile.className = 'intake-tile';
        tile.setAttribute('data-intake-tile', task.id);
        var img = document.createElement('img');
        img.className = 'intake-tile-img';
        img.setAttribute('data-intake-tile-img', '');
        img.alt = '';
        var status = document.createElement('div');
        status.className = 'intake-tile-status';
        status.setAttribute('data-intake-tile-status', '');
        var retry = document.createElement('button');
        retry.type = 'button';
        retry.className = 'intake-tile-retry';
        retry.setAttribute('data-intake-tile-retry', '');
        retry.textContent = 'Retry';
        retry.hidden = true;
        retry.addEventListener('click', function () {
          retryTask(task.id);
        });
        tile.appendChild(img);
        tile.appendChild(status);
        tile.appendChild(retry);
        grid.insertBefore(tile, grid.firstChild);
      }
      var imgEl = tile.querySelector('[data-intake-tile-img]');
      var statusEl = tile.querySelector('[data-intake-tile-status]');
      var retryEl = tile.querySelector('[data-intake-tile-retry]');
      if (imgEl && task.mediaURL) imgEl.src = task.mediaURL;
      tile.classList.toggle('intake-tile-uploading', task.status === 'uploading');
      tile.classList.toggle('intake-tile-ok', task.status === 'ok');
      tile.classList.toggle('intake-tile-failed', task.status === 'failed');
      if (statusEl) {
        if (task.status === 'uploading') statusEl.textContent = 'Uploading…';
        else if (task.status === 'ok') statusEl.textContent = 'Uploaded';
        else if (task.status === 'failed') statusEl.textContent = task.error || 'Failed';
      }
      if (retryEl) retryEl.hidden = task.status !== 'failed';
    }

    function startTask(task) {
      task.status = 'uploading';
      task.error = '';
      renderTask(task);
      renderTray();

      // Step 1: create anon entry.
      fetch(anonURL, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Accept': 'application/json' }
      }).then(function (resp) {
        if (!resp.ok) throw new Error('anon entry failed (' + resp.status + ')');
        return resp.json();
      }).then(function (payload) {
        if (!payload || !payload.upload_url) throw new Error('missing upload url');
        task.entryID = payload.entry_id;
        // Step 2: upload media.
        var fd = new FormData();
        fd.append('media', task.file, task.file.name || 'photo.jpg');
        fd.append('section', 'intake');
        return fetch(payload.upload_url, {
          method: 'POST',
          credentials: 'same-origin',
          body: fd,
          headers: { 'Accept': 'text/html, application/json' }
        });
      }).then(function (resp) {
        if (!resp) throw new Error('upload aborted');
        if (!resp.ok) throw new Error('upload failed (' + resp.status + ')');
        task.status = 'ok';
        renderTask(task);
        renderTray();
      }).catch(function (err) {
        task.status = 'failed';
        task.error = err && err.message ? err.message : 'failed';
        renderTask(task);
        renderTray();
      });
    }

    function retryTask(taskID) {
      for (var i = 0; i < queue.length; i++) {
        if (queue[i].id === taskID) {
          startTask(queue[i]);
          return;
        }
      }
    }

    function enqueueFile(file) {
      var task = {
        id: nextID(),
        file: file,
        status: 'uploading',
        mediaURL: null,
        entryID: null,
        error: ''
      };
      try {
        task.mediaURL = URL.createObjectURL(file);
      } catch (e) {
        task.mediaURL = null;
      }
      queue.push(task);
      setEmptyVisibility();
      renderTask(task);
      renderTray();
      startTask(task);
      // After we've ingested at least one photo, surface the next-class CTA.
      showNextCta();
    }

    if (captureInput) {
      captureInput.addEventListener('change', function (ev) {
        var files = ev.target.files;
        if (!files || files.length === 0) return;
        for (var i = 0; i < files.length; i++) {
          enqueueFile(files[i]);
        }
        // Reset so the same file can be picked again, and to keep the input
        // ready for the next sequential photo.
        try { ev.target.value = ''; } catch (e) { /* noop */ }
      });
    }

    if (retryButton) {
      retryButton.addEventListener('click', function () {
        for (var i = 0; i < queue.length; i++) {
          if (queue[i].status === 'failed') {
            startTask(queue[i]);
          }
        }
      });
    }

    // Expose for tests / debugging.
    window.__flowershowIntake = {
      queue: queue,
      retryAll: function () {
        if (retryButton) retryButton.click();
      },
      classID: classID
    };
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
