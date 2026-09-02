// PV Signal Radar v2 - Frontend Controller
// Handles multi-tab navigation, comparative FDA x ANVISA analysis, Volcano Plot rendering,
// and in-context statistical flagging with interactive feedback queue.

let currentAnalysis = null;
let currentJurisdiction = 'fda'; // 'fda' or 'anvisa'
let plotPoints = [];
let flaggedQueue = [];

document.addEventListener('DOMContentLoaded', () => {
  initTabNavigation();
  initSearch();
  initFeedbackForm();
  initCanvas();

  // Initial demo load
  performAnalysis('Semaglutide');
});

// 1. Tab Navigation
function initTabNavigation() {
  const tabButtons = document.querySelectorAll('.tab-btn');
  const tabContents = document.querySelectorAll('.tab-content');

  tabButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      const targetTab = btn.dataset.tab;

      tabButtons.forEach(b => b.classList.remove('active'));
      tabContents.forEach(c => c.style.display = 'none');

      btn.classList.add('active');
      const activeContent = document.getElementById(targetTab);
      if (activeContent) {
        activeContent.style.display = 'block';
      }

      if (targetTab === 'radarTabView' && currentAnalysis) {
        setTimeout(() => renderJurisdictionData(), 50);
      }
    });
  });

  // Jurisdiction Sub-tabs (FDA vs ANVISA)
  const jButtons = document.querySelectorAll('.j-tab-btn');
  jButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      jButtons.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentJurisdiction = btn.dataset.jurisdiction;
      renderJurisdictionData();
    });
  });

  // Floating Feedback bar click jumps to feedback tab
  const floatingBar = document.getElementById('floatingFeedbackBar');
  if (floatingBar) {
    floatingBar.addEventListener('click', () => {
      document.querySelector('[data-tab="feedbackTabView"]').click();
    });
  }
}

// 2. Search & Analysis Handling
function initSearch() {
  const searchForm = document.getElementById('searchForm');
  const drugInput = document.getElementById('drugInput');
  const presetTags = document.querySelectorAll('.tag-preset');

  searchForm.addEventListener('submit', (e) => {
    e.preventDefault();
    const query = drugInput.value.trim();
    if (query) {
      performAnalysis(query);
    }
  });

  presetTags.forEach(tag => {
    tag.addEventListener('click', () => {
      const drug = tag.dataset.drug;
      drugInput.value = drug;
      performAnalysis(drug);
    });
  });
}

async function performAnalysis(drug) {
  const stateBox = document.getElementById('stateBox');
  const resultsContainer = document.getElementById('resultsContainer');
  const loadingSpinner = document.getElementById('loadingSpinner');

  stateBox.style.display = 'block';
  resultsContainer.style.display = 'none';
  loadingSpinner.style.display = 'block';
  stateBox.querySelector('.state-text').innerText = `Harmonizing FDA FAERS & ANVISA VigiMed data for "${drug}"...`;

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
    stateBox.querySelector('.state-text').innerText = `Analysis Error: ${err.message}`;
  }
}

function renderResults(data) {
  const stateBox = document.getElementById('stateBox');
  const resultsContainer = document.getElementById('resultsContainer');

  stateBox.style.display = 'none';
  resultsContainer.style.display = 'block';

  // 1. Render Comparative Summary Banner
  renderComparativeBanner(data);

  // 2. Render Current Jurisdiction View (FDA or ANVISA)
  renderJurisdictionData();
}

