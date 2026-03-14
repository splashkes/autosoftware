package main

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"unicode"

	"as/kernel/internal/materializer"
)

//go:embed assets/sprout-logo.css assets/sprout-logo.js
var bootloaderAssets embed.FS

var bootPageTemplate = template.Must(template.New("boot-page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>AS Kernel Boot</title>
  <link rel="stylesheet" href="/assets/sprout-logo.css">
  <style nonce="{{.CSPNonce}}">
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background:
        radial-gradient(circle at top, rgba(34, 197, 94, 0.08), transparent 28rem),
        linear-gradient(180deg, #eceef2 0%, #e7eaee 100%);
      color: #20242c;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .page {
      width: min(31rem, calc(100vw - 2rem));
      padding: 1.25rem 0 2rem;
    }
    .brand {
      display: grid;
      justify-items: center;
      text-align: center;
      gap: 0.4rem;
      margin-bottom: 1rem;
    }
    .wordmark {
      font-size: 2rem;
      font-weight: 700;
      letter-spacing: 0.52rem;
      color: #181c24;
      padding-left: 0.52rem;
    }
    .tagline {
      font-size: 0.72rem;
      letter-spacing: 0.22rem;
      text-transform: uppercase;
      color: #7e8592;
      padding-left: 0.22rem;
    }
    .lede {
      margin: 0.4rem auto 0;
      max-width: 19rem;
      color: #69707c;
      font-size: 0.82rem;
      line-height: 1.6;
    }
    .boot-meta {
      display: flex;
      justify-content: center;
      gap: 0.5rem;
      flex-wrap: wrap;
      margin: 1rem 0 1.25rem;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.24rem 0.62rem;
      border-radius: 999px;
      border: 1px solid #c8cdd6;
      background: rgba(255, 255, 255, 0.7);
      color: #6c7380;
      font-size: 0.7rem;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    .shell {
      border: 1px solid #cfd4dc;
      background: rgba(245, 246, 248, 0.92);
      box-shadow: 0 1rem 2.5rem rgba(28, 35, 48, 0.08);
    }
    .seed {
      border-bottom: 1px solid #d4d8df;
    }
    .seed:last-of-type {
      border-bottom: none;
    }
    .seed-head {
      width: 100%;
      display: flex;
      align-items: center;
      gap: 0.9rem;
      padding: 1rem 1.15rem;
      border: none;
      background: transparent;
      cursor: pointer;
      text-align: left;
    }
    .seed-count {
      min-width: 2rem;
      font-size: 0.76rem;
      color: #8d94a0;
      text-align: right;
      flex-shrink: 0;
    }
    .seed-copy {
      flex: 1;
      min-width: 0;
    }
    .seed-name {
      display: block;
      color: #222730;
      font-size: 0.92rem;
      font-weight: 500;
    }
    .seed-id {
      display: block;
      margin-top: 0.16rem;
      color: #88909c;
      font-size: 0.72rem;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    .seed-arrow {
      color: #a4aab5;
      font-size: 1rem;
      transition: transform 0.18s ease;
      flex-shrink: 0;
    }
    .seed.open .seed-arrow {
      transform: rotate(90deg);
    }
    .seed-body {
      display: none;
      padding: 0 1.15rem 1rem 4rem;
      gap: 0.55rem;
    }
    .seed.open .seed-body {
      display: grid;
    }
    .realization {
      display: flex;
      align-items: center;
      gap: 0.8rem;
      padding: 0.35rem 0;
    }
    .realization-copy {
      flex: 1;
      min-width: 0;
    }
    .realization-title {
      display: block;
      color: #505764;
      font-size: 0.8rem;
      line-height: 1.4;
    }
    .realization-meta {
      display: flex;
      gap: 0.42rem;
      flex-wrap: wrap;
      margin-top: 0.2rem;
      color: #959ca7;
      font-size: 0.64rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.18rem 0.48rem;
      border-radius: 999px;
      border: 1px solid #cbd1da;
      background: rgba(255, 255, 255, 0.78);
      color: #2f855a;
      line-height: 1;
    }
    .status.draft {
      color: #9a6700;
    }
    .status.published,
    .status.accepted {
      color: #15803d;
    }
    .status.failed,
    .status.error {
      color: #b91c1c;
    }
    .boot-button {
      padding: 0.26rem 0.7rem;
      border: 1px solid #c8ccd4;
      background: transparent;
      color: #616875;
      font: inherit;
      font-size: 0.68rem;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      cursor: pointer;
      flex-shrink: 0;
    }
    .boot-button:hover,
    .boot-button:focus-visible {
      border-color: #22a05a;
      color: #178243;
      outline: none;
    }
    .materialization {
      margin-top: 1rem;
      padding: 1.1rem;
      min-height: 12rem;
      border: 1px solid #d0d5dd;
      background: rgba(255, 255, 255, 0.62);
    }
    .indicator {
      display: grid;
      align-content: center;
      min-height: 100%;
      gap: 0.45rem;
    }
    .indicator-title {
      font-size: 0.72rem;
      color: #7a818d;
      letter-spacing: 0.12em;
      text-transform: uppercase;
    }
    .indicator-copy {
      margin: 0;
      color: #69707c;
      font-size: 0.82rem;
      line-height: 1.6;
    }
    .empty {
      margin: 0;
      color: #69707c;
      font-size: 0.82rem;
      line-height: 1.6;
    }
    .stack {
      display: grid;
      gap: 0.85rem;
    }
    .row {
      display: flex;
      gap: 0.75rem;
      align-items: center;
      justify-content: space-between;
      flex-wrap: wrap;
    }
    .subtle {
      color: #7a818d;
      font-size: 0.76rem;
      line-height: 1.5;
    }
    .source {
      border-top: 1px solid #d4d8df;
      padding-top: 0.85rem;
    }
    .source h3 {
      margin: 0 0 0.25rem;
      font-size: 0.9rem;
      color: #222730;
    }
    .pathline {
      color: #848b96;
      font-size: 0.74rem;
      line-height: 1.5;
      word-break: break-word;
    }
    pre {
      margin: 0.5rem 0 0;
      padding: 0.75rem;
      white-space: pre-wrap;
      border: 1px solid #d0d5dd;
      background: rgba(240, 243, 247, 0.92);
      color: #303744;
      font-size: 0.78rem;
      line-height: 1.5;
      overflow-x: auto;
    }
    #boot-status {
      margin-top: 0.9rem;
      text-align: center;
      color: #7b828f;
      font-size: 0.76rem;
      min-height: 1.2rem;
    }
    .footer {
      margin-top: 1rem;
      text-align: center;
      color: #848b96;
      font-size: 0.72rem;
      line-height: 1.6;
    }
    .footer code {
      color: #4f5664;
    }
    @media (max-width: 720px) {
      .page {
        width: min(31rem, calc(100vw - 1rem));
      }
      .seed-body {
        padding-left: 1.15rem;
      }
      .realization {
        align-items: flex-start;
      }
    }
  </style>
</head>
<body>
  <main class="page">
    <section class="brand">
      <div class="sprout-logo-shell" data-sprout-logo aria-hidden="true"></div>
      <div class="wordmark">AS</div>
      <div class="tagline">Kernel Bootloader</div>
      <p class="lede">Software that evolves from within.</p>
      <p class="lede">Built to scale securely, share data across apps, and give agents and humans a common surface to use, inspect, and build.</p>
    </section>

    <div class="boot-meta">
      <span class="pill">{{len .Seeds}} seeds</span>
      <span class="pill">{{.RealizationCount}} realizations</span>
      {{if .RemoteConfigured}}<span class="pill">remote on</span>{{else}}<span class="pill">remote off</span>{{end}}
    </div>

    <section class="shell">
      {{range .Seeds}}
      <section class="seed{{if .InitiallyOpen}} open{{end}}">
        <button class="seed-head" type="button" data-seed-toggle>
          <span class="seed-count">{{.Count}}</span>
          <span class="seed-copy">
            <span class="seed-name">{{.DisplayName}}</span>
            <span class="seed-id">{{.SeedID}}</span>
          </span>
          <span class="seed-arrow">&#8250;</span>
        </button>
        <div class="seed-body">
          {{range .Realizations}}
          <div class="realization">
            <div class="realization-copy">
              <span class="realization-title">{{.Summary}}</span>
              <span class="realization-meta">
                <span class="status {{.Status}}">{{.Status}}</span>
                {{if .ApproachID}}<span>{{.ApproachID}}</span>{{end}}
                <span>{{.Reference}}</span>
              </span>
            </div>
            <button class="boot-button" type="button" data-reference="{{.Reference}}" data-label="{{.Summary}}">Boot</button>
          </div>
          {{end}}
        </div>
      </section>
      {{end}}
    </section>

    <section id="materialization" class="materialization">
      <div id="loader-indicator" class="indicator">
        <div class="indicator-title">Paused</div>
        <p class="indicator-copy">Select a seed, choose a realization, and boot it into the materialized runtime.</p>
      </div>
    </section>

    <div id="boot-status"></div>
    <p class="footer">Materialization persists into <code>materialized/</code> after boot. Click the sprout to regrow the mark.</p>

    <script src="/assets/sprout-logo.js"></script>
    <script nonce="{{.CSPNonce}}">{{.LoaderScript}}</script>
    <script nonce="{{.CSPNonce}}">{{.FeedbackScript}}</script>
  </main>
</body>
</html>`))

type bootPageView struct {
	Seeds            []seedBootView
	RealizationCount int
	RemoteConfigured bool
	CSPNonce         string
	LoaderScript     template.JS
	FeedbackScript   template.JS
}

type seedBootView struct {
	SeedID        string
	DisplayName   string
	Count         int
	InitiallyOpen bool
	Realizations  []realizationBootView
}

type realizationBootView struct {
	Reference     string
	RealizationID string
	ApproachID    string
	Summary       string
	Status        string
}

func newBootPageView(options []materializer.RealizationOption, remoteConfigured bool, nonce string, feedbackScript string) bootPageView {
	seen := make(map[string]int)
	seeds := make([]seedBootView, 0)
	for _, option := range options {
		index, ok := seen[option.SeedID]
		if !ok {
			index = len(seeds)
			seen[option.SeedID] = index
			seeds = append(seeds, seedBootView{
				SeedID:        option.SeedID,
				DisplayName:   humanizeSeedID(option.SeedID),
				InitiallyOpen: len(seeds) == 0,
			})
		}

		seeds[index].Realizations = append(seeds[index].Realizations, realizationBootView{
			Reference:     option.Reference,
			RealizationID: option.RealizationID,
			ApproachID:    option.ApproachID,
			Summary:       firstNonEmpty(strings.TrimSpace(option.Summary), option.RealizationID),
			Status:        firstNonEmpty(strings.TrimSpace(option.Status), "draft"),
		})
		seeds[index].Count = len(seeds[index].Realizations)
	}

	return bootPageView{
		Seeds:            seeds,
		RealizationCount: len(options),
		RemoteConfigured: remoteConfigured,
		CSPNonce:         nonce,
		LoaderScript:     template.JS(bootLoaderScript()),
		FeedbackScript:   template.JS(feedbackScript),
	}
}

func sproutAssetHandler() http.Handler {
	sub, err := fs.Sub(bootloaderAssets, "assets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/assets/", http.FileServer(http.FS(sub)))
}

func humanizeSeedID(seedID string) string {
	trimmed := strings.TrimSpace(seedID)
	if trimmed == "" {
		return "Unnamed Seed"
	}

	parts := strings.SplitN(trimmed, "-", 2)
	label := trimmed
	if len(parts) == 2 && parts[1] != "" {
		label = parts[1]
	}

	words := strings.FieldsFunc(label, func(r rune) bool {
		return r == '-' || r == '_' || unicode.IsSpace(r)
	})
	if len(words) == 0 {
		return trimmed
	}

	for index, word := range words {
		words[index] = capitalizeWord(word)
	}
	return strings.Join(words, " ")
}

func capitalizeWord(word string) string {
	if word == "" {
		return word
	}
	runes := []rune(strings.ToLower(word))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func bootLoaderScript() string {
	return `(function () {
  var materialization = document.getElementById("materialization");
  var indicator = document.getElementById("loader-indicator");
  var status = document.getElementById("boot-status");
  if (!materialization || !indicator || !status) {
    return;
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"]/g, function (char) {
      return {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "\"": "&quot;"
      }[char];
    });
  }

  function setStatus(copy) {
    status.textContent = copy || "";
  }

  function setLoading(label) {
    indicator.innerHTML = [
      '<div class="indicator-title">Booting</div>',
      '<p class="indicator-copy">Materializing ' + escapeHTML(label) + ' into the current runtime.</p>'
    ].join("");
    materialization.replaceChildren(indicator);
    setStatus("Growing " + label + " into the running surface...");
  }

  function setError(message) {
    materialization.innerHTML = [
      '<div class="indicator">',
      '  <div class="indicator-title">Boot Failed</div>',
      '  <p class="indicator-copy">The kernel could not materialize that realization.</p>',
      '  <pre>' + escapeHTML(message) + '</pre>',
      '</div>'
    ].join("\n");
    setStatus("Boot failed.");
  }

  async function boot(button) {
    var reference = button.getAttribute("data-reference");
    var label = button.getAttribute("data-label") || reference;
    button.disabled = true;
    setLoading(label);

    try {
      var response = await fetch("/partials/materialization?reference=" + encodeURIComponent(reference), {
        method: "GET",
        credentials: "same-origin",
        headers: { "Accept": "text/html" }
      });
      var html = await response.text();
      if (!response.ok) {
        throw new Error(html || ("Boot failed with status " + response.status));
      }
      materialization.innerHTML = html;
      setStatus("Booted " + label + ".");
    } catch (err) {
      setError(err && err.message ? err.message : err);
      console.error(err);
    } finally {
      button.disabled = false;
    }
  }

  document.addEventListener("click", function (event) {
    var toggle = event.target.closest("[data-seed-toggle]");
    if (toggle) {
      var seed = toggle.closest(".seed");
      var wasOpen = seed.classList.contains("open");
      document.querySelectorAll(".seed.open").forEach(function (item) {
        item.classList.remove("open");
      });
      if (!wasOpen) {
        seed.classList.add("open");
      }
      return;
    }

    var bootButton = event.target.closest("[data-reference]");
    if (bootButton) {
      event.preventDefault();
      boot(bootButton);
    }
  });

  var firstSeed = document.querySelector(".seed");
  if (firstSeed && !document.querySelector(".seed.open")) {
    firstSeed.classList.add("open");
  }
})();`
}
