// PV Signal Radar - Scientific Frontend Client
// Handles real-time pharmacovigilance disproportionality queries, Volcano Plot Canvas rendering, and 2x2 matrix expansion.

let currentAnalysis = null;
let plotPoints = [];

document.addEventListener('DOMContentLoaded', () => {
  const searchForm = document.getElementById('searchForm');
  const drugInput = document.getElementById('drugInput');
  const presetTags = document.querySelectorAll('.tag-preset');

  // Search submission
  searchForm.addEventListener('submit', (e) => {
    e.preventDefault();
    const query = drugInput.value.trim();
    if (query) {
      performAnalysis(query);
    }
  });

  // Preset click
  presetTags.forEach((tag) => {
    tag.addEventListener('click', () => {
      const drug = tag.dataset.drug;
      drugInput.value = drug;
      performAnalysis(drug);
    });
  });

  // Canvas tooltip handler
  const canvas = document.getElementById('volcanoCanvas');
  if (canvas) {
    canvas.addEventListener('mousemove', handleCanvasHover);
    window.addEventListener('resize', () => {
      if (currentAnalysis) drawVolcanoPlot(currentAnalysis.signals);
    });
  }

  // Load default demo search on load
  performAnalysis('Semaglutide');
});

async function performAnalysis(drug) {
  const stateBox = document.getElementById('stateBox');
  const resultsContainer = document.getElementById('resultsContainer');
  const loadingSpinner = document.getElementById('loadingSpinner');

  stateBox.style.display = 'block';
  resultsContainer.style.display = 'none';
  loadingSpinner.style.display = 'block';
  stateBox.querySelector('.state-text').innerText = `Mining OpenFDA FAERS data for "${drug}"...`;

  try {
    const res = await fetch(`/api/v1/analyze?drug=${encodeURIComponent(drug)}`);
    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || `HTTP error ${res.status}`);
    }

    const data = await res.json();
    currentAnalysis = data;
    renderResults(data);
  } catch (err) {
    loadingSpinner.style.display = 'none';
    stateBox.querySelector('.state-text').innerText = `Error: ${err.message}`;
  }
}

function renderResults(data) {
  const stateBox = document.getElementById('stateBox');
  const resultsContainer = document.getElementById('resultsContainer');

  stateBox.style.display = 'none';
  resultsContainer.style.display = 'block';

  // 1. Fill Summary Cards
  document.getElementById('statDrugName').innerText = data.normalized_drug || data.query_drug;
  document.getElementById('statTotalReports').innerText = Number(data.drug_total_reports).toLocaleString();
  document.getElementById('statActiveSignals').innerText = data.active_signals_count;
  document.getElementById('statUniverseN').innerText = Number(data.database_universe_n).toLocaleString();
  document.getElementById('statReactionsAnalyzed').innerText = data.total_reactions_analyzed;

  const activeCard = document.getElementById('activeSignalsCard');
  if (data.active_signals_count > 0) {
    activeCard.classList.add('active-alert');
  } else {
    activeCard.classList.remove('active-alert');
  }

  // 2. Draw Volcano Plot
  drawVolcanoPlot(data.signals || []);

  // 3. Render Table
  renderTable(data.signals || []);
}

