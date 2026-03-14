(function () {
  window.TZ = window.TZ || {};

  function pad2(n) {
    return String(n).padStart(2, "0");
  }

  function clamp01(x) {
    if (!isFinite(x)) return 0;
    return Math.max(0, Math.min(1, x));
  }

  function fmtBytesIEC(bytes) {
    if (!isFinite(bytes) || bytes < 0) bytes = 0;
    const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
    let v = bytes;
    let i = 0;
    while (v >= 1024 && i < units.length - 1) {
      v /= 1024;
      i++;
    }
    const prec = v >= 100 ? 0 : v >= 10 ? 1 : 2;
    return `${v.toFixed(prec)} ${units[i]}`;
  }

  function fmtBps(bytesPerSec) {
    return `${fmtBytesIEC(bytesPerSec)}/s`;
  }

  function pct(used, total) {
    if (!isFinite(used) || !isFinite(total) || total <= 0) return 0;
    return (used / total) * 100;
  }

  function fmtPct(v) {
    if (!isFinite(v)) v = 0;
    v = Math.max(0, Math.min(100, v));
    return `${Math.round(v)}%`;
  }

  function fmtLoad(v) {
    if (!isFinite(v)) return "0.00";
    return v.toFixed(2);
  }

  function fmtUptime(seconds) {
    if (!isFinite(seconds) || seconds < 0) seconds = 0;
    const totalHours = Math.floor(seconds / 3600);
    const days = Math.floor(totalHours / 24);
    const hours = totalHours % 24;
    if (days > 0) return hours > 0 ? `${days}天${hours}小时` : `${days}天`;
    if (hours > 0) return `${hours}小时`;
    const mins = Math.floor(seconds / 60);
    return `${mins}分`;
  }

  function escapeHtml(s) {
    return String(s || "")
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;")
      .replaceAll("'", "&#39;");
  }

  function fmtAgo(ms) {
    if (!ms) return "-";
    const s = Math.floor(ms / 1000);
    if (s < 60) return `${s}秒前`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m}分前`;
    const h = Math.floor(m / 60);
    if (h < 48) return `${h}小时前`;
    const d = Math.floor(h / 24);
    return `${d}天前`;
  }

  function bytesToTB(b) {
    if (!b) return "";
    return (b / 1024 / 1024 / 1024 / 1024).toFixed(2);
  }

  function tbToBytes(tb) {
    const v = parseFloat(tb);
    if (!isFinite(v) || v <= 0) return 0;
    return Math.round(v * 1024 * 1024 * 1024 * 1024);
  }

  const store = {
    get(key) {
      try {
        return localStorage.getItem(key);
      } catch {
        return null;
      }
    },
    set(key, value) {
      try {
        localStorage.setItem(key, value);
      } catch {}
    },
    del(key) {
      try {
        localStorage.removeItem(key);
      } catch {}
    },
  };

  TZ.util = {
    pad2,
    clamp01,
    fmtBytesIEC,
    fmtBps,
    pct,
    fmtPct,
    fmtLoad,
    fmtUptime,
    escapeHtml,
    fmtAgo,
    bytesToTB,
    tbToBytes,
    store,
  };
})();

