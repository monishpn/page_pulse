const form = document.getElementById('audit-form');
const urlInput = document.getElementById('url-input');
const submitBtn = document.getElementById('submit-btn');
const resultEl = document.getElementById('result');

const API_BASE_URL = window.API_BASE_URL || 'https://page-pulse-bwc6.onrender.com';

form.addEventListener('submit', async (event) => {
  event.preventDefault();

  const url = urlInput.value.trim();
  if (!url) return;

  setLoading(true);
  resultEl.className = 'result';
  resultEl.innerHTML = '';

  try {
    const response = await fetch(`${API_BASE_URL}/audit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    });

    const data = await response.json();

    if (!response.ok) {
      throw new Error(data?.error?.message || `Request failed with status ${response.status}`);
    }

    renderAudit(data);
  } catch (error) {
    resultEl.className = 'result error';
    resultEl.textContent = error.message;
  } finally {
    setLoading(false);
  }
});

function setLoading(isLoading) {
  submitBtn.disabled = isLoading;
  submitBtn.textContent = isLoading ? 'Auditing…' : 'Audit';
}

function renderAudit(audit) {
  const fields = [
    ['URL', audit.url],
    ['Final URL', audit.finalUrl],
    ['Status Code', audit.statusCode],
    ['Response Time', audit.responseTimeMs != null ? `${audit.responseTimeMs} ms` : undefined],
    ['Page Title', audit.pageTitle],
    ['Meta Description', audit.metaDescription],
    ['H1', audit.h1],
    ['Content Type', audit.contentType],
    ['Content Length', audit.contentLength],
    ['Server', audit.server],
    ['HTTPS', audit.https === undefined ? undefined : (audit.https ? 'Yes' : 'No')],
    ['Audited At', audit.auditedAt],
  ];

  const dl = document.createElement('dl');

  for (const [label, value] of fields) {
    const dt = document.createElement('dt');
    dt.textContent = label;

    const dd = document.createElement('dd');
    dd.textContent = value === undefined || value === '' ? '—' : String(value);

    dl.append(dt, dd);
  }

  resultEl.append(dl);
}
