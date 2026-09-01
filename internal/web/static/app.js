// PV Signal Radar client: localization, accessible result rendering, and canvas plotting.
// All translated text remains client-side because cached API payloads are language-neutral.

const translations = {
  'pt-BR': {
    pageTitle: 'PV Signal Radar — triagem exploratória de sinais em farmacovigilância',
    pageDescription: 'Triagem estatística exploratória de sinais de farmacovigilância com dados FAERS do openFDA, PRR, ROR e qui-quadrado de Yates.',
    brandTitle: 'Radar de Sinais',
    brandSubtitle: 'Mineração de dados em farmacovigilância',
    methodologyLink: 'Metodologia',
    languageLabel: 'Idioma da interface',
    githubLink: 'GitHub (MIT)',
    heroTitle: 'Triagem aberta de sinais de desproporcionalidade',
    heroText: 'Explore relatos espontâneos no FAERS com PRR, ROR e qui-quadrado de Yates. Os resultados geram hipóteses; não demonstram causalidade.',
    drugInputLabel: 'Substância ativa',
    drugPlaceholder: 'Pesquise uma substância ativa (ex.: Semaglutida, Pembrolizumabe, Metformina)…',
    analyze: 'Analisar',
    popularBenchmarks: 'Exemplos:',
    stateReady: 'Pronto para consultar a base FAERS do openFDA.',
    stateLoading: 'Consultando dados FAERS do openFDA para “{drug}”…',
    stateError: 'Erro: {message}',
    errorDrugRequired: 'Informe uma substância ativa para análise.',
    errorInvalidDrug: 'Use uma substância ativa sem quebras de linha e com até 120 caracteres.',
    errorBusy: 'O serviço está processando outras análises. Tente novamente em alguns segundos.',
    errorRateLimited: 'O serviço atingiu temporariamente o ritmo seguro de consultas ao openFDA. Tente novamente em alguns segundos.',
    errorUnavailable: 'A análise não pôde ser concluída com dados completos do openFDA. Tente novamente mais tarde.',
    errorTimeout: 'A consulta excedeu o tempo máximo. Tente novamente mais tarde.',
    errorUnexpected: 'Não foi possível processar a resposta da análise.',
    statAnalyzedSubstance: 'Substância analisada',
    statQueriedTarget: 'Medicamento-alvo consultado',
    statDrugReports: 'Relatos do medicamento (a + b)',
    statReportsMentionTarget: 'Relatos que mencionam o medicamento-alvo',
    statActiveSignals: 'Sinais de triagem ativos',
    statActiveCriteria: 'PRR ≥ 2,0; χ² ≥ 4,0; a ≥ 3',
    statUniverseReports: 'Universo de relatos (N)',
    statFaersRecords: 'Registros totais da base FAERS',
    volcanoTitle: 'Gráfico vulcão de desproporcionalidade',
    plotAriaLabel: 'Gráfico vulcão de PRR e qui-quadrado de Yates',
    plotDescription: 'O gráfico mostra cada reação analisada por log2 do PRR e raiz quadrada do qui-quadrado de Yates.',
    signalActive: 'Sinal ativo de triagem',
    signalPotential: 'Sinal potencial',
    signalBaseline: 'Sem sinal',
    plotLegendLabel: 'Legenda do gráfico',
    eventsTitle: 'Principais eventos adversos e métricas de desproporcionalidade',
    adverseReaction: 'Reação adversa (MedDRA PT)',
    reports: 'Relatos (a)',
    prrCI: 'PRR [IC 95%]',
    rorCIHeader: 'ROR [IC 95%]',
    chiYates: 'χ² (Yates)',
    signalStatus: 'Status de triagem',
    disclaimerTitle: 'Limitação metodológica:',
    disclaimerText: 'os cálculos usam relatos espontâneos do FAERS. Coocorrência no relato não vincula individualmente um medicamento à reação, não comprova causalidade e não estima incidência clínica.',
    methodologyTitle: 'Metodologia estatística e formulação',
    methodologyIntro: 'A triagem avalia se uma reação é relatada proporcionalmente mais para o medicamento-alvo do que no background dos demais relatos. Isso prioriza hipóteses para revisão; não estabelece uma conclusão de segurança.',
    prrTitle: '1. Proportional Reporting Ratio (PRR)',
    prrText: 'Compara a proporção de relatos da reação para o medicamento de interesse com a proporção no background.',
    prrCriteria: 'Limiar configurado de triagem: PRR ≥ 2,0, a ≥ 3 e χ² ≥ 4,0.',
    rorTitle: '2. Reporting Odds Ratio (ROR)',
    rorText: 'Compara as odds de relatar o evento com o medicamento-alvo e com os demais medicamentos.',
    rorCI: 'Os intervalos de confiança usam variância assintótica com correção de Haldane-Anscombe quando há célula zero.',
    chiTitle: '3. Qui-quadrado corrigido de Yates',
    chiText: 'Estatística de continuidade para uma tabela 2 × 2 com um grau de liberdade.',
    footerText: 'PV Signal Radar • código aberto (MIT) • Go e dados FAERS do openFDA •',
    noData: 'Nenhum dado de reação adversa foi encontrado para esta substância no FAERS.',
    inspectMatrix: 'Ver matriz 2 × 2',
    collapseMatrix: 'Ocultar matriz 2 × 2',
    matrixHeading: 'Matriz de contingência 2 × 2 para {drug} × {reaction}',
    matrixA: 'Medicamento-alvo + reação-alvo (a)',
    matrixB: 'Medicamento-alvo + outras reações (b)',
    matrixDrugTotal: 'Total do medicamento-alvo (a + b)',
    matrixC: 'Outros medicamentos + reação-alvo (c)',
    matrixD: 'Outros medicamentos + outras reações (d)',
    matrixUniverse: 'Universo da base (N)',
    statisticalInterpretation: 'Interpretação de triagem:',
    interpretationActive: 'O limiar configurado de triagem foi atingido. Revisão científica e clínica é necessária antes de qualquer conclusão de segurança.',
    interpretationPotential: 'Há desproporcionalidade potencial de relato. Monitore e revise os relatos subjacentes.',
    interpretationNone: 'Nenhum limiar configurado de triagem foi atingido nesta análise exploratória.',
    plotNoData: 'Não há pontos para exibir',
    plotXAxis: 'log₂(PRR) → limiar de alerta = 1,0 (PRR = 2)',
    plotYAxis: '√(χ² de Yates)',
    tooltipPRR: 'PRR',
    tooltipROR: 'ROR',
    tooltipChi: 'χ²',
    tooltipReports: 'Relatos (a)'
  },
  es: {
    pageTitle: 'PV Signal Radar — cribado exploratorio de señales de farmacovigilancia',
    pageDescription: 'Cribado estadístico exploratorio de señales de farmacovigilancia con datos FAERS de openFDA, PRR, ROR y chi-cuadrado de Yates.',
    brandTitle: 'Radar de Señales',
    brandSubtitle: 'Minería de datos en farmacovigilancia',
    methodologyLink: 'Metodología',
    languageLabel: 'Idioma de la interfaz',
    githubLink: 'GitHub (MIT)',
    heroTitle: 'Cribado abierto de señales de desproporcionalidad',
    heroText: 'Explore informes espontáneos de FAERS con PRR, ROR y chi-cuadrado de Yates. Los resultados generan hipótesis; no demuestran causalidad.',
    drugInputLabel: 'Sustancia activa',
    drugPlaceholder: 'Busque una sustancia activa (p. ej., semaglutida, pembrolizumab, metformina)…',
    analyze: 'Analizar',
    popularBenchmarks: 'Ejemplos:',
    stateReady: 'Listo para consultar la base FAERS de openFDA.',
    stateLoading: 'Consultando datos FAERS de openFDA para “{drug}”…',
    stateError: 'Error: {message}',
    errorDrugRequired: 'Indique una sustancia activa para analizar.',
    errorInvalidDrug: 'Use una sustancia activa sin saltos de línea y de hasta 120 caracteres.',
    errorBusy: 'El servicio está procesando otros análisis. Inténtelo de nuevo en unos segundos.',
    errorRateLimited: 'El servicio alcanzó temporalmente el ritmo seguro de consultas a openFDA. Inténtelo de nuevo en unos segundos.',
    errorUnavailable: 'El análisis no pudo completarse con datos íntegros de openFDA. Inténtelo de nuevo más tarde.',
    errorTimeout: 'La consulta excedió el tiempo máximo. Inténtelo de nuevo más tarde.',
    errorUnexpected: 'No se pudo procesar la respuesta del análisis.',
    statAnalyzedSubstance: 'Sustancia analizada',
    statQueriedTarget: 'Medicamento objetivo consultado',
    statDrugReports: 'Informes del medicamento (a + b)',
    statReportsMentionTarget: 'Informes que mencionan el medicamento objetivo',
    statActiveSignals: 'Señales de cribado activas',
    statActiveCriteria: 'PRR ≥ 2,0; χ² ≥ 4,0; a ≥ 3',
    statUniverseReports: 'Universo de informes (N)',
    statFaersRecords: 'Registros totales de la base FAERS',
    volcanoTitle: 'Gráfico volcán de desproporcionalidad',
    plotAriaLabel: 'Gráfico volcán de PRR y chi-cuadrado de Yates',
    plotDescription: 'El gráfico muestra cada reacción analizada por log2 del PRR y raíz cuadrada del chi-cuadrado de Yates.',
    signalActive: 'Señal activa de cribado',
    signalPotential: 'Señal potencial',
    signalBaseline: 'Sin señal',
    plotLegendLabel: 'Leyenda del gráfico',
    eventsTitle: 'Principales eventos adversos y métricas de desproporcionalidad',
    adverseReaction: 'Reacción adversa (MedDRA PT)',
    reports: 'Informes (a)',
    prrCI: 'PRR [IC 95%]',
    rorCIHeader: 'ROR [IC 95%]',
    chiYates: 'χ² (Yates)',
    signalStatus: 'Estado de cribado',
    disclaimerTitle: 'Limitación metodológica:',
    disclaimerText: 'los cálculos usan informes espontáneos de FAERS. La coocurrencia en un informe no vincula individualmente un medicamento con la reacción, no prueba causalidad ni estima incidencia clínica.',
    methodologyTitle: 'Metodología estadística y formulación',
    methodologyIntro: 'El cribado evalúa si una reacción se informa proporcionalmente más para el medicamento objetivo que en el background de los demás informes. Prioriza hipótesis para revisión; no establece una conclusión de seguridad.',
    prrTitle: '1. Proportional Reporting Ratio (PRR)',
    prrText: 'Compara la proporción de informes de la reacción para el medicamento de interés con la proporción del background.',
    prrCriteria: 'Umbral configurado de cribado: PRR ≥ 2,0, a ≥ 3 y χ² ≥ 4,0.',
    rorTitle: '2. Reporting Odds Ratio (ROR)',
    rorText: 'Compara las odds de informar el evento con el medicamento objetivo y con los demás medicamentos.',
    rorCI: 'Los intervalos de confianza usan varianza asintótica con corrección de Haldane-Anscombe cuando hay una celda cero.',
    chiTitle: '3. Chi-cuadrado corregido de Yates',
    chiText: 'Estadístico de continuidad para una tabla 2 × 2 con un grado de libertad.',
    footerText: 'PV Signal Radar • código abierto (MIT) • Go y datos FAERS de openFDA •',
    noData: 'No se encontraron datos de reacciones adversas para esta sustancia en FAERS.',
    inspectMatrix: 'Ver matriz 2 × 2',
    collapseMatrix: 'Ocultar matriz 2 × 2',
    matrixHeading: 'Matriz de contingencia 2 × 2 para {drug} × {reaction}',
    matrixA: 'Medicamento objetivo + reacción objetivo (a)',
    matrixB: 'Medicamento objetivo + otras reacciones (b)',
    matrixDrugTotal: 'Total del medicamento objetivo (a + b)',
    matrixC: 'Otros medicamentos + reacción objetivo (c)',
    matrixD: 'Otros medicamentos + otras reacciones (d)',
    matrixUniverse: 'Universo de la base (N)',
    statisticalInterpretation: 'Interpretación de cribado:',
    interpretationActive: 'Se alcanzó el umbral configurado de cribado. Se requiere revisión científica y clínica antes de cualquier conclusión de seguridad.',
    interpretationPotential: 'Hay desproporcionalidad potencial de informes. Monitoree y revise los informes subyacentes.',
    interpretationNone: 'No se alcanzó ningún umbral configurado de cribado en este análisis exploratorio.',
    plotNoData: 'No hay puntos para mostrar',
    plotXAxis: 'log₂(PRR) → umbral de alerta = 1,0 (PRR = 2)',
    plotYAxis: '√(χ² de Yates)',
    tooltipPRR: 'PRR',
    tooltipROR: 'ROR',
    tooltipChi: 'χ²',
    tooltipReports: 'Informes (a)'
  },
  en: {
    pageTitle: 'PV Signal Radar — exploratory pharmacovigilance signal screening',
    pageDescription: 'Exploratory pharmacovigilance signal screening with openFDA FAERS data, PRR, ROR, and Yates chi-square.',
    brandTitle: 'Signal Radar',
    brandSubtitle: 'Pharmacovigilance data mining',
    methodologyLink: 'Methodology',
    languageLabel: 'Interface language',
    githubLink: 'GitHub (MIT)',
    heroTitle: 'Open disproportionality signal screening',
    heroText: 'Explore spontaneous FAERS reports with PRR, ROR, and Yates chi-square. Results generate hypotheses; they do not establish causality.',
    drugInputLabel: 'Active substance',
    drugPlaceholder: 'Search an active substance (e.g., Semaglutide, Pembrolizumab, Metformin)…',
    analyze: 'Analyze',
    popularBenchmarks: 'Examples:',
    stateReady: 'Ready to query the openFDA FAERS dataset.',
    stateLoading: 'Querying openFDA FAERS data for “{drug}”…',
    stateError: 'Error: {message}',
    errorDrugRequired: 'Enter an active substance to analyze.',
    errorInvalidDrug: 'Use an active substance without line breaks and with at most 120 characters.',
    errorBusy: 'The service is processing other analyses. Try again in a few seconds.',
    errorRateLimited: 'The service has temporarily reached its safe openFDA query pace. Try again in a few seconds.',
    errorUnavailable: 'The analysis could not be completed with complete openFDA data. Try again later.',
    errorTimeout: 'The query exceeded the maximum time. Try again later.',
    errorUnexpected: 'The analysis response could not be processed.',
    statAnalyzedSubstance: 'Analyzed substance',
    statQueriedTarget: 'Queried target drug',
    statDrugReports: 'Drug reports (a + b)',
    statReportsMentionTarget: 'Reports mentioning the target drug',
    statActiveSignals: 'Active screening signals',
    statActiveCriteria: 'PRR ≥ 2.0; χ² ≥ 4.0; a ≥ 3',
    statUniverseReports: 'Report universe (N)',
    statFaersRecords: 'Total FAERS database records',
    volcanoTitle: 'Disproportionality volcano plot',
    plotAriaLabel: 'Volcano plot of PRR and Yates chi-square',
    plotDescription: 'The plot shows each analyzed reaction by log2 PRR and square root of Yates chi-square.',
    signalActive: 'Active screening signal',
    signalPotential: 'Potential signal',
    signalBaseline: 'No signal',
    plotLegendLabel: 'Plot legend',
    eventsTitle: 'Top adverse events and disproportionality metrics',
    adverseReaction: 'Adverse reaction (MedDRA PT)',
    reports: 'Reports (a)',
    prrCI: 'PRR [95% CI]',
    rorCIHeader: 'ROR [95% CI]',
    chiYates: 'χ² (Yates)',
    signalStatus: 'Screening status',
    disclaimerTitle: 'Methodological limitation:',
    disclaimerText: 'calculations use spontaneous FAERS reports. Report-level co-occurrence does not individually link a drug to a reaction, establish causality, or estimate clinical incidence.',
    methodologyTitle: 'Statistical methodology and formulation',
    methodologyIntro: 'The screening evaluates whether a reaction is reported proportionally more often for the target drug than in the background of other reports. It prioritizes hypotheses for review; it does not establish a safety conclusion.',
    prrTitle: '1. Proportional Reporting Ratio (PRR)',
    prrText: 'Compares the reaction reporting proportion for the drug of interest against the background proportion.',
    prrCriteria: 'Configured screening threshold: PRR ≥ 2.0, a ≥ 3, and χ² ≥ 4.0.',
    rorTitle: '2. Reporting Odds Ratio (ROR)',
    rorText: 'Compares the odds of reporting the event with the target drug and with other drugs.',
    rorCI: 'Confidence intervals use asymptotic variance with Haldane-Anscombe correction when any cell is zero.',
    chiTitle: '3. Yates-corrected chi-square',
    chiText: 'Continuity statistic for a 2 × 2 table with one degree of freedom.',
    footerText: 'PV Signal Radar • open source (MIT) • Go and openFDA FAERS data •',
    noData: 'No adverse-reaction data was found for this substance in FAERS.',
    inspectMatrix: 'View 2 × 2 matrix',
    collapseMatrix: 'Hide 2 × 2 matrix',
    matrixHeading: '2 × 2 contingency matrix for {drug} × {reaction}',
    matrixA: 'Target drug + target reaction (a)',
    matrixB: 'Target drug + other reactions (b)',
    matrixDrugTotal: 'Target drug total (a + b)',
    matrixC: 'Other drugs + target reaction (c)',
    matrixD: 'Other drugs + other reactions (d)',
    matrixUniverse: 'Database universe (N)',
    statisticalInterpretation: 'Screening interpretation:',
    interpretationActive: 'The configured screening threshold was met. Scientific and clinical review are required before any safety conclusion.',
    interpretationPotential: 'Potential reporting disproportionality was detected. Monitor and review the underlying reports.',
    interpretationNone: 'No configured screening threshold was met in this exploratory analysis.',
    plotNoData: 'No data points to display',
    plotXAxis: 'log₂(PRR) → alert threshold = 1.0 (PRR = 2)',
    plotYAxis: '√(Yates χ²)',
    tooltipPRR: 'PRR',
    tooltipROR: 'ROR',
    tooltipChi: 'χ²',
    tooltipReports: 'Reports (a)'
  }
};

