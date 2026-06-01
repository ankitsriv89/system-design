// Tab switching
const tabs = document.querySelectorAll(".tab");
const panels = document.querySelectorAll(".tab-panel");

tabs.forEach((tab) => {
  tab.addEventListener("click", () => {
    tabs.forEach((t) => t.classList.remove("active"));
    panels.forEach((p) => p.classList.add("hidden"));
    tab.classList.add("active");
    const target = tab.dataset.tab;
    document.querySelector(`[data-panel="${target}"]`).classList.remove("hidden");
  });
});

// Output elements
const pasteIdDisplay = document.querySelector("#paste-id-display");
const copyIdBtn = document.querySelector("#copy-id");
const metaEl = document.querySelector("#meta");
const resultEl = document.querySelector("#result");

let latestID = null;

function showJSON(data) {
  resultEl.textContent = JSON.stringify(data, null, 2);
}

function showText(text) {
  resultEl.textContent = text;
}

function makeChip(text, extraClass) {
  const span = document.createElement("span");
  span.className = extraClass ? `meta-chip ${extraClass}` : "meta-chip";
  span.textContent = text;
  return span;
}

function showMeta(p) {
  metaEl.classList.remove("hidden");
  metaEl.replaceChildren();
  if (p.language) metaEl.appendChild(makeChip(p.language));
  if (p.visibility) metaEl.appendChild(makeChip(p.visibility));
  if (p.size_bytes != null) metaEl.appendChild(makeChip(`${p.size_bytes} bytes`));
  if (p.expires_at) metaEl.appendChild(makeChip(`expires ${new Date(p.expires_at).toLocaleString()}`, "ttl"));
  if (p.burn_after_read) metaEl.appendChild(makeChip("burn after read", "burn"));
}

function setLatestID(id) {
  latestID = id;
  pasteIdDisplay.textContent = id;
  copyIdBtn.disabled = false;
  // Pre-fill the fetch panel
  document.querySelector("#fetch-id").value = id;
}

// Create paste
document.querySelector("#create-form").addEventListener("submit", async (e) => {
  e.preventDefault();

  const body = {
    content: document.querySelector("#content").value,
    visibility: document.querySelector("#visibility").value,
  };

  const title = document.querySelector("#title").value.trim();
  if (title) body.title = title;

  const lang = document.querySelector("#language").value;
  if (lang) body.language = lang;

  const ttl = document.querySelector("#ttl").value;
  if (ttl) body.ttl_seconds = parseInt(ttl, 10);

  if (document.querySelector("#burn").checked) body.burn_after_read = true;

  const res = await fetch("/v1/pastes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  const data = await res.json();
  showJSON(data);

  if (res.ok) {
    setLatestID(data.id);
    showMeta(data);
  } else {
    metaEl.classList.add("hidden");
  }
});

// Fetch paste
document.querySelector("#fetch-btn").addEventListener("click", async () => {
  const id = document.querySelector("#fetch-id").value.trim();
  if (!id) return;

  const res = await fetch(`/v1/pastes/${id}`);
  const data = await res.json();
  showJSON(data);

  if (res.ok) {
    setLatestID(data.id);
    showMeta(data);
  } else {
    metaEl.classList.add("hidden");
  }
});

// Raw view — opens in new tab
document.querySelector("#raw-btn").addEventListener("click", () => {
  const id = document.querySelector("#fetch-id").value.trim();
  if (!id) return;
  window.open(`/v1/pastes/${id}/raw`, "_blank", "noopener,noreferrer");
});

// Delete paste
document.querySelector("#delete-btn").addEventListener("click", async () => {
  const id = document.querySelector("#fetch-id").value.trim();
  if (!id) return;

  const res = await fetch(`/v1/pastes/${id}`, { method: "DELETE" });

  if (res.status === 204) {
    showJSON({ deleted: id });
    metaEl.classList.add("hidden");
    pasteIdDisplay.textContent = "Deleted";
    latestID = null;
    copyIdBtn.disabled = true;
  } else {
    const data = await res.json();
    showJSON(data);
  }
});

// Copy ID
copyIdBtn.addEventListener("click", async () => {
  if (!latestID) return;
  await navigator.clipboard.writeText(latestID);
  copyIdBtn.textContent = "Copied";
  setTimeout(() => { copyIdBtn.textContent = "Copy ID"; }, 1200);
});
