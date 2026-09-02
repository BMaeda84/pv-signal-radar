// ==========================================================================
// PV Signal Radar - Swiss Clinical Master Frontend Controller
// Implements: Light/Dark theme, Autocomplete with DCB/ATC, Volcano & Forest Plot,
// 2x2 Contingency Table Inspector, and Contextual Feedback Queue.
// ==========================================================================

let currentAnalysis = null;
let currentJurisdiction = 'fda'; // 'fda' or 'anvisa'
let currentChartMode = 'volcano'; // 'volcano' or 'forest'
let selectedSignalIndex = 0;
let plotPoints = [];
let flaggedQueue = [];

// Comprehensive Pharmaceutical Catalog with DCB and WHO-ATC
const PHARMA_CATALOG = [
  { canonical: 'Semaglutida', synonyms: ['Semaglutide', 'Ozempic', 'Wegovy', 'Rybelsus'], dcb: 'DCB 09842', atc: 'A10BJ06', class: 'Análogo de GLP-1 / Antidiabético' },
  { canonical: 'Dipirona', synonyms: ['Metamizol', 'Novalgina', 'Anador', 'Dipyrone', 'Metamizole', 'Neosaldina'], dcb: 'DCB 03247', atc: 'N02BB02', class: 'Analgésico / Antitérmico Pirazolônico' },
  { canonical: 'Metformina', synonyms: ['Metformin', 'Glifage', 'Glucophage', 'Dimefor'], dcb: 'DCB 05786', atc: 'A10BA02', class: 'Biguanida / Antidiabético' },
  { canonical: 'Rosuvastatina', synonyms: ['Rosuvastatin', 'Crestor', 'Vivacor'], dcb: 'DCB 07799', atc: 'C10AA07', class: 'Estatina / Hipolipemiante' },
  { canonical: 'Pembrolizumabe', synonyms: ['Pembrolizumab', 'Keytruda'], dcb: 'DCB 09641', atc: 'L01FF02', class: 'Anticorpo Monoclonal Anti-PD-1' },
  { canonical: 'Adalimumabe', synonyms: ['Adalimumab', 'Humira', 'Hyrimoz'], dcb: 'DCB 00138', atc: 'L04AB04', class: 'Inibidor de TNF-alfa / Imunossupressor' },
  { canonical: 'Rivaroxabana', synonyms: ['Rivaroxaban', 'Xarelto'], dcb: 'DCB 07774', atc: 'B01AF01', class: 'Inibidor Direto do Fator Xa' },
  { canonical: 'Apixabana', synonyms: ['Apixaban', 'Eliquis'], dcb: 'DCB 08851', atc: 'B01AF02', class: 'Inibidor Direto do Fator Xa' },
  { canonical: 'Empagliflozina', synonyms: ['Empagliflozin', 'Jardiance'], dcb: 'DCB 09618', atc: 'A10BK03', class: 'Inibidor de SGLT2 / Antidiabético' },
  { canonical: 'Dapagliflozina', synonyms: ['Dapagliflozin', 'Forxiga'], dcb: 'DCB 09292', atc: 'A10BK01', class: 'Inibidor de SGLT2 / Antidiabético' },
  { canonical: 'Tirzepatida', synonyms: ['Tirzepatide', 'Mounjaro', 'Zepbound'], dcb: 'DCB 10452', atc: 'A10BX16', class: 'Agonista Duplo GIP / GLP-1' },
  { canonical: 'Dulaglutida', synonyms: ['Dulaglutide', 'Trulicity'], dcb: 'DCB 09598', atc: 'A10BJ05', class: 'Análogo de GLP-1' },
  { canonical: 'Liraglutida', synonyms: ['Liraglutide', 'Victoza', 'Saxenda'], dcb: 'DCB 08818', atc: 'A10BJ02', class: 'Análogo de GLP-1' },
  { canonical: 'Omeprazol', synonyms: ['Omeprazole', 'Losec', 'Peprazol'], dcb: 'DCB 06558', atc: 'A02BC01', class: 'Inibidor da Bomba de Prótons' },
  { canonical: 'Esomeprazol', synonyms: ['Esomeprazole', 'Nexium'], dcb: 'DCB 03632', atc: 'A02BC05', class: 'Inibidor da Bomba de Prótons' },
  { canonical: 'Losartana', synonyms: ['Losartan', 'Cozaar', 'Aradois'], dcb: 'DCB 05445', atc: 'C09CA01', class: 'Antagonista do Receptor AT1' },
  { canonical: 'Amoxicilina', synonyms: ['Amoxicillin', 'Amoxil', 'Clavulin'], dcb: 'DCB 00646', atc: 'J01CA04', class: 'Antibiótico Betalactâmico' },
  { canonical: 'Paracetamol', synonyms: ['Acetaminophen', 'Tylenol', 'Dofalgan'], dcb: 'DCB 06775', atc: 'N02BE01', class: 'Analgésico e Antitérmico' },
  { canonical: 'Ibuprofeno', synonyms: ['Ibuprofen', 'Advil', 'Alivium', 'Motrin'], dcb: 'DCB 04812', atc: 'M01AE01', class: 'Anti-inflamatório Não Esteroidal (AINE)' },
  { canonical: 'Atorvastatina', synonyms: ['Atorvastatin', 'Lipitor', 'Citalor'], dcb: 'DCB 00921', atc: 'C10AA05', class: 'Estatina / Hipolipemiante' },
  { canonical: 'Sinvastatina', synonyms: ['Simvastatin', 'Zocor'], dcb: 'DCB 08064', atc: 'C10AA01', class: 'Estatina / Hipolipemiante' },
  { canonical: 'Levotiroxina', synonyms: ['Levothyroxine', 'Puran T4', 'Synthroid', 'Euthyrox'], dcb: 'DCB 05282', atc: 'H03AA01', class: 'Hormônio Tireoidiano' },
  { canonical: 'Escitalopram', synonyms: ['Lexapro', 'Exodus', 'Reconter'], dcb: 'DCB 03623', atc: 'N06AB10', class: 'ISRS / Antidepressivo' },
  { canonical: 'Sertralina', synonyms: ['Sertraline', 'Zoloft', 'Assert', 'Tolrest'], dcb: 'DCB 07997', atc: 'N06AB06', class: 'ISRS / Antidepressivo' },
  { canonical: 'Fluoxetina', synonyms: ['Fluoxetine', 'Prozac', 'Daforin'], dcb: 'DCB 04077', atc: 'N06AB03', class: 'ISRS / Antidepressivo' },
  { canonical: 'Clonazepam', synonyms: ['Rivotril', 'Clonavitae'], dcb: 'DCB 02446', atc: 'N03AE01', class: 'Benzodiazepínico' },
  { canonical: 'Diazepam', synonyms: ['Valium', 'Compaz'], dcb: 'DCB 03009', atc: 'N05BA01', class: 'Benzodiazepínico' },
  { canonical: 'Alprazolam', synonyms: ['Frontal', 'Tranquinal', 'Apraz'], dcb: 'DCB 00388', atc: 'N05BA12', class: 'Benzodiazepínico' },
  { canonical: 'Pregabalina', synonyms: ['Pregabalin', 'Lyrica', 'Prebictal'], dcb: 'DCB 09148', atc: 'N02BF02', class: 'Gabapentinoide' },
  { canonical: 'Gabapentina', synonyms: ['Gabapentin', 'Neurontin'], dcb: 'DCB 04323', atc: 'N02BF01', class: 'Gabapentinoide' },
  { canonical: 'Enoxaparina', synonyms: ['Enoxaparin', 'Clexane', 'Versa'], dcb: 'DCB 03487', atc: 'B01AB05', class: 'Heparina de Baixo Peso Molecular' },
  { canonical: 'Varfarina', synonyms: ['Warfarin', 'Marevan', 'Coumadin'], dcb: 'DCB 09072', atc: 'B01AA03', class: 'Anticoagulante Cumarínico' },
  { canonical: 'Metotrexato', synonyms: ['Methotrexate', 'Miantrex'], dcb: 'DCB 05831', atc: 'L01BA01', class: 'Antimetabólito' },
  { canonical: 'Rituximabe', synonyms: ['Rituximab', 'MabThera'], dcb: 'DCB 07770', atc: 'L01FA01', class: 'Anticorpo Monoclonal Anti-CD20' },
  { canonical: 'Infliximabe', synonyms: ['Infliximab', 'Remicade'], dcb: 'DCB 04958', atc: 'L04AB02', class: 'Inibidor de TNF-alfa' },
  { canonical: 'Trastuzumabe', synonyms: ['Trastuzumab', 'Herceptin'], dcb: 'DCB 08726', atc: 'L01FD01', class: 'Anticorpo Monoclonal Anti-HER2' },
  { canonical: 'Nivolumabe', synonyms: ['Nivolumab', 'Opdivo'], dcb: 'DCB 09605', atc: 'L01FF01', class: 'Anticorpo Monoclonal Anti-PD-1' }
];