const localeByLanguage = {
  'pt-BR': 'pt-BR',
  es: 'es-ES',
  en: 'en-US'
};

let currentLanguage = 'pt-BR';
let currentAnalysis = null;
let plotPoints = [];
let analysisSequence = 0;
let activeAnalysisController = null;
let stateKey = 'stateReady';
let stateParams = {};

document.addEventListener('DOMContentLoaded', () => {
  const searchForm = document.getElementById('searchForm');
  const drugInput = document.getElementById('drugInput');
  const languageSelect = document.getElementById('languageSelect');

  setLanguage(resolveLanguage(), false);
  showState('stateReady');

  searchForm.addEventListener('submit', (event) => {
    event.preventDefault();
    performAnalysis(drugInput.value.trim());
  });

  document.querySelectorAll('.tag-preset').forEach((tag) => {
    tag.addEventListener('click', () => {
      drugInput.value = tag.dataset.drug || '';
      performAnalysis(drugInput.value);
    });
  });

  languageSelect.addEventListener('change', (event) => {
    setLanguage(event.target.value);
  });

  const canvas = document.getElementById('volcanoCanvas');
  if (canvas) {
    canvas.addEventListener('mousemove', handleCanvasHover);
    canvas.addEventListener('mouseleave', hidePlotTooltip);
    window.addEventListener('resize', () => {
      if (currentAnalysis) drawVolcanoPlot(currentAnalysis.signals || []);
    });
  }
});

