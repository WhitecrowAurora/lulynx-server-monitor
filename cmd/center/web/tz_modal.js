(function () {
  window.TZ = window.TZ || {};

  const modal = {
    el: null,
    title: null,
    list: null,
    onSave: null,
  };

  function q(id) {
    return document.getElementById(id);
  }

  function openListModal({ title, options, currentList, onSave }) {
    if (!modal.el) initModal();
    modal.title.textContent = title;
    modal.onSave = onSave;

    const cur = Array.isArray(currentList)
      ? currentList.map((x) => String(x || "").trim()).filter((x) => x)
      : [];
    const curSet = new Set(cur);
    const ordered = [
      ...cur.filter((id) => options.some((o) => o.id === id)),
      ...options.map((o) => o.id).filter((id) => !curSet.has(id)),
    ];

    modal.list.textContent = "";
    for (const id of ordered) {
      const opt = options.find((o) => o.id === id);
      if (!opt) continue;
      modal.list.appendChild(buildSortItem(opt, curSet.has(id)));
    }

    modal.el.classList.remove("hidden");
    modal.el.setAttribute("aria-hidden", "false");
  }

  function closeListModal() {
    if (!modal.el) return;
    modal.el.classList.add("hidden");
    modal.el.setAttribute("aria-hidden", "true");
    modal.onSave = null;
  }

  function initModal() {
    modal.el = q("listModal");
    modal.title = q("modalTitle");
    modal.list = q("modalList");

    q("modalClose").addEventListener("click", closeListModal);
    q("modalCancel").addEventListener("click", closeListModal);
    q("modalBackdrop").addEventListener("click", closeListModal);
    q("modalSave").addEventListener("click", () => {
      const items = [...modal.list.querySelectorAll(".sort-item")];
      const out = [];
      for (const it of items) {
        const id = it.getAttribute("data-id");
        const chk = it.querySelector('input[type="checkbox"]');
        if (chk && chk.checked && id) out.push(id);
      }
      if (typeof modal.onSave === "function") modal.onSave(out);
      closeListModal();
    });
  }

  function buildSortItem(opt, checked) {
    const item = document.createElement("div");
    item.className = "sort-item";
    item.draggable = true;
    item.setAttribute("data-id", opt.id);

    const handle = document.createElement("div");
    handle.className = "drag-handle";
    handle.textContent = "≡";

    const chk = document.createElement("input");
    chk.type = "checkbox";
    chk.checked = !!checked;

    const label = document.createElement("div");
    label.className = "sort-label";
    label.textContent = opt.label;

    const desc = document.createElement("div");
    desc.className = "sort-desc";
    desc.textContent = opt.desc || "";

    item.appendChild(handle);
    item.appendChild(chk);
    item.appendChild(label);
    item.appendChild(desc);

    item.addEventListener("dragstart", (e) => {
      item.classList.add("dragging");
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", opt.id);
    });
    item.addEventListener("dragend", () => item.classList.remove("dragging"));
    item.addEventListener("dragover", (e) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = "move";
      const dragging = modal.list.querySelector(".sort-item.dragging");
      if (!dragging || dragging === item) return;
      const rect = item.getBoundingClientRect();
      const before = e.clientY < rect.top + rect.height / 2;
      modal.list.insertBefore(dragging, before ? item : item.nextSibling);
    });

    return item;
  }

  TZ.modal = { openListModal, closeListModal };
})();