document.addEventListener('DOMContentLoaded', () => {
  initTheme();
  initTabNavigation();
  initChartSwitcher();
  initSearch();
  initFeedbackForm();
  initCanvas();

  // Initial load with benchmark substance
  performAnalysis('Semaglutide');
});

// 1. Theme Management (Light / Dark)
function initTheme() {
  const savedTheme = localStorage.getItem('pv_theme') || 'light';
  setTheme(savedTheme);

  const themeBtn = document.getElementById('themeToggleBtn');
  if (themeBtn) {
    themeBtn.addEventListener('click', () => {
      const current = document.documentElement.getAttribute('data-theme') || 'light';
      const next = current === 'dark' ? 'light' : 'dark';
      setTheme(next);
    });
  }
}

function setTheme(theme) {
  if (theme === 'dark') {
    document.documentElement.setAttribute('data-theme', 'dark');
    document.getElementById('themeToggleText').innerText = 'Modo Claro';
    document.getElementById('themeToggleBtn').innerHTML = `
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"></circle><line x1="12" y1="1" x2="12" y2="3"></line><line x1="12" y1="21" x2="12" y2="23"></line><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line><line x1="1" y1="12" x2="3" y2="12"></line><line x1="21" y1="12" x2="23" y2="12"></line><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line></svg>
      <span>Modo Claro</span>
    `;
  } else {
    document.documentElement.removeAttribute('data-theme');
    document.getElementById('themeToggleText').innerText = 'Modo Escuro';
    document.getElementById('themeToggleBtn').innerHTML = `
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path></svg>
      <span>Modo Escuro</span>
    `;
  }
  localStorage.setItem('pv_theme', theme);
  if (currentAnalysis) {
    renderCurrentChart();
  }
}