function resolveLanguage() {
  try {
    const saved = window.localStorage.getItem('pv-signal-radar-language');
    if (saved && translations[saved]) return saved;
  } catch (_) {
    // Private browsing can deny localStorage. Locale negotiation remains usable.
  }

  const browserLanguages = navigator.languages || [navigator.language];
  if (browserLanguages.some((language) => language.toLowerCase().startsWith('pt'))) return 'pt-BR';
  if (browserLanguages.some((language) => language.toLowerCase().startsWith('es'))) return 'es';
  return 'pt-BR';
}

function setLanguage(language, persist = true) {
  if (!translations[language]) return;

  currentLanguage = language;
  document.documentElement.lang = language;
  document.title = t('pageTitle');
  const description = document.querySelector('meta[name="description"]');
  if (description) description.setAttribute('content', t('pageDescription'));

  document.querySelectorAll('[data-i18n]').forEach((element) => {
    element.textContent = t(element.dataset.i18n);
  });
  document.querySelectorAll('[data-i18n-placeholder]').forEach((element) => {
    element.setAttribute('placeholder', t(element.dataset.i18nPlaceholder));
  });
  document.querySelectorAll('[data-i18n-aria-label]').forEach((element) => {
    element.setAttribute('aria-label', t(element.dataset.i18nAriaLabel));
  });

  const languageSelect = document.getElementById('languageSelect');
  if (languageSelect) languageSelect.value = language;
  if (persist) {
    try {
      window.localStorage.setItem('pv-signal-radar-language', language);
    } catch (_) {
      // Persistence is a convenience; an unavailable browser store must not block use.
    }
  }

  if (stateKey === 'resultsReady' && currentAnalysis) {
    renderResults(currentAnalysis);
  } else {
    showState(stateKey, stateParams);
  }
}

