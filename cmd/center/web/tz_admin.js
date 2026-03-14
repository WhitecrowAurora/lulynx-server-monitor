(function () {
  window.TZ = window.TZ || {};
  const U = TZ.util;
  const M = TZ.modal;

  const widgetOptions = [
    { id: "meta", label: "头部信息", desc: "CPU核/内存总量/磁盘总量" },
    { id: "expiry", label: "到期信息", desc: "到期日期/距离续费" },
    { id: "lastseen", label: "最后上报", desc: "离线时显示最后上报秒数" },
    { id: "region", label: "地区", desc: "地区标签" },
    { id: "traffic_renew", label: "流量续期", desc: "流量续期日期/剩余天数" },
    { id: "cpu", label: "CPU", desc: "使用率 + 趋势" },
    { id: "mem", label: "内存", desc: "使用率 + 趋势" },
    { id: "swap", label: "SWAP", desc: "使用率 + 趋势" },
    { id: "disk", label: "硬盘", desc: "使用率 + 趋势" },
    { id: "net", label: "网速", desc: "↑↓实时 + 趋势" },
    { id: "traffic", label: "流量", desc: "↑↓累计 + 趋势" },
    { id: "quota", label: "配额条", desc: "总流量/已用/百分比" },
    { id: "load", label: "负载", desc: "1/5/15 + 趋势" },
    { id: "uptime", label: "在线时长", desc: "uptime" },
    { id: "ports", label: "端口探活", desc: "端口绿/红点 + 延迟" },
  ];

  const tapeOptions = [
    { id: "time", label: "时间", desc: "HH:mm:ss" },
    { id: "traffic_today", label: "今日流量", desc: "↑↓" },
    { id: "speed_now", label: "当前总网速", desc: "↑↓" },
    { id: "conn_total", label: "总连接数", desc: "在线服务器 TCP 总连接" },
    { id: "online", label: "在线统计", desc: "在线/总数" },
    { id: "regions", label: "地区统计", desc: "地区数量" },
    { id: "traffic_window", label: "窗口流量", desc: "按 1天/1周/1月选择" },
    { id: "offline", label: "离线提示", desc: "列出离线服务器" },
    { id: "expire_soon", label: "到期≤7天", desc: "列出即将到期" },
    { id: "traffic_renew_soon", label: "流量续期≤7天", desc: "列出即将续期" },
  ];

  const adminState = {
    token: U.store.get("admin_token") || "",
    settings: null,
    servers: [],
    bans: [],
    adminBans: [],
  };

  function q(id) {
    return document.getElementById(id);
  }

  function setTape(text) {
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

  function tokenOk() {
    return !!adminState.token;
  }

  function inferCenterPort() {
    const p = parseInt(location.port || "", 10);
    if (Number.isInteger(p) && p > 0 && p <= 65535) return p;
    return location.protocol === "https:" ? 443 : 80;
  }

  function setLocked(locked) {
    document.body.classList.toggle("admin-locked", !!locked);
  }

  function showMsg(el, msg) {
    el.textContent = msg;
    setTimeout(() => {
      if (el.textContent === msg) el.textContent = "";
    }, 3000);
  }

  async function apiGet(path) {
    const res = await fetch(path, {
      headers: adminState.token ? { "X-Admin-Token": adminState.token } : {},
      cache: "no-store",
    });
    if (!res.ok) throw new Error(`${path} ${res.status}`);
    return res.json();
  }

  async function apiPost(path, body) {
    const res = await fetch(path, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(adminState.token ? { "X-Admin-Token": adminState.token } : {}),
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(`${path} ${res.status}`);
    return res.json();
  }

  async function loadSettings() {
    const data = await apiGet("/api/admin/settings");
    adminState.settings = data.settings;
    q("setCollect").value = adminState.settings.default_collect_interval_seconds || 5;
    q("setRetention").value = adminState.settings.retention_days || 30;
    q("setPoll").value = adminState.settings.dashboard_poll_seconds || 3;
    q("setGrouping").checked = !!adminState.settings.enable_grouping;
    q("setTape").value = (adminState.settings.tape_fields || []).join(",");
  }

  async function loadServers() {
    const data = await apiGet("/api/admin/servers");
    adminState.servers = data.servers || [];
  }

  async function loadBans() {
    const data = await apiGet("/api/admin/bans");
    adminState.bans = data.entries || [];
  }

  async function loadAdminBans() {
    const data = await apiGet("/api/admin/admin_bans");
    adminState.adminBans = data.entries || [];
  }

  function fmtMS(ms) {
    if (!ms) return "-";
    const d = new Date(ms);
    const pad = (n) => String(n).padStart(2, "0");
    return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  }

  function renderBans() {
    const tbody = q("bansTable")?.querySelector("tbody");
    if (!tbody) return;
    tbody.textContent = "";
    const now = Date.now();
    for (const e of adminState.bans || []) {
      const tr = document.createElement("tr");
      const bannedUntil = e.banned_until_ms || 0;
      const isBanned = bannedUntil > now;
      tr.innerHTML = `
        <td>${U.escapeHtml(e.ip || "")}</td>
        <td>${e.fail_count || 0}</td>
        <td><span class="tag ${isBanned ? "bad" : "ok"}">${U.escapeHtml(fmtMS(bannedUntil))}</span></td>
        <td>${U.escapeHtml(fmtMS(e.last_fail_ms || 0))}</td>
        <td><button class="pill pill-sm unban" type="button">解除</button></td>
      `;
      tr.querySelector(".unban").addEventListener("click", async () => {
        try {
          await apiPost("/api/admin/bans", { ip: e.ip || "" });
          showMsg(q("bansMsg"), `已解除：${e.ip || ""}`);
          await refreshAll();
        } catch (err) {
          showMsg(q("bansMsg"), `解除失败：${err.message || err}`);
        }
      });
      tbody.appendChild(tr);
    }
  }

  function renderAdminBans() {
    const tbl = q("adminBansTable");
    if (!tbl) return;
    const tbody = tbl.querySelector("tbody");
    if (!tbody) return;
    tbody.textContent = "";
    const now = Date.now();
    for (const e of adminState.adminBans || []) {
      const tr = document.createElement("tr");
      const bannedUntil = e.banned_until_ms || 0;
      const isBanned = bannedUntil > now;
      tr.innerHTML = `
        <td>${U.escapeHtml(e.ip || "")}</td>
        <td>${e.fail_count || 0}</td>
        <td><span class="tag ${isBanned ? "bad" : "ok"}">${U.escapeHtml(fmtMS(bannedUntil))}</span></td>
        <td>${U.escapeHtml(fmtMS(e.last_fail_ms || 0))}</td>
        <td><button class="pill pill-sm unban" type="button">解除</button></td>
      `;
      tr.querySelector(".unban").addEventListener("click", async () => {
        try {
          await apiPost("/api/admin/admin_bans", { ip: e.ip || "" });
          showMsg(q("adminBansMsg"), `已解除：${e.ip || ""}`);
          await refreshAll();
        } catch (err) {
          showMsg(q("adminBansMsg"), `解除失败：${err.message || err}`);
        }
      });
      tbody.appendChild(tr);
    }
  }

  function matchesFilter(row, term) {
    if (!term) return true;
    term = term.toLowerCase();
    const c = row.config || {};
    const s = `${c.id || ""} ${c.name || ""} ${c.region || ""}`.toLowerCase();
    return s.includes(term);
  }

  function renderServers() {
    const tbody = q("serversTable").querySelector("tbody");
    tbody.textContent = "";
    const term = (q("filterInput").value || "").trim();
    const now = Date.now();
    const centerPort = inferCenterPort();

    for (const row of adminState.servers) {
      if (!matchesFilter(row, term)) continue;

      const c = row.config || {};
      const tr = document.createElement("tr");
      const visible = c.visible !== false;
      const online = !!row.online;
      const ctrlActive = String(c.control_mode || "").trim().toLowerCase() === "active";

      tr.innerHTML = `
        <td><input type="checkbox" class="chk-visible" ${visible ? "checked" : ""}></td>
        <td class="mono">${U.escapeHtml(c.id || "")}</td>
        <td><input class="input input-sm name" value="${U.escapeHtml(c.name || "")}"></td>
        <td><input class="input input-sm region" value="${U.escapeHtml(c.region || "")}"></td>
        <td><input class="input input-sm tags" placeholder="prod,us,db" value="${U.escapeHtml((c.tags || []).join(","))}"></td>
        <td>${online ? '<span class="tag ok">在线</span>' : '<span class="tag bad">离线</span>'}</td>
        <td class="mono">${row.last_seen_ms ? U.fmtAgo(now - row.last_seen_ms) : "-"}</td>
        <td><input class="input input-sm expires" placeholder="YYYY-MM-DD" value="${U.escapeHtml(c.expires_date || "")}"></td>
        <td><input class="input input-sm quota" placeholder="TB" value="${U.escapeHtml(U.bytesToTB(c.traffic_total_bytes || 0))}"></td>
        <td><input class="input input-sm renew" placeholder="YYYY-MM-DD" value="${U.escapeHtml(c.traffic_renew_date || "")}"></td>
        <td>
          <div class="row-mini">
            <input class="input input-sm widgets" placeholder="cpu,mem,disk,net,uptime..." value="${U.escapeHtml(
              (c.dashboard_widgets || []).join(",")
            )}">
            <div class="mini-actions">
              <button class="pill pill-sm edit-widgets" type="button">编辑</button>
              <button class="pill pill-sm preset-default" type="button" style="background:#fff">默认</button>
              <button class="pill pill-sm preset-compact" type="button">简洁</button>
            </div>
          </div>
        </td>
        <td><input type="checkbox" class="chk-control" ${ctrlActive ? "checked" : ""}></td>
        <td><input class="input input-sm ctrl-port" placeholder="38088" title="提示：同机部署时受控端口不要与中心端口(${centerPort})冲突" value="${U.escapeHtml(
          String(c.control_port || 38088)
        )}" ${ctrlActive ? "" : "disabled"}></td>
        <td><input type="checkbox" class="chk-port" ${c.port_probe_enabled ? "checked" : ""}></td>
        <td><input class="input input-sm ports" placeholder="22,80,443" value="${U.escapeHtml((c.ports || []).join(","))}"></td>
        <td><button class="pill pill-sm save" type="button">保存</button></td>
      `;

      tr.querySelector(".preset-default").addEventListener("click", () => {
        tr.querySelector(".widgets").value = "";
      });
      tr.querySelector(".preset-compact").addEventListener("click", () => {
        tr.querySelector(".widgets").value = "meta,expiry,region,traffic_renew,cpu,mem,swap,disk,net,uptime";
      });
      tr.querySelector(".edit-widgets").addEventListener("click", () => {
        const cur = (tr.querySelector(".widgets").value || "")
          .split(",")
          .map((x) => x.trim())
          .filter((x) => x);
        M.openListModal({
          title: `编辑模块：${c.id || ""}`,
          options: widgetOptions,
          currentList: cur,
          onSave: (out) => {
            tr.querySelector(".widgets").value = out.join(",");
          },
        });
      });

      tr.querySelector(".chk-control").addEventListener("change", () => {
        const on = tr.querySelector(".chk-control").checked;
        tr.querySelector(".ctrl-port").disabled = !on;
        if (on) {
          const altPort = centerPort >= 65535 ? 38089 : centerPort + 1;
          const p = parseInt(tr.querySelector(".ctrl-port").value.trim(), 10) || 38088;
          if (p === centerPort) {
            showMsg(
              q("serversMsg"),
              `提示：如果中心和该客户端在同一台机器，受控端口不要与中心端口(${centerPort})相同，可改为 ${altPort}`
            );
          }
        }
      });

      tr.querySelector(".save").addEventListener("click", async () => {
        try {
          const next = { ...c };
          next.visible = tr.querySelector(".chk-visible").checked;
          next.name = tr.querySelector(".name").value.trim() || c.id;
          next.region = tr.querySelector(".region").value.trim();
          const tagsStr = tr.querySelector(".tags").value.trim();
          next.tags = tagsStr
            ? tagsStr
                .split(",")
                .map((x) => x.trim())
                .filter((x) => x.length > 0)
            : [];
          next.expires_date = tr.querySelector(".expires").value.trim();
          next.traffic_total_bytes = U.tbToBytes(tr.querySelector(".quota").value.trim());
          next.traffic_renew_date = tr.querySelector(".renew").value.trim();

          const widgetsStr = tr.querySelector(".widgets").value.trim();
          next.dashboard_widgets = widgetsStr
            ? widgetsStr
                .split(",")
                .map((x) => x.trim())
                .filter((x) => x.length > 0)
            : [];

          const controlOn = tr.querySelector(".chk-control").checked;
          next.control_mode = controlOn ? "active" : "passive";
          if (controlOn) {
            const p = parseInt(tr.querySelector(".ctrl-port").value.trim(), 10);
            next.control_port = Number.isInteger(p) && p > 0 && p <= 65535 ? p : 38088;
          } else {
            next.control_port = 0;
          }

          next.port_probe_enabled = tr.querySelector(".chk-port").checked;
          const portsStr = tr.querySelector(".ports").value.trim();
          next.ports = portsStr
            ? portsStr
                .split(",")
                .map((x) => parseInt(x.trim(), 10))
                .filter((n) => Number.isInteger(n) && n > 0 && n <= 65535)
            : [];

          await apiPost("/api/admin/server", next);
          showMsg(q("serversMsg"), `已保存：${next.id}`);
          await refreshAll();
        } catch (e) {
          showMsg(q("serversMsg"), `保存失败：${e.message || e}`);
        }
      });

      tbody.appendChild(tr);
    }
  }

  async function refreshAll() {
    if (!tokenOk()) return;
    setTape("控制面板 /// 正在加载... ///");
    await loadSettings();
    await loadServers();
    await loadBans();
    await loadAdminBans();
    renderServers();
    renderBans();
    renderAdminBans();
    setTape(`控制面板 /// 已探测 ${adminState.servers.length} 台服务器 /// 在此勾选“显示”以出现在监控主页 ///`);
  }

  function bind() {
    q("tokenInput").value = adminState.token;

    q("loginBtn").addEventListener("click", async () => {
      adminState.token = q("tokenInput").value.trim();
      if (!adminState.token) {
        showMsg(q("settingsMsg"), "请输入管理密码");
        setLocked(true);
        return;
      }
      U.store.set("admin_token", adminState.token);
      try {
        setLocked(true);
        await refreshAll();
        setLocked(false);
        showMsg(q("settingsMsg"), "登录成功");
      } catch {
        showMsg(q("settingsMsg"), "登录失败（管理密码不正确？）");
        adminState.token = "";
        U.store.del("admin_token");
        setLocked(true);
        setTape("控制面板 /// 请先登录 ///");
      }
    });

    q("logoutBtn").addEventListener("click", () => {
      adminState.token = "";
      U.store.del("admin_token");
      q("tokenInput").value = "";
      q("serversTable").querySelector("tbody").textContent = "";
      const bt = q("bansTable");
      if (bt) {
        const btb = bt.querySelector("tbody");
        if (btb) btb.textContent = "";
      }
      const at = q("adminBansTable");
      if (at) {
        const atb = at.querySelector("tbody");
        if (atb) atb.textContent = "";
      }
      setLocked(true);
      setTape("控制面板 /// 请先登录 ///");
    });

    q("saveSettings").addEventListener("click", async () => {
      try {
        const patch = {
          default_collect_interval_seconds: parseInt(q("setCollect").value, 10) || 5,
          retention_days: parseInt(q("setRetention").value, 10) || 30,
          dashboard_poll_seconds: parseInt(q("setPoll").value, 10) || 3,
          enable_grouping: !!q("setGrouping").checked,
          tape_fields: (q("setTape").value || "")
            .split(",")
            .map((x) => x.trim())
            .filter((x) => x.length > 0),
        };
        await apiPost("/api/admin/settings", patch);
        showMsg(q("settingsMsg"), "已保存");
      } catch (e) {
        showMsg(q("settingsMsg"), `保存失败：${e.message || e}`);
      }
    });

    q("tapeDefault").addEventListener("click", () => {
      q("setTape").value = "time,traffic_today,speed_now,conn_total,offline,expire_soon,traffic_renew_soon";
    });

    q("editTape").addEventListener("click", () => {
      const cur = (q("setTape").value || "")
        .split(",")
        .map((x) => x.trim())
        .filter((x) => x);
      M.openListModal({
        title: "编辑滚动条字段",
        options: tapeOptions,
        currentList: cur,
        onSave: (out) => {
          q("setTape").value = out.join(",");
        },
      });
    });

    q("refreshBtn").addEventListener("click", async () => {
      try {
        await refreshAll();
      } catch (e) {
        showMsg(q("serversMsg"), `刷新失败：${e.message || e}`);
      }
    });

    q("refreshBansBtn")?.addEventListener("click", async () => {
      try {
        await refreshAll();
      } catch (e) {
        showMsg(q("bansMsg"), `刷新失败：${e.message || e}`);
      }
    });

    q("unbanBtn")?.addEventListener("click", async () => {
      const ip = (q("banIpInput")?.value || "").trim();
      if (!ip) return;
      try {
        await apiPost("/api/admin/bans", { ip });
        q("banIpInput").value = "";
        showMsg(q("bansMsg"), `已解除：${ip}`);
        await refreshAll();
      } catch (e) {
        showMsg(q("bansMsg"), `解除失败：${e.message || e}`);
      }
    });

    q("refreshAdminBansBtn")?.addEventListener("click", async () => {
      try {
        await refreshAll();
      } catch (e) {
        showMsg(q("adminBansMsg"), `刷新失败：${e.message || e}`);
      }
    });

    q("adminUnbanBtn")?.addEventListener("click", async () => {
      const ip = (q("adminBanIpInput")?.value || "").trim();
      if (!ip) return;
      try {
        await apiPost("/api/admin/admin_bans", { ip });
        q("adminBanIpInput").value = "";
        showMsg(q("adminBansMsg"), `已解除：${ip}`);
        await refreshAll();
      } catch (e) {
        showMsg(q("adminBansMsg"), `解除失败：${e.message || e}`);
      }
    });

    q("filterInput").addEventListener("input", () => renderServers());
  }

  function init() {
    bind();
    if (tokenOk()) {
      setLocked(true);
      refreshAll()
        .then(() => setLocked(false))
        .catch(() => {
          setLocked(true);
          setTape("控制面板 /// 请检查管理密码 ///");
        });
    } else {
      setLocked(true);
      setTape("控制面板 /// 请先登录 ///");
    }
  }

  TZ.admin = { init };
})();