// 2. Navigation & View Switchers
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
      selectedSignalIndex = 0;
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

function initChartSwitcher() {
  const chartBtns = document.querySelectorAll('.chart-tab-btn');
  chartBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      chartBtns.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentChartMode = btn.dataset.chart;
      renderCurrentChart();
    });
  });
}

// 3. Search & Autocomplete
function initSearch() {
  const searchForm = document.getElementById('searchForm');
  const drugInput = document.getElementById('drugInput');
  const dropdown = document.getElementById('autocompleteDropdown');
  const presetTags = document.querySelectorAll('.tag-preset');

  let selectedIndex = -1;
  let currentMatches = [];

  function normalize(str) {
    return (str || '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
  }

  function highlightMatch(text, query) {
    if (!text) return '';
    const normText = normalize(text);
    const normQuery = normalize(query);
    const idx = normText.indexOf(normQuery);
    if (idx === -1) return escapeHtml(text);
    return escapeHtml(text.substring(0, idx)) + '<mark>' + escapeHtml(text.substring(idx, idx + query.length)) + '</mark>' + escapeHtml(text.substring(idx + query.length));
  }

  function renderDropdown(matches, query) {
    if (!matches.length) {
      dropdown.style.display = 'none';
      return;
    }

    currentMatches = matches;
    selectedIndex = -1;

    dropdown.innerHTML = matches.map((item, i) => {
      const matchBrand = item.synonyms.find(s => normalize(s).includes(normalize(query)));
      const brandDisplay = matchBrand ? `Marca/Sinônimo: ${highlightMatch(matchBrand, query)}` : escapeHtml(item.synonyms.slice(0, 3).join(', '));

      return `
        <div class="autocomplete-item" data-index="${i}" data-canonical="${escapeHtml(item.canonical)}">
          <div class="autocomplete-item-main">
            <span class="substance-name">${highlightMatch(item.canonical, query)}</span>
            <span class="substance-brand">${brandDisplay} &bull; <span style="color: var(--text-secondary);">${escapeHtml(item.class)}</span></span>
          </div>
          <div class="substance-tags">
            <span class="substance-dcb">${escapeHtml(item.dcb || 'DCB')}</span>
            <span class="substance-atc">${escapeHtml(item.atc)}</span>
          </div>
        </div>
      `;
    }).join('');

    dropdown.style.display = 'block';

    dropdown.querySelectorAll('.autocomplete-item').forEach(el => {
      el.addEventListener('click', () => {
        const canonical = el.dataset.canonical;
        drugInput.value = canonical;
        dropdown.style.display = 'none';
        performAnalysis(canonical);
      });
    });
  }

  drugInput.addEventListener('input', () => {
    const q = drugInput.value.trim();
    if (q.length < 1) {
      dropdown.style.display = 'none';
      return;
    }

    const normQ = normalize(q);
    const matches = PHARMA_CATALOG.filter(item => {
      if (normalize(item.canonical).includes(normQ)) return true;
      if (normalize(item.atc).includes(normQ)) return true;
      if (normalize(item.dcb).includes(normQ)) return true;
      return item.synonyms.some(s => normalize(s).includes(normQ));
    }).slice(0, 7);

    renderDropdown(matches, q);
  });

  drugInput.addEventListener('keydown', (e) => {
    const items = dropdown.querySelectorAll('.autocomplete-item');
    if (!items.length || dropdown.style.display === 'none') return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectedIndex = (selectedIndex + 1) % items.length;
      updateActiveItem(items);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectedIndex = (selectedIndex - 1 + items.length) % items.length;
      updateActiveItem(items);
    } else if (e.key === 'Enter') {
      if (selectedIndex >= 0 && selectedIndex < currentMatches.length) {
        e.preventDefault();
        const selected = currentMatches[selectedIndex];
        drugInput.value = selected.canonical;
        dropdown.style.display = 'none';
        performAnalysis(selected.canonical);
      }
    } else if (e.key === 'Escape') {
      dropdown.style.display = 'none';
    }
  });

  function updateActiveItem(items) {
    items.forEach((item, i) => {
      if (i === selectedIndex) {
        item.classList.add('active');
        item.scrollIntoView({ block: 'nearest' });
      } else {
        item.classList.remove('active');
      }
    });
  }

  document.addEventListener('click', (e) => {
    if (!searchForm.contains(e.target)) {
      dropdown.style.display = 'none';
    }
  });

  searchForm.addEventListener('submit', (e) => {
    e.preventDefault();
    dropdown.style.display = 'none';
    const query = drugInput.value.trim();
    if (query) {
      performAnalysis(query);
    }
  });

  presetTags.forEach(tag => {
    tag.addEventListener('click', () => {
      const drug = tag.dataset.drug;
      drugInput.value = drug;
      dropdown.style.display = 'none';
      performAnalysis(drug);
    });
  });
}