function t(key, params = {}) {
  const message = translations[currentLanguage][key] || translations['pt-BR'][key] || key;
  return message.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? `{${name}}`));
}

function showState(key, params = {}) {
  stateKey = key;
  stateParams = params;
  const stateBox = document.getElementById('stateBox');
  const resultsContainer = document.getElementById('resultsContainer');
  const loadingSpinner = document.getElementById('loadingSpinner');
  const stateText = stateBox.querySelector('.state-text');

  stateBox.style.display = 'block';
  resultsContainer.style.display = 'none';
  loadingSpinner.style.display = key === 'stateLoading' ? 'block' : 'none';
  // Persist an error key rather than a translated sentence. A user can change
  // locale while the error is visible, and the complete state must then render
  // in the new language without retaining an earlier localized fragment.
  const displayParams = params.errorKey
    ? { ...params, message: t(params.errorKey, params.errorParams || {}) }
    : params;
  stateText.textContent = t(key, displayParams);
}

async function performAnalysis(drug) {
  // Every submission, including an invalid one, owns the screen. Invalidating
  // and aborting the older fetch prevents a late response from resurrecting a
  // prior result over a newer loading or error state.
  const requestSequence = ++analysisSequence;
  if (activeAnalysisController) {
    activeAnalysisController.abort();
    activeAnalysisController = null;
  }

  const query = String(drug || '').trim();
  if (!query) {
    showState('stateError', { errorKey: 'errorDrugRequired' });
    return;
  }

  const controller = new AbortController();
  activeAnalysisController = controller;
  const timeout = window.setTimeout(() => controller.abort(), 30000);
  showState('stateLoading', { drug: query });

  try {
    const response = await fetch(`/api/v1/analyze?drug=${encodeURIComponent(query)}`, {
      signal: controller.signal,
      headers: { Accept: 'application/json' }
    });
    const payload = await response.json().catch(() => null);
    if (requestSequence !== analysisSequence) return;

    if (!response.ok) {
      const errorKeyByCode = {
        drug_required: 'errorDrugRequired',
        invalid_drug: 'errorInvalidDrug',
        analysis_busy: 'errorBusy',
        analysis_rate_limited: 'errorRateLimited',
        analysis_unavailable: 'errorUnavailable'
      };
      showState('stateError', { errorKey: errorKeyByCode[payload?.code] || 'errorUnexpected' });
      return;
    }

    currentAnalysis = payload;
    renderResults(payload);
  } catch (error) {
    if (requestSequence !== analysisSequence) return;
    showState('stateError', { errorKey: error.name === 'AbortError' ? 'errorTimeout' : 'errorUnexpected' });
  } finally {
    window.clearTimeout(timeout);
    if (activeAnalysisController === controller) activeAnalysisController = null;
  }
}

