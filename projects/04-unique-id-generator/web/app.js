// Tab switching
const tabs = document.querySelectorAll(".tab");
const panels = document.querySelectorAll(".tab-panel");

tabs.forEach((tab) => {
  tab.addEventListener("click", () => {
    tabs.forEach((t) => t.classList.remove("active"));
    panels.forEach((p) => p.classList.add("hidden"));
    tab.classList.add("active");
    document.querySelector(`[data-panel="${tab.dataset.tab}"]`).classList.remove("hidden");
  });
});

// Output elements
const latestIdEl = document.querySelector("#latest-id");
const copyIdBtn = document.querySelector("#copy-id");
const chipsEl = document.querySelector("#chips");
const resultEl = document.querySelector("#result");

let latestID = null;

function showJSON(data) {
  resultEl.textContent = JSON.stringify(data, null, 2);
}

function makeChip(text, cls) {
  const span = document.createElement("span");
  span.className = cls ? `chip ${cls}` : "chip";
  span.textContent = text;
  return span;
}

function setLatest(idStr) {
  latestID = idStr;
  latestIdEl.textContent = idStr;
  copyIdBtn.disabled = false;
}

function clearChips() {
  chipsEl.replaceChildren();
}

function showSingleChips(data) {
  clearChips();
  chipsEl.appendChild(makeChip(`worker ${data.worker_id}`, "worker"));
  chipsEl.appendChild(makeChip(data.region));
}

function showInspectChips(data) {
  clearChips();
  chipsEl.appendChild(makeChip(`worker ${data.worker_id}`, "worker"));
  chipsEl.appendChild(makeChip(`seq ${data.sequence}`, "seq"));
  chipsEl.appendChild(makeChip(new Date(data.timestamp_ms).toLocaleString(), "time"));
}

// Single ID
document.querySelector("#next-btn").addEventListener("click", async () => {
  const res = await fetch("/v1/ids/next", { method: "POST" });
  const data = await res.json();
  showJSON(data);
  if (res.ok) {
    setLatest(data.id_string);
    showSingleChips(data);
    // Pre-fill inspect field
    document.querySelector("#inspect-id").value = data.id_string;
  }
});

// Batch
document.querySelector("#batch-btn").addEventListener("click", async () => {
  const count = parseInt(document.querySelector("#batch-count").value, 10) || 10;
  const res = await fetch("/v1/ids/batch", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ count }),
  });
  const data = await res.json();
  showJSON(data);
  if (res.ok && data.id_strings && data.id_strings.length > 0) {
    setLatest(`${data.id_strings[0]} … (${data.count} total)`);
    clearChips();
    chipsEl.appendChild(makeChip(`${data.count} IDs`, "worker"));
    chipsEl.appendChild(makeChip(`worker ${data.worker_id}`, "worker"));
    // Pre-fill inspect with first ID
    document.querySelector("#inspect-id").value = data.id_strings[0];
  }
});

// Inspect
document.querySelector("#inspect-btn").addEventListener("click", async () => {
  const id = document.querySelector("#inspect-id").value.trim();
  if (!id) return;
  const res = await fetch(`/v1/ids/${id}/inspect`);
  const data = await res.json();
  showJSON(data);
  if (res.ok) {
    setLatest(data.id);
    showInspectChips(data);
  }
});

// Worker health
document.querySelector("#worker-btn").addEventListener("click", async () => {
  const res = await fetch("/v1/workers/health");
  const data = await res.json();
  showJSON(data);
  if (res.ok) {
    clearChips();
    chipsEl.appendChild(makeChip(`worker ${data.worker_id}`, "worker"));
    chipsEl.appendChild(makeChip(data.region));
    chipsEl.appendChild(makeChip(data.status));
  }
});

// Copy ID
copyIdBtn.addEventListener("click", async () => {
  if (!latestID) return;
  await navigator.clipboard.writeText(latestID);
  copyIdBtn.textContent = "Copied";
  setTimeout(() => { copyIdBtn.textContent = "Copy ID"; }, 1200);
});