// 4. Data Fetching & Rendering
async function performAnalysis(drug) {
  const stateBox = document.getElementById('stateBox');
  const resultsContainer = document.getElementById('resultsContainer');
  const loadingSpinner = document.getElementById('loadingSpinner');

  stateBox.style.display = 'block';
  resultsContainer.style.display = 'none';
  loadingSpinner.style.display = 'block';
  stateBox.querySelector('.state-text').innerText = `Harmonizando dados do FDA FAERS e ANVISA VigiMed para "${drug}"...`;

  try {
    const res = await fetch(`/api/v1/analyze?drug=${encodeURIComponent(drug)}`);
    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || `HTTP error ${res.status}`);
    }

    const data = await res.json();
    currentAnalysis = data;
    selectedSignalIndex = 0;
    renderResults(data);
  } catch (err) {
    loadingSpinner.style.display = 'none';
    stateBox.querySelector('.state-text').innerText = `Erro na Análise: ${err.message}`;
  }
}

function renderResults(data) {
  const stateBox = document.getElementById('stateBox');
  const resultsContainer = document.getElementById('resultsContainer');

  stateBox.style.display = 'none';
  resultsContainer.style.display = 'block';

  renderComparativeBanner(data);
  renderJurisdictionData();
}

function renderComparativeBanner(data) {
  const comp = data.comparative_summary;

  document.getElementById('compDrugName').innerText = data.normalized_drug || data.query_drug;
  document.getElementById('compATCCode').innerText = data.atc_code ? `ATC: ${data.atc_code}` : 'ATC: N/A';
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
    li.innerText = 'Ambas as jurisdições monitoradas. Concordância calculada sobre os termos MedDRA Preferred Terms.';
    insightsList.appendChild(li);
  }

  const flagBtn = document.getElementById('flagComparativeSummary');
  if (flagBtn) {
    flagBtn.onclick = () => {
      flagStatistic({
        drug: data.normalized_drug,
        reaction: 'COMPARATIVO_INTERNACIONAL',
        jurisdiction: 'COMPARATIVE',
        metric: 'Resumo Comparativo FDA x ANVISA',
        displayed_value: `Sinais FDA: ${comp.fda_active_signals}, Sinais ANVISA: ${comp.anvisa_active_signals}, Razão de Reporte: ${comp.reporting_ratio_fda_vs_br ? comp.reporting_ratio_fda_vs_br.toFixed(1) : 'N/A'}x`,
        reason: 'Revisão de concordância entre jurisdições.'
      }, flagBtn);
    };
  }
}

function renderJurisdictionData() {
  if (!currentAnalysis) return;

  const isFDA = currentJurisdiction === 'fda';
  const dataset = isFDA ? currentAnalysis.fda_analysis : currentAnalysis.anvisa_analysis;

  if (!dataset) return;

  const drugTotal = dataset.drug_total_reports || dataset.total_reports_br || 0;
  const universeN = dataset.database_universe_n || dataset.database_universe_n_br || 0;

  document.getElementById('statJurisdictionTitle').innerText = isFDA ? 'US FDA FAERS' : 'ANVISA VigiMed';
  document.getElementById('statTotalReports').innerText = Number(drugTotal).toLocaleString();
  document.getElementById('statActiveSignals').innerText = dataset.active_signals_count || 0;
  document.getElementById('statUniverseN').innerText = Number(universeN).toLocaleString();

  const activeCard = document.getElementById('activeSignalsCard');
  if (dataset.active_signals_count > 0) {
    activeCard.style.borderColor = 'rgba(220, 38, 38, 0.4)';
  } else {
    activeCard.style.borderColor = 'var(--border-color)';
  }

  setupStatCardFlags(dataset, isFDA ? 'FDA' : 'ANVISA');

  const signals = dataset.signals || [];
  renderCurrentChart();
  renderTable(signals, isFDA ? 'FDA' : 'ANVISA', drugTotal, universeN);

  // Update 2x2 inspector with first signal or empty
  if (signals.length > 0) {
    updateInspector(signals[Math.min(selectedSignalIndex, signals.length - 1)], drugTotal, universeN);
  } else {
    clearInspector();
  }
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
        metric: 'Total de Relatos do Fármaco',
        displayed_value: String(dataset.drug_total_reports || dataset.total_reports_br || 0),
        reason: 'Questionar volume de relatos ou denominador amostral.'
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
        metric: 'Quantidade de Sinais Ativos',
        displayed_value: String(dataset.active_signals_count || 0),
        reason: 'Questionar critério de classificação de sinal.'
      }, flagActiveSignals);
    };
  }
}