function renderResults(data) {
  // `currentAnalysis` may intentionally survive a later loading/error state so
  // language changes can redraw only a confirmed result. The explicit view
  // state, rather than payload presence, decides whether it is visible.
  currentAnalysis = data;
  stateKey = 'resultsReady';
  stateParams = {};
  const stateBox = document.getElementById('stateBox');
  const resultsContainer = document.getElementById('resultsContainer');
  stateBox.style.display = 'none';
  resultsContainer.style.display = 'block';

  document.getElementById('statDrugName').textContent = data.normalized_drug || data.query_drug || '—';
  document.getElementById('statTotalReports').textContent = formatNumber(data.drug_total_reports);
  document.getElementById('statActiveSignals').textContent = formatNumber(data.active_signals_count);
  document.getElementById('statUniverseN').textContent = formatNumber(data.database_universe_n);
  document.getElementById('statReactionsAnalyzed').textContent = formatNumber(data.total_reactions_analyzed);

  const activeCard = document.getElementById('activeSignalsCard');
  activeCard.classList.toggle('active-alert', Number(data.active_signals_count) > 0);

  drawVolcanoPlot(data.signals || []);
  renderTable(data.signals || []);
}

function renderTable(signals) {
  const tbody = document.getElementById('signalsTableBody');
  tbody.replaceChildren();

  if (!signals.length) {
    const row = document.createElement('tr');
    const cell = document.createElement('td');
    cell.colSpan = 6;
    cell.className = 'state-box empty-table';
    cell.textContent = t('noData');
    row.appendChild(cell);
    tbody.appendChild(row);
    return;
  }

  const sorted = [...signals].sort((left, right) => {
    if (left.signal_level === 'ACTIVE_SIGNAL' && right.signal_level !== 'ACTIVE_SIGNAL') return -1;
    if (right.signal_level === 'ACTIVE_SIGNAL' && left.signal_level !== 'ACTIVE_SIGNAL') return 1;
    return Number(right.prr || 0) - Number(left.prr || 0);
  });

  sorted.forEach((signal) => {
    const row = document.createElement('tr');
    row.className = 'table-row-item';

    const reactionCell = document.createElement('td');
    const reactionName = document.createElement('strong');
    reactionName.textContent = signal.reaction || '—';
    const toggleButton = document.createElement('button');
    toggleButton.type = 'button';
    toggleButton.className = 'detail-toggle';
    toggleButton.textContent = t('inspectMatrix');
    toggleButton.setAttribute('aria-expanded', 'false');
    toggleButton.addEventListener('click', () => toggleDetailRow(row, signal, toggleButton));
    reactionCell.append(reactionName, toggleButton);
    row.appendChild(reactionCell);

    row.appendChild(numberCell(formatNumber(signal.count_a)));
    row.appendChild(intervalCell(formatFixed(signal.prr, 2), signal.prr_lower_95, signal.prr_upper_95));
    row.appendChild(intervalCell(formatFixed(signal.ror, 2), signal.ror_lower_95, signal.ror_upper_95));
    row.appendChild(numberCell(formatFixed(signal.chi_square_yates, 1)));

    const statusCell = document.createElement('td');
    const badge = document.createElement('span');
    const status = signalPresentation(signal.signal_level);
    badge.className = `badge ${status.className}`;
    badge.textContent = t(status.labelKey);
    statusCell.appendChild(badge);
    row.appendChild(statusCell);

    tbody.appendChild(row);
  });
}