function renderTable(signals) {
  const tbody = document.getElementById('signalsTableBody');
  tbody.innerHTML = '';

  if (!signals || signals.length === 0) {
    tbody.innerHTML = `<tr><td colspan="6" class="state-box" style="padding: 2rem;">No adverse reaction data found for this drug in FAERS.</td></tr>`;
    return;
  }

  // Sort: Active signals first, then by PRR descending
  const sorted = [...signals].sort((a, b) => {
    if (a.signal_level === 'ACTIVE_SIGNAL' && b.signal_level !== 'ACTIVE_SIGNAL') return -1;
    if (b.signal_level === 'ACTIVE_SIGNAL' && a.signal_level !== 'ACTIVE_SIGNAL') return 1;
    return b.prr - a.prr;
  });

  sorted.forEach((sig, index) => {
    const tr = document.createElement('tr');
    tr.className = 'table-row-item';

    let badgeClass = 'badge-none';
    let badgeText = 'Background';
    if (sig.signal_level === 'ACTIVE_SIGNAL') {
      badgeClass = 'badge-active';
      badgeText = 'Active Signal';
    } else if (sig.signal_level === 'POTENTIAL_SIGNAL') {
      badgeClass = 'badge-potential';
      badgeText = 'Potential Signal';
    }

    const prrFormatted = sig.prr ? sig.prr.toFixed(2) : '0.00';
    const prrCI = `[${sig.prr_lower_95.toFixed(2)} - ${sig.prr_upper_95.toFixed(2)}]`;
    const rorFormatted = sig.ror ? sig.ror.toFixed(2) : '0.00';
    const rorCI = `[${sig.ror_lower_95.toFixed(2)} - ${sig.ror_upper_95.toFixed(2)}]`;
    const chi2Formatted = sig.chi_square_yates ? sig.chi_square_yates.toFixed(1) : '0.0';

    tr.innerHTML = `
      <td>
        <strong>${escapeHtml(sig.reaction)}</strong>
        <div style="font-size: 0.72rem; color: var(--text-muted);">Click to inspect 2x2 matrix</div>
      </td>
      <td class="mono">${Number(sig.count_a).toLocaleString()}</td>
      <td>
        <span class="mono" style="font-weight: 600;">${prrFormatted}</span>
        <div style="font-size: 0.72rem; color: var(--text-secondary);">${prrCI}</div>
      </td>
      <td>
        <span class="mono" style="font-weight: 600;">${rorFormatted}</span>
        <div style="font-size: 0.72rem; color: var(--text-secondary);">${rorCI}</div>
      </td>
      <td>
        <span class="mono">${chi2Formatted}</span>
      </td>
      <td>
        <span class="badge ${badgeClass}">${badgeText}</span>
      </td>
    `;

    // Toggle detailed contingency row on click
    tr.addEventListener('click', () => {
      toggleDetailRow(tr, sig);
    });

    tbody.appendChild(tr);
  });
}

function toggleDetailRow(parentRow, sig) {
  const existingDetail = parentRow.nextElementSibling;
  if (existingDetail && existingDetail.classList.contains('detail-row')) {
    existingDetail.remove();
    return;
  }

  // Remove any open detail rows elsewhere
  document.querySelectorAll('.detail-row').forEach(row => row.remove());

  const detailRow = document.createElement('tr');
  detailRow.className = 'detail-row';

  const b = sig.drug_total - sig.count_a;
  const c = sig.reaction_total - sig.count_a;
  const d = (currentAnalysis.database_universe_n - (sig.count_a + b + c));

  detailRow.innerHTML = `
    <td colspan="6" style="background: #0d131f; padding: 1.25rem;">
      <div style="font-size: 0.85rem; font-weight: 600; margin-bottom: 0.5rem;">
        Contingency Matrix (2x2) for <em>${escapeHtml(currentAnalysis.normalized_drug)}</em> &times; <em>${escapeHtml(sig.reaction)}</em>
      </div>
      <div class="matrix-grid">
        <div class="matrix-cell">
          <span>Target Drug + Target Reaction (a)</span>
          <strong>${Number(sig.count_a).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Target Drug + Other Reactions (b)</span>
          <strong>${Number(b).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Target Drug Total (a + b)</span>
          <strong>${Number(sig.drug_total).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Other Drugs + Target Reaction (c)</span>
          <strong>${Number(c).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Other Drugs + Other Reactions (d)</span>
          <strong>${Number(d).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Database Universe (N)</span>
          <strong>${Number(currentAnalysis.database_universe_n).toLocaleString()}</strong>
        </div>
      </div>
      <div style="font-size: 0.775rem; color: var(--text-secondary); margin-top: 0.5rem;">
        <strong>Statistical Interpretation:</strong> ${escapeHtml(sig.interpretation)}
      </div>
    </td>
  `;

  parentRow.after(detailRow);
}

