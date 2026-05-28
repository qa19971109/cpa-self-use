package api

import "bytes"

var managementAuthFileTestScript = []byte(`<script id="cpa-auth-file-test-ui">
(function () {
  if (window.__cpaAuthFileTestUI) return;
  window.__cpaAuthFileTestUI = true;

  var state = { files: [], headers: {}, results: {}, configYAML: "", configURL: "" };
  var originalFetch = window.fetch;
  if (typeof originalFetch !== "function") return;

  function asHeaders(input, init) {
    try {
      if (init && init.headers) return new Headers(init.headers);
      if (input && input.headers) return new Headers(input.headers);
    } catch (_) {}
    return new Headers();
  }

  function captureHeaders(input, init) {
    var headers = asHeaders(input, init);
    ["authorization", "x-management-key"].forEach(function (name) {
      var value = headers.get(name);
      if (value) state.headers[name] = value;
    });
  }

  function requestURL(input) {
    if (typeof input === "string") return input;
    if (input && input.url) return input.url;
    return "";
  }

  function isAuthFilesList(url) {
    url = String(url || "");
    return /(?:^|\/)(?:v0\/management\/)?auth-files(?:\?|$)/.test(url);
  }

  function isConfigYAML(url) {
    url = String(url || "").split("#")[0].split("?")[0];
    return /(?:^|\/)(?:v0\/management\/)?config\.yaml$/.test(url);
  }

  function rememberConfigYAML(text, url) {
    if (typeof text !== "string" || text.length < 1) return;
    state.configYAML = text;
    state.configURL = String(url || "").split("#")[0].split("?")[0];
    setTimeout(scanConfigPanel, 50);
  }

  function rememberAuthFiles(data, url) {
    if (data && Array.isArray(data.files)) {
      state.files = data.files;
      state.authFilesURL = String(url || "").split("#")[0].split("?")[0];
      setTimeout(scanRows, 50);
    }
  }

  window.fetch = function (input, init) {
    captureHeaders(input, init);
    var url = requestURL(input);
    var method = ((init && init.method) || (input && input.method) || "GET").toUpperCase();
    return originalFetch.apply(this, arguments).then(function (response) {
      if (method === "GET" && isAuthFilesList(url)) {
        response.clone().json().then(function (data) {
          rememberAuthFiles(data, url);
        }).catch(function () {});
      }
      if (method === "GET" && isConfigYAML(url)) {
        response.clone().text().then(function (text) {
          rememberConfigYAML(text, url);
        }).catch(function () {});
      }
      return response;
    });
  };

  if (window.XMLHttpRequest) {
    var originalOpen = XMLHttpRequest.prototype.open;
    var originalSend = XMLHttpRequest.prototype.send;
    var originalSetRequestHeader = XMLHttpRequest.prototype.setRequestHeader;
    XMLHttpRequest.prototype.open = function (method, url) {
      this.__cpaAuthTestMethod = String(method || "GET").toUpperCase();
      this.__cpaAuthTestURL = String(url || "");
      this.__cpaAuthTestHeaders = {};
      return originalOpen.apply(this, arguments);
    };
    XMLHttpRequest.prototype.setRequestHeader = function (name, value) {
      var key = String(name || "").toLowerCase();
      if (key === "authorization" || key === "x-management-key") {
        state.headers[key] = String(value || "");
      }
      if (this.__cpaAuthTestHeaders) this.__cpaAuthTestHeaders[key] = String(value || "");
      return originalSetRequestHeader.apply(this, arguments);
    };
    XMLHttpRequest.prototype.send = function () {
      this.addEventListener("load", function () {
        if (this.__cpaAuthTestMethod === "GET" && isAuthFilesList(this.__cpaAuthTestURL)) {
          try { rememberAuthFiles(JSON.parse(this.responseText || "{}"), this.__cpaAuthTestURL); } catch (_) {}
        }
        if (this.__cpaAuthTestMethod === "GET" && isConfigYAML(this.__cpaAuthTestURL)) {
          rememberConfigYAML(this.responseText || "", this.__cpaAuthTestURL);
        }
      });
      return originalSend.apply(this, arguments);
    };
  }

  function norm(value) {
    return String(value || "").replace(/\s+/g, " ").trim();
  }

  function fieldValues(file) {
    return [file.name, file.auth_index, file.email, file.account, file.id, file.label]
      .map(norm)
      .filter(function (value) { return value.length >= 3; });
  }

  function fileKey(file) {
    if (!file) return "";
    return norm(file.auth_index || file.authIndex || file.name || file.id);
  }

  function rowFile(row) {
    var text = norm(row && row.textContent);
    if (!text) return null;
    var best = null;
    var bestLen = 0;
    state.files.forEach(function (file) {
      fieldValues(file).forEach(function (value) {
        if (value.length > bestLen && text.indexOf(value) !== -1) {
          best = file;
          bestLen = value.length;
        }
      });
    });
    return best;
  }

  function isAuthFilesPage() {
    var route = String(window.location.pathname || "") + String(window.location.hash || "");
    if (/\/auth-files(?:[/?#]|$)/.test(route)) return true;
    return !!document.querySelector('[class*="AuthFilesPage-module__authFilesShell"],[class*="AuthFilesPage-module__authFilesHeader"]');
  }

  function authFilesScope() {
    return document.querySelector('[class*="AuthFilesPage-module__authFilesShell"]') ||
      document.querySelector('[class*="AuthFilesPage-module__page"]') ||
      document;
  }

  function cleanupMisplacedButtons() {
    document.querySelectorAll(".cpa-auth-test-btn").forEach(function (button) {
      var row = button.closest("tr,[role='row'],.ant-table-row,.el-table__row");
      button.remove();
      if (row) delete row.dataset.cpaAuthTestAttached;
    });
    document.querySelectorAll(".cpa-auth-page-test-btn").forEach(function (button) { button.remove(); });
  }

  function actionTarget(row) {
    var cardActions = row.querySelector('[class*="AuthFilesPage-module__cardActions"]');
    if (cardActions) return cardActions;
    return row.querySelector("td:last-child,[role='cell']:last-child") || row;
  }

  function authStatsTarget(card) {
    var nodes = card.querySelectorAll("div,span");
    var best = null;
    var bestLen = Infinity;
    nodes.forEach(function (node) {
      var text = norm(node.textContent);
      if (text.indexOf("\u6210\u529f") === -1 || text.indexOf("\u5931\u8d25") === -1) return;
      if (text.length < bestLen) {
        best = node;
        bestLen = text.length;
      }
    });
    return best || card;
  }

  function contextTooLargeMessage(text) {
    text = String(text || "");
    return text.indexOf("context_too_large") !== -1 ||
      text.indexOf("Your input exceeds the context window of this model") !== -1 ||
      text.indexOf("\u8d85\u8fc7\u4e0a\u4e0b\u6587\u7a97\u53e3") !== -1;
  }

  function requestClassificationTarget(node) {
    return node.closest("tr,[role='row'],[class*='request'],[class*='Request'],[class*='log'],[class*='Log'],[class*='card'],[class*='Card']") ||
      node.parentElement ||
      node;
  }

  function addContextTooLargeBadge(target) {
    if (!target || target.querySelector(".cpa-request-classification-badge")) return;
    var badge = document.createElement("span");
    badge.className = "cpa-request-classification-badge";
    badge.textContent = "\u4e0a\u4e0b\u6587\u8fc7\u957f / \u975e\u8d26\u53f7\u95ee\u9898";
    badge.title = "Upstream rejected the request because the input exceeded the model context window. Retrying another auth file will not fix the same payload.";
    badge.style.cssText = "display:inline-flex;align-items:center;margin-left:8px;padding:2px 9px;border-radius:999px;font-size:12px;font-weight:700;line-height:18px;color:#92400e;background:#fef3c7;border:1px solid #f59e0b;white-space:nowrap;";

    var titleLike = Array.prototype.slice.call(target.querySelectorAll("div,span,td,p"))
      .filter(function (node) {
        var text = norm(node.textContent);
        return text && text.length < 220 && !contextTooLargeMessage(text) && !node.querySelector(".cpa-request-classification-badge");
      })[0];
    (titleLike || target).appendChild(badge);
  }

  function scanRequestFailureClassifications() {
    if (!document.body) return;
    var nodes = Array.prototype.slice.call(document.querySelectorAll("tr,[role='row'],td,div,span,pre,code,p"))
      .filter(function (node) {
        if (node.closest(".cpa-request-classification-badge")) return false;
        var text = node.textContent || "";
        if (text.length < 20 || text.length > 20000 || !contextTooLargeMessage(text)) return false;
        return !Array.prototype.slice.call(node.children || []).some(function (child) {
          return !child.classList.contains("cpa-request-classification-badge") &&
            contextTooLargeMessage(child.textContent || "");
        });
      });
    nodes.slice(0, 30).forEach(function (node) {
      addContextTooLargeBadge(requestClassificationTarget(node));
    });
  }

  function markAuthResult(card, file, result) {
    if (!card || !file || !result) return;
    var key = fileKey(file);
    if (key) state.results[key] = result;
    var badge = card.querySelector(".cpa-auth-validity-badge");
    if (!badge) {
      badge = document.createElement("span");
      badge.className = "cpa-auth-validity-badge";
      authStatsTarget(card).appendChild(badge);
    }
    var text = result.ok ? "\u8d26\u53f7\u6709\u6548" : "\u8d26\u53f7\u5df2\u5931\u6548";
    var title = result.ok ? "Last model test succeeded" : (result.error || "Last model test failed");
    var style = "display:inline-flex;align-items:center;margin-left:10px;padding:2px 9px;border-radius:999px;font-size:12px;font-weight:700;line-height:18px;color:" +
      (result.ok ? "#047857;background:#dcfce7;border:1px solid #86efac;" : "#b91c1c;background:#fee2e2;border:1px solid #fecaca;");
    var okValue = result.ok ? "1" : "0";
    if (badge.dataset.cpaOk === okValue && badge.textContent === text && badge.title === title) return;
    badge.dataset.cpaOk = okValue;
    badge.textContent = text;
    badge.title = title;
    if (badge.style.cssText !== style) badge.style.cssText = style;
  }

  function buildButton(file) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "cpa-auth-test-btn";
    button.textContent = "\u6d4b\u8bd5\u6a21\u578b";
    button.title = "Test this auth file with a minimal model call. Shift-click to choose model.";
    button.style.cssText = "margin-left:6px;padding:4px 8px;border:1px solid #2563eb;background:#2563eb;color:#fff;border-radius:6px;cursor:pointer;font-size:12px;line-height:18px;white-space:nowrap;";
    button.addEventListener("click", function (event) {
      event.preventDefault();
      event.stopPropagation();
      testAuthFile(file, button, event.shiftKey);
    });
    return button;
  }

  function buildPageTestButton() {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "cpa-auth-page-test-btn";
    button.textContent = "\u6d4b\u8bd5\u672c\u9875";
    button.title = "Test all visible auth files on this page. Shift-click to choose model.";
    button.style.cssText = "padding:7px 12px;border:1px solid #2563eb;background:#2563eb;color:#fff;border-radius:8px;cursor:pointer;font-size:13px;font-weight:600;line-height:18px;white-space:nowrap;";
    button.addEventListener("click", function (event) {
      event.preventDefault();
      event.stopPropagation();
      testCurrentPage(button, event.shiftKey);
    });
    return button;
  }

  function ensurePageTestButton() {
    if (!isAuthFilesPage()) return;
    var scope = authFilesScope();
    var header = scope.querySelector('[class*="AuthFilesPage-module__authFilesHeader"] [class*="AuthFilesPage-module__headerActions"]') ||
      scope.querySelector('[class*="AuthFilesPage-module__headerActions"]');
    if (!header || header.querySelector(".cpa-auth-page-test-btn")) return;
    header.insertBefore(buildPageTestButton(), header.firstChild || null);
  }

  function visibleAuthCards() {
    if (!isAuthFilesPage()) return [];
    var scope = authFilesScope();
    var cards = Array.prototype.slice.call(scope.querySelectorAll('[class*="AuthFilesPage-module__fileCard"]'));
    return cards.map(function (card) {
      return { card: card, file: rowFile(card) };
    }).filter(function (item) {
      return item.file && fileKey(item.file);
    });
  }

  function scanRows() {
    if (!state.files.length) return;
    if (!isAuthFilesPage()) {
      cleanupMisplacedButtons();
      return;
    }
    ensurePageTestButton();
    var scope = authFilesScope();
    var rows = scope.querySelectorAll('tr,[role="row"],.ant-table-row,.el-table__row,[class*="AuthFilesPage-module__fileCard"]');
    rows.forEach(function (row) {
      if (!row) return;
      var file = rowFile(row);
      if (!file) return;
      var result = state.results[fileKey(file)];
      if (result) markAuthResult(row, file, result);
      if (row.dataset.cpaAuthTestAttached === "1" || row.querySelector(".cpa-auth-test-btn")) return;
      row.dataset.cpaAuthTestAttached = "1";
      actionTarget(row).appendChild(buildButton(file));
    });
  }

  function resultText(result) {
    if (!result) return "empty response";
    if (result.ok) {
      return (result.text || result.raw_response || "success") + "\n\nlatency: " + result.latency_ms + "ms";
    }
    return (result.error || "request failed") + "\n\nstatus: " + (result.status_code || "unknown") + "\nlatency: " + result.latency_ms + "ms";
  }

  function showModal(title, text, ok) {
    var cover = document.createElement("div");
    cover.style.cssText = "position:fixed;inset:0;background:rgba(15,23,42,.35);z-index:2147483647;display:flex;align-items:center;justify-content:center;padding:24px;";
    var panel = document.createElement("div");
    panel.style.cssText = "max-width:720px;width:min(720px,96vw);max-height:80vh;overflow:auto;background:#fff;color:#111827;border-radius:8px;box-shadow:0 20px 50px rgba(15,23,42,.25);padding:18px;";
    var header = document.createElement("div");
    header.style.cssText = "font-weight:600;margin-bottom:10px;color:" + (ok ? "#047857" : "#b91c1c") + ";";
    header.textContent = title;
    var pre = document.createElement("pre");
    pre.style.cssText = "white-space:pre-wrap;word-break:break-word;background:#f8fafc;border:1px solid #e5e7eb;border-radius:6px;padding:12px;font-size:12px;line-height:1.5;";
    pre.textContent = text;
    var close = document.createElement("button");
    close.type = "button";
    close.textContent = "Close";
    close.style.cssText = "margin-top:12px;padding:6px 12px;border:1px solid #d1d5db;background:#fff;border-radius:6px;cursor:pointer;";
    close.onclick = function () { cover.remove(); };
    panel.appendChild(header);
    panel.appendChild(pre);
    panel.appendChild(close);
    cover.appendChild(panel);
    cover.addEventListener("click", function (event) {
      if (event.target === cover) cover.remove();
    });
    document.body.appendChild(cover);
  }

  function fetchWithTimeout(url, options, timeoutMs) {
    timeoutMs = timeoutMs || 90000;
    var controller = new AbortController();
    var timer = setTimeout(function () { controller.abort(); }, timeoutMs);
    var nextOptions = Object.assign({}, options || {}, { signal: controller.signal });
    return originalFetch(url, nextOptions).finally(function () { clearTimeout(timer); });
  }

  function testAuthFile(file, button, chooseModel, card, silent) {
    var model = norm(file && file.__test_model) || "gpt-5.5";
    if (chooseModel) {
      model = window.prompt("Model", model) || model;
    }
    if (button) {
      button.disabled = true;
      button.textContent = "\u6d4b\u8bd5\u4e2d";
    }
    var headers = Object.assign({}, state.headers, { "content-type": "application/json" });
    var endpoint = state.authFilesURL ? state.authFilesURL + "/test" : "/v0/management/auth-files/test";
    return fetchWithTimeout(endpoint, {
      method: "POST",
      headers: headers,
      body: JSON.stringify({ auth_index: file.auth_index, name: file.name, model: model })
    }, 90000).then(function (response) {
      return response.json().catch(function () {
        return { ok: false, status_code: response.status, error: "invalid json response" };
      });
    }).then(function (result) {
      markAuthResult(card || authCardForFile(file), file, result);
      var title = (result.ok ? "Auth test succeeded: " : "Auth test failed: ") + (file.name || file.auth_index || "");
      if (!silent) showModal(title, resultText(result), !!result.ok);
      return result;
    }).catch(function (error) {
      var errorText = error && error.name === "AbortError" ? "request timed out after 90s" : String(error && error.message || error);
      var result = { ok: false, status_code: 0, error: errorText, latency_ms: 0 };
      markAuthResult(card || authCardForFile(file), file, result);
      if (!silent) showModal("Auth test failed", result.error, false);
      return result;
    }).finally(function () {
      if (button) {
        button.disabled = false;
        button.textContent = "\u6d4b\u8bd5\u6a21\u578b";
      }
    });
  }

  function authCardForFile(file) {
    var items = visibleAuthCards();
    var key = fileKey(file);
    for (var i = 0; i < items.length; i++) {
      if (fileKey(items[i].file) === key) return items[i].card;
    }
    return null;
  }

  function testCurrentPage(button, chooseModel) {
    var model = "gpt-5.5";
    if (chooseModel) {
      model = window.prompt("Model", model) || model;
    }
    var items = visibleAuthCards();
    var seen = {};
    items = items.filter(function (item) {
      var key = fileKey(item.file);
      if (!key || seen[key]) return false;
      seen[key] = true;
      return true;
    });
    if (!items.length) {
      showModal("\u6d4b\u8bd5\u672c\u9875", "\u5f53\u524d\u9875\u6ca1\u6709\u53ef\u6d4b\u8bd5\u7684\u8ba4\u8bc1\u6587\u4ef6", false);
      return;
    }
    button.disabled = true;
    var done = 0;
    var success = 0;
    var failed = 0;
    var failedLines = [];
    var runOne = function (item) {
      button.textContent = "\u6d4b\u8bd5 " + done + "/" + items.length;
      return testAuthFile(Object.assign({}, item.file, { __test_model: model }), null, false, item.card, true).then(function (result) {
        done++;
        if (result && result.ok) {
          success++;
        } else {
          failed++;
          failedLines.push((item.file.name || item.file.auth_index || "unknown") + ": " + ((result && result.error) || "failed"));
        }
        button.textContent = "\u6d4b\u8bd5 " + done + "/" + items.length;
      });
    };
    var chain = Promise.resolve();
    items.forEach(function (item) {
      chain = chain.then(function () { return runOne(item); });
    });
    chain.then(function () {
      var summary = "\u6210\u529f " + success + " \u4e2a\uff0c\u5931\u8d25 " + failed + " \u4e2a";
      if (failedLines.length) summary += "\n\n\u5931\u8d25\u8d26\u53f7:\n" + failedLines.join("\n");
      showModal("\u5f53\u524d\u9875\u6a21\u578b\u6d4b\u8bd5\u5b8c\u6210", summary, failed === 0);
    }).finally(function () {
      button.disabled = false;
      button.textContent = "\u6d4b\u8bd5\u672c\u9875";
    });
  }

  function isConfigPage() {
    var route = String(window.location.pathname || "") + String(window.location.hash || "");
    if (/\/config(?:[/?#]|$)/.test(route)) return true;
    var bodyText = norm(document.body && document.body.textContent);
    return bodyText.indexOf("\u914d\u7f6e\u9762\u677f") !== -1 && bodyText.indexOf("\u53ef\u89c6\u5316\u7f16\u8f91") !== -1;
  }

  function yamlTimeoutValue(yaml) {
    var match = String(yaml || "").match(/^codex-response-header-timeout-seconds\s*:\s*(-?\d+)\s*$/m);
    return match ? match[1] : "180";
  }

  function upsertTimeoutValue(yaml, value) {
    yaml = String(yaml || "");
    var line = "codex-response-header-timeout-seconds: " + value;
    if (/^codex-response-header-timeout-seconds\s*:/m.test(yaml)) {
      return yaml.replace(/^codex-response-header-timeout-seconds\s*:.*$/m, line);
    }
    if (/^nonstream-keepalive-interval\s*:.*$/m.test(yaml)) {
      return yaml.replace(/^(nonstream-keepalive-interval\s*:.*)$/m,
        "$1\n# Codex upstream response-header timeout. Only applies before headers arrive; streaming body remains unlimited.\n" + line);
    }
    return line + "\n" + yaml;
  }

  function configCandidates() {
    var urls = [];
    if (state.configURL) urls.push(state.configURL);
    urls.push("/v0/management/config.yaml");
    if (window.location && window.location.hostname) {
      urls.push(window.location.protocol + "//" + window.location.hostname + ":8317/v0/management/config.yaml");
    }
    return urls.filter(function (url, index) { return url && urls.indexOf(url) === index; });
  }

  function loadConfigYAML() {
    if (state.configYAML) return Promise.resolve(state.configYAML);
    var headers = Object.assign({}, state.headers);
    var urls = configCandidates();
    var attempt = function (index) {
      if (index >= urls.length) return Promise.reject(new Error("cannot load config.yaml"));
      return fetchWithTimeout(urls[index], { method: "GET", headers: headers }, 10000).then(function (response) {
        if (!response.ok) throw new Error("GET " + urls[index] + " returned " + response.status);
        return response.text().then(function (text) {
          rememberConfigYAML(text, urls[index]);
          return text;
        });
      }).catch(function () {
        return attempt(index + 1);
      });
    };
    return attempt(0);
  }

  function saveConfigYAML(yaml) {
    var headers = Object.assign({}, state.headers, { "content-type": "application/yaml; charset=utf-8" });
    var urls = configCandidates();
    var attempt = function (index) {
      if (index >= urls.length) return Promise.reject(new Error("cannot save config.yaml"));
      return fetchWithTimeout(urls[index], { method: "PUT", headers: headers, body: yaml }, 15000).then(function (response) {
        if (!response.ok) {
          return response.text().then(function (text) {
            throw new Error("PUT " + urls[index] + " returned " + response.status + ": " + text);
          });
        }
        rememberConfigYAML(yaml, urls[index]);
        return response;
      }).catch(function (error) {
        if (index + 1 >= urls.length) throw error;
        return attempt(index + 1);
      });
    };
    return attempt(0);
  }

  function configAnchor() {
    var labels = Array.prototype.slice.call(document.querySelectorAll("button,[role='tab'],div,span"))
      .filter(function (node) { return norm(node.textContent) === "\u53ef\u89c6\u5316\u7f16\u8f91"; });
    for (var i = 0; i < labels.length; i++) {
      var parent = labels[i].parentElement;
      if (parent && parent.parentElement) return parent.parentElement;
    }
    return document.querySelector("main") || document.querySelector(".content") || document.body;
  }

  function statusText(node, text, ok) {
    if (!node) return;
    node.textContent = text;
    node.style.color = ok ? "#047857" : "#b91c1c";
  }

  function isVisibleNode(node) {
    if (!node || !node.getBoundingClientRect) return false;
    var rect = node.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  }

  function findConfigTitle(text) {
    var candidates = Array.prototype.slice.call(document.querySelectorAll("h1,h2,h3,h4,div,span,p"))
      .filter(function (node) {
        return norm(node.textContent) === text && isVisibleNode(node);
      });
    if (!candidates.length) return null;
    var contentCandidates = candidates.filter(function (node) {
      return node.getBoundingClientRect().left > 320;
    });
    return (contentCandidates.length ? contentCandidates : candidates)[0];
  }

  function findNextConfigTitle(afterNode) {
    if (!afterNode) return null;
    var afterRect = afterNode.getBoundingClientRect();
    var titles = ["\u989d\u5ea6\u56de\u9000", "\u6d41\u5f0f\u4f20\u8f93\u914d\u7f6e", "\u6a21\u578b\u914d\u7f6e", "\u4ee3\u7406\u914d\u7f6e"];
    var matches = [];
    titles.forEach(function (title) {
      var node = findConfigTitle(title);
      if (!node) return;
      var rect = node.getBoundingClientRect();
      if (rect.top > afterRect.top || (Math.abs(rect.top - afterRect.top) < 2 && rect.left > afterRect.left)) {
        matches.push({ node: node, top: rect.top, left: rect.left });
      }
    });
    matches.sort(function (a, b) {
      if (a.top !== b.top) return a.top - b.top;
      return a.left - b.left;
    });
    return matches.length ? matches[0].node : null;
  }

  function directChildOf(parent, node) {
    var child = node;
    while (child && child.parentElement && child.parentElement !== parent) {
      child = child.parentElement;
    }
    return child && child.parentElement === parent ? child : null;
  }

  function networkConfigMount() {
    var networkTitle = findConfigTitle("\u7f51\u7edc\u914d\u7f6e");
    if (!networkTitle) return null;
    var nextTitle = findNextConfigTitle(networkTitle);
    if (nextTitle) {
      var parent = networkTitle.parentElement;
      while (parent && parent !== document.body) {
        if (parent.contains(nextTitle)) {
          var before = directChildOf(parent, nextTitle);
          var networkChild = directChildOf(parent, networkTitle);
          if (before && networkChild && before !== networkChild && parent.getBoundingClientRect().width > 500) {
            return { parent: parent, before: before };
          }
        }
        parent = parent.parentElement;
      }
    }
    var fallback = networkTitle.parentElement;
    for (var i = 0; fallback && i < 5; i++) {
      if (fallback.getBoundingClientRect && fallback.getBoundingClientRect().width > 500) {
        return { parent: fallback, before: null };
      }
      fallback = fallback.parentElement;
    }
    return null;
  }

  function buildConfigTimeoutCard() {
    var card = document.createElement("div");
    card.className = "cpa-codex-timeout-card";
    card.style.cssText = "margin:12px 0 0;padding:16px 22px;border:1px solid #e2e8f0;background:#fff;border-radius:16px;display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;";

    var copy = document.createElement("div");
    copy.style.cssText = "min-width:260px;flex:1 1 360px;";
    var title = document.createElement("div");
    title.textContent = "Codex \u54cd\u5e94\u5934\u8d85\u65f6";
    title.style.cssText = "font-weight:700;color:#111827;font-size:15px;line-height:22px;";
    var hint = document.createElement("div");
    hint.textContent = "\u53ea\u9650\u5236\u4e0a\u6e38 headers \u524d\u7684\u7b49\u5f85\uff1bheaders \u540e\u7684\u6d41\u5f0f\u601d\u8003\u4e0d\u9650\u65f6\u30020 \u4f7f\u7528\u9ed8\u8ba4 180\uff0c\u8d1f\u6570\u5173\u95ed\u3002";
    hint.style.cssText = "margin-top:4px;color:#4b5563;font-size:13px;line-height:20px;";
    copy.appendChild(title);
    copy.appendChild(hint);

    var inputWrap = document.createElement("label");
    inputWrap.style.cssText = "display:flex;flex-direction:column;gap:6px;font-size:13px;font-weight:600;color:#374151;min-width:160px;flex:0 1 220px;";
    inputWrap.textContent = "\u79d2";
    var input = document.createElement("input");
    input.type = "number";
    input.className = "cpa-codex-timeout-input";
    input.value = yamlTimeoutValue(state.configYAML);
    input.style.cssText = "height:44px;border:1px solid #d1d5db;border-radius:12px;padding:0 14px;font-size:15px;background:#fff;color:#111827;outline:none;";
    inputWrap.appendChild(input);

    var actions = document.createElement("div");
    actions.style.cssText = "display:flex;align-items:center;gap:10px;justify-content:flex-end;flex:0 0 auto;";
    var status = document.createElement("span");
    status.className = "cpa-codex-timeout-status";
    status.style.cssText = "font-size:12px;min-width:74px;text-align:right;color:#64748b;";
    var button = document.createElement("button");
    button.type = "button";
    button.textContent = "\u4fdd\u5b58";
    button.style.cssText = "height:44px;padding:0 18px;border:1px solid #2563eb;background:#2563eb;color:#fff;border-radius:10px;font-size:14px;font-weight:700;cursor:pointer;";
    button.addEventListener("click", function (event) {
      event.preventDefault();
      event.stopPropagation();
      var value = parseInt(input.value, 10);
      if (!Number.isFinite(value)) {
        statusText(status, "\u8bf7\u8f93\u5165\u6574\u6570", false);
        return;
      }
      button.disabled = true;
      button.textContent = "\u4fdd\u5b58\u4e2d";
      statusText(status, "", true);
      loadConfigYAML().then(function (yaml) {
        return saveConfigYAML(upsertTimeoutValue(yaml, String(value)));
      }).then(function () {
        statusText(status, "\u5df2\u4fdd\u5b58", true);
      }).catch(function (error) {
        statusText(status, "\u4fdd\u5b58\u5931\u8d25", false);
        showModal("Codex \u54cd\u5e94\u5934\u8d85\u65f6\u4fdd\u5b58\u5931\u8d25", String(error && error.message || error), false);
      }).finally(function () {
        button.disabled = false;
        button.textContent = "\u4fdd\u5b58";
      });
    });
    actions.appendChild(status);
    actions.appendChild(button);

    card.appendChild(copy);
    card.appendChild(inputWrap);
    card.appendChild(actions);
    return card;
  }

  function scanConfigPanel() {
    if (!isConfigPage()) return;
    var mount = networkConfigMount();
    if (!mount || !mount.parent) return;
    var existing = document.querySelector(".cpa-codex-timeout-card");
    if (existing) {
      var input = existing.querySelector(".cpa-codex-timeout-input");
      if (input && document.activeElement !== input) input.value = yamlTimeoutValue(state.configYAML);
      if (existing.parentElement !== mount.parent) {
        mount.parent.insertBefore(existing, mount.before || null);
      }
      return;
    }
    mount.parent.insertBefore(buildConfigTimeoutCard(), mount.before || null);
    loadConfigYAML().then(function () { scanConfigPanel(); }).catch(function () {});
  }

  var scanTimer = 0;
  function scheduleScan() {
    if (scanTimer) return;
    scanTimer = setTimeout(function () {
      scanTimer = 0;
      scanRows();
      scanRequestFailureClassifications();
      scanConfigPanel();
    }, 250);
  }

  new MutationObserver(scheduleScan).observe(document.documentElement, { childList: true, subtree: true });
  setInterval(function () {
    scanRows();
    scanRequestFailureClassifications();
    scanConfigPanel();
  }, 2000);
})();
</script>`)

func injectManagementAuthFileTestUI(html []byte) []byte {
	if len(html) == 0 || bytes.Contains(html, []byte("cpa-auth-file-test-ui")) {
		return html
	}
	lower := bytes.ToLower(html)
	idx := bytes.LastIndex(lower, []byte("</body>"))
	if idx < 0 {
		out := make([]byte, 0, len(html)+len(managementAuthFileTestScript))
		out = append(out, html...)
		out = append(out, managementAuthFileTestScript...)
		return out
	}
	out := make([]byte, 0, len(html)+len(managementAuthFileTestScript))
	out = append(out, html[:idx]...)
	out = append(out, managementAuthFileTestScript...)
	out = append(out, html[idx:]...)
	return out
}