function numberCell(value) {
  const cell = document.createElement('td');
  cell.className = 'mono';
  cell.textContent = value;
  return cell;
}

function intervalCell(value, lower, upper) {
  const cell = document.createElement('td');
  const estimate = document.createElement('span');
  estimate.className = 'mono metric-estimate';
  estimate.textContent = value;
  const interval = document.createElement('div');
  interval.className = 'metric-interval';
  interval.textContent = `[${formatFixed(lower, 2)} – ${formatFixed(upper, 2)}]`;
  cell.append(estimate, interval);
  return cell;
}

function signalPresentation(level) {
  if (level === 'ACTIVE_SIGNAL') return { className: 'badge-active', labelKey: 'signalActive', interpretationKey: 'interpretationActive' };
  if (level === 'POTENTIAL_SIGNAL') return { className: 'badge-potential', labelKey: 'signalPotential', interpretationKey: 'interpretationPotential' };
  return { className: 'badge-none', labelKey: 'signalBaseline', interpretationKey: 'interpretationNone' };
}

function toggleDetailRow(parentRow, signal, toggleButton) {
  const existingDetail = parentRow.nextElementSibling;
  if (existingDetail?.classList.contains('detail-row')) {
    existingDetail.remove();
    toggleButton.setAttribute('aria-expanded', 'false');
    toggleButton.textContent = t('inspectMatrix');
    return;
  }

  document.querySelectorAll('.detail-row').forEach((row) => row.remove());
  document.querySelectorAll('.detail-toggle[aria-expanded="true"]').forEach((button) => {
    button.setAttribute('aria-expanded', 'false');
    button.textContent = t('inspectMatrix');
  });

  const b = Number(signal.drug_total) - Number(signal.count_a);
  const c = Number(signal.reaction_total) - Number(signal.count_a);
  const d = Number(currentAnalysis.database_universe_n) - (Number(signal.count_a) + b + c);

  const detailRow = document.createElement('tr');
  detailRow.className = 'detail-row';
  const detailCell = document.createElement('td');
  detailCell.colSpan = 6;
  detailCell.className = 'detail-cell';

  const heading = document.createElement('div');
  heading.className = 'detail-heading';
  heading.textContent = t('matrixHeading', {
    drug: currentAnalysis.normalized_drug || currentAnalysis.query_drug || '—',
    reaction: signal.reaction || '—'
  });

  const matrix = document.createElement('div');
  matrix.className = 'matrix-grid';
  [
    [t('matrixA'), signal.count_a],
    [t('matrixB'), b],
    [t('matrixDrugTotal'), signal.drug_total],
    [t('matrixC'), c],
    [t('matrixD'), d],
    [t('matrixUniverse'), currentAnalysis.database_universe_n]
  ].forEach(([label, value]) => matrix.appendChild(matrixCell(label, value)));

  const interpretation = document.createElement('div');
  interpretation.className = 'detail-interpretation';
  const interpretationLabel = document.createElement('strong');
  interpretationLabel.textContent = `${t('statisticalInterpretation')} `;
  interpretation.append(interpretationLabel, document.createTextNode(t(signalPresentation(signal.signal_level).interpretationKey)));

  detailCell.append(heading, matrix, interpretation);
  detailRow.appendChild(detailCell);
  parentRow.after(detailRow);
  toggleButton.setAttribute('aria-expanded', 'true');
  toggleButton.textContent = t('collapseMatrix');
}

