// People roster CTA modals — search-first add-judge / add-helper / add-member.
// Single shared modal swaps between three modes via data-people-modal-open
// buttons. The search input debounces against the show-scoped search endpoint
// and selecting a row collapses the edit fields into a "Using: X" pill so we
// never double-create a person.
(function () {
  "use strict";

  function init(root) {
    if (!root || root.dataset.peopleRosterBound === "1") return;
    root.dataset.peopleRosterBound = "1";

    var searchURL = root.getAttribute("data-people-search-url") || "";
    var showName = root.getAttribute("data-show-name") || "this show";
    var modal = root.querySelector("[data-people-roster-modal]");
    var form = root.querySelector("[data-people-roster-form]");
    if (!modal || !form) return;

    var titleEl = modal.querySelector("[data-people-modal-title]");
    var bannerEl = modal.querySelector("[data-people-modal-banner]");
    var personIDInput = form.querySelector("[data-people-person-id]");
    var roleHiddenInput = form.querySelector("[data-people-role-hidden]");
    var memberRoleSelect = form.querySelector("[data-people-member-role]");

    var searchInput = form.querySelector("[data-people-search-input]");
    var resultsEl = form.querySelector("[data-people-search-results]");
    var selectedEl = form.querySelector("[data-people-search-selected]");
    var selectedNameEl = form.querySelector("[data-people-search-selected-name]");
    var selectedMetaEl = form.querySelector("[data-people-search-selected-meta]");
    var changeBtn = form.querySelector("[data-people-search-change]");

    var firstNameInput = form.querySelector("[data-people-first-name]");
    var lastNameInput = form.querySelector("[data-people-last-name]");
    var editFieldsEl = form.querySelector("[data-people-edit-fields]");
    var judgeFields = form.querySelector("[data-people-fields-judge]");
    var memberFields = form.querySelector("[data-people-fields-member]");

    var currentMode = "judge";

    function setMode(mode) {
      currentMode = mode;
      var titles = {
        judge: "Add Judge",
        show_admin: "Add Show Helper Admin",
        member: "Add Member or Entrant",
      };
      titleEl.textContent = titles[mode] || "Add person";

      var actionAttr = "data-action-" + mode;
      var action = form.getAttribute(actionAttr);
      if (action) form.setAttribute("action", action);

      if (judgeFields) judgeFields.hidden = mode !== "judge";
      if (memberFields) memberFields.hidden = mode !== "member";

      if (mode === "show_admin") {
        roleHiddenInput.value = "flowershow_show_intake_operator";
      } else if (mode === "member") {
        roleHiddenInput.value = memberRoleSelect ? memberRoleSelect.value : "flowershow_entrant";
      } else {
        roleHiddenInput.value = "";
      }

      if (mode === "show_admin") {
        bannerEl.textContent =
          "This person will become a show admin for " +
          showName +
          " — they can upload photos, edit entries, set names, and mark winners.";
        bannerEl.hidden = false;
      } else {
        bannerEl.textContent = "";
        bannerEl.hidden = true;
      }
    }

    function clearSelection() {
      personIDInput.value = "";
      selectedEl.hidden = true;
      selectedNameEl.textContent = "";
      selectedMetaEl.textContent = "";
      if (editFieldsEl) editFieldsEl.hidden = false;
    }

    function applySelection(person) {
      personIDInput.value = person.id || "";
      selectedNameEl.textContent =
        ((person.first_name || "") + " " + (person.last_name || "")).trim() || "(unnamed)";
      var metaParts = [];
      if (person.email) metaParts.push(person.email);
      if (person.phone) metaParts.push(person.phone);
      selectedMetaEl.textContent = metaParts.length ? " · " + metaParts.join(" · ") : "";
      selectedEl.hidden = false;
      if (editFieldsEl) editFieldsEl.hidden = true;
      if (firstNameInput) firstNameInput.value = person.first_name || "";
      if (lastNameInput) lastNameInput.value = person.last_name || "";
      resultsEl.hidden = true;
      resultsEl.innerHTML = "";
    }

    function renderResults(rows) {
      resultsEl.innerHTML = "";
      if (!rows || !rows.length) {
        resultsEl.hidden = true;
        return;
      }
      rows.forEach(function (row) {
        var btn = document.createElement("button");
        btn.type = "button";
        btn.className = "people-search-row";
        var label = document.createElement("span");
        label.className = "people-search-row-name";
        label.textContent = ((row.first_name || "") + " " + (row.last_name || "")).trim() || "(unnamed)";
        btn.appendChild(label);
        if (row.email || row.phone || row.is_judge) {
          var meta = document.createElement("span");
          meta.className = "people-search-row-meta";
          var parts = [];
          if (row.is_judge) parts.push("Judge");
          if (row.email) parts.push(row.email);
          if (row.phone) parts.push(row.phone);
          meta.textContent = parts.join(" · ");
          btn.appendChild(meta);
        }
        btn.addEventListener("click", function () {
          applySelection(row);
        });
        resultsEl.appendChild(btn);
      });
      resultsEl.hidden = false;
    }

    var debounceTimer = null;
    function runSearch(q) {
      if (!searchURL) return;
      var url = searchURL + "?q=" + encodeURIComponent(q);
      fetch(url, { credentials: "same-origin" })
        .then(function (r) {
          if (!r.ok) throw new Error("search failed");
          return r.json();
        })
        .then(function (data) {
          renderResults(data && data.results ? data.results : []);
        })
        .catch(function () {
          renderResults([]);
        });
    }

    if (searchInput) {
      searchInput.addEventListener("input", function (event) {
        var q = (event.target.value || "").trim();
        // mirror typed name into the create-fields so submitting without
        // a search match still creates the right person
        if (firstNameInput || lastNameInput) {
          var parts = q.split(/\s+/);
          if (firstNameInput) firstNameInput.value = parts.shift() || "";
          if (lastNameInput) lastNameInput.value = parts.join(" ");
        }
        if (debounceTimer) clearTimeout(debounceTimer);
        if (!q) {
          renderResults([]);
          return;
        }
        debounceTimer = setTimeout(function () {
          runSearch(q);
        }, 200);
      });
    }

    if (changeBtn) {
      changeBtn.addEventListener("click", clearSelection);
    }

    if (memberRoleSelect) {
      memberRoleSelect.addEventListener("change", function () {
        if (currentMode === "member") {
          roleHiddenInput.value = memberRoleSelect.value;
        }
      });
    }

    function openModal(mode) {
      setMode(mode);
      clearSelection();
      if (searchInput) searchInput.value = "";
      if (firstNameInput) firstNameInput.value = "";
      if (lastNameInput) lastNameInput.value = "";
      var emailInput = form.querySelector('input[name="email"]');
      if (emailInput) emailInput.value = "";
      var phoneInput = form.querySelector('input[name="phone"]');
      if (phoneInput) phoneInput.value = "";
      var qualInput = form.querySelector('input[name="qualifications"]');
      if (qualInput) qualInput.value = "";
      var yearsInput = form.querySelector('input[name="years_in_flower_shows"]');
      if (yearsInput) yearsInput.value = "";
      modal.hidden = false;
      if (searchInput) {
        try {
          searchInput.focus();
        } catch (_e) {}
      }
    }

    function closeModal() {
      modal.hidden = true;
      renderResults([]);
    }

    root.querySelectorAll("[data-people-modal-open]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        openModal(btn.getAttribute("data-people-modal-open") || "judge");
      });
    });
    modal.querySelectorAll("[data-people-modal-dismiss]").forEach(function (btn) {
      btn.addEventListener("click", closeModal);
    });

    form.addEventListener("submit", function () {
      // Final guard: in member mode, sync hidden role from the dropdown.
      if (currentMode === "member" && memberRoleSelect) {
        roleHiddenInput.value = memberRoleSelect.value;
      }
      // If a person was selected via search, ensure the editable fields
      // don't override that selection — only first/last name carry through
      // for the no-match-create path.
    });
  }

  function bootstrap() {
    document.querySelectorAll("[data-people-roster]").forEach(init);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bootstrap);
  } else {
    bootstrap();
  }
  // Re-init after htmx swaps replace the panel.
  document.body.addEventListener("htmx:afterSwap", bootstrap);
})();