// 5. 2x2 Contingency Table Inspector
function updateInspector(sig, drugTotal, universeN) {
  if (!sig) {
    clearInspector();
    return;
  }

  const countA = sig.count_a || 0;
  const countRxTotal = sig.reaction_total || 0;
  const b = Math.max(0, drugTotal - countA);
  const c = Math.max(0, countRxTotal - countA);
  const d = Math.max(0, universeN - (countA + b + c));

  // Expected value E = (a+b)(a+c) / N
  const expectedE = universeN > 0 ? (drugTotal * countRxTotal) / universeN : 0;
  const obsExpRatio = expectedE > 0 ? countA / expectedE : 0;

  const reactionLabel = sig.reaction_pt_br ? `${sig.reaction_pt_br} (${sig.reaction || sig.reaction_pt_en})` : (sig.reaction || sig.reaction_pt_en);

  document.getElementById('inspectorReactionTitle').innerText = reactionLabel;
  document.getElementById('inspectorReactionBadge').innerText = sig.signal_level === 'ACTIVE_SIGNAL' ? 'Sinal Ativo' : (sig.signal_level === 'POTENTIAL_SIGNAL' ? 'Potencial' : 'Baseline');

  document.getElementById('cellA').innerText = Number(countA).toLocaleString();
  document.getElementById('cellB').innerText = Number(b).toLocaleString();
  document.getElementById('cellMarginDrug').innerText = Number(drugTotal).toLocaleString();

  document.getElementById('cellC').innerText = Number(c).toLocaleString();
  document.getElementById('cellD').innerText = Number(d).toLocaleString();
  document.getElementById('cellMarginOtherDrug').innerText = Number(c + d).toLocaleString();

  document.getElementById('cellMarginEvent').innerText = Number(countRxTotal).toLocaleString();
  document.getElementById('cellMarginOtherEvent').innerText = Number(b + d).toLocaleString();
  document.getElementById('cellTotalUniverse').innerText = Number(universeN).toLocaleString();

  document.getElementById('inspectorExpectedVal').innerText = expectedE.toFixed(1);
  document.getElementById('inspectorObsExpRatio').innerText = `${obsExpRatio.toFixed(2)}x`;
}

function clearInspector() {
  document.getElementById('inspectorReactionTitle').innerText = 'Nenhuma reação selecionada';
  document.getElementById('inspectorReactionBadge').innerText = '-';
  ['cellA', 'cellB', 'cellC', 'cellD', 'cellMarginDrug', 'cellMarginOtherDrug', 'cellMarginEvent', 'cellMarginOtherEvent', 'cellTotalUniverse'].forEach(id => {
    document.getElementById(id).innerText = '-';
  });
  document.getElementById('inspectorExpectedVal').innerText = '-';
  document.getElementById('inspectorObsExpRatio').innerText = '-';
}

// 6. Clinical Data Table
function renderTable(signals, jurisdiction, drugTotal, universeN) {
  const tbody = document.getElementById('signalsTableBody');
  tbody.innerHTML = '';

  if (!signals || signals.length === 0) {
    tbody.innerHTML = `<tr><td colspan="8" class="state-box" style="padding: 2.5rem;">Nenhum relato registrado nesta base de dados para a substância pesquisada.</td></tr>`;
    return;
  }

  const sorted = [...signals].sort((a, b) => {
    if (a.signal_level === 'ACTIVE_SIGNAL' && b.signal_level !== 'ACTIVE_SIGNAL') return -1;
    if (b.signal_level === 'ACTIVE_SIGNAL' && a.signal_level !== 'ACTIVE_SIGNAL') return 1;
    return (b.prr || 0) - (a.prr || 0);
  });

  sorted.forEach((sig, idx) => {
    const tr = document.createElement('tr');
    tr.className = `table-row-hover ${idx === selectedSignalIndex ? 'row-selected' : ''}`;

    let badgeClass = 'badge-none';
    let badgeText = 'Baseline';
    if (sig.signal_level === 'ACTIVE_SIGNAL') {
      badgeClass = 'badge-active';
      badgeText = 'Forte / Ativo';
    } else if (sig.signal_level === 'POTENTIAL_SIGNAL') {
      badgeClass = 'badge-potential';
      badgeText = 'Moderado';
    }

    const countA = sig.count_a || 0;
    const countRxTotal = sig.reaction_total || 0;
    const expectedE = universeN > 0 ? (drugTotal * countRxTotal) / universeN : 0;

    const reactionName = sig.reaction_pt_br ? `${sig.reaction_pt_br} (${sig.reaction || sig.reaction_pt_en})` : (sig.reaction || sig.reaction_pt_en);
    const prrVal = sig.prr ? sig.prr.toFixed(2) : '0.00';
    const prrCI = `[${(sig.prr_lower_95 || 0).toFixed(2)} - ${(sig.prr_upper_95 || 0).toFixed(2)}]`;
    const rorVal = sig.ror ? sig.ror.toFixed(2) : '0.00';
    const rorCI = `[${(sig.ror_lower_95 || 0).toFixed(2)} - ${(sig.ror_upper_95 || 0).toFixed(2)}]`;
    const chi2Val = sig.chi_square_yates ? sig.chi_square_yates.toFixed(1) : '0.0';

    tr.innerHTML = `
      <td>
        <div style="font-weight: 600; color: var(--text-primary);">${escapeHtml(reactionName)}</div>
      </td>
      <td class="mono" style="font-weight: 600; color: var(--accent-cyan);">${Number(countA).toLocaleString()}</td>
      <td class="mono" style="color: var(--text-secondary);">${expectedE.toFixed(1)}</td>
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
        <button class="btn-flag-stat" title="Anexar esta estatística ao feedback" data-reaction="${escapeHtml(sig.reaction || sig.reaction_pt_en)}">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"></path><line x1="4" y1="22" x2="4" y2="15"></line></svg>
        </button>
      </td>
    `;

    // Click on row updates 2x2 Inspector
    tr.addEventListener('click', () => {
      selectedSignalIndex = idx;
      tbody.querySelectorAll('tr').forEach(r => r.classList.remove('row-selected'));
      tr.classList.add('row-selected');
      updateInspector(sig, drugTotal, universeN);
    });

    // Flag button
    const flagBtn = tr.querySelector('.btn-flag-stat');
    flagBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      flagStatistic({
        drug: currentAnalysis.normalized_drug,
        reaction: sig.reaction || sig.reaction_pt_en,
        jurisdiction: jurisdiction,
        metric: `PRR=${prrVal}, ROR=${rorVal}, Chi2=${chi2Val}, a=${countA}, E=${expectedE.toFixed(1)}`,
        displayed_value: `PRR: ${prrVal} ${prrCI} | ROR: ${rorVal} | χ²: ${chi2Val}`,
        reason: 'Revisão de desproporcionalidade ou relevância clínica.'
      }, flagBtn);
    });

    tbody.appendChild(tr);
  });
}