function matrixCell(label, value) {
  const cell = document.createElement('div');
  cell.className = 'matrix-cell';
  const labelNode = document.createElement('span');
  labelNode.textContent = label;
  const valueNode = document.createElement('strong');
  valueNode.textContent = formatNumber(value);
  cell.append(labelNode, valueNode);
  return cell;
}

function drawVolcanoPlot(signals) {
  const canvas = document.getElementById('volcanoCanvas');
  if (!canvas) return;

  const context = canvas.getContext('2d');
  const devicePixelRatio = window.devicePixelRatio || 1;
  const rect = canvas.getBoundingClientRect();
  const width = Math.max(1, rect.width);
  const height = Math.max(1, rect.height);
  canvas.width = Math.round(width * devicePixelRatio);
  canvas.height = Math.round(height * devicePixelRatio);
  context.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);
  context.clearRect(0, 0, width, height);
  plotPoints = [];

  if (!signals.length) {
    context.fillStyle = '#8fa0b5';
    context.font = '12px sans-serif';
    context.textAlign = 'center';
    context.fillText(t('plotNoData'), width / 2, height / 2);
    return;
  }

  // A volcano plot is defined over log2(PRR), including protective/underreported
  // associations below zero. Clamping negative values to zero hides that half of
  // the distribution and falsely depicts the horizontal axis as log2(PRR).
  const logValues = signals.map((signal) => (Number(signal.prr) > 0 ? Math.log2(Number(signal.prr)) : 0));
  let minLogPRR = Math.min(-1, ...logValues);
  let maxLogPRR = Math.max(2, ...logValues);
  minLogPRR = Math.floor(minLogPRR) - 0.5;
  maxLogPRR = Math.ceil(maxLogPRR) + 0.5;
  const maxSqrtChi = Math.max(10, ...signals.map((signal) => Math.sqrt(Math.max(0, Number(signal.chi_square_yates) || 0)))) + 2;
  const padding = { top: 20, right: 30, bottom: 40, left: 45 };
  const plotWidth = width - padding.left - padding.right;
  const plotHeight = height - padding.top - padding.bottom;
  const logRange = maxLogPRR - minLogPRR || 1;
  const getX = (logPRR) => padding.left + ((Math.min(maxLogPRR, Math.max(minLogPRR, logPRR)) - minLogPRR) / logRange) * plotWidth;
  const getY = (sqrtChi) => height - padding.bottom - (Math.min(sqrtChi, maxSqrtChi) / maxSqrtChi) * plotHeight;

  context.lineWidth = 1;
  context.setLineDash([4, 4]);
  context.strokeStyle = 'rgba(100, 116, 139, 0.45)';
  const neutralX = getX(0);
  context.beginPath();
  context.moveTo(neutralX, padding.top);
  context.lineTo(neutralX, height - padding.bottom);
  context.stroke();
  context.strokeStyle = 'rgba(245, 158, 11, 0.5)';
  const thresholdX = getX(1);
  context.beginPath();
  context.moveTo(thresholdX, padding.top);
  context.lineTo(thresholdX, height - padding.bottom);
  context.stroke();
  const thresholdY = getY(2);
  context.beginPath();
  context.moveTo(padding.left, thresholdY);
  context.lineTo(width - padding.right, thresholdY);
  context.stroke();
  context.setLineDash([]);

  context.fillStyle = '#8fa0b5';
  context.font = '10px ui-monospace, monospace';
  context.textAlign = 'center';
  context.fillText(t('plotXAxis'), padding.left + plotWidth / 2, height - 10);
  context.save();
  context.translate(12, padding.top + plotHeight / 2);
  context.rotate(-Math.PI / 2);
  context.fillText(t('plotYAxis'), 0, 0);
  context.restore();

  signals.forEach((signal) => {
    const logPRR = Number(signal.prr) > 0 ? Math.log2(Number(signal.prr)) : minLogPRR;
    const sqrtChi = Math.sqrt(Math.max(0, Number(signal.chi_square_yates) || 0));
    const presentation = signalPresentation(signal.signal_level);
    const colorByClass = {
      'badge-active': '#ef4444',
      'badge-potential': '#f59e0b',
      'badge-none': '#10b981'
    };
    const color = colorByClass[presentation.className];
    const x = getX(logPRR);
    const y = getY(sqrtChi);

    context.beginPath();
    context.arc(x, y, signal.signal_level === 'ACTIVE_SIGNAL' ? 5 : 3.5, 0, Math.PI * 2);
    context.fillStyle = color;
    context.shadowColor = color;
    context.shadowBlur = signal.signal_level === 'ACTIVE_SIGNAL' ? 8 : 0;
    context.fill();
    context.shadowBlur = 0;
    plotPoints.push({ x, y, signal });
  });
}