function renderComparativeBanner(data) {
  const banner = document.getElementById('comparativeBanner');
  const comp = data.comparative_summary;

  document.getElementById('compDrugName').innerText = data.normalized_drug || data.query_drug;
  document.getElementById('compATCCode').innerText = data.atc_code || 'N/A';
  document.getElementById('compFDASignals').innerText = comp.fda_active_signals || 0;
  document.getElementById('compAnvisaSignals').innerText = comp.anvisa_active_signals || 0;

  const insightsList = document.getElementById('compInsightsList');
  insightsList.innerHTML = '';
  if (comp.key_insights && comp.key_insights.length > 0) {
    comp.key_insights.forEach(insight => {
      const li = document.createElement('li');
      li.innerText = insight;
      insightsList.appendChild(li);
    });
  } else {
    const li = document.createElement('li');
    li.innerText = 'Both jurisdictions actively monitored. Concordance evaluated across standard MedDRA Preferred Terms.';
    insightsList.appendChild(li);
  }

  // Add flag button for comparative banner
  const flagBtn = document.getElementById('flagComparativeSummary');
  if (flagBtn) {
    flagBtn.onclick = () => {
      flagStatistic({
        drug: data.normalized_drug,
        reaction: 'CROSS_JURISDICTION_COMPARISON',
        jurisdiction: 'COMPARATIVE',
        metric: 'Cross-Jurisdiction Summary',
        displayed_value: `FDA Signals: ${comp.fda_active_signals}, ANVISA Signals: ${comp.anvisa_active_signals}, Ratio: ${comp.reporting_ratio_fda_vs_br ? comp.reporting_ratio_fda_vs_br.toFixed(1) : 'N/A'}x`,
        reason: 'Flagged comparative cross-country discrepancy.'
      }, flagBtn);
    };
  }
}

function renderJurisdictionData() {
  if (!currentAnalysis) return;

  const isFDA = currentJurisdiction === 'fda';
  const dataset = isFDA ? currentAnalysis.fda_analysis : currentAnalysis.anvisa_analysis;

  if (!dataset) return;

  // Stat cards
  document.getElementById('statJurisdictionTitle').innerText = isFDA ? 'US FDA FAERS (Global)' : 'Brasil ANVISA VigiMed';
  document.getElementById('statTotalReports').innerText = Number(dataset.drug_total_reports || dataset.total_reports_br || 0).toLocaleString();
  document.getElementById('statActiveSignals').innerText = dataset.active_signals_count || 0;
  document.getElementById('statUniverseN').innerText = Number(dataset.database_universe_n || dataset.database_universe_n_br || 0).toLocaleString();

  const activeCard = document.getElementById('activeSignalsCard');
  if (dataset.active_signals_count > 0) {
    activeCard.classList.add('active-alert');
  } else {
    activeCard.classList.remove('active-alert');
  }

  // Setup stat card flag buttons
  setupStatCardFlags(dataset, isFDA ? 'FDA' : 'ANVISA');

  // Draw Volcano Plot
  const signals = dataset.signals || [];
  drawVolcanoPlot(signals);

  // Render Table
  renderTable(signals, isFDA ? 'FDA' : 'ANVISA');
}

function setupStatCardFlags(dataset, jurisdiction) {
  const drug = currentAnalysis.normalized_drug;

  const flagTotalReports = document.getElementById('flagTotalReports');
  if (flagTotalReports) {
    flagTotalReports.onclick = () => {
      flagStatistic({
        drug: drug,
        reaction: 'TOTAL_REPORTS',
        jurisdiction: jurisdiction,
        metric: 'Total Drug Reports',
        displayed_value: String(dataset.drug_total_reports || dataset.total_reports_br || 0),
        reason: 'Questioning denominator or reported volume.'
      }, flagTotalReports);
    };
  }

  const flagActiveSignals = document.getElementById('flagActiveSignals');
  if (flagActiveSignals) {
    flagActiveSignals.onclick = () => {
      flagStatistic({
        drug: drug,
        reaction: 'ACTIVE_SIGNALS_COUNT',
        jurisdiction: jurisdiction,
        metric: 'Active Signals Count',
        displayed_value: String(dataset.active_signals_count || 0),
        reason: 'Questioning signal threshold or classification.'
      }, flagActiveSignals);
    };
  }
}