// 7. Statistical Charts (Volcano Plot & Forest Plot)
function initCanvas() {
  const canvas = document.getElementById('volcanoCanvas');
  if (canvas) {
    canvas.addEventListener('mousemove', handleCanvasHover);
    window.addEventListener('resize', () => {
      if (currentAnalysis) renderCurrentChart();
    });
  }
}

function renderCurrentChart() {
  if (!currentAnalysis) return;
  const isFDA = currentJurisdiction === 'fda';
  const dataset = isFDA ? currentAnalysis.fda_analysis : currentAnalysis.anvisa_analysis;
  const signals = dataset ? dataset.signals || [] : [];

  if (currentChartMode === 'forest') {
    drawForestPlot(signals);
  } else {
    drawVolcanoPlot(signals);
  }
}

function getThemeColors() {
  const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
  return {
    axis: isDark ? '#334155' : '#cbd5e1',
    grid: isDark ? '#1e293b' : '#f1f5f9',
    text: isDark ? '#94a3b8' : '#64748b',
    activeDot: isDark ? '#ef4444' : '#dc2626',
    potentialDot: isDark ? '#f59e0b' : '#d97706',
    noneDot: isDark ? '#64748b' : '#94a3b8',
    threshold: isDark ? 'rgba(239, 68, 68, 0.4)' : 'rgba(220, 38, 38, 0.4)',
    forestLine: isDark ? '#0284c7' : '#0d9488'
  };
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
  const colors = getThemeColors();

  ctx.clearRect(0, 0, w, h);
  plotPoints = [];

  if (!signals || signals.length === 0) {
    ctx.fillStyle = colors.text;
    ctx.font = '12px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('Nenhum dado registrado para o gráfico', w / 2, h / 2);
    return;
  }

  let maxLogPRR = 4;
  let maxSqrtChi = 10;

  signals.forEach(s => {
    const prr = s.prr || 1;
    const logPrr = Math.log2(Math.max(0.1, prr));
    const sqrtChi = Math.sqrt(Math.max(0, s.chi_square_yates || 0));
    if (Math.abs(logPrr) > maxLogPRR) maxLogPRR = Math.ceil(Math.abs(logPrr));
    if (sqrtChi > maxSqrtChi) maxSqrtChi = Math.ceil(sqrtChi);
  });

  const xScale = (val) => padding.left + ((val + maxLogPRR) / (2 * maxLogPRR)) * (w - padding.left - padding.right);
  const yScale = (val) => (h - padding.bottom) - (val / maxSqrtChi) * (h - padding.top - padding.bottom);

  // Axes and grid
  ctx.strokeStyle = colors.axis;
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(padding.left, padding.top);
  ctx.lineTo(padding.left, h - padding.bottom);
  ctx.lineTo(w - padding.right, h - padding.bottom);
  ctx.stroke();

  // Thresholds (PRR >= 2.0 -> log2(2) = 1.0, Chi2 >= 4.0 -> sqrt(4) = 2.0)
  ctx.strokeStyle = colors.threshold;
  ctx.setLineDash([4, 4]);

  const xThresh = xScale(1.0);
  ctx.beginPath();
  ctx.moveTo(xThresh, padding.top);
  ctx.lineTo(xThresh, h - padding.bottom);
  ctx.stroke();

  const yThresh = yScale(2.0);
  ctx.beginPath();
  ctx.moveTo(padding.left, yThresh);
  ctx.lineTo(w - padding.right, yThresh);
  ctx.stroke();
  ctx.setLineDash([]);

  // Axis Labels
  ctx.fillStyle = colors.text;
  ctx.font = '11px monospace';
  ctx.textAlign = 'center';
  ctx.fillText('0', xScale(0), h - padding.bottom + 16);
  ctx.fillText('PRR 2.0 (1.0)', xScale(1.0), h - padding.bottom + 16);
  ctx.fillText(`+${maxLogPRR}`, xScale(maxLogPRR), h - padding.bottom + 16);
  ctx.fillText(`-${maxLogPRR}`, xScale(-maxLogPRR), h - padding.bottom + 16);

  ctx.textAlign = 'right';
  ctx.fillText('0', padding.left - 6, yScale(0) + 4);
  ctx.fillText('χ² 4.0 (2.0)', padding.left - 6, yScale(2.0) + 4);
  ctx.fillText(`√χ² ${maxSqrtChi}`, padding.left - 6, yScale(maxSqrtChi) + 4);

  // Plot Points
  signals.forEach(s => {
    const prr = s.prr || 1;
    const logPrr = Math.log2(Math.max(0.1, prr));
    const sqrtChi = Math.sqrt(Math.max(0, s.chi_square_yates || 0));

    const cx = xScale(logPrr);
    const cy = yScale(sqrtChi);

    let color = colors.noneDot;
    let radius = 4;
    if (s.signal_level === 'ACTIVE_SIGNAL') {
      color = colors.activeDot;
      radius = 6;
    } else if (s.signal_level === 'POTENTIAL_SIGNAL') {
      color = colors.potentialDot;
      radius = 5;
    }

    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.arc(cx, cy, radius, 0, Math.PI * 2);
    ctx.fill();

    plotPoints.push({ x: cx, y: cy, radius, signal: s, mode: 'volcano' });
  });
}