function handleCanvasHover(event) {
  const canvas = document.getElementById('volcanoCanvas');
  const tooltip = document.getElementById('plotTooltip');
  if (!canvas || !tooltip || !plotPoints.length) return;

  const rect = canvas.getBoundingClientRect();
  const mouseX = event.clientX - rect.left;
  const mouseY = event.clientY - rect.top;
  let nearest = null;
  let minDistance = 12;
  plotPoints.forEach((point) => {
    const distance = Math.hypot(point.x - mouseX, point.y - mouseY);
    if (distance < minDistance) {
      minDistance = distance;
      nearest = point;
    }
  });

  if (!nearest) {
    hidePlotTooltip();
    return;
  }

  const signal = nearest.signal;
  tooltip.replaceChildren();
  const heading = document.createElement('div');
  heading.className = 'tooltip-heading';
  heading.textContent = signal.reaction || '—';
  const metrics = document.createElement('div');
  metrics.className = 'tooltip-metrics';
  metrics.textContent = `${t('tooltipPRR')}: ${formatFixed(signal.prr, 2)} | ${t('tooltipROR')}: ${formatFixed(signal.ror, 2)} | ${t('tooltipChi')}: ${formatFixed(signal.chi_square_yates, 1)}`;
  const reports = document.createElement('div');
  reports.className = 'tooltip-reports';
  reports.textContent = `${t('tooltipReports')}: ${formatNumber(signal.count_a)}`;
  tooltip.append(heading, metrics, reports);
  tooltip.style.display = 'block';
  tooltip.style.left = `${event.clientX + 12}px`;
  tooltip.style.top = `${event.clientY - 20}px`;
  tooltip.setAttribute('aria-hidden', 'false');
}

function hidePlotTooltip() {
  const tooltip = document.getElementById('plotTooltip');
  if (!tooltip) return;
  tooltip.style.display = 'none';
  tooltip.setAttribute('aria-hidden', 'true');
}

function formatNumber(value) {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) return '—';
  return new Intl.NumberFormat(localeByLanguage[currentLanguage]).format(Number(value));
}

function formatFixed(value, digits) {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) return '—';
  return new Intl.NumberFormat(localeByLanguage[currentLanguage], {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits
  }).format(Number(value));
}