function renderTable(signals, jurisdiction) {
  const tbody = document.getElementById('signalsTableBody');
  tbody.innerHTML = '';

  if (!signals || signals.length === 0) {
    tbody.innerHTML = `<tr><td colspan="7" class="state-box" style="padding: 2.5rem;">Nenhum relato de evento adverso registrado nesta base de dados para a substância selecionada.</td></tr>`;
    return;
  }

  // Sort by Active signal then PRR descending
  const sorted = [...signals].sort((a, b) => {
    if (a.signal_level === 'ACTIVE_SIGNAL' && b.signal_level !== 'ACTIVE_SIGNAL') return -1;
    if (b.signal_level === 'ACTIVE_SIGNAL' && a.signal_level !== 'ACTIVE_SIGNAL') return 1;
    return (b.prr || 0) - (a.prr || 0);
  });

  sorted.forEach(sig => {
    const tr = document.createElement('tr');
    tr.className = 'table-row-item';

    let badgeClass = 'badge-none';
    let badgeText = 'Ruído / Baseline';
    if (sig.signal_level === 'ACTIVE_SIGNAL') {
      badgeClass = 'badge-active';
      badgeText = 'Sinal Ativo';
    } else if (sig.signal_level === 'POTENTIAL_SIGNAL') {
      badgeClass = 'badge-potential';
      badgeText = 'Potencial';
    }

    const reactionName = sig.reaction_pt_br ? `${sig.reaction_pt_br} (${sig.reaction_pt_en || sig.reaction})` : (sig.reaction || sig.reaction_pt_en);
    const countA = sig.count_a || 0;
    const prrVal = sig.prr ? sig.prr.toFixed(2) : '0.00';
    const prrCI = `[${(sig.prr_lower_95 || 0).toFixed(2)} - ${(sig.prr_upper_95 || 0).toFixed(2)}]`;
    const rorVal = sig.ror ? sig.ror.toFixed(2) : '0.00';
    const rorCI = `[${(sig.ror_lower_95 || 0).toFixed(2)} - ${(sig.ror_upper_95 || 0).toFixed(2)}]`;
    const chi2Val = sig.chi_square_yates ? sig.chi_square_yates.toFixed(1) : '0.0';

    tr.innerHTML = `
      <td>
        <div style="font-weight: 600; color: var(--text-primary);">${escapeHtml(reactionName)}</div>
        <div style="font-size: 0.72rem; color: var(--text-muted);">Clique para expandir matriz 2x2</div>
      </td>
      <td class="mono">${Number(countA).toLocaleString()}</td>
      <td>
        <span class="mono" style="font-weight: 600;">${prrVal}</span>
        <div style="font-size: 0.7rem; color: var(--text-secondary);">${prrCI}</div>
      </td>
      <td>
        <span class="mono" style="font-weight: 600;">${rorVal}</span>
        <div style="font-size: 0.7rem; color: var(--text-secondary);">${rorCI}</div>
      </td>
      <td>
        <span class="mono">${chi2Val}</span>
      </td>
      <td>
        <span class="badge ${badgeClass}">${badgeText}</span>
      </td>
      <td style="text-align: right;">
        <button class="btn-flag-stat" title="Capturar e anexar esta estatística ao feedback" data-reaction="${escapeHtml(sig.reaction || sig.reaction_pt_en)}">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"></path><line x1="4" y1="22" x2="4" y2="15"></line></svg>
        </button>
      </td>
    `;

    // Flag button click
    const flagBtn = tr.querySelector('.btn-flag-stat');
    flagBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      flagStatistic({
        drug: currentAnalysis.normalized_drug,
        reaction: sig.reaction || sig.reaction_pt_en,
        jurisdiction: jurisdiction,
        metric: `PRR=${prrVal}, ROR=${rorVal}, Chi2=${chi2Val}, Count=${countA}`,
        displayed_value: `PRR: ${prrVal} ${prrCI} | ROR: ${rorVal} | Chi2: ${chi2Val}`,
        reason: 'Revisar cálculo ou relevância biológica.'
      }, flagBtn);
    });

    // Expand 2x2 matrix row
    tr.addEventListener('click', () => {
      toggleDetailRow(tr, sig, jurisdiction);
    });

    tbody.appendChild(tr);
  });
}