function drawForestPlot(signals) {
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
  const padding = { top: 20, right: 30, bottom: 40, left: 160 };
  const colors = getThemeColors();

  ctx.clearRect(0, 0, w, h);
  plotPoints = [];

  const topSignals = (signals || []).slice(0, 8);
  if (!topSignals.length) {
    ctx.fillStyle = colors.text;
    ctx.font = '12px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('Nenhum dado registrado para o Forest Plot', w / 2, h / 2);
    return;
  }

  let maxUpper = 5.0;
  topSignals.forEach(s => {
    if (s.prr_upper_95 && s.prr_upper_95 > maxUpper) maxUpper = Math.min(30, Math.ceil(s.prr_upper_95));
  });

  const xScale = (val) => padding.left + (Math.max(0, val) / maxUpper) * (w - padding.left - padding.right);
  const rowHeight = (h - padding.top - padding.bottom) / topSignals.length;

  // Vertical line at null PRR = 1.0
  ctx.strokeStyle = colors.threshold;
  ctx.lineWidth = 1;
  ctx.setLineDash([4, 4]);
  ctx.beginPath();
  ctx.moveTo(xScale(1.0), padding.top);
  ctx.lineTo(xScale(1.0), h - padding.bottom);
  ctx.stroke();
  ctx.setLineDash([]);

  // Axis
  ctx.strokeStyle = colors.axis;
  ctx.beginPath();
  ctx.moveTo(padding.left, h - padding.bottom);
  ctx.lineTo(w - padding.right, h - padding.bottom);
  ctx.stroke();

  // Axis Labels
  ctx.fillStyle = colors.text;
  ctx.font = '10px monospace';
  ctx.textAlign = 'center';
  [0, 1.0, 2.0, maxUpper / 2, maxUpper].forEach(tick => {
    ctx.fillText(tick.toFixed(1), xScale(tick), h - padding.bottom + 15);
  });

  // Render each reaction row
  topSignals.forEach((s, idx) => {
    const yCenter = padding.top + idx * rowHeight + rowHeight / 2;
    const reactionLabel = s.reaction_pt_br || s.reaction || s.reaction_pt_en;

    // Label on the left
    ctx.fillStyle = colors.text;
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'right';
    const truncatedLabel = reactionLabel.length > 20 ? reactionLabel.substring(0, 18) + '…' : reactionLabel;
    ctx.fillText(truncatedLabel, padding.left - 10, yCenter + 4);

    const prr = s.prr || 1;
    const lower = Math.max(0, s.prr_lower_95 || prr * 0.8);
    const upper = Math.min(maxUpper, s.prr_upper_95 || prr * 1.2);

    const xPt = xScale(prr);
    const xLow = xScale(lower);
    const xHigh = xScale(upper);

    let dotColor = colors.noneDot;
    if (s.signal_level === 'ACTIVE_SIGNAL') dotColor = colors.activeDot;
    else if (s.signal_level === 'POTENTIAL_SIGNAL') dotColor = colors.potentialDot;

    // Error bar
    ctx.strokeStyle = colors.forestLine;
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(xLow, yCenter);
    ctx.lineTo(xHigh, yCenter);
    // Whiskers
    ctx.moveTo(xLow, yCenter - 4);
    ctx.lineTo(xLow, yCenter + 4);
    ctx.moveTo(xHigh, yCenter - 4);
    ctx.lineTo(xHigh, yCenter + 4);
    ctx.stroke();

    // Point estimate dot
    ctx.fillStyle = dotColor;
    ctx.beginPath();
    ctx.arc(xPt, yCenter, 4.5, 0, Math.PI * 2);
    ctx.fill();

    plotPoints.push({ x: xPt, y: yCenter, radius: 8, signal: s, mode: 'forest' });
  });
}

