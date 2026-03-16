(function () {
  window.TZ = window.TZ || {};
  const U = TZ.util;
  const S = TZ.spark;

  const state = {
    trafficWindow: "1d",
    pollMs: 3000,
    history: new Map(),
    isAdmin: false,
  };

  // Sparkline history window (in-memory): target ~5 minutes.
  const HIST_WINDOW_MS = 5 * 60 * 1000;

  const ui = {
    compact: (U.store.get("ui_compact") || "") === "1",
  };

  function updateAdminBtn() {
    const el = document.getElementById("adminBtn");
    if (!el) return;
    if (state.isAdmin) {
      el.textContent = "管理";
      el.title = "控制面板";
      el.href = "/admin";
    } else {
      el.textContent = "登录";
      el.title = "登录控制面板";
      el.href = "/admin/login?next=%2Fadmin";
    }
  }

  async function checkAdminSession() {
    try {
      const res = await fetch("/api/admin/session", { cache: "no-store", credentials: "same-origin" });
      const data = await res.json().catch(() => null);
      state.isAdmin = !!data?.authed;
    } catch {
      state.isAdmin = false;
    }
    updateAdminBtn();
  }

  async function adminPatchServer(patch) {
    const res = await fetch("/api/admin/server_patch", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      credentials: "same-origin",
      body: JSON.stringify(patch || {}),
    });
    if (res.status === 401) {
      state.isAdmin = false;
      updateAdminBtn();
      location.href = `/admin/login?next=${encodeURIComponent("/")}`;
      throw new Error("unauthorized");
    }
    if (!res.ok) throw new Error(`patch ${res.status}`);
    const data = await res.json().catch(() => null);
    if (!data || !data.ok) throw new Error("patch failed");
    return data;
  }

  const cardShadows = ["shadow-pink", "shadow-cyan", "shadow-yellow", "shadow-magenta", "shadow-purple"];
  function pickShadowClass(id) {
    let h = 2166136261;
    for (let i = 0; i < id.length; i++) {
      h ^= id.charCodeAt(i);
      h = Math.imul(h, 16777619);
    }
    const idx = Math.abs(h) % cardShadows.length;
    return cardShadows[idx];
  }

  function setNowTime() {
    const d = new Date();
    const el = document.getElementById("nowTime");
    if (!el) return;
    el.textContent = `${U.pad2(d.getHours())}:${U.pad2(d.getMinutes())}:${U.pad2(d.getSeconds())}`;
  }

  function applyCompact() {
    document.body.classList.toggle("compact", ui.compact);
    const btn = document.getElementById("compactBtn");
    if (btn) btn.textContent = ui.compact ? "展开" : "简洁";
  }

  function getHist(key, max = 60) {
    let h = state.history.get(key);
    if (!h) {
      h = { vals: [], max };
      state.history.set(key, h);
    }
    h.max = max;
    return h;
  }

  function histMaxPoints() {
    const poll = Math.max(1000, state.pollMs || 3000);
    const pts = Math.ceil(HIST_WINDOW_MS / poll) + 1;
    return Math.max(60, Math.min(300, pts));
  }

  function pushHist(key, v) {
    const h = getHist(key, histMaxPoints());
    h.vals.push(v);
    if (h.vals.length > h.max) h.vals.splice(0, h.vals.length - h.max);
  }

  function windowLabel(key) {
    switch (key) {
      case "1d":
        return "1天";
      case "1w":
        return "1周";
      case "1m":
        return "1月";
      default:
        return key;
    }
  }

  function metricColor() {
    return "rgba(0,0,0,0.95)";
  }

  function renderTop(data) {
    document.getElementById("onlineCount").textContent = data.totals.online_servers;
    document.getElementById("totalCount").textContent = data.totals.total_servers;
    document.getElementById("regionCount").textContent = data.totals.region_count;

    const win = data.traffic.windows[state.trafficWindow] || data.traffic.windows["1d"];
    document.getElementById("trafficUp").textContent = U.fmtBytesIEC(win.tx_bytes);
    document.getElementById("trafficDown").textContent = U.fmtBytesIEC(win.rx_bytes);

    const w1d = data.traffic.windows["1d"];
    const w1w = data.traffic.windows["1w"];
    const w1m = data.traffic.windows["1m"];
    const nowUp = U.fmtBps(data.traffic.now_tx_bps);
    const nowDown = U.fmtBps(data.traffic.now_rx_bps);
    const s1d = w1d ? `日均 ↑${U.fmtBps(w1d.avg_tx_bps)} ↓${U.fmtBps(w1d.avg_rx_bps)}` : "";
    const s1w = w1w ? `周均 ↑${U.fmtBps(w1w.avg_tx_bps)} ↓${U.fmtBps(w1w.avg_rx_bps)}` : "";
    const s1m = w1m ? `月均 ↑${U.fmtBps(w1m.avg_tx_bps)} ↓${U.fmtBps(w1m.avg_rx_bps)}` : "";
    document.getElementById("speedLine").textContent = `现在 ↑${nowUp} ↓${nowDown} · ${s1d} · ${s1w} · ${s1m}`;

    setTapeFromSnapshot(data);
  }

  function summarizeSnapshot(data) {
    let totalConn = 0;
    const offlineNames = [];
    const expSoon = [];
    const trafficSoon = [];
    for (const s of data.servers || []) {
      if (s.online) totalConn += Math.round(s.metrics?.tcp_conn_total || 0);
      if (!s.online) offlineNames.push(s.name || s.id);
      if (typeof s.renew_days === "number" && s.renew_days >= 0 && s.renew_days <= 7) {
        expSoon.push(`${s.name || s.id}(${s.renew_days}天)`);
      }
      if (typeof s.traffic_renew_days === "number" && s.traffic_renew_days >= 0 && s.traffic_renew_days <= 7) {
        trafficSoon.push(`${s.name || s.id}(${s.traffic_renew_days}天)`);
      }
    }
    return { totalConn, offlineNames, expSoon, trafficSoon };
  }

  function tapeField(field, ctx) {
    const { data, key, win, w1d, sum } = ctx;
    const now = new Date();
    const t = `${U.pad2(now.getHours())}:${U.pad2(now.getMinutes())}:${U.pad2(now.getSeconds())}`;
    switch (String(field || "").trim()) {
      case "time":
        return `时间 ${t}`;
      case "online":
        return `在线 ${data.totals.online_servers}/${data.totals.total_servers}`;
      case "regions":
        return `地区 ${data.totals.region_count}`;
      case "traffic_today":
        return `今日流量 ↑${U.fmtBytesIEC(w1d.tx_bytes)} ↓${U.fmtBytesIEC(w1d.rx_bytes)}`;
      case "traffic_window":
        return `流量(${windowLabel(key)}) ↑${U.fmtBytesIEC(win.tx_bytes)} ↓${U.fmtBytesIEC(win.rx_bytes)}`;
      case "speed_now":
        return `速度 ↑${U.fmtBps(data.traffic.now_tx_bps)} ↓${U.fmtBps(data.traffic.now_rx_bps)}`;
      case "conn_total":
        return `连接数 ${sum.totalConn}`;
      case "offline":
        if (!sum.offlineNames.length) return "";
        return `离线 ${sum.offlineNames.length}台: ${sum.offlineNames.slice(0, 6).join(", ")}${
          sum.offlineNames.length > 6 ? "..." : ""
        }`;
      case "expire_soon":
        if (!sum.expSoon.length) return "";
        return `到期≤7天: ${sum.expSoon.slice(0, 6).join(", ")}${sum.expSoon.length > 6 ? "..." : ""}`;
      case "traffic_renew_soon":
        if (!sum.trafficSoon.length) return "";
        return `流量续期≤7天: ${sum.trafficSoon.slice(0, 6).join(", ")}${sum.trafficSoon.length > 6 ? "..." : ""}`;
      default:
        return "";
    }
  }

  let lastTapeText = "";
  function setTapeFromSnapshot(data) {
    const key = state.trafficWindow;
    const win = data.traffic.windows[key] || data.traffic.windows["1d"];
    const w1d = data.traffic.windows["1d"] || win;
    const sum = summarizeSnapshot(data);
    const fields =
      Array.isArray(data.settings?.tape_fields) && data.settings.tape_fields.length
        ? data.settings.tape_fields
        : ["time", "traffic_today", "speed_now", "conn_total", "offline", "expire_soon", "traffic_renew_soon"];

    const parts = [];
    for (const f of fields) {
      const seg = tapeField(f, { data, key, win, w1d, sum });
      if (seg) parts.push(seg);
    }
    let text = parts.join(" /// ");
    if (!text.endsWith("///")) text += " ///";
    if (text === lastTapeText) return;
    lastTapeText = text;
    const inner = document.getElementById("tapeInner");
    if (!inner) return;
    inner.textContent = "";
    for (let i = 0; i < 2; i++) {
      const chunk = document.createElement("div");
      chunk.className = "tape-chunk";
      chunk.textContent = text + " " + text;
      inner.appendChild(chunk);
    }
  }

  const defaultWidgets = [
    "meta",
    "expiry",
    "lastseen",
    "region",
    "traffic_renew",
    "cpu",
    "mem",
    "swap",
    "disk",
    "net",
    "traffic",
    "quota",
    "load",
    "uptime",
    "ports",
  ];

  function applyWidgets(card, widgetsList) {
    const allow =
      Array.isArray(widgetsList) && widgetsList.length
        ? new Set(widgetsList.map((x) => String(x || "").trim()).filter((x) => x))
        : new Set(defaultWidgets);

    card.querySelectorAll("[data-widget]").forEach((node) => {
      const w = node.getAttribute("data-widget");
      if (!w) return;
      node.style.display = allow.has(w) ? "" : "none";
    });
  }

  function expiryLine(s) {
    if (s.expires_date) {
      if (typeof s.renew_days === "number") {
        if (s.renew_days < 0) return `到期日期: ${s.expires_date} · 已过期: ${Math.abs(s.renew_days)}天`;
        return `到期日期: ${s.expires_date} · 距离续费: ${s.renew_days}天`;
      }
      return `到期日期: ${s.expires_date}`;
    }
    if (s.expires_text) return `到期: ${s.expires_text}`;
    return "";
  }

  function primaryTag(s) {
    const tags = Array.isArray(s.tags) ? s.tags : [];
    return tags.length ? String(tags[0] || "").trim() : "";
  }

  function groupKey(tag) {
    return tag ? `tag_${tag}` : "tag__none";
  }

  function isCollapsed(key) {
    return (U.store.get(`group_collapsed_${key}`) || "") === "1";
  }

  function setCollapsed(key, v) {
    U.store.set(`group_collapsed_${key}`, v ? "1" : "0");
  }

  function buildServerCard(tpl, data, s) {
    const el = tpl.content.cloneNode(true);
    const card = el.querySelector(".card");
    card.classList.add(pickShadowClass(s.id || ""));
    applyWidgets(card, s.dashboard_widgets);

    // control mode (active/passive)
    const ctrlBtn = card.querySelector(".ctrl-toggle");
    const ctrlOverlay = card.querySelector(".ctrl-overlay");
    const mode = String(s.control_mode || "").trim().toLowerCase();
    const isActive = mode === "active";
    const controlPort = s.control_port || 38088;
    const controlOK = !!s.control_ok;
    const showBlur = isActive && !!s.online && !controlOK;
    card.classList.toggle("ctrl-pending", showBlur);
    if (ctrlOverlay) {
      ctrlOverlay.classList.toggle("hidden", !showBlur);
      const altPort = controlPort >= 65535 ? 38089 : controlPort === 38088 ? 38089 : controlPort + 1;
      ctrlOverlay.textContent = showBlur
        ? `主动模式未连通：请确认受控端 ${controlPort} 端口可达（防火墙放行/服务监听）。同机部署请避免端口冲突，可改用 ${altPort}`
        : "";
    }
    if (ctrlBtn) {
      if (!state.isAdmin) {
        ctrlBtn.classList.add("hidden");
        card.classList.remove("has-ctrl-toggle");
      } else {
        ctrlBtn.classList.remove("hidden");
        card.classList.add("has-ctrl-toggle");
        ctrlBtn.classList.toggle("on", isActive);
        ctrlBtn.classList.toggle("off", !isActive);
        ctrlBtn.textContent = isActive ? "主动" : "被动";
        ctrlBtn.addEventListener("click", async (ev) => {
          ev.preventDefault();
          ev.stopPropagation();
          try {
            ctrlBtn.disabled = true;
            await adminPatchServer({ id: s.id || "", control_mode: isActive ? "passive" : "active" });
          } catch (e) {
            // no-op
          } finally {
            ctrlBtn.disabled = false;
            tick();
          }
        });
      }
    } else {
      card.classList.remove("has-ctrl-toggle");
    }

    card.querySelector(".server-name").textContent = s.name || s.id;
    const dot = card.querySelector(".status-dot");
    dot.classList.toggle("on", !!s.online);
    dot.classList.toggle("off", !s.online);

    card.querySelector(".expiry").textContent = expiryLine(s);
    card.querySelector(".region").textContent = s.region ? `地区: ${s.region}` : "";

    const lastSeenEl = card.querySelector(".lastseen");
    if (!s.online && s.last_seen_ms) {
      const secAgo = Math.max(0, Math.floor((data.now_ms - s.last_seen_ms) / 1000));
      lastSeenEl.textContent = `最后上报: ${secAgo}秒前`;
    } else {
      lastSeenEl.textContent = "";
    }

    const trEl = card.querySelector(".traffic-renew");
    if (s.traffic_renew_date) {
      if (typeof s.traffic_renew_days === "number") {
        trEl.textContent =
          s.traffic_renew_days < 0
            ? `流量续期: ${s.traffic_renew_date} · 已过期`
            : `流量续期: ${s.traffic_renew_date} · ${s.traffic_renew_days}天`;
      } else {
        trEl.textContent = `流量续期: ${s.traffic_renew_date}`;
      }
    } else {
      trEl.textContent = "";
    }

    const cores = s.meta?.cores || s.metrics?.cpu_cores || 0;
    card.querySelector(".cores").textContent = `${Math.round(cores)} Cores`;
    card.querySelector(".cores2").textContent = `${Math.round(cores)} Cores`;

    const memTotal = s.metrics?.mem_total_bytes || 0;
    const diskTotal = s.metrics?.disk_total_bytes || 0;
    card.querySelector(".mem").textContent = U.fmtBytesIEC(memTotal);
    card.querySelector(".disk").textContent = U.fmtBytesIEC(diskTotal);

    const cpuPct = s.metrics?.cpu_pct || 0;
    card.querySelector(".cpu-pct").textContent = U.fmtPct(cpuPct);
    card.querySelector(".fill.cpu").style.width = `${U.clamp01(cpuPct / 100) * 100}%`;
    pushHist(`${s.id}:cpu`, cpuPct);
    S.drawSpark(card.querySelector('[data-metric="cpu"] canvas'), getHist(`${s.id}:cpu`).vals, metricColor("cpu"));

    const memUsed = s.metrics?.mem_used_bytes || 0;
    const memPct = U.pct(memUsed, memTotal);
    card.querySelector(".mem-pct").textContent = U.fmtPct(memPct);
    card.querySelector(".fill.mem").style.width = `${U.clamp01(memPct / 100) * 100}%`;
    card.querySelector(".mem-sub").textContent = `${U.fmtBytesIEC(memTotal)}  ${U.fmtBytesIEC(memUsed)}`;
    pushHist(`${s.id}:mem`, memPct);
    S.drawSpark(card.querySelector('[data-metric="mem"] canvas'), getHist(`${s.id}:mem`).vals, metricColor("mem"));

    const swapTotal = s.metrics?.swap_total_bytes || 0;
    const swapUsed = s.metrics?.swap_used_bytes || 0;
    const swapPct = U.pct(swapUsed, swapTotal);
    card.querySelector(".swap-pct").textContent = U.fmtPct(swapPct);
    card.querySelector(".fill.swap").style.width = `${U.clamp01(swapPct / 100) * 100}%`;
    card.querySelector(".swap-sub").textContent = `${U.fmtBytesIEC(swapTotal)}  ${U.fmtBytesIEC(swapUsed)}`;
    pushHist(`${s.id}:swap`, swapPct);
    S.drawSpark(card.querySelector('[data-metric="swap"] canvas'), getHist(`${s.id}:swap`).vals, metricColor("swap"));

    const diskUsed = s.metrics?.disk_used_bytes || 0;
    const diskPct = U.pct(diskUsed, diskTotal);
    card.querySelector(".disk-pct").textContent = U.fmtPct(diskPct);
    card.querySelector(".fill.disk").style.width = `${U.clamp01(diskPct / 100) * 100}%`;
    card.querySelector(".disk-sub").textContent = `${U.fmtBytesIEC(diskTotal)}  ${U.fmtBytesIEC(diskUsed)}`;
    pushHist(`${s.id}:disk`, diskPct);
    S.drawSpark(card.querySelector('[data-metric="disk"] canvas'), getHist(`${s.id}:disk`).vals, metricColor("disk"));

    const upBps = s.metrics?.net_tx_bps || 0;
    const downBps = s.metrics?.net_rx_bps || 0;
    card.querySelector(".net-up").textContent = U.fmtBps(upBps);
    card.querySelector(".net-down").textContent = U.fmtBps(downBps);
    pushHist(`${s.id}:net`, upBps + downBps);
    S.drawSpark(card.querySelector('[data-metric="net"] canvas'), getHist(`${s.id}:net`).vals, metricColor("net"));

    const tu = s.metrics?.net_tx_total_bytes || 0;
    const td = s.metrics?.net_rx_total_bytes || 0;
    card.querySelector(".traffic-up").textContent = U.fmtBytesIEC(tu);
    card.querySelector(".traffic-down").textContent = U.fmtBytesIEC(td);
    pushHist(`${s.id}:traffic`, tu + td);
    S.drawSpark(card.querySelector('[data-metric="traffic"] canvas'), getHist(`${s.id}:traffic`).vals, metricColor("traffic"));

    const qWrap = card.querySelector('[data-metric="traffic"] .quota-wrap');
    const qFill = card.querySelector('[data-metric="traffic"] .quota-fill');
    const qText = card.querySelector('[data-metric="traffic"] .quota-text');
    if (s.traffic_total_bytes && s.traffic_total_bytes > 0) {
      const used = s.traffic_used_bytes || 0;
      const ratio = U.clamp01(used / s.traffic_total_bytes);
      qWrap.style.display = "block";
      qFill.style.width = `${ratio * 100}%`;
      qText.textContent = `配额 ${U.fmtBytesIEC(s.traffic_total_bytes)} · 已用 ${U.fmtBytesIEC(used)} · ${Math.round(
        ratio * 100
      )}%`;
    } else {
      qWrap.style.display = "none";
      qText.textContent = "";
      qFill.style.width = "0%";
    }

    const l1 = s.metrics?.load1 || 0;
    const l5 = s.metrics?.load5 || 0;
    const l15 = s.metrics?.load15 || 0;
    card.querySelector(".load").textContent = `${U.fmtLoad(l1)} | ${U.fmtLoad(l5)} | ${U.fmtLoad(l15)}`;
    pushHist(`${s.id}:load`, l1);
    S.drawSpark(card.querySelector('[data-metric="load"] canvas'), getHist(`${s.id}:load`).vals, metricColor("load"));

    const upSec = s.metrics?.uptime_seconds || 0;
    card.querySelector(".uptime").textContent = U.fmtUptime(upSec);

    const portsEl = card.querySelector(".ports");
    portsEl.textContent = "";
    if (Array.isArray(s.ports) && s.ports.length > 0) {
      portsEl.style.display = "flex";
      for (const p of s.ports) {
        const item = document.createElement("div");
        item.className = "port";
        const d = document.createElement("div");
        d.className = "dot " + (p.ok ? "ok" : "bad");
        const txt = document.createElement("div");
        txt.textContent = String(p.port);
        if (typeof p.latency_ms === "number") item.title = `${p.latency_ms}ms`;
        item.appendChild(d);
        item.appendChild(txt);
        portsEl.appendChild(item);
      }
    } else {
      portsEl.style.display = "none";
    }

    return el;
  }

  function renderGrouped(container, tpl, data) {
    const groups = new Map();
    for (const s of data.servers) {
      const tag = primaryTag(s);
      const key = groupKey(tag);
      if (!groups.has(key)) groups.set(key, { tag, items: [] });
      groups.get(key).items.push(s);
    }
    const ordered = [...groups.values()].sort((a, b) => {
      const an = a.tag || "未分组";
      const bn = b.tag || "未分组";
      if (an === "未分组" && bn !== "未分组") return -1;
      if (bn === "未分组" && an !== "未分组") return 1;
      return an.localeCompare(bn);
    });

    for (const g of ordered) {
      const key = groupKey(g.tag);
      const section = document.createElement("section");
      section.className = "group";
      if (isCollapsed(key)) section.classList.add("collapsed");

      const head = document.createElement("div");
      head.className = "group-head";
      const title = document.createElement("div");
      title.className = "group-title";
      title.textContent = `${g.tag ? g.tag : "未分组"}（${g.items.length}）`;
      const btn = document.createElement("button");
      btn.className = "pill pill-sm group-btn";
      btn.type = "button";
      btn.textContent = section.classList.contains("collapsed") ? "展开" : "折叠";
      btn.addEventListener("click", () => {
        const next = !section.classList.contains("collapsed");
        section.classList.toggle("collapsed", next);
        btn.textContent = next ? "展开" : "折叠";
        setCollapsed(key, next);
      });
      head.appendChild(title);
      head.appendChild(btn);

      const body = document.createElement("div");
      body.className = "group-body";
      const grid = document.createElement("div");
      grid.className = "group-grid";
      for (const s of g.items) {
        grid.appendChild(buildServerCard(tpl, data, s));
      }
      body.appendChild(grid);

      section.appendChild(head);
      section.appendChild(body);
      container.appendChild(section);
    }
  }

  function renderCards(data) {
    const container = document.getElementById("cards");
    container.textContent = "";
    const tpl = document.getElementById("cardTpl");

    if (data.settings?.enable_grouping) {
      renderGrouped(container, tpl, data);
      return;
    }
    for (const s of data.servers) {
      container.appendChild(buildServerCard(tpl, data, s));
    }
  }

  async function fetchSnapshot() {
    const res = await fetch("/api/snapshot", { cache: "no-store" });
    if (!res.ok) throw new Error(`snapshot ${res.status}`);
    return res.json();
  }

  let timer = null;
  async function tick() {
    try {
      const data = await fetchSnapshot();
      renderTop(data);
      renderCards(data);

      const newPollMs = Math.max(1000, (data.settings?.dashboard_poll_seconds || 3) * 1000);
      if (newPollMs !== state.pollMs) {
        state.pollMs = newPollMs;
        if (timer) clearInterval(timer);
        timer = setInterval(tick, state.pollMs);
      }
    } catch {
      // keep last UI; retry next tick
    }
  }

  function init() {
    applyCompact();
    checkAdminSession();
    setNowTime();
    setInterval(setNowTime, 1000);
    setInterval(checkAdminSession, 30000);

    const compactBtn = document.getElementById("compactBtn");
    if (compactBtn) {
      compactBtn.addEventListener("click", () => {
        ui.compact = !ui.compact;
        U.store.set("ui_compact", ui.compact ? "1" : "0");
        applyCompact();
      });
    }

    const sel = document.getElementById("trafficWindow");
    sel.value = state.trafficWindow;
    sel.addEventListener("change", () => {
      state.trafficWindow = sel.value;
      tick();
    });

    tick();
    timer = setInterval(tick, state.pollMs);
  }

  TZ.dashboard = { init };
})();
