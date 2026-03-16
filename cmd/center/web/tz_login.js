(function () {
  function q(id) {
    return document.getElementById(id);
  }

  function safeNext() {
    const sp = new URLSearchParams(location.search || "");
    const next = (sp.get("next") || "").trim();
    if (next.startsWith("/")) return next;
    return "/admin";
  }

  function pad2(n) {
    return String(n).padStart(2, "0");
  }

  function fmtMS(ms) {
    if (!ms) return "";
    const d = new Date(ms);
    return `${d.getFullYear()}/${pad2(d.getMonth() + 1)}/${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(
      d.getSeconds()
    )}`;
  }

  function setMsg(t) {
    const el = q("loginMsg");
    if (el) el.textContent = t || "";
  }

  async function login() {
    const username = (q("userInput")?.value || "").trim();
    const password = (q("passInput")?.value || "").trim();
    if (!username || !password) {
      setMsg("请输入用户名和密码");
      return;
    }
    setMsg("正在登录...");

    let res;
    try {
      res = await fetch("/api/admin/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ username, password }),
      });
    } catch (e) {
      setMsg("网络错误，请重试");
      return;
    }

    if (res.ok) {
      location.href = safeNext();
      return;
    }

    if (res.status === 429) {
      const data = await res.json().catch(() => null);
      const until = data?.banned_until_ms ? fmtMS(data.banned_until_ms) : "";
      setMsg(until ? `已被临时封禁至：${until}` : "已被临时封禁，请稍后再试");
      return;
    }

    setMsg("用户名或密码错误");
  }

  document.addEventListener("DOMContentLoaded", () => {
    const user = q("userInput");
    const pass = q("passInput");
    const btn = q("loginBtn");
    if (btn) btn.addEventListener("click", login);
    if (user) {
      user.addEventListener("keydown", (ev) => {
        if (ev.key === "Enter") login();
      });
    }
    if (pass) {
      pass.addEventListener("keydown", (ev) => {
        if (ev.key === "Enter") login();
      });
    }
    user?.focus();
  });
})();