function toggleDetailRow(parentRow, sig, jurisdiction) {
  const existingDetail = parentRow.nextElementSibling;
  if (existingDetail && existingDetail.classList.contains('detail-row')) {
    existingDetail.remove();
    return;
  }

  document.querySelectorAll('.detail-row').forEach(r => r.remove());

  const detailRow = document.createElement('tr');
  detailRow.className = 'detail-row';

  const countA = sig.count_a || 0;
  const drugTotal = sig.drug_total || 0;
  const reactionTotal = sig.reaction_total || 0;
  const b = drugTotal - countA;
  const c = reactionTotal - countA;
  const universeN = jurisdiction === 'FDA' ? currentAnalysis.fda_analysis.database_universe_n : currentAnalysis.anvisa_analysis.database_universe_n_br;
  const d = universeN - (countA + b + c);

  const reactionLabel = sig.reaction_pt_br ? `${sig.reaction_pt_br} / ${sig.reaction_pt_en}` : (sig.reaction || sig.reaction_pt_en);

  detailRow.innerHTML = `
    <td colspan="7" style="background: #0d131f; padding: 1.25rem;">
      <div style="font-size: 0.85rem; font-weight: 600; margin-bottom: 0.5rem; display: flex; justify-content: space-between;">
        <span>Matriz de Contingência 2&times;2 [${jurisdiction}]: <em>${escapeHtml(currentAnalysis.normalized_drug)}</em> &times; <em>${escapeHtml(reactionLabel)}</em></span>
        <button class="btn-flag-stat" id="flagMatrix_${escapeHtml(sig.reaction || sig.reaction_pt_en)}" style="font-size: 0.75rem;">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"></path><line x1="4" y1="22" x2="4" y2="15"></line></svg>
          Anexar Matriz 2x2 ao Feedback
        </button>
      </div>
      <div class="matrix-grid">
        <div class="matrix-cell">
          <span>Fármaco Alvo + Reação Alvo (a)</span>
          <strong>${Number(countA).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Fármaco Alvo + Outras Reações (b)</span>
          <strong>${Number(b).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Total Fármaco Alvo (a + b)</span>
          <strong>${Number(drugTotal).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Outros Fármacos + Reação Alvo (c)</span>
          <strong>${Number(c).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Outros Fármacos + Outras Reações (d)</span>
          <strong>${Number(d).toLocaleString()}</strong>
        </div>
        <div class="matrix-cell">
          <span>Universo da Base (N)</span>
          <strong>${Number(universeN).toLocaleString()}</strong>
        </div>
      </div>
      <div style="font-size: 0.775rem; color: var(--text-secondary); margin-top: 0.5rem;">
        <strong>Interpretação Estatística:</strong> ${escapeHtml(sig.interpretation)}
      </div>
    </td>
  `;

  const matrixFlagBtn = detailRow.querySelector('.btn-flag-stat');
  if (matrixFlagBtn) {
    matrixFlagBtn.onclick = () => {
      flagStatistic({
        drug: currentAnalysis.normalized_drug,
        reaction: sig.reaction || sig.reaction_pt_en,
        jurisdiction: jurisdiction,
        metric: '2x2 Contingency Matrix',
        displayed_value: `a=${countA}, b=${b}, c=${c}, d=${d}, N=${universeN}`,
        reason: 'Inconsistência nos denominadores marginais ou contagens da tabela 2x2.'
      }, matrixFlagBtn);
    };
  }

  parentRow.after(detailRow);
}

