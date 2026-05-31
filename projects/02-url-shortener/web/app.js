const form = document.querySelector("#create-form");
const owner = document.querySelector("#owner");
const longUrl = document.querySelector("#long-url");
const expiresAt = document.querySelector("#expires-at");
const codeEl = document.querySelector("#code");
const shortUrlEl = document.querySelector("#short-url");
const result = document.querySelector("#result");
const copy = document.querySelector("#copy");
const visit = document.querySelector("#visit");
const stats = document.querySelector("#stats");

let latest = null;

function show(data) {
  result.textContent = JSON.stringify(data, null, 2);
}

function setLatest(link) {
  latest = link;
  codeEl.textContent = link.code;
  shortUrlEl.textContent = link.short_url;
  shortUrlEl.href = link.short_url;
  copy.disabled = false;
  visit.disabled = false;
  stats.disabled = false;
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const body = { long_url: longUrl.value };
  if (expiresAt.value) {
    body.expires_at = new Date(expiresAt.value).toISOString();
  }
  const response = await fetch("/v1/links", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Owner-ID": owner.value,
    },
    body: JSON.stringify(body),
  });
  const data = await response.json();
  show(data);
  if (response.ok) {
    setLatest(data);
  }
});

copy.addEventListener("click", async () => {
  if (!latest) return;
  await navigator.clipboard.writeText(latest.short_url);
  copy.textContent = "Copied";
  setTimeout(() => {
    copy.textContent = "Copy";
  }, 1200);
});

visit.addEventListener("click", () => {
  if (!latest) return;
  window.open(latest.short_url, "_blank", "noopener,noreferrer");
});

stats.addEventListener("click", async () => {
  if (!latest) return;
  const response = await fetch(`/v1/links/${latest.code}/stats`, {
    headers: { "X-Owner-ID": owner.value },
  });
  show(await response.json());
});