function handleCanvasHover(e) {
  const canvas = document.getElementById('volcanoCanvas');
  const tooltip = document.getElementById('plotTooltip');
  if (!canvas || !tooltip) return;

  const rect = canvas.getBoundingClientRect();
  const mouseX = e.clientX - rect.left;
  const mouseY = e.clientY - rect.top;

  let hovered = null;
  for (const pt of plotPoints) {
    const dist = Math.hypot(pt.x - mouseX, pt.y - mouseY);
    if (dist < pt.radius + 4) {
      hovered = pt.signal;
      break;
    }
  }

  if (hovered) {
    const label = hovered.reaction_pt_br ? `${hovered.reaction_pt_br} (${hovered.reaction_pt_en || hovered.reaction})` : (hovered.reaction || hovered.reaction_pt_en);
    tooltip.innerHTML = `
      <div style="font-weight: 700; color: var(--text-primary); margin-bottom: 0.2rem;">${escapeHtml(label)}</div>
      <div style="font-size: 0.75rem; color: var(--text-secondary);">
        PRR: <strong style="color: var(--accent-cyan);">${(hovered.prr || 0).toFixed(2)}</strong> [${(hovered.prr_lower_95 || 0).toFixed(2)} - ${(hovered.prr_upper_95 || 0).toFixed(2)}]<br>
        ROR: <strong>${(hovered.ror || 0).toFixed(2)}</strong> &bull; χ²: <strong>${(hovered.chi_square_yates || 0).toFixed(1)}</strong><br>
        Relatos Observados (a): <strong>${Number(hovered.count_a || 0).toLocaleString()}</strong>
      </div>
    `;
    tooltip.style.display = 'block';
    tooltip.style.left = `${e.clientX + 14}px`;
    tooltip.style.top = `${e.clientY + 14}px`;
  } else {
    tooltip.style.display = 'none';
  }
}

// 8. Feedback Queue & Form
function initFeedbackForm() {
  const form = document.getElementById('feedbackForm');
  if (!form) return;

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = document.getElementById('feedbackEmail').value.trim();
    const comments = document.getElementById('feedbackComments').value.trim();
    const statusMsg = document.getElementById('feedbackStatusMsg');

    if (!email || !comments) return;

    statusMsg.innerHTML = '<span style="color: var(--accent-cyan);">Enviando feedback...</span>';

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

      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.error || 'Falha ao enviar feedback.');
      }

      const resData = await res.json();
      statusMsg.innerHTML = `<span style="color: var(--signal-none);">✓ Feedback registrado com sucesso! ID: ${resData.feedback_id}</span>`;
      form.reset();
      flaggedQueue = [];
      updateFeedbackQueueUI();
    } catch (err) {
      statusMsg.innerHTML = `<span style="color: var(--signal-active);">Erro: ${err.message}</span>`;
    }
  });
}

function flagStatistic(stat, btnElement) {
  const existingIdx = flaggedQueue.findIndex(f => f.drug === stat.drug && f.reaction === stat.reaction && f.jurisdiction === stat.jurisdiction);

  if (existingIdx >= 0) {
    flaggedQueue.splice(existingIdx, 1);
    if (btnElement) btnElement.classList.remove('flagged');
  } else {
    flaggedQueue.push(stat);
    if (btnElement) btnElement.classList.add('flagged');
  }

  updateFeedbackQueueUI();
}

function updateFeedbackQueueUI() {
  const count = flaggedQueue.length;
  document.getElementById('headerFeedbackBadge').innerText = count;
  document.getElementById('queueCountPill').innerText = count;

  const floatingBar = document.getElementById('floatingFeedbackBar');
  if (floatingBar) {
    floatingBar.style.display = count > 0 ? 'flex' : 'none';
  }

  const queueList = document.getElementById('flaggedQueueList');
  const noPrompt = document.getElementById('noFlaggedItemsPrompt');

  if (!queueList || !noPrompt) return;

  if (count === 0) {
    noPrompt.style.display = 'block';
    queueList.innerHTML = '';
  } else {
    noPrompt.style.display = 'none';
    queueList.innerHTML = flaggedQueue.map((item, i) => `
      <div class="flagged-queue-item">
        <div class="flagged-item-info">
          <strong>${escapeHtml(item.drug)} &bull; ${escapeHtml(item.reaction)} [${escapeHtml(item.jurisdiction)}]</strong>
          <span>${escapeHtml(item.displayed_value)}</span>
        </div>
        <button class="btn-remove-flag" onclick="removeFlaggedItem(${i})" title="Remover item">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
        </button>
      </div>
    `).join('');
  }
}

window.removeFlaggedItem = function(index) {
  flaggedQueue.splice(index, 1);
  updateFeedbackQueueUI();
};

function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}
