package api

import (
	"bytes"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var managementConfigVersionGuardScript = []byte(`<script id="cliproxy-config-version-guard">
(function () {
  if (window.__cliproxyConfigVersionGuard) return;
  window.__cliproxyConfigVersionGuard = true;
  var latestConfigVersion = "";
  var originalFetch = window.fetch ? window.fetch.bind(window) : null;
  if (!originalFetch) return;

  function requestPath(input) {
    try {
      var raw = typeof input === "string" ? input : input && input.url;
      return new URL(raw || "", window.location.href).pathname;
    } catch (_) {
      return "";
    }
  }

  function requestMethod(input, init) {
    return String((init && init.method) || (input && input.method) || "GET").toUpperCase();
  }

  function shouldGuard(method, path) {
    if (method === "GET" || method === "HEAD") return false;
    return path.indexOf("/v0/management/") === 0;
  }

  window.fetch = async function (input, init) {
    init = init || {};
    var method = requestMethod(input, init);
    var path = requestPath(input);
    var headers = new Headers(init.headers || (input && input.headers) || {});
    if (latestConfigVersion && shouldGuard(method, path) && !headers.has("If-Match") && !headers.has("X-Config-Version")) {
      headers.set("If-Match", '"' + latestConfigVersion + '"');
      init = Object.assign({}, init, { headers: headers });
    }

    var response = await originalFetch(input, init);
    var nextVersion = response.headers && response.headers.get("X-Config-Version");
    if (nextVersion) latestConfigVersion = nextVersion;

    if (response.status === 409) {
      response.clone().json().then(function (body) {
        if (body && body.error === "config_conflict") {
          window.alert("配置文件已被其他操作修改，请刷新页面后再保存，避免覆盖最新配置。");
        }
      }).catch(function () {});
    }
    return response;
  };

  if (window.XMLHttpRequest) {
    var originalOpen = window.XMLHttpRequest.prototype.open;
    var originalSend = window.XMLHttpRequest.prototype.send;
    var originalSetRequestHeader = window.XMLHttpRequest.prototype.setRequestHeader;

    window.XMLHttpRequest.prototype.open = function (method, url) {
      this.__cliproxyMethod = String(method || "GET").toUpperCase();
      this.__cliproxyPath = requestPath(url || "");
      this.__cliproxyHeaders = {};
      return originalOpen.apply(this, arguments);
    };

    window.XMLHttpRequest.prototype.setRequestHeader = function (name, value) {
      if (name) this.__cliproxyHeaders[String(name).toLowerCase()] = true;
      return originalSetRequestHeader.apply(this, arguments);
    };

    window.XMLHttpRequest.prototype.send = function () {
      var headers = this.__cliproxyHeaders || {};
      if (latestConfigVersion && shouldGuard(this.__cliproxyMethod || "GET", this.__cliproxyPath || "") && !headers["if-match"] && !headers["x-config-version"]) {
        originalSetRequestHeader.call(this, "If-Match", '"' + latestConfigVersion + '"');
      }
      this.addEventListener("loadend", function () {
        var nextVersion = "";
        try { nextVersion = this.getResponseHeader("X-Config-Version") || ""; } catch (_) {}
        if (nextVersion) latestConfigVersion = nextVersion;
        if (this.status === 409) {
          try {
            var body = JSON.parse(this.responseText || "{}");
            if (body && body.error === "config_conflict") {
              window.alert("配置文件已被其他操作修改，请刷新页面后再保存，避免覆盖最新配置。");
            }
          } catch (_) {}
        }
      });
      return originalSend.apply(this, arguments);
    };
  }
})();
</script>`)

func injectManagementConfigVersionGuard(data []byte) []byte {
	if bytes.Contains(data, []byte("cliproxy-config-version-guard")) {
		return data
	}
	lower := bytes.ToLower(data)
	if idx := bytes.LastIndex(lower, []byte("</body>")); idx >= 0 {
		out := make([]byte, 0, len(data)+len(managementConfigVersionGuardScript))
		out = append(out, data[:idx]...)
		out = append(out, managementConfigVersionGuardScript...)
		out = append(out, data[idx:]...)
		return out
	}
	out := make([]byte, 0, len(data)+len(managementConfigVersionGuardScript))
	out = append(out, data...)
	out = append(out, managementConfigVersionGuardScript...)
	return out
}

func serveManagementControlPanelFile(c *gin.Context, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		c.File(filePath)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", injectManagementConfigVersionGuard(data))
}