// 3. Canvas Volcano Plot
function initCanvas() {
  const canvas = document.getElementById('volcanoCanvas');
  if (canvas) {
    canvas.addEventListener('mousemove', handleCanvasHover);
    window.addEventListener('resize', () => {
      if (currentAnalysis) renderJurisdictionData();
    });
  }
}

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
    ctx.fillText('Nenhum dado para o gráfico de dispersão', w / 2, h / 2);
    return;
  }

  let maxLogPRR = 4;
  let maxSqrtChi = 10;

  signals.forEach(s => {
    const prr = s.prr || 0;
    const chi = s.chi_square_yates || 0;
    const logPRR = prr > 0 ? Math.log2(prr) : 0;
    const sqrtChi = chi > 0 ? Math.sqrt(chi) : 0;
    if (logPRR > maxLogPRR) maxLogPRR = Math.ceil(logPRR) + 0.5;
    if (sqrtChi > maxSqrtChi) maxSqrtChi = Math.ceil(sqrtChi) + 2;
  });

  const plotW = w - padding.left - padding.right;
  const plotH = h - padding.top - padding.bottom;

  const getX = (logPRR) => padding.left + (Math.max(0, logPRR) / maxLogPRR) * plotW;
  const getY = (sqrtChi) => h - padding.bottom - (Math.min(sqrtChi, maxSqrtChi) / maxSqrtChi) * plotH;

  // Threshold lines
  ctx.strokeStyle = '#1e293b';
  ctx.lineWidth = 1;

  // Yates Chi2 = 4 -> sqrt(4) = 2
  const threshY = getY(2);
  ctx.strokeStyle = 'rgba(245, 158, 11, 0.3)';
  ctx.setLineDash([4, 4]);
  ctx.beginPath();
  ctx.moveTo(padding.left, threshY);
  ctx.lineTo(w - padding.right, threshY);
  ctx.stroke();

  // PRR = 2 -> log2(2) = 1
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
  ctx.fillText('log₂(PRR) → Corte = 1.0 (PRR=2)', padding.left + plotW / 2, h - 10);

  ctx.save();
  ctx.translate(12, padding.top + plotH / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.fillText('√(χ² Yates)', 0, 0);
  ctx.restore();

  // Points
  signals.forEach(s => {
    const prr = s.prr || 0;
    const chi = s.chi_square_yates || 0;
    const logPRR = prr > 0 ? Math.log2(prr) : 0;
    const sqrtChi = chi > 0 ? Math.sqrt(chi) : 0;

    const px = getX(logPRR);
    const py = getY(sqrtChi);

    let color = '#10b981';
    if (s.signal_level === 'ACTIVE_SIGNAL') {
      color = '#ef4444';
    } else if (s.signal_level === 'POTENTIAL_SIGNAL') {
      color = '#f59e0b';
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
    const reaction = s.reaction_pt_br ? `${s.reaction_pt_br} (${s.reaction_pt_en})` : (s.reaction || s.reaction_pt_en);
    tooltip.style.display = 'block';
    tooltip.style.left = `${e.clientX + 12}px`;
    tooltip.style.top = `${e.clientY - 20}px`;
    tooltip.innerHTML = `
      <div style="font-weight: 700; color: #fff;">${escapeHtml(reaction)}</div>
      <div style="color: var(--accent-cyan); font-family: var(--font-mono); font-size: 0.75rem;">
        PRR: ${(s.prr || 0).toFixed(2)} | ROR: ${(s.ror || 0).toFixed(2)} | χ²: ${(s.chi_square_yates || 0).toFixed(1)}
      </div>
      <div style="color: var(--text-muted); font-size: 0.7rem;">Relatos (a): ${s.count_a || 0}</div>
    `;
  } else {
    tooltip.style.display = 'none';
  }
}

// 4. In-Context Flagging & Feedback Queue Management
function flagStatistic(item, buttonElement) {
  // Check if already in queue
  const exists = flaggedQueue.some(q => q.drug === item.drug && q.reaction === item.reaction && q.jurisdiction === item.jurisdiction && q.metric === item.metric);
  if (!exists) {
    flaggedQueue.push(item);
    if (buttonElement) {
      buttonElement.classList.add('flagged');
    }
  }

  updateFeedbackUI();
}

function removeFlaggedItem(index) {
  flaggedQueue.splice(index, 1);
  updateFeedbackUI();
}

function updateFeedbackUI() {
  const floatingBar = document.getElementById('floatingFeedbackBar');
  const countPill = document.getElementById('queueCountPill');
  const headerBadge = document.getElementById('headerFeedbackBadge');
  const queueList = document.getElementById('flaggedQueueList');
  const noItemsPrompt = document.getElementById('noFlaggedItemsPrompt');

  const count = flaggedQueue.length;

  if (count > 0) {
    floatingBar.style.display = 'flex';
    countPill.innerText = `${count} selecionada${count > 1 ? 's' : ''}`;
    headerBadge.style.display = 'inline-block';
    headerBadge.innerText = count;
  } else {
    floatingBar.style.display = 'none';
    headerBadge.style.display = 'none';
  }

  // Update list in Feedback tab
  if (queueList) {
    queueList.innerHTML = '';
    if (count === 0) {
      if (noItemsPrompt) noItemsPrompt.style.display = 'block';
    } else {
      if (noItemsPrompt) noItemsPrompt.style.display = 'none';
      flaggedQueue.forEach((item, idx) => {
        const card = document.createElement('div');
        card.className = 'flagged-item-card';
        card.innerHTML = `
          <div class="flagged-item-header">
            <div>
              <span class="badge ${item.jurisdiction === 'FDA' ? 'badge-active' : (item.jurisdiction === 'ANVISA' ? 'badge-potential' : 'badge-none')}" style="margin-right: 0.4rem;">${item.jurisdiction}</span>
              <strong class="flagged-item-title">${escapeHtml(item.drug)} &bull; ${escapeHtml(item.reaction)}</strong>
            </div>
            <button type="button" class="btn-remove-flag" onclick="removeFlaggedItem(${idx})">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
            </button>
          </div>
          <div class="flagged-item-meta">${escapeHtml(item.metric)}: ${escapeHtml(item.displayed_value)}</div>
          <input type="text" class="flagged-item-reason-input" placeholder="Motivo do apontamento (ex: viés de Weber suspeito, erro na tabela 2x2)..." value="${escapeHtml(item.reason || '')}" onchange="flaggedQueue[${idx}].reason = this.value">
        `;
        queueList.appendChild(card);
      });
    }
  }
}

// 5. Feedback Form Submission
function initFeedbackForm() {
  const form = document.getElementById('feedbackForm');
  const statusMsg = document.getElementById('feedbackStatusMsg');

  if (!form) return;

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = document.getElementById('feedbackEmail').value.trim();
    const comments = document.getElementById('feedbackComments').value.trim();

    if (!email) {
      statusMsg.innerHTML = '<span style="color: var(--signal-active);">Por favor, informe seu e-mail institucional ou de contato.</span>';
      return;
    }

    statusMsg.innerHTML = '<span style="color: var(--text-secondary);">Enviando feedback...</span>';

    try {
      const payload = {
        email: email,
        comments: comments,
        flagged_statistics: flaggedQueue
      };

      const res = await fetch('/api/v1/feedback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      const resData = await res.json();
      if (!res.ok) {
        throw new Error(resData.error || 'Erro no envio');
      }

      statusMsg.innerHTML = `<span style="color: var(--accent-green); font-weight: 600;">Feedback registrado com sucesso! ID: ${resData.feedback_id}. Obrigado por contribuir com a validação do PV Signal Radar.</span>`;
      form.reset();
      flaggedQueue = [];
      updateFeedbackUI();
    } catch (err) {
      statusMsg.innerHTML = `<span style="color: var(--signal-active);">Erro ao registrar feedback: ${err.message}</span>`;
    }
  });
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