// Canvas Volcano Plot (Log2 PRR vs Sqrt Chi2)
function drawVolcanoPlot(signals) {
  const canvas = document.getElementById('volcanoCanvas');
  if (!canvas) return;

  const ctx = canvas.getContext('2d');
  const dpr = window.devicePixelRatio || 1;
  const rect = canvas.getBoundingClientRect();

  canvas.width = rect.width * dpr;
  canvas.height = rect.height * dpr;
  ctx.scale(dpr, dpr);

  const w = rect.width;
  const h = rect.height;
  const padding = { top: 20, right: 30, bottom: 40, left: 45 };

  ctx.clearRect(0, 0, w, h);
  plotPoints = [];

  if (!signals || signals.length === 0) {
    ctx.fillStyle = '#64748b';
    ctx.font = '12px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('No data points for plot', w / 2, h / 2);
    return;
  }

  // Determine bounds
  let maxLogPRR = 4; // log2(PRR)
  let maxSqrtChi = 10; // sqrt(Chi2)

  signals.forEach(s => {
    const logPRR = s.prr > 0 ? Math.log2(s.prr) : 0;
    const sqrtChi = s.chi_square_yates > 0 ? Math.sqrt(s.chi_square_yates) : 0;
    if (logPRR > maxLogPRR) maxLogPRR = Math.ceil(logPRR) + 0.5;
    if (sqrtChi > maxSqrtChi) maxSqrtChi = Math.ceil(sqrtChi) + 2;
  });

  const plotW = w - padding.left - padding.right;
  const plotH = h - padding.top - padding.bottom;

  // Coordinate mappers
  const getX = (logPRR) => padding.left + (Math.max(0, logPRR) / maxLogPRR) * plotW;
  const getY = (sqrtChi) => h - padding.bottom - (Math.min(sqrtChi, maxSqrtChi) / maxSqrtChi) * plotH;

  // Draw Grid & Axes
  ctx.strokeStyle = '#1e293b';
  ctx.lineWidth = 1;

  // Horizontal threshold (Chi2 = 4 => sqrt(4) = 2)
  const threshY = getY(2);
  ctx.strokeStyle = 'rgba(245, 158, 11, 0.3)';
  ctx.setLineDash([4, 4]);
  ctx.beginPath();
  ctx.moveTo(padding.left, threshY);
  ctx.lineTo(w - padding.right, threshY);
  ctx.stroke();

  // Vertical threshold (PRR = 2 => log2(2) = 1)
  const threshX = getX(1);
  ctx.beginPath();
  ctx.moveTo(threshX, padding.top);
  ctx.lineTo(threshX, h - padding.bottom);
  ctx.stroke();
  ctx.setLineDash([]);

  // Axis Labels
  ctx.fillStyle = '#64748b';
  ctx.font = '10px ui-monospace, monospace';
  ctx.textAlign = 'center';
  ctx.fillText('log₂(PRR) → Threshold = 1.0 (PRR=2)', padding.left + plotW / 2, h - 10);

  ctx.save();
  ctx.translate(12, padding.top + plotH / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.fillText('√(χ² Yates)', 0, 0);
  ctx.restore();

  // Plot Data Points
  signals.forEach(s => {
    const logPRR = s.prr > 0 ? Math.log2(s.prr) : 0;
    const sqrtChi = s.chi_square_yates > 0 ? Math.sqrt(s.chi_square_yates) : 0;

    const px = getX(logPRR);
    const py = getY(sqrtChi);

    let color = '#10b981'; // Green
    if (s.signal_level === 'ACTIVE_SIGNAL') {
      color = '#ef4444'; // Red
    } else if (s.signal_level === 'POTENTIAL_SIGNAL') {
      color = '#f59e0b'; // Amber
    }

    ctx.beginPath();
    ctx.arc(px, py, s.signal_level === 'ACTIVE_SIGNAL' ? 5 : 3.5, 0, Math.PI * 2);
    ctx.fillStyle = color;
    ctx.shadowColor = color;
    ctx.shadowBlur = s.signal_level === 'ACTIVE_SIGNAL' ? 8 : 0;
    ctx.fill();
    ctx.shadowBlur = 0;

    plotPoints.push({
      x: px,
      y: py,
      signal: s,
      logPRR,
      sqrtChi
    });
  });
}

function handleCanvasHover(e) {
  const canvas = document.getElementById('volcanoCanvas');
  const tooltip = document.getElementById('plotTooltip');
  if (!canvas || !tooltip || plotPoints.length === 0) return;

  const rect = canvas.getBoundingClientRect();
  const mouseX = e.clientX - rect.left;
  const mouseY = e.clientY - rect.top;

  let nearest = null;
  let minDist = 12;

  plotPoints.forEach(pt => {
    const dist = Math.hypot(pt.x - mouseX, pt.y - mouseY);
    if (dist < minDist) {
      minDist = dist;
      nearest = pt;
    }
  });

  if (nearest) {
    const s = nearest.signal;
    tooltip.style.display = 'block';
    tooltip.style.left = `${e.clientX + 12}px`;
    tooltip.style.top = `${e.clientY - 20}px`;
    tooltip.innerHTML = `
      <div style="font-weight: 700; color: #fff;">${escapeHtml(s.reaction)}</div>
      <div style="color: var(--accent-cyan); font-family: var(--font-mono); font-size: 0.75rem;">
        PRR: ${s.prr.toFixed(2)} | ROR: ${s.ror.toFixed(2)} | χ²: ${s.chi_square_yates.toFixed(1)}
      </div>
      <div style="color: var(--text-muted); font-size: 0.7rem;">Reports (a): ${s.count_a}</div>
    `;
  } else {
    tooltip.style.display = 'none';
  }
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
