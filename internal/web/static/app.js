"use strict";

const SVG_NS = "http://www.w3.org/2000/svg";
// The API and export remain complete. These limits only bound browser DOM/SVG
// work so a valid 50,000-row analysis cannot lock the page or assistive tech.
const RESEARCH_PAGE_SIZE = 50;
const RESEARCH_FOREST_LIMIT = 30;
const state = {
  locale: localStorage.getItem("pv_locale") || "pt-BR",
  mode: localStorage.getItem("pv_view_mode") || "guided",
  theme: localStorage.getItem("pv_theme") || "light",
  chart: "map",
  analysis: null,
  selectedSignal: null,
  datasets: [],
  researchEnabled: false,
  researchResult: null,
  researchRowsOrdered: [],
  researchPage: 1,
  // Dynamic status is stored as a translation key plus raw parameters so a
  // locale change can re-render in place instead of preserving stale text.
  liveStatus: null
};

// API error codes are stable machine identifiers, not user-facing prose. Keep
// the mapping client-side so the same failure can be re-rendered after a locale
// change while retaining the technical code and any useful retry information.
const LIVE_ERROR_MESSAGE_KEYS = Object.freeze({
  drug_required: "liveErrorDrugRequired",
  invalid_drug: "liveErrorInvalidDrug",
  analysis_busy: "liveErrorBusy",
  analysis_rate_limited: "liveErrorRateLimited",
  analysis_unavailable: "liveErrorUnavailable",
  method_not_allowed: "liveErrorMethodNotAllowed",
  network_error: "liveErrorNetwork"
});

const messages = {
  "pt-BR": {
    skip: "Pular para o conteúdo", brandSubtitle: "Pesquisa reprodutível em farmacovigilância",
    mainNav: "Navegação principal", navAnalyze: "Analisar", navMethods: "Métodos", navResearch: "Pesquisa",
    viewMode: "Modo", guided: "Guiado", advanced: "Avançado", language: "Idioma", toggleTheme: "Alternar tema",
    liveTitle: "Exploração ao vivo — não citável como resultado de pesquisa",
    liveBody: "A API openFDA muda ao longo do tempo, não deduplica casos, não filtra o papel do medicamento e avalia somente os termos mais reportados. Use esta tela para aprender e formular hipóteses; um snapshot v2 registrado e com integridade verificada ainda exige matriz de validação e revisão científica independente para uso formal.",
    eyebrow: "FAERS / openFDA · triagem de relatos",
    heroTitle: "Explore desproporcionalidade sem confundir associação com causalidade",
    heroBody: "Consulte uma substância, examine a tabela 2 × 2 e revise PRR, ROR e seus intervalos. O resultado é um SDR para revisão, não uma conclusão clínica.",
    drugLabel: "Substância ou medicamento", drugPlaceholder: "Ex.: Semaglutide", analyzeButton: "Analisar", examples: "Exemplos:",
    currentAnalysis: "Análise atual", interpretationBoundary: "Limite de interpretação",
    interpretationText: "Desproporcionalidade mede frequência relativa de reporte. Não estima risco, incidência, prevalência nem causalidade.",
    sourceDocumentation: "Documentação da fonte", drugReports: "Relatos que mencionam a substância",
    notExposure: "Não é número de pacientes expostos", sdrReview: "SDRs para revisão",
    evansProfile: "Perfil didático de Evans; não regulatório", termsEvaluated: "Termos avaliados",
    universe: "Universo openFDA atual", mutableUniverse: "Fonte mutável e sem snapshot nesta tela",
    guidedQuestion: "Pergunta correta para esta tela",
    guidedQuestionBody: "“Este par medicamento–evento é reportado proporcionalmente mais vezes do que o mesmo evento com outros medicamentos?” A tela não responde “o medicamento causa o evento?”.",
    visualAnalysis: "Análise visual", mapTitle: "Mapa de desproporcionalidade", map: "Mapa", forest: "Forest plot",
    chartType: "Tipo de gráfico", meetsProfile: "Atende ao perfil", reviewPriority: "Prioridade intermediária",
    belowProfile: "Abaixo do perfil", selectedPair: "Par selecionado",
    matrixCaption: "Tabela de contingência 2 × 2, em relatos", targetEvent: "Evento-alvo (E)",
    otherEvents: "Outros eventos (não E)", total: "Total", targetDrug: "Substância-alvo (D)",
    otherDrugs: "Outras substâncias (não D)", expected: "Esperado",
    matrixHelp: "A célula “a” contém relatos em que a substância e o evento aparecem no mesmo relato; isso não prova que estejam causalmente ligados.",
    tableEquivalent: "Equivalente textual dos gráficos", resultsTable: "Resultados por termo MedDRA PT",
    filter: "Filtro", all: "Todos", sort: "Ordenar", observedDesc: "Observados ↓", termAsc: "Termo A–Z",
    reaction: "Evento reportado", observed: "Observado (a)", screening: "Triagem",
    methodology: "Metodologia", methodsTitle: "Do relato espontâneo ao SDR: pressupostos visíveis",
    methodsLead: "O sistema usa desproporcionalidade para priorizar pares que exigem revisão. Detecção estatística, validação clínica, confirmação e avaliação são etapas diferentes.",
    evansHeading: "Perfil didático de Evans",
    evansBody: "O rótulo atende ao perfil quando a ≥ 3, PRR ≥ 2 e χ² de Yates ≥ 4. É uma heurística histórica configurada para ensino, não um limiar universal da EMA.",
    zeroCorrection: "Quando existe célula zero, a API informa a correção de Haldane–Anscombe aplicada às estimativas logarítmicas.",
    failureModes: "Modos de falha", failureDuplicate: "Duplicatas e versões de caso podem inflar contagens.",
    failureStimulated: "Notoriedade, mídia e tempo de mercado alteram o reporte.",
    failureCooccurrence: "Vários medicamentos e eventos no mesmo caso impedem atribuição individual pela API pública.",
    failureDrugRole: "A consulta ao vivo não filtra drugcharacterization; papéis de suspeito, concomitante e interagente ficam combinados.",
    failureSelection: "A tela ao vivo seleciona termos frequentes e pode omitir eventos raros importantes.",
    failureDenominator: "Sem exposição populacional, não há cálculo de incidência.", primarySources: "Fontes primárias e reporte",
    researchWorkspace: "Espaço de pesquisa", researchTitle: "Snapshots, protocolos e evidência reproduzível",
    researchLead: "O catálogo confirma registro e integridade estrutural do snapshot; uso formal ainda exige a matriz de validação e aprovação científica independente.",
    protocolEyebrow: "Protocolo determinístico", protocolTitle: "Configurar análise sobre snapshot congelado",
    researchFormBoundary: "A configuração integral participa do hash da análise. Repetir o mesmo protocolo sobre o mesmo manifesto produz o mesmo analysis_id.",
    datasetLabel: "Dataset registrado", datasetSelectPlaceholder: "Carregando snapshots registrados…",
    datasetHelp: "Somente manifests aceitos pelo registro do servidor são listados.",
    drugConceptLabel: "Identificador explícito do medicamento",
    drugConceptHelp: "Use faers-prod_ai:<texto exato> ou faers-drugname:<texto exato>. Isto não inventa RxNorm, DCB ou ATC.",
    drugRoleLabel: "Papel do medicamento", rolePrimary: "Suspeito primário (PS)", roleSecondary: "Suspeito secundário (SS)",
    roleConcomitant: "Concomitante (C)", roleInteracting: "Interagente (I)", roleSuspect: "Suspeitos agregados (PS + SS)", roleAll: "Todos os papéis elegíveis",
    thresholdLabel: "Perfil de triagem", thresholdNone: "Nenhum threshold", thresholdEvans: "Evans didático",
    guidedProtocolTitle: "Leitura guiada",
    guidedProtocolBody: "O protocolo usa todos os PTs preservados na fonte, compara com os demais relatos elegíveis e executa PRR, ROR e Fisher. O perfil Evans apenas adiciona flags de revisão; não remove linhas nem confirma causalidade.",
    methodsLabel: "Métodos executados", methodsHelp: "As escolhas alteram o protocolo e, portanto, o analysis_id; não alteram contagens a, b, c, d.",
    eventScopeLabel: "Escopo de eventos", comparatorLabel: "Comparator", periodLabel: "Período e strata", periodUnavailable: "Não disponíveis neste agregado",
    unavailableTitle: "Não simulados nesta interface",
    unavailableBody: "BCPNN/IC, GPS/EBGM, análises temporais e estratificadas exigem artefato batch validado compatível. A interface não substitui esses métodos por aproximações.",
    runResearch: "Executar análise reproduzível", datasetCatalogTitle: "Snapshots disponíveis",
    datasetCatalogBody: "Confirme cobertura, geografia e hash do manifesto antes de interpretar qualquer resultado.",
    researchResultEyebrow: "Resultado determinístico", researchResultTitle: "Análise reproduzível", exportBundle: "Exportar bundle ZIP",
    analysisIDLabel: "analysis_id", resultDigestLabel: "result_digest (linhas emitidas)", resultDatasetLabel: "Dataset e manifesto", resultCoverageLabel: "Cobertura", resultRowsLabel: "row_count (pares emitidos)",
    caveatsTitle: "Limitações vinculadas ao resultado", caveatsExact: "Os textos abaixo são preservados exatamente como retornados pelo artefato científico.",
    researchVisualEyebrow: "Visualização acessível", researchForestTitle: "Forest plot de PRR em escala logarítmica",
    researchForestDescription: "O forest mostra um subconjunto visual declarado da página atual; a tabela paginada e o bundle preservam o resultado completo.",
    researchTableEquivalent: "Equivalente textual paginado do resultado completo", researchTableTitle: "Contagens e estimativas por evento",
    researchTableHelp: "a, b, c, d e N são contagens de relatos no universo documentado; p e q ausentes são exibidos como “—”, nunca como zero. As linhas são ordenadas por PRR decrescente, com estimativas ausentes ao final e desempate estável pelo texto do evento.",
    researchPaginationLabel: "Paginação dos resultados", researchPreviousPage: "Página anterior", researchNextPage: "Próxima página",
    researchPaginationSummary: "Mostrando {start}–{end} de {total} pares (página {page} de {pages}; até {pageSize} por página). O bundle exportado preserva todos os {total} pares.",
    researchPageStatus: "Página {page} de {pages}", researchTableRegion: "Tabela paginada de resultados da pesquisa",
    researchTableCaption: "Resultado completo apresentado em páginas, ordenado por PRR decrescente.",
    researchForestSelection: "Seleção visual: até {limit} PRRs positivos da página atual, após a ordenação da tabela. Exibidos {shown}; {unplottable} sem estimativa e intervalo positivos e {deferred} elegíveis além do limite permanecem nesta página da tabela; {outside} pares estão em outras páginas. O eixo inclui integralmente os intervalos dos pares exibidos, sem clipping.",
    researchEvent: "Evento / categoria", researchFlags: "Flags de revisão",
    datasetNoSelection: "Nenhum snapshot registrado disponível", researchRunning: "Executando protocolo determinístico…",
    researchUnavailable: "Snapshots catalogados, mas o engine de análise v2 não está habilitado neste servidor.",
    researchError: "A análise v2 não pôde ser concluída: {message}", conceptInvalid: "Use um identificador faers-prod_ai: ou faers-drugname: com texto não vazio.",
    methodsRequired: "Selecione ao menos um método implementado.", responseInvalid: "A API retornou um resultado sem identidade, manifesto ou integridade de linhas válidos.",
    noResearchRows: "Nenhum par retornado para este protocolo.", noCaveats: "Nenhuma limitação foi retornada; o resultado não deve ser tratado como validado.",
    noFlags: "Sem flag", forestResearchAria: "Forest plot de PRR com {shown} pares da página {page}; a análise completa contém {total} pares. Consulte a descrição e a tabela paginada.",
    forestOmitted: "{count} pares não representáveis na escala logarítmica permanecem na tabela.",
    forestResearchEmpty: "Nenhum PRR com estimativa e intervalo positivos está disponível nesta página; consulte a tabela ou avance para outra página.", manifestLabel: "manifesto",
    students: "Estudantes", studentsBody: "Reproduza a matriz 2 × 2, compare métodos e entregue configuração e limitações junto ao trabalho.",
    researchers: "Pesquisadores", researchersBody: "Pré-especifique snapshot, comparator, papel do medicamento, período, strata e thresholds; exporte o bundle completo.",
    professionals: "Profissionais", professionalsBody: "Use SDRs para triagem humana e combine-os com revisão de casos, literatura e outras fontes de evidência.",
    formalUseChecklist: "Checklist mínimo para uso formal", formal1: "Citar versão do software, commit e dataset_id.",
    formal2: "Publicar protocolo e parâmetros antes da interpretação.",
    formal3: "Guardar manifestos, checksums, resultados e ambiente de execução.",
    formal4: "Relatar vieses, dados ausentes e análises de sensibilidade.",
    formal5: "Não converter SDR em causalidade, incidência ou recomendação clínica.",
    openIssue: "Abrir issue no GitHub",
    footer: "PV Signal Radar · software MIT · dados sujeitos às licenças das fontes · não é aconselhamento médico",
    loading: "Consultando a exploração ao vivo do openFDA para “{drug}”…",
    error: "Não foi possível concluir a exploração: {message}", genericError: "requisição indisponível",
    liveErrorDrugRequired: "Informe um medicamento para iniciar a consulta.", liveErrorInvalidDrug: "A consulta é inválida; remova quebras de linha e use no máximo 120 caracteres.",
    liveErrorBusy: "O limite de consultas simultâneas foi atingido.", liveErrorRateLimited: "O limite temporário de consultas foi atingido.",
    liveErrorUnavailable: "A fonte openFDA não conseguiu concluir a consulta agora.", liveErrorMethodNotAllowed: "Este endpoint não aceita o método HTTP usado.",
    liveErrorNetwork: "Não foi possível alcançar o serviço openFDA.", liveErrorHTTPStatus: "O servidor recusou a consulta (HTTP {status}).",
    liveErrorRetry: "Tente novamente em cerca de {seconds} s.", liveErrorCode: "Código técnico: {code}.", liveErrorDetail: "Detalhe: {detail}.",
    noResults: "Nenhum termo retornado para esta consulta.", topTerms: "Somente os {count} termos mais reportados; cobertura incompleta",
    sourceLive: "openFDA ao vivo · exploratório", mapDescription: "Cada ponto é um termo: eixo x = log₂(PRR), eixo y = log₁₀(relatos observados). A cor indica o perfil didático completo, que também considera χ².",
    forestDescription: "PRR em escala logarítmica com IC 95%; a linha vertical em 1 representa ausência de desproporcionalidade. Todos os termos retornados são exibidos.",
    mapAria: "Mapa de desproporcionalidade com {count} termos. Consulte a tabela abaixo para valores exatos.",
    forestAria: "Forest plot de PRR com {count} termos e intervalos de confiança. Consulte a tabela abaixo para valores exatos.",
    chartEmpty: "Sem dados para o gráfico.", datasetLoading: "Carregando catálogo de datasets…",
    datasetEmptyTitle: "Nenhum snapshot liberado para análise formal",
    datasetEmptyBody: "O contrato v2 está disponível, mas a análise permanece bloqueada até um snapshot FAERS oficial passar pela matriz de validação.",
    datasetError: "Catálogo indisponível. A análise formal continua bloqueada por segurança.",
    datasetRegistered: "registrado · integridade verificada", datasetPending: "não liberado", coverage: "Cobertura", retrieved: "Obtido em",
    profileActive: "Atende ao perfil", profilePotential: "Revisão intermediária", profileNone: "Abaixo do perfil",
    chartSelected: "Selecionado: {term}. PRR {prr}; intervalo de {lower} a {upper}.",
    title: "PV Signal Radar — triagem exploratória de farmacovigilância"
  },
  es: {
    skip: "Saltar al contenido", brandSubtitle: "Investigación reproducible en farmacovigilancia",
    mainNav: "Navegación principal", navAnalyze: "Analizar", navMethods: "Métodos", navResearch: "Investigación",
    viewMode: "Modo", guided: "Guiado", advanced: "Avanzado", language: "Idioma", toggleTheme: "Cambiar tema",
    liveTitle: "Exploración en vivo — no citable como resultado de investigación",
    liveBody: "La API openFDA cambia con el tiempo, aquí no deduplica casos, no filtra el rol del medicamento y evalúa solo los términos más notificados. Use esta pantalla para aprender y formular hipótesis; un snapshot v2 registrado y con integridad verificada todavía requiere una matriz de validación y revisión científica independiente para uso formal.",
    eyebrow: "FAERS / openFDA · cribado de notificaciones",
    heroTitle: "Explore desproporcionalidad sin confundir asociación con causalidad",
    heroBody: "Consulte una sustancia, examine la tabla 2 × 2 y revise PRR, ROR y sus intervalos. El resultado es un SDR para revisión, no una conclusión clínica.",
    drugLabel: "Sustancia o medicamento", drugPlaceholder: "Ej.: Semaglutide", analyzeButton: "Analizar", examples: "Ejemplos:",
    currentAnalysis: "Análisis actual", interpretationBoundary: "Límite de interpretación",
    interpretationText: "La desproporcionalidad mide frecuencia relativa de notificación. No estima riesgo, incidencia, prevalencia ni causalidad.",
    sourceDocumentation: "Documentación de la fuente", drugReports: "Notificaciones que mencionan la sustancia",
    notExposure: "No es el número de pacientes expuestos", sdrReview: "SDR para revisión",
    evansProfile: "Perfil didáctico de Evans; no regulatorio", termsEvaluated: "Términos evaluados",
    universe: "Universo openFDA actual", mutableUniverse: "Fuente mutable y sin snapshot en esta pantalla",
    guidedQuestion: "Pregunta correcta para esta pantalla",
    guidedQuestionBody: "“¿Este par medicamento–evento se notifica proporcionalmente más que el mismo evento con otros medicamentos?” La pantalla no responde “¿el medicamento causa el evento?”.",
    visualAnalysis: "Análisis visual", mapTitle: "Mapa de desproporcionalidad", map: "Mapa", forest: "Forest plot",
    chartType: "Tipo de gráfico", meetsProfile: "Cumple el perfil", reviewPriority: "Prioridad intermedia",
    belowProfile: "Por debajo del perfil", selectedPair: "Par seleccionado",
    matrixCaption: "Tabla de contingencia 2 × 2, en notificaciones", targetEvent: "Evento objetivo (E)",
    otherEvents: "Otros eventos (no E)", total: "Total", targetDrug: "Sustancia objetivo (D)",
    otherDrugs: "Otras sustancias (no D)", expected: "Esperado",
    matrixHelp: "La celda “a” contiene notificaciones donde la sustancia y el evento aparecen juntos; esto no demuestra un vínculo causal.",
    tableEquivalent: "Equivalente textual de los gráficos", resultsTable: "Resultados por término MedDRA PT",
    filter: "Filtro", all: "Todos", sort: "Ordenar", observedDesc: "Observados ↓", termAsc: "Término A–Z",
    reaction: "Evento notificado", observed: "Observado (a)", screening: "Cribado",
    methodology: "Metodología", methodsTitle: "De la notificación espontánea al SDR: supuestos visibles",
    methodsLead: "El sistema usa desproporcionalidad para priorizar pares que requieren revisión. Detección estadística, validación clínica, confirmación y evaluación son etapas distintas.",
    evansHeading: "Perfil didáctico de Evans",
    evansBody: "La etiqueta cumple el perfil cuando a ≥ 3, PRR ≥ 2 y χ² de Yates ≥ 4. Es una heurística histórica para enseñanza, no un umbral universal de la EMA.",
    zeroCorrection: "Si hay una celda cero, la API informa la corrección de Haldane–Anscombe aplicada a las estimaciones logarítmicas.",
    failureModes: "Modos de fallo", failureDuplicate: "Duplicados y versiones de casos pueden inflar recuentos.",
    failureStimulated: "Notoriedad, medios y tiempo en el mercado alteran la notificación.",
    failureCooccurrence: "Varios medicamentos y eventos en un caso impiden atribución individual por la API pública.",
    failureDrugRole: "La consulta en vivo no filtra drugcharacterization; combina los roles sospechoso, concomitante e interacción.",
    failureSelection: "La pantalla en vivo selecciona términos frecuentes y puede omitir eventos raros importantes.",
    failureDenominator: "Sin exposición poblacional no se puede calcular incidencia.", primarySources: "Fuentes primarias y reporte",
    researchWorkspace: "Espacio de investigación", researchTitle: "Snapshots, protocolos y evidencia reproducible",
    researchLead: "El catálogo confirma el registro y la integridad estructural del snapshot; el uso formal todavía requiere la matriz de validación y una revisión científica independiente.",
    protocolEyebrow: "Protocolo determinista", protocolTitle: "Configurar análisis sobre un snapshot congelado",
    researchFormBoundary: "La configuración completa participa en el hash del análisis. Repetir el mismo protocolo sobre el mismo manifiesto produce el mismo analysis_id.",
    datasetLabel: "Dataset registrado", datasetSelectPlaceholder: "Cargando snapshots registrados…",
    datasetHelp: "Solo se listan manifests aceptados por el registro del servidor.",
    drugConceptLabel: "Identificador explícito del medicamento",
    drugConceptHelp: "Use faers-prod_ai:<texto exacto> o faers-drugname:<texto exacto>. Esto no inventa RxNorm, DCB ni ATC.",
    drugRoleLabel: "Rol del medicamento", rolePrimary: "Sospechoso primario (PS)", roleSecondary: "Sospechoso secundario (SS)",
    roleConcomitant: "Concomitante (C)", roleInteracting: "Interacción (I)", roleSuspect: "Sospechosos agregados (PS + SS)", roleAll: "Todos los roles elegibles",
    thresholdLabel: "Perfil de cribado", thresholdNone: "Sin threshold", thresholdEvans: "Evans didáctico",
    guidedProtocolTitle: "Lectura guiada",
    guidedProtocolBody: "El protocolo usa todos los PT preservados en la fuente, compara con las demás notificaciones elegibles y ejecuta PRR, ROR y Fisher. El perfil Evans solo añade flags de revisión; no elimina filas ni confirma causalidad.",
    methodsLabel: "Métodos ejecutados", methodsHelp: "Las elecciones cambian el protocolo y, por tanto, el analysis_id; no cambian los recuentos a, b, c, d.",
    eventScopeLabel: "Alcance de eventos", comparatorLabel: "Comparador", periodLabel: "Período y estratos", periodUnavailable: "No disponibles en este agregado",
    unavailableTitle: "No simulados en esta interfaz",
    unavailableBody: "BCPNN/IC, GPS/EBGM y los análisis temporales y estratificados requieren un artefacto batch validado compatible. La interfaz no sustituye estos métodos por aproximaciones.",
    runResearch: "Ejecutar análisis reproducible", datasetCatalogTitle: "Snapshots disponibles",
    datasetCatalogBody: "Confirme cobertura, geografía y hash del manifiesto antes de interpretar resultados.",
    researchResultEyebrow: "Resultado determinista", researchResultTitle: "Análisis reproducible", exportBundle: "Exportar bundle ZIP",
    analysisIDLabel: "analysis_id", resultDigestLabel: "result_digest (filas emitidas)", resultDatasetLabel: "Dataset y manifiesto", resultCoverageLabel: "Cobertura", resultRowsLabel: "row_count (pares emitidos)",
    caveatsTitle: "Limitaciones vinculadas al resultado", caveatsExact: "Los textos siguientes se preservan exactamente como los devuelve el artefacto científico.",
    researchVisualEyebrow: "Visualización accesible", researchForestTitle: "Forest plot de PRR en escala logarítmica",
    researchForestDescription: "El forest muestra un subconjunto visual declarado de la página actual; la tabla paginada y el bundle preservan el resultado completo.",
    researchTableEquivalent: "Equivalente textual paginado del resultado completo", researchTableTitle: "Recuentos y estimaciones por evento",
    researchTableHelp: "a, b, c, d y N son recuentos de notificaciones en el universo documentado; p y q ausentes se muestran como “—”, nunca como cero. Las filas se ordenan por PRR descendente, con estimaciones ausentes al final y desempate estable por el texto del evento.",
    researchPaginationLabel: "Paginación de resultados", researchPreviousPage: "Página anterior", researchNextPage: "Página siguiente",
    researchPaginationSummary: "Mostrando {start}–{end} de {total} pares (página {page} de {pages}; hasta {pageSize} por página). El bundle exportado preserva los {total} pares.",
    researchPageStatus: "Página {page} de {pages}", researchTableRegion: "Tabla paginada de resultados de investigación",
    researchTableCaption: "Resultado completo presentado en páginas, ordenado por PRR descendente.",
    researchForestSelection: "Selección visual: hasta {limit} PRR positivos de la página actual, después de ordenar la tabla. Se muestran {shown}; {unplottable} sin estimación e intervalo positivos y {deferred} elegibles después del límite permanecen en esta página de la tabla; {outside} pares están en otras páginas. El eje incluye íntegramente los intervalos de los pares mostrados, sin clipping.",
    researchEvent: "Evento / categoría", researchFlags: "Flags de revisión",
    datasetNoSelection: "No hay snapshots registrados disponibles", researchRunning: "Ejecutando protocolo determinista…",
    researchUnavailable: "Hay snapshots catalogados, pero el motor de análisis v2 no está habilitado en este servidor.",
    researchError: "No se pudo completar el análisis v2: {message}", conceptInvalid: "Use un identificador faers-prod_ai: o faers-drugname: con texto no vacío.",
    methodsRequired: "Seleccione al menos un método implementado.", responseInvalid: "La API devolvió un resultado sin identidad, manifiesto o integridad de filas válidos.",
    noResearchRows: "Este protocolo no devolvió pares.", noCaveats: "No se devolvieron limitaciones; el resultado no debe tratarse como validado.",
    noFlags: "Sin flag", forestResearchAria: "Forest plot de PRR con {shown} pares de la página {page}; el análisis completo contiene {total} pares. Consulte la descripción y la tabla paginada.",
    forestOmitted: "{count} pares no representables en escala logarítmica permanecen en la tabla.",
    forestResearchEmpty: "No hay PRR con estimación e intervalo positivos en esta página; consulte la tabla o avance a otra página.", manifestLabel: "manifiesto",
    students: "Estudiantes", studentsBody: "Reproduzca la matriz 2 × 2, compare métodos y entregue configuración y limitaciones con el trabajo.",
    researchers: "Investigadores", researchersBody: "Preespecifique snapshot, comparador, rol del medicamento, período, estratos y umbrales; exporte el bundle completo.",
    professionals: "Profesionales", professionalsBody: "Use SDR para cribado humano y combínelos con revisión de casos, literatura y otras fuentes.",
    formalUseChecklist: "Checklist mínimo para uso formal", formal1: "Citar versión del software, commit y dataset_id.",
    formal2: "Publicar protocolo y parámetros antes de interpretar.",
    formal3: "Conservar manifiestos, checksums, resultados y entorno.",
    formal4: "Informar sesgos, datos faltantes y análisis de sensibilidad.",
    formal5: "No convertir SDR en causalidad, incidencia o recomendación clínica.",
    openIssue: "Abrir issue en GitHub",
    footer: "PV Signal Radar · software MIT · datos sujetos a licencias de origen · no es consejo médico",
    loading: "Consultando la exploración en vivo de openFDA para “{drug}”…",
    error: "No fue posible completar la exploración: {message}", genericError: "solicitud no disponible",
    liveErrorDrugRequired: "Indique un medicamento para iniciar la consulta.", liveErrorInvalidDrug: "La consulta no es válida; elimine los saltos de línea y use como máximo 120 caracteres.",
    liveErrorBusy: "Se alcanzó el límite de consultas simultáneas.", liveErrorRateLimited: "Se alcanzó el límite temporal de consultas.",
    liveErrorUnavailable: "La fuente openFDA no pudo completar la consulta en este momento.", liveErrorMethodNotAllowed: "Este endpoint no acepta el método HTTP utilizado.",
    liveErrorNetwork: "No fue posible conectarse al servicio openFDA.", liveErrorHTTPStatus: "El servidor rechazó la consulta (HTTP {status}).",
    liveErrorRetry: "Vuelva a intentarlo en aproximadamente {seconds} s.", liveErrorCode: "Código técnico: {code}.", liveErrorDetail: "Detalle: {detail}.",
    noResults: "No se devolvieron términos para esta consulta.", topTerms: "Solo los {count} términos más notificados; cobertura incompleta",
    sourceLive: "openFDA en vivo · exploratorio", mapDescription: "Cada punto es un término: eje x = log₂(PRR), eje y = log₁₀(notificaciones observadas). El color indica el perfil didáctico completo, que también considera χ².",
    forestDescription: "PRR en escala logarítmica con IC 95%; la línea vertical en 1 representa ausencia de desproporcionalidad. Se muestran todos los términos devueltos.",
    mapAria: "Mapa de desproporcionalidad con {count} términos. Consulte la tabla para valores exactos.",
    forestAria: "Forest plot de PRR con {count} términos e intervalos de confianza. Consulte la tabla para valores exactos.",
    chartEmpty: "Sin datos para el gráfico.", datasetLoading: "Cargando catálogo de datasets…",
    datasetEmptyTitle: "Ningún snapshot liberado para análisis formal",
    datasetEmptyBody: "El contrato v2 está disponible, pero el análisis permanece bloqueado hasta que un snapshot FAERS oficial supere la matriz de validación.",
    datasetError: "Catálogo no disponible. El análisis formal sigue bloqueado por seguridad.",
    datasetRegistered: "registrado · integridad verificada", datasetPending: "no liberado", coverage: "Cobertura", retrieved: "Obtenido el",
    profileActive: "Cumple el perfil", profilePotential: "Revisión intermedia", profileNone: "Por debajo del perfil",
    chartSelected: "Seleccionado: {term}. PRR {prr}; intervalo de {lower} a {upper}.",
    title: "PV Signal Radar — cribado exploratorio de farmacovigilancia"
  },
  en: {
    skip: "Skip to content", brandSubtitle: "Reproducible pharmacovigilance research",
    mainNav: "Main navigation", navAnalyze: "Analyze", navMethods: "Methods", navResearch: "Research",
    viewMode: "Mode", guided: "Guided", advanced: "Advanced", language: "Language", toggleTheme: "Toggle theme",
    liveTitle: "Live exploration — not citable as a research result",
    liveBody: "The openFDA API changes over time, cases are not deduplicated, drug role is not filtered, and only the most reported terms are evaluated. Use this screen to learn and form hypotheses; a registered, integrity-checked v2 snapshot still requires the validation matrix and independent scientific review for formal use.",
    eyebrow: "FAERS / openFDA · report screening",
    heroTitle: "Explore disproportionality without confusing association with causation",
    heroBody: "Search a substance, inspect the 2 × 2 table, and review PRR, ROR, and confidence intervals. The output is an SDR for review, not a clinical conclusion.",
    drugLabel: "Substance or medicine", drugPlaceholder: "E.g., Semaglutide", analyzeButton: "Analyze", examples: "Examples:",
    currentAnalysis: "Current analysis", interpretationBoundary: "Interpretation boundary",
    interpretationText: "Disproportionality measures relative reporting frequency. It does not estimate risk, incidence, prevalence, or causality.",
    sourceDocumentation: "Source documentation", drugReports: "Reports mentioning the substance",
    notExposure: "Not a count of exposed patients", sdrReview: "SDRs for review",
    evansProfile: "Educational Evans profile; not regulatory", termsEvaluated: "Terms evaluated",
    universe: "Current openFDA universe", mutableUniverse: "Mutable source with no snapshot on this screen",
    guidedQuestion: "The right question for this screen",
    guidedQuestionBody: "“Is this drug–event pair reported proportionally more often than the same event with other drugs?” The screen does not answer “does the drug cause the event?”.",
    visualAnalysis: "Visual analysis", mapTitle: "Disproportionality map", map: "Map", forest: "Forest plot",
    chartType: "Chart type", meetsProfile: "Meets profile", reviewPriority: "Intermediate priority",
    belowProfile: "Below profile", selectedPair: "Selected pair",
    matrixCaption: "2 × 2 contingency table, in reports", targetEvent: "Target event (E)",
    otherEvents: "Other events (not E)", total: "Total", targetDrug: "Target substance (D)",
    otherDrugs: "Other substances (not D)", expected: "Expected",
    matrixHelp: "Cell “a” counts reports where the substance and event co-occur; this does not establish a causal link.",
    tableEquivalent: "Text equivalent of the charts", resultsTable: "Results by MedDRA PT",
    filter: "Filter", all: "All", sort: "Sort", observedDesc: "Observed ↓", termAsc: "Term A–Z",
    reaction: "Reported event", observed: "Observed (a)", screening: "Screening",
    methodology: "Methodology", methodsTitle: "From spontaneous report to SDR: visible assumptions",
    methodsLead: "The system uses disproportionality to prioritize pairs for review. Statistical detection, clinical validation, confirmation, and assessment are separate stages.",
    evansHeading: "Educational Evans profile",
    evansBody: "The label meets the profile when a ≥ 3, PRR ≥ 2, and Yates χ² ≥ 4. This is a historical teaching heuristic, not a universal EMA threshold.",
    zeroCorrection: "When a cell is zero, the API reports the Haldane–Anscombe correction applied to log-scale estimates.",
    failureModes: "Failure modes", failureDuplicate: "Duplicate reports and case versions can inflate counts.",
    failureStimulated: "Notoriety, media coverage, and time on market alter reporting.",
    failureCooccurrence: "Multiple drugs and events in one case prevent individual attribution through the public API.",
    failureDrugRole: "The live query does not filter drugcharacterization; suspect, concomitant, and interacting roles are pooled.",
    failureSelection: "The live screen selects frequent terms and can omit important rare events.",
    failureDenominator: "Without population exposure, incidence cannot be calculated.", primarySources: "Primary sources and reporting",
    researchWorkspace: "Research workspace", researchTitle: "Snapshots, protocols, and reproducible evidence",
    researchLead: "The catalog confirms snapshot registration and structural integrity; formal use still requires the validation matrix and independent scientific approval.",
    protocolEyebrow: "Deterministic protocol", protocolTitle: "Configure an analysis over a frozen snapshot",
    researchFormBoundary: "The complete configuration contributes to the analysis hash. Repeating the same protocol against the same manifest produces the same analysis_id.",
    datasetLabel: "Registered dataset", datasetSelectPlaceholder: "Loading registered snapshots…",
    datasetHelp: "Only manifests accepted by the server registry are listed.",
    drugConceptLabel: "Explicit drug identifier",
    drugConceptHelp: "Use faers-prod_ai:<exact text> or faers-drugname:<exact text>. This does not invent RxNorm, DCB, or ATC mappings.",
    drugRoleLabel: "Drug role", rolePrimary: "Primary suspect (PS)", roleSecondary: "Secondary suspect (SS)",
    roleConcomitant: "Concomitant (C)", roleInteracting: "Interacting (I)", roleSuspect: "Aggregated suspects (PS + SS)", roleAll: "All eligible roles",
    thresholdLabel: "Screening profile", thresholdNone: "No threshold", thresholdEvans: "Educational Evans",
    guidedProtocolTitle: "Guided reading",
    guidedProtocolBody: "The protocol uses every source PT, compares against all other eligible reports, and runs PRR, ROR, and Fisher. The Evans profile only adds review flags; it does not remove rows or confirm causality.",
    methodsLabel: "Executed methods", methodsHelp: "Selections change the protocol and therefore the analysis_id; they do not change a, b, c, d counts.",
    eventScopeLabel: "Event scope", comparatorLabel: "Comparator", periodLabel: "Period and strata", periodUnavailable: "Unavailable in this aggregate",
    unavailableTitle: "Not simulated in this interface",
    unavailableBody: "BCPNN/IC, GPS/EBGM, temporal, and stratified analyses require a compatible validated batch artifact. The interface does not replace those methods with approximations.",
    runResearch: "Run reproducible analysis", datasetCatalogTitle: "Available snapshots",
    datasetCatalogBody: "Confirm coverage, geography, and manifest hash before interpreting any result.",
    researchResultEyebrow: "Deterministic result", researchResultTitle: "Reproducible analysis", exportBundle: "Export ZIP bundle",
    analysisIDLabel: "analysis_id", resultDigestLabel: "result_digest (emitted rows)", resultDatasetLabel: "Dataset and manifest", resultCoverageLabel: "Coverage", resultRowsLabel: "row_count (emitted pairs)",
    caveatsTitle: "Limitations bound to this result", caveatsExact: "The text below is preserved exactly as returned by the scientific artifact.",
    researchVisualEyebrow: "Accessible visualization", researchForestTitle: "PRR forest plot on a logarithmic scale",
    researchForestDescription: "The forest displays a declared visual subset of the current page; the paginated table and bundle preserve the complete result.",
    researchTableEquivalent: "Paginated text equivalent of the complete result", researchTableTitle: "Counts and estimates by event",
    researchTableHelp: "a, b, c, d, and N are report counts in the documented universe; absent p and q values display as “—”, never as zero. Rows are ordered by descending PRR, with missing estimates last and a stable event-text tie-break.",
    researchPaginationLabel: "Result pagination", researchPreviousPage: "Previous page", researchNextPage: "Next page",
    researchPaginationSummary: "Showing {start}–{end} of {total} pairs (page {page} of {pages}; up to {pageSize} per page). The exported bundle preserves all {total} pairs.",
    researchPageStatus: "Page {page} of {pages}", researchTableRegion: "Paginated research results table",
    researchTableCaption: "Complete result presented in pages, ordered by descending PRR.",
    researchForestSelection: "Visual selection: up to {limit} positive PRRs from the current page, after applying the table order. Showing {shown}; {unplottable} without a positive estimate and interval and {deferred} eligible beyond the limit remain on this table page; {outside} pairs are on other pages. The axis fully includes the intervals of displayed pairs, without clipping.",
    researchEvent: "Event / category", researchFlags: "Review flags",
    datasetNoSelection: "No registered snapshot is available", researchRunning: "Running deterministic protocol…",
    researchUnavailable: "Snapshots are cataloged, but the v2 analysis engine is not enabled on this server.",
    researchError: "The v2 analysis could not be completed: {message}", conceptInvalid: "Use a faers-prod_ai: or faers-drugname: identifier with non-empty text.",
    methodsRequired: "Select at least one implemented method.", responseInvalid: "The API returned a result without valid identity, manifest, or row-integrity metadata.",
    noResearchRows: "No pairs were returned for this protocol.", noCaveats: "No caveats were returned; do not treat the result as validated.",
    noFlags: "No flag", forestResearchAria: "PRR forest plot with {shown} pairs from page {page}; the complete analysis contains {total} pairs. See the description and paginated table.",
    forestOmitted: "{count} pairs that cannot be represented on a logarithmic scale remain in the table.",
    forestResearchEmpty: "No PRR with a positive estimate and interval is available on this page; use the table or move to another page.", manifestLabel: "manifest",
    students: "Students", studentsBody: "Reproduce the 2 × 2 matrix, compare methods, and submit the configuration and limitations with your work.",
    researchers: "Researchers", researchersBody: "Prespecify snapshot, comparator, drug role, period, strata, and thresholds; export the complete bundle.",
    professionals: "Professionals", professionalsBody: "Use SDRs for human triage and combine them with case review, literature, and other evidence sources.",
    formalUseChecklist: "Minimum checklist for formal use", formal1: "Cite software version, commit, and dataset_id.",
    formal2: "Publish the protocol and parameters before interpretation.",
    formal3: "Retain manifests, checksums, results, and the execution environment.",
    formal4: "Report biases, missing data, and sensitivity analyses.",
    formal5: "Do not convert an SDR into causality, incidence, or a clinical recommendation.",
    openIssue: "Open a GitHub issue",
    footer: "PV Signal Radar · MIT software · source data licenses apply · not medical advice",
    loading: "Querying the live openFDA exploration for “{drug}”…",
    error: "The exploration could not be completed: {message}", genericError: "request unavailable",
    liveErrorDrugRequired: "Enter a medicine before starting the query.", liveErrorInvalidDrug: "The query is invalid; remove line breaks and use at most 120 characters.",
    liveErrorBusy: "The concurrent-query limit has been reached.", liveErrorRateLimited: "The temporary query-rate limit has been reached.",
    liveErrorUnavailable: "The openFDA source could not complete the query at this time.", liveErrorMethodNotAllowed: "This endpoint does not accept the HTTP method used.",
    liveErrorNetwork: "The openFDA service could not be reached.", liveErrorHTTPStatus: "The server rejected the query (HTTP {status}).",
    liveErrorRetry: "Try again in about {seconds} s.", liveErrorCode: "Technical code: {code}.", liveErrorDetail: "Detail: {detail}.",
    noResults: "No terms were returned for this query.", topTerms: "Only the {count} most reported terms; incomplete coverage",
    sourceLive: "live openFDA · exploratory", mapDescription: "Each point is a term: x-axis = log₂(PRR), y-axis = log₁₀(observed reports). Color represents the full educational profile, which also considers χ².",
    forestDescription: "PRR on a logarithmic scale with 95% CI; the vertical line at 1 represents no disproportionality. All returned terms are shown.",
    mapAria: "Disproportionality map with {count} terms. Use the table below for exact values.",
    forestAria: "PRR forest plot with {count} terms and confidence intervals. Use the table below for exact values.",
    chartEmpty: "No chart data.", datasetLoading: "Loading dataset catalog…",
    datasetEmptyTitle: "No snapshot released for formal analysis",
    datasetEmptyBody: "The v2 contract is available, but analysis remains blocked until an official FAERS snapshot passes the validation matrix.",
    datasetError: "Catalog unavailable. Formal analysis remains safely blocked.",
    datasetRegistered: "registered · integrity checked", datasetPending: "not released", coverage: "Coverage", retrieved: "Retrieved",
    profileActive: "Meets profile", profilePotential: "Intermediate review", profileNone: "Below profile",
    chartSelected: "Selected: {term}. PRR {prr}; interval {lower} to {upper}.",
    title: "PV Signal Radar — exploratory pharmacovigilance screening"
  }
};

document.addEventListener("DOMContentLoaded", function () {
  normalizeStoredPreferences();
  initializeTheme();
  initializeLocale();
  initializeViewMode();
  initializeTabs();
  initializeSearch();
  initializeCharts();
  initializeTableControls();
  initializeResearch();
  fetchHealth();
  fetchDatasets();
});

function normalizeStoredPreferences() {
  if (!messages[state.locale]) state.locale = "pt-BR";
  if (state.mode !== "guided" && state.mode !== "advanced") state.mode = "guided";
  if (state.theme !== "light" && state.theme !== "dark") state.theme = "light";
}

function t(key, values) {
  let value = (messages[state.locale] && messages[state.locale][key]) || messages["pt-BR"][key] || key;
  Object.keys(values || {}).forEach(function (name) {
    value = value.replaceAll("{" + name + "}", String(values[name]));
  });
  return value;
}

function initializeTheme() {
  document.documentElement.dataset.theme = state.theme;
  document.getElementById("themeToggle").addEventListener("click", function () {
    state.theme = state.theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = state.theme;
    localStorage.setItem("pv_theme", state.theme);
  });
}

function initializeLocale() {
  const select = document.getElementById("languageSelect");
  select.value = state.locale;
  select.addEventListener("change", function () {
    state.locale = select.value;
    localStorage.setItem("pv_locale", state.locale);
    applyTranslations();
    renderLiveStatus();
    if (state.analysis) renderAnalysis();
    renderDatasets(state.datasets);
    populateDatasetSelect(state.datasets);
    if (state.researchResult) renderResearchResult(state.researchResult);
  });
  applyTranslations();
}

function applyTranslations() {
  document.documentElement.lang = state.locale;
  document.title = t("title");
  document.querySelectorAll("[data-i18n]").forEach(function (element) {
    element.textContent = t(element.dataset.i18n);
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach(function (element) {
    element.placeholder = t(element.dataset.i18nPlaceholder);
  });
  document.querySelectorAll("[data-i18n-aria]").forEach(function (element) {
    element.setAttribute("aria-label", t(element.dataset.i18nAria));
  });
}

function initializeViewMode() {
  const select = document.getElementById("viewMode");
  select.value = state.mode;
  document.body.dataset.mode = state.mode;
  select.addEventListener("change", function () {
    state.mode = select.value;
    document.body.dataset.mode = state.mode;
    localStorage.setItem("pv_view_mode", state.mode);
    if (state.mode === "guided") {
      document.querySelectorAll('input[name="research_method"]').forEach(function (input) { input.checked = true; });
    }
  });
}

function initializeTabs() {
  const tabs = Array.from(document.querySelectorAll(".tabs [role=tab]"));
  function activate(tab) {
    tabs.forEach(function (candidate) {
      const active = candidate === tab;
      candidate.classList.toggle("active", active);
      candidate.setAttribute("aria-selected", String(active));
      candidate.tabIndex = active ? 0 : -1;
      document.getElementById(candidate.dataset.panel).hidden = !active;
    });
    tab.focus();
    history.replaceState(null, "", tab.dataset.panel === "panelAnalyze" ? "/" : (tab.dataset.panel === "panelMethods" ? "/methodology" : "/research"));
  }
  tabs.forEach(function (tab, index) {
    tab.addEventListener("click", function () { activate(tab); });
    tab.addEventListener("keydown", function (event) {
      if (event.key !== "ArrowRight" && event.key !== "ArrowLeft" && event.key !== "Home" && event.key !== "End") return;
      event.preventDefault();
      let next = index;
      if (event.key === "ArrowRight") next = (index + 1) % tabs.length;
      if (event.key === "ArrowLeft") next = (index - 1 + tabs.length) % tabs.length;
      if (event.key === "Home") next = 0;
      if (event.key === "End") next = tabs.length - 1;
      activate(tabs[next]);
    });
  });
  const pathPanel = location.pathname === "/methodology" ? "panelMethods" : (location.pathname === "/research" ? "panelResearch" : "panelAnalyze");
  const initial = tabs.find(function (tab) { return tab.dataset.panel === pathPanel; });
  if (initial && pathPanel !== "panelAnalyze") activate(initial);
}

function initializeSearch() {
  const form = document.getElementById("searchForm");
  const input = document.getElementById("drugInput");
  form.addEventListener("submit", function (event) {
    event.preventDefault();
    const drug = input.value.trim();
    if (drug) performAnalysis(drug);
  });
  document.querySelectorAll("[data-drug]").forEach(function (button) {
    button.addEventListener("click", function () {
      input.value = button.dataset.drug;
      performAnalysis(button.dataset.drug);
    });
  });
}

function initializeCharts() {
  document.querySelectorAll("[data-chart]").forEach(function (button) {
    button.addEventListener("click", function () {
      state.chart = button.dataset.chart;
      document.querySelectorAll("[data-chart]").forEach(function (candidate) {
        const active = candidate === button;
        candidate.classList.toggle("active", active);
        candidate.setAttribute("aria-pressed", String(active));
      });
      renderChart();
    });
  });
}

function initializeTableControls() {
  document.getElementById("levelFilter").addEventListener("change", renderTable);
  document.getElementById("sortResults").addEventListener("change", renderTable);
}

async function fetchHealth() {
  try {
    const response = await fetch("/api/v1/health", { headers: { "Accept": "application/json" } });
    if (!response.ok) return;
    const health = await response.json();
    if (health.version) document.getElementById("versionBadge").textContent = "v" + health.version;
  } catch (_) {
    // A failed version badge must not block the analytical interface.
  }
}

async function performAnalysis(drug) {
  const results = document.getElementById("results");
  setLiveStatus("loading", { drug: drug });
  results.hidden = true;
  try {
    const response = await fetch("/api/v1/analyze?drug=" + encodeURIComponent(drug), { headers: { "Accept": "application/json" } });
    let payload = {};
    try { payload = await response.json(); } catch (_) { payload = {}; }
    if (!response.ok) {
      state.analysis = null;
      setLiveStatus("error", { failure: normalizeLiveFailure(payload, response) });
      return;
    }
    state.analysis = normalizeAnalysis(payload);
    state.selectedSignal = state.analysis.signals[0] || null;
    setLiveStatus(null);
    results.hidden = false;
    renderAnalysis();
  } catch (error) {
    state.analysis = null;
    setLiveStatus("error", { failure: { code: "network_error", detail: error && error.message ? error.message : "" } });
  }
}

function normalizeLiveFailure(payload, response) {
  return {
    code: payload && typeof payload.code === "string" ? payload.code : "",
    detail: payload && typeof payload.error === "string" ? payload.error : "",
    status: response && Number.isInteger(response.status) ? response.status : 0,
    retryAfter: response && response.headers ? response.headers.get("Retry-After") || "" : ""
  };
}

function sanitizeLiveErrorDetail(value, maximumLength) {
  // textContent already prevents markup execution; this additional boundary
  // removes control characters and unbounded upstream text from status output.
  return String(value || "")
    .replace(/[\u0000-\u001f\u007f]/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, maximumLength);
}

function formatLiveFailure(failure) {
  const value = failure && typeof failure === "object" ? failure : {};
  const code = sanitizeLiveErrorDetail(value.code, 64);
  const messageKey = /^[a-z0-9_]+$/.test(code) ? LIVE_ERROR_MESSAGE_KEYS[code] : "";
  const status = Number(value.status);
  const parts = [messageKey ? t(messageKey) : (Number.isInteger(status) && status >= 400 ? t("liveErrorHTTPStatus", { status: status }) : t("genericError"))];

  const retryAfter = Number(value.retryAfter);
  if (Number.isFinite(retryAfter) && retryAfter > 0 && retryAfter <= 3600) {
    parts.push(t("liveErrorRetry", { seconds: Math.ceil(retryAfter) }));
  }
  if (code) parts.push(t("liveErrorCode", { code: code }));

  const detail = sanitizeLiveErrorDetail(value.detail, 240);
  if (detail && detail.toLowerCase() !== "request could not be processed") {
    parts.push(t("liveErrorDetail", { detail: detail }));
  }
  return parts.join(" ");
}

function setLiveStatus(key, params) {
  state.liveStatus = key ? { key: key, params: params || {} } : null;
  renderLiveStatus();
}

function renderLiveStatus() {
  const status = document.getElementById("statusBox");
  if (!status) return;
  status.classList.toggle("error", Boolean(state.liveStatus && state.liveStatus.key === "error"));
  if (!state.liveStatus) {
    status.textContent = "";
    return;
  }
  const params = Object.assign({}, state.liveStatus.params);
  // Failure data remains untranslated until render time so changing locale also
  // updates known API codes, retry guidance, and sanitized fallback details.
  if (state.liveStatus.key === "error") {
    params.message = params.failure ? formatLiveFailure(params.failure) : (params.message || t("genericError"));
  }
  status.textContent = t(state.liveStatus.key, params);
}

function normalizeAnalysis(payload) {
  const source = payload.fda_analysis || payload;
  return {
    drug: source.normalized_drug || payload.normalized_drug || source.query_drug || payload.query_drug || "",
    drugTotal: Number(source.drug_total_reports || 0),
    universe: Number(source.database_universe_n || 0),
    reviewCount: Number(source.sdr_review_count ?? 0),
    timestamp: source.timestamp || payload.timestamp || "",
    signals: Array.isArray(source.signals) ? source.signals : [],
    selectionLimit: Number(source.selection_limit || source.total_reactions_analyzed || 0)
  };
}

function renderAnalysis() {
  const analysis = state.analysis;
  if (!analysis) return;
  document.getElementById("resultsTitle").textContent = analysis.drug;
  document.getElementById("sourceMode").textContent = t("sourceLive");
  document.getElementById("analysisTime").textContent = formatTimestamp(analysis.timestamp);
  document.getElementById("drugReports").textContent = formatInteger(analysis.drugTotal);
  document.getElementById("sdrCount").textContent = formatInteger(analysis.reviewCount);
  document.getElementById("termCount").textContent = formatInteger(analysis.signals.length);
  document.getElementById("universeCount").textContent = formatInteger(analysis.universe);
  document.getElementById("coverageNote").textContent = t("topTerms", { count: analysis.selectionLimit || analysis.signals.length });
  renderTable();
  renderInspector(state.selectedSignal);
  renderChart();
}

function formatInteger(value) {
  return new Intl.NumberFormat(state.locale, { maximumFractionDigits: 0 }).format(Number(value) || 0);
}

function formatDecimal(value, digits) {
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  return new Intl.NumberFormat(state.locale, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(number);
}

function formatTimestamp(value) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(state.locale, { dateStyle: "medium", timeStyle: "short" }).format(date);
}

function profileClass(level) {
  if (level === "MEETS_PROFILE") return "review";
  if (level === "INTERMEDIATE_REVIEW") return "watch";
  return "baseline";
}

function screeningOutcome(signal) {
  if (signal.screening_outcome === "MEETS_EVANS_EDUCATIONAL_PROFILE") return "MEETS_PROFILE";
  if (signal.screening_outcome === "INTERMEDIATE_REVIEW") return "INTERMEDIATE_REVIEW";
  return "BELOW_PROFILE";
}

function profileLabel(level) {
  if (level === "MEETS_PROFILE") return t("profileActive");
  if (level === "INTERMEDIATE_REVIEW") return t("profilePotential");
  return t("profileNone");
}

function visibleSignals() {
  if (!state.analysis) return [];
  const filter = document.getElementById("levelFilter").value;
  const sort = document.getElementById("sortResults").value;
  const rows = state.analysis.signals.filter(function (signal) {
    return filter === "all" || screeningOutcome(signal) === filter;
  });
  rows.sort(function (left, right) {
    if (sort === "count_desc") return Number(right.count_a || 0) - Number(left.count_a || 0);
    if (sort === "name_asc") return String(left.reaction || "").localeCompare(String(right.reaction || ""), state.locale);
    return Number(right.prr || 0) - Number(left.prr || 0);
  });
  return rows;
}

function renderTable() {
  const body = document.getElementById("signalsBody");
  body.replaceChildren();
  const signals = visibleSignals();
  if (!signals.length) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 7;
    cell.className = "empty-row";
    cell.textContent = t("noResults");
    row.appendChild(cell);
    body.appendChild(row);
    return;
  }

  signals.forEach(function (signal) {
    const row = document.createElement("tr");
    row.tabIndex = 0;
    row.classList.toggle("selected", state.selectedSignal === signal);
    const expected = state.analysis.universe > 0
      ? state.analysis.drugTotal * Number(signal.reaction_total || 0) / state.analysis.universe
      : 0;
    appendCell(row, signal.reaction || "—");
    appendCell(row, formatInteger(signal.count_a), "mono");
    appendCell(row, formatDecimal(expected, 1), "mono");
    appendCell(row, metricWithCI(signal.prr, signal.prr_lower_95, signal.prr_upper_95), "mono");
    appendCell(row, metricWithCI(signal.ror, signal.ror_lower_95, signal.ror_upper_95), "mono");
    appendCell(row, formatDecimal(signal.chi_square_yates, 2), "mono");
    const labelCell = document.createElement("td");
    const label = document.createElement("span");
    label.className = "screening-label " + profileClass(screeningOutcome(signal));
    label.textContent = profileLabel(screeningOutcome(signal));
    labelCell.appendChild(label);
    row.appendChild(labelCell);
    function selectRow() {
      state.selectedSignal = signal;
      renderInspector(signal);
      renderTable();
    }
    row.addEventListener("click", selectRow);
    row.addEventListener("keydown", function (event) {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        selectRow();
      }
    });
    body.appendChild(row);
  });
}

function appendCell(row, value, className) {
  const cell = document.createElement("td");
  if (className) cell.className = className;
  cell.textContent = value;
  row.appendChild(cell);
}

function metricWithCI(value, lower, upper) {
  return formatDecimal(value, 2) + " [" + formatDecimal(lower, 2) + "–" + formatDecimal(upper, 2) + "]";
}

function renderInspector(signal) {
  const ids = ["cellA", "cellB", "cellC", "cellD", "rowDrug", "rowOther", "colEvent", "colOther", "matrixN", "pairPrr", "pairRor", "pairChi", "pairExpected"];
  if (!signal || !state.analysis) {
    document.getElementById("inspectorReaction").textContent = "—";
    document.getElementById("inspectorBadge").textContent = "—";
    ids.forEach(function (id) { document.getElementById(id).textContent = "—"; });
    return;
  }
  const a = Number(signal.count_a || 0);
  const b = Math.max(0, state.analysis.drugTotal - a);
  const c = Math.max(0, Number(signal.reaction_total || 0) - a);
  const d = Math.max(0, state.analysis.universe - a - b - c);
  const expected = state.analysis.universe > 0 ? state.analysis.drugTotal * (a + c) / state.analysis.universe : 0;
  document.getElementById("inspectorReaction").textContent = signal.reaction || "—";
  const badge = document.getElementById("inspectorBadge");
  badge.textContent = profileLabel(screeningOutcome(signal));
  badge.className = "screening-label " + profileClass(screeningOutcome(signal));
  const values = {
    cellA: a, cellB: b, cellC: c, cellD: d, rowDrug: a + b, rowOther: c + d,
    colEvent: a + c, colOther: b + d, matrixN: state.analysis.universe
  };
  Object.keys(values).forEach(function (id) { document.getElementById(id).textContent = formatInteger(values[id]); });
  document.getElementById("pairPrr").textContent = metricWithCI(signal.prr, signal.prr_lower_95, signal.prr_upper_95);
  document.getElementById("pairRor").textContent = metricWithCI(signal.ror, signal.ror_lower_95, signal.ror_upper_95);
  document.getElementById("pairChi").textContent = formatDecimal(signal.chi_square_yates, 2);
  document.getElementById("pairExpected").textContent = formatDecimal(expected, 1);
}

function renderChart() {
  const chart = document.getElementById("chart");
  chart.replaceChildren();
  if (!state.analysis || !state.analysis.signals.length) {
    chart.textContent = t("chartEmpty");
    return;
  }
  if (state.chart === "forest") renderForest(chart, state.analysis.signals);
  else renderMap(chart, state.analysis.signals);
}

function svgElement(name, attributes, textContent) {
  const element = document.createElementNS(SVG_NS, name);
  Object.keys(attributes || {}).forEach(function (key) { element.setAttribute(key, String(attributes[key])); });
  if (textContent !== undefined) element.textContent = textContent;
  return element;
}

function renderMap(container, signals) {
  document.getElementById("chartTitle").textContent = t("mapTitle");
  document.getElementById("chartDescription").textContent = t("mapDescription");
  const width = 900, height = 380, left = 62, right = 25, top = 22, bottom = 56;
  const svg = svgElement("svg", { viewBox: "0 0 " + width + " " + height, "aria-hidden": "true" });
  const points = signals.map(function (signal) {
    return {
      signal: signal,
      x: Math.log2(Math.max(Number(signal.prr || 0), 0.01)),
      y: Math.log10(Math.max(Number(signal.count_a || 0), 1))
    };
  });
  const maxAbsX = Math.max(2, ...points.map(function (point) { return Math.abs(point.x); }));
  const maxY = Math.max(1, ...points.map(function (point) { return point.y; }));
  const xScale = function (value) { return left + ((value + maxAbsX) / (2 * maxAbsX)) * (width - left - right); };
  const yScale = function (value) { return height - bottom - (value / maxY) * (height - top - bottom); };
  [-maxAbsX, 0, 1, maxAbsX].forEach(function (tick) {
    const x = xScale(tick);
    svg.appendChild(svgElement("line", { x1: x, x2: x, y1: top, y2: height - bottom, class: tick === 1 ? "threshold" : "grid" }));
    svg.appendChild(svgElement("text", { x: x, y: height - bottom + 20, "text-anchor": "middle" }, formatDecimal(tick, 1)));
  });
  [0, maxY / 2, maxY].forEach(function (tick) {
    const y = yScale(tick);
    svg.appendChild(svgElement("line", { x1: left, x2: width - right, y1: y, y2: y, class: "grid" }));
    svg.appendChild(svgElement("text", { x: left - 8, y: y + 4, "text-anchor": "end" }, formatDecimal(tick, 1)));
  });
  svg.appendChild(svgElement("line", { x1: left, x2: width - right, y1: height - bottom, y2: height - bottom, class: "axis" }));
  svg.appendChild(svgElement("line", { x1: left, x2: left, y1: top, y2: height - bottom, class: "axis" }));
  svg.appendChild(svgElement("text", { x: (left + width - right) / 2, y: height - 13, "text-anchor": "middle", class: "axis-label" }, "log₂(PRR)"));
  svg.appendChild(svgElement("text", { x: 15, y: (top + height - bottom) / 2, "text-anchor": "middle", transform: "rotate(-90 15 " + ((top + height - bottom) / 2) + ")", class: "axis-label" }, "log₁₀(a)"));
  points.forEach(function (point) {
    const circle = svgElement("circle", {
      cx: xScale(point.x), cy: yScale(point.y), r: 6,
      class: "point " + profileClass(screeningOutcome(point.signal))
    });
    const title = t("chartSelected", {
      term: point.signal.reaction,
      prr: formatDecimal(point.signal.prr, 2),
      lower: formatDecimal(point.signal.prr_lower_95, 2),
      upper: formatDecimal(point.signal.prr_upper_95, 2)
    });
    circle.appendChild(svgElement("title", {}, title));
    circle.addEventListener("click", function () { state.selectedSignal = point.signal; renderInspector(point.signal); renderTable(); });
    svg.appendChild(circle);
  });
  container.setAttribute("aria-label", t("mapAria", { count: signals.length }));
  container.appendChild(svg);
}

function renderForest(container, originalSignals) {
  document.getElementById("chartTitle").textContent = t("forest");
  document.getElementById("chartDescription").textContent = t("forestDescription");
  const signals = originalSignals.slice().sort(function (a, b) { return Number(b.prr || 0) - Number(a.prr || 0); });
  const width = 900, rowHeight = 34, top = 30, bottom = 52, left = 230, right = 35;
  const height = Math.max(380, top + bottom + rowHeight * signals.length);
  const positive = [];
  signals.forEach(function (signal) {
    [signal.prr_lower_95, signal.prr, signal.prr_upper_95].forEach(function (value) {
      if (Number(value) > 0 && Number.isFinite(Number(value))) positive.push(Number(value));
    });
  });
  const minValue = Math.min(0.1, ...positive);
  const maxValue = Math.max(10, ...positive);
  const minLog = Math.log10(minValue), maxLog = Math.log10(maxValue);
  const xScale = function (value) {
    return left + ((Math.log10(Math.max(value, minValue)) - minLog) / (maxLog - minLog)) * (width - left - right);
  };
  const svg = svgElement("svg", { viewBox: "0 0 " + width + " " + height, width: width, height: height, "aria-hidden": "true" });
  const ticks = [];
  for (let exponent = Math.floor(minLog); exponent <= Math.ceil(maxLog); exponent += 1) {
    const value = Math.pow(10, exponent);
    if (value >= minValue && value <= maxValue) ticks.push(value);
  }
  if (!ticks.includes(1)) ticks.push(1);
  ticks.sort(function (a, b) { return a - b; }).forEach(function (tick) {
    const x = xScale(tick);
    svg.appendChild(svgElement("line", { x1: x, x2: x, y1: top, y2: height - bottom, class: tick === 1 ? "threshold" : "grid" }));
    svg.appendChild(svgElement("text", { x: x, y: height - 20, "text-anchor": "middle" }, formatDecimal(tick, tick < 1 ? 2 : 0)));
  });
  signals.forEach(function (signal, index) {
    const y = top + index * rowHeight + rowHeight / 2;
    const lower = Math.max(Number(signal.prr_lower_95 || signal.prr || 1), minValue);
    const estimate = Math.max(Number(signal.prr || 1), minValue);
    const upper = Math.max(Number(signal.prr_upper_95 || signal.prr || 1), minValue);
    svg.appendChild(svgElement("text", { x: left - 10, y: y + 4, "text-anchor": "end" }, String(signal.reaction || "—").slice(0, 31)));
    svg.appendChild(svgElement("line", { x1: xScale(lower), x2: xScale(upper), y1: y, y2: y, class: "ci" }));
    svg.appendChild(svgElement("line", { x1: xScale(lower), x2: xScale(lower), y1: y - 5, y2: y + 5, class: "ci" }));
    svg.appendChild(svgElement("line", { x1: xScale(upper), x2: xScale(upper), y1: y - 5, y2: y + 5, class: "ci" }));
    const circle = svgElement("circle", {
      cx: xScale(estimate), cy: y, r: 5, class: "point " + profileClass(screeningOutcome(signal))
    });
    const title = t("chartSelected", {
      term: signal.reaction, prr: formatDecimal(signal.prr, 2),
      lower: formatDecimal(signal.prr_lower_95, 2), upper: formatDecimal(signal.prr_upper_95, 2)
    });
    circle.appendChild(svgElement("title", {}, title));
    circle.addEventListener("click", function () { state.selectedSignal = signal; renderInspector(signal); renderTable(); });
    svg.appendChild(circle);
  });
  svg.appendChild(svgElement("text", { x: (left + width - right) / 2, y: height - 4, "text-anchor": "middle", class: "axis-label" }, "PRR (log scale)"));
  container.setAttribute("aria-label", t("forestAria", { count: signals.length }));
  container.appendChild(svg);
}

async function fetchDatasets() {
  const catalog = document.getElementById("datasetCatalog");
  catalog.textContent = t("datasetLoading");
  try {
    const response = await fetch("/api/v2/datasets", { headers: { "Accept": "application/json" } });
    if (!response.ok) throw new Error("catalog_unavailable");
    const payload = await response.json();
    state.datasets = Array.isArray(payload) ? payload : (Array.isArray(payload.datasets) ? payload.datasets : []);
    state.researchEnabled = Array.isArray(payload) ? state.datasets.length > 0 : payload.research_analysis_enabled === true;
    renderDatasets(state.datasets);
    populateDatasetSelect(state.datasets);
  } catch (_) {
    state.datasets = [];
    state.researchEnabled = false;
    catalog.textContent = t("datasetError");
    populateDatasetSelect([]);
  }
}

function renderDatasets(datasets) {
  const catalog = document.getElementById("datasetCatalog");
  if (!catalog) return;
  catalog.replaceChildren();
  if (!datasets.length) {
    const item = document.createElement("article");
    item.className = "dataset-item";
    const content = document.createElement("div");
    const heading = document.createElement("h2");
    heading.textContent = t("datasetEmptyTitle");
    const body = document.createElement("p");
    body.textContent = t("datasetEmptyBody");
    content.append(heading, body);
    const status = document.createElement("span");
    status.className = "pill dataset-state";
    status.textContent = t("datasetPending");
    item.append(content, status);
    catalog.appendChild(item);
    return;
  }
  datasets.forEach(function (dataset) {
    const item = document.createElement("article");
    item.className = "dataset-item";
    const content = document.createElement("div");
    const heading = document.createElement("h2");
    heading.textContent = dataset.dataset_id || dataset.id || "dataset";
    const body = document.createElement("p");
    const sourceLabel = dataset.source && typeof dataset.source === "object" ? dataset.source.name : dataset.source;
    body.textContent = dataset.description || sourceLabel || "";
    const meta = document.createElement("div");
    meta.className = "dataset-meta";
    const coverageLabel = dataset.coverage && typeof dataset.coverage === "object"
      ? [dataset.coverage.start_date, dataset.coverage.end_date, dataset.coverage.geography].filter(Boolean).join(" · ")
      : (dataset.coverage || "—");
    [t("coverage") + ": " + coverageLabel, "SHA-256: " + (dataset.manifest_sha256 || "—")].forEach(function (value) {
      const tag = document.createElement("span");
      tag.className = "pill";
      tag.textContent = value;
      meta.appendChild(tag);
    });
    content.append(heading, body, meta);
    const status = document.createElement("span");
    status.className = "pill dataset-state";
    status.textContent = dataset.registration_state === "integrity_checked" ? t("datasetRegistered") : t("datasetPending");
    item.append(content, status);
    catalog.appendChild(item);
  });
}

function initializeResearch() {
  const form = document.getElementById("researchForm");
  if (!form) return;
  initializeResearchPagination();
  form.addEventListener("submit", async function (event) {
    event.preventDefault();
    const status = document.getElementById("researchStatus");
    const resultSection = document.getElementById("researchResult");
    const exportLink = document.getElementById("researchExport");
    const submit = document.getElementById("researchSubmit");
    status.classList.remove("error");

    const datasetID = document.getElementById("researchDataset").value;
    if (!datasetID) {
      status.classList.add("error");
      status.textContent = t("datasetNoSelection");
      return;
    }
    const drugConceptID = document.getElementById("researchDrugConcept").value.trim();
    if (!/^faers-(prod_ai|drugname):\S(?:.*\S)?$/.test(drugConceptID)) {
      status.classList.add("error");
      status.textContent = t("conceptInvalid");
      document.getElementById("researchDrugConcept").focus();
      return;
    }
    const methods = Array.from(document.querySelectorAll('input[name="research_method"]:checked'))
      .map(function (input) { return input.value; })
      .sort();
    if (!methods.length) {
      status.classList.add("error");
      status.textContent = t("methodsRequired");
      return;
    }

    const protocol = {
      schema_version: "pv-signal-radar.research/v1",
      dataset_id: datasetID,
      drug_concept_id: drugConceptID,
      drug_role: document.getElementById("researchDrugRole").value,
      event_scope: "all_recorded_source_pts",
      comparator: "all_other_eligible_reports",
      period: {},
      methods: methods,
      threshold_profile_id: document.getElementById("researchThreshold").value
    };

    state.researchResult = null;
    state.researchRowsOrdered = [];
    state.researchPage = 1;
    resultSection.hidden = true;
    exportLink.hidden = true;
    exportLink.removeAttribute("href");
    submit.disabled = true;
    status.textContent = t("researchRunning");
    try {
      const response = await fetch("/api/v2/analyses", {
        method: "POST",
        headers: { "Accept": "application/json", "Content-Type": "application/json" },
        body: JSON.stringify(protocol)
      });
      let payload = {};
      try { payload = await response.json(); } catch (_) { payload = {}; }
      if (!response.ok) throw new Error(payload.code || payload.error || "request_failed");
      const rowsValid = payload && Array.isArray(payload.rows) && Number.isSafeInteger(payload.row_count) && payload.row_count === payload.rows.length && payload.row_count > 0;
      const family = payload && payload.result_family;
      const familyValid = family && family.family_id === "normalized_protocol_emitted_rows_v1" && family.membership_rule === "all_matching_aggregate_events_emitted_without_ranking_or_top_n" && family.row_unit === "unique_drug_event_pair" && family.row_order === "event_concept_id_asc_utf8_binary_then_drug_concept_id_asc_utf8_binary" && family.digest_algorithm === "sha256_canonical_json_v1";
      const manifestValid = payload && payload.dataset && payload.dataset_manifest && payload.dataset.dataset_id === payload.dataset_manifest.dataset_id;
      if (!payload || !/^[a-f0-9]{64}$/.test(String(payload.analysis_id || "")) || !/^[a-f0-9]{64}$/.test(String(payload.result_digest || "")) || !rowsValid || !familyValid || !manifestValid) {
        throw new Error(t("responseInvalid"));
      }
      state.researchResult = payload;
      renderResearchResult(payload);
      status.textContent = "";
      resultSection.hidden = false;
      exportLink.href = "/api/v2/analyses/" + encodeURIComponent(payload.analysis_id) + "/export";
      exportLink.download = "pv-signal-radar-" + payload.analysis_id + ".zip";
      exportLink.hidden = false;
      document.getElementById("researchResultTitle").focus({ preventScroll: true });
      const reducedMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      resultSection.scrollIntoView({ behavior: reducedMotion ? "auto" : "smooth", block: "start" });
    } catch (error) {
      status.classList.add("error");
      status.textContent = t("researchError", { message: error.message || "request_failed" });
    } finally {
      submit.disabled = !state.researchEnabled || state.datasets.filter(datasetIsSelectable).length === 0;
    }
  });
}

function datasetIsSelectable(dataset) {
  return dataset && dataset.registration_state === "integrity_checked" && typeof dataset.dataset_id === "string" && dataset.dataset_id !== "";
}

function populateDatasetSelect(datasets) {
  const select = document.getElementById("researchDataset");
  const submit = document.getElementById("researchSubmit");
  if (!select || !submit) return;
  const previous = select.value;
  const selectable = datasets.filter(datasetIsSelectable);
  select.replaceChildren();
  if (!state.researchEnabled || !selectable.length) {
    const option = document.createElement("option");
    option.value = "";
    option.textContent = selectable.length ? t("researchUnavailable") : t("datasetNoSelection");
    select.appendChild(option);
    select.disabled = true;
    submit.disabled = true;
    return;
  }
  selectable.forEach(function (dataset) {
    const option = document.createElement("option");
    option.value = dataset.dataset_id;
    const release = dataset.coverage && dataset.coverage.release ? " · " + dataset.coverage.release : "";
    option.textContent = dataset.dataset_id + release;
    select.appendChild(option);
  });
  if (selectable.some(function (dataset) { return dataset.dataset_id === previous; })) select.value = previous;
  select.disabled = false;
  submit.disabled = false;
}

function renderResearchResult(result) {
  if (!result) return;
  const dataset = result.dataset || {};
  const rows = Array.isArray(result.rows) ? result.rows : [];
  document.getElementById("researchAnalysisID").textContent = result.analysis_id || "—";
  document.getElementById("researchResultDigest").textContent = result.result_digest || "—";
  document.getElementById("researchResultDataset").textContent = (dataset.dataset_id || "—") + " · " + t("manifestLabel") + " " + (dataset.manifest_sha256 || "—");
  document.getElementById("researchResultCoverage").textContent = coverageText(dataset.coverage);
  document.getElementById("researchResultRows").textContent = Number.isSafeInteger(result.row_count) ? formatInteger(result.row_count) : "—";

  const caveatList = document.getElementById("researchCaveats");
  caveatList.replaceChildren();
  const caveats = Array.isArray(result.caveats) && result.caveats.length ? result.caveats : [t("noCaveats")];
  caveats.forEach(function (caveat) {
    const item = document.createElement("li");
    item.textContent = String(caveat);
    caveatList.appendChild(item);
  });
  state.researchRowsOrdered = orderResearchRows(rows);
  renderResearchPage();
}

function initializeResearchPagination() {
  const previous = document.getElementById("researchPreviousPage");
  const next = document.getElementById("researchNextPage");
  if (!previous || !next) return;
  previous.addEventListener("click", function () { changeResearchPage(-1); });
  next.addEventListener("click", function () { changeResearchPage(1); });
}

function changeResearchPage(delta) {
  const totalPages = Math.max(1, Math.ceil(state.researchRowsOrdered.length / RESEARCH_PAGE_SIZE));
  const target = Math.min(totalPages, Math.max(1, state.researchPage + delta));
  if (target === state.researchPage) return;
  state.researchPage = target;
  renderResearchPage();
  document.getElementById("researchPaginationSummary").focus({ preventScroll: true });
}

function orderResearchRows(rows) {
  // A single declared order drives both representations. Missing/non-finite
  // PRRs are retained at the end; source order breaks otherwise exact ties.
  return Array.from(rows, function (row, sourceIndex) { return { row: row, sourceIndex: sourceIndex }; })
    .sort(function (left, right) {
      const leftPRR = researchPRREstimate(left.row);
      const rightPRR = researchPRREstimate(right.row);
      if (leftPRR === null && rightPRR !== null) return 1;
      if (leftPRR !== null && rightPRR === null) return -1;
      if (leftPRR !== null && rightPRR !== null && rightPRR !== leftPRR) return rightPRR - leftPRR;
      const leftTerm = String(left.row.event_term || "");
      const rightTerm = String(right.row.event_term || "");
      if (leftTerm < rightTerm) return -1;
      if (leftTerm > rightTerm) return 1;
      return left.sourceIndex - right.sourceIndex;
    })
    .map(function (entry) { return entry.row; });
}

function researchPRREstimate(row) {
  const metric = metricsByMethod(row).prr;
  if (!metric) return null;
  const estimate = Number(metric.estimate);
  return Number.isFinite(estimate) ? estimate : null;
}

function renderResearchPage() {
  const rows = state.researchRowsOrdered;
  const totalPages = Math.max(1, Math.ceil(rows.length / RESEARCH_PAGE_SIZE));
  state.researchPage = Math.min(totalPages, Math.max(1, state.researchPage));
  const startIndex = (state.researchPage - 1) * RESEARCH_PAGE_SIZE;
  const endIndex = Math.min(rows.length, startIndex + RESEARCH_PAGE_SIZE);
  // Filtering by index deliberately caps created DOM nodes without modifying
  // the underlying result or the server-side export bundle.
  const pageRows = rows.filter(function (_, index) { return index >= startIndex && index < endIndex; });
  const start = rows.length ? startIndex + 1 : 0;

  const summary = document.getElementById("researchPaginationSummary");
  summary.textContent = t("researchPaginationSummary", {
    start: start,
    end: endIndex,
    total: rows.length,
    page: state.researchPage,
    pages: totalPages,
    pageSize: RESEARCH_PAGE_SIZE
  });
  document.getElementById("researchPageStatus").textContent = t("researchPageStatus", { page: state.researchPage, pages: totalPages });
  document.getElementById("researchPreviousPage").disabled = state.researchPage <= 1;
  document.getElementById("researchNextPage").disabled = state.researchPage >= totalPages;

  renderResearchTable(pageRows);
  renderResearchForest(pageRows, {
    page: state.researchPage,
    totalRows: rows.length,
    outsidePage: rows.length - pageRows.length
  });
}

function coverageText(coverage) {
  if (!coverage || typeof coverage !== "object") return "—";
  return [coverage.start_date && coverage.end_date ? coverage.start_date + " – " + coverage.end_date : (coverage.start_date || coverage.end_date), coverage.geography, coverage.release]
    .filter(Boolean)
    .join(" · ") || "—";
}

function metricsByMethod(row) {
  const metrics = {};
  (Array.isArray(row.metrics) ? row.metrics : []).forEach(function (metric) {
    if (metric && typeof metric.method === "string") metrics[metric.method] = metric;
  });
  return metrics;
}

function researchNumber(value) {
  if (value === null || value === undefined || value === "") return "—";
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  const absolute = Math.abs(number);
  if (absolute !== 0 && (absolute < 0.001 || absolute >= 1000000)) return number.toExponential(3);
  return new Intl.NumberFormat(state.locale, { maximumSignificantDigits: 5 }).format(number);
}

function researchMetric(metric) {
  if (!metric) return "—";
  const estimate = researchNumber(metric.estimate);
  if (metric.lower_95 === null || metric.lower_95 === undefined || metric.upper_95 === null || metric.upper_95 === undefined) return estimate;
  return estimate + " [" + researchNumber(metric.lower_95) + "–" + researchNumber(metric.upper_95) + "]";
}

function researchFlagLabel(flag) {
  if (!flag) return t("noFlags");
  if (flag.outcome === "meets_profile") return t("profileActive");
  if (flag.outcome === "intermediate_review") return t("profilePotential");
  if (flag.outcome === "below_profile") return t("profileNone");
  return flag.outcome || t("noFlags");
}

function renderResearchTable(rows) {
  const body = document.getElementById("researchResultsBody");
  body.replaceChildren();
  if (!rows.length) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 11;
    cell.className = "empty-row";
    cell.textContent = t("noResearchRows");
    row.appendChild(cell);
    body.appendChild(row);
    return;
  }
  rows.forEach(function (resultRow) {
    const row = document.createElement("tr");
    const identityCell = document.createElement("th");
    identityCell.scope = "row";
    const identity = document.createElement("span");
    identity.className = "event-identity";
    const term = document.createElement("strong");
    term.textContent = resultRow.event_term || "—";
    const category = document.createElement("small");
    category.textContent = [resultRow.event_category, resultRow.event_concept_id].filter(Boolean).join(" · ") || "—";
    identity.append(term, category);
    identityCell.appendChild(identity);
    row.appendChild(identityCell);

    const table = resultRow.contingency_table || {};
    [table.a, table.b, table.c, table.d, table.n].forEach(function (value) { appendCell(row, value === null || value === undefined ? "—" : formatInteger(value)); });
    const metrics = metricsByMethod(resultRow);
    appendCell(row, researchMetric(metrics.prr));
    appendCell(row, researchMetric(metrics.ror));
    appendCell(row, metrics.fisher_exact ? researchNumber(metrics.fisher_exact.p_value) : "—");
    appendCell(row, metrics.fisher_exact ? researchNumber(metrics.fisher_exact.q_value) : "—");

    const flagsCell = document.createElement("td");
    const flags = document.createElement("span");
    flags.className = "review-flags";
    const reviewFlags = Array.isArray(resultRow.review_flags) ? resultRow.review_flags : [];
    if (!reviewFlags.length) {
      flags.textContent = t("noFlags");
    } else {
      reviewFlags.forEach(function (flag) {
        const label = document.createElement("strong");
        label.textContent = researchFlagLabel(flag);
        const reason = document.createElement("span");
        reason.textContent = flag.reason || "";
        flags.append(label, reason);
      });
    }
    flagsCell.appendChild(flags);
    row.appendChild(flagsCell);
    body.appendChild(row);
  });
}

function renderResearchForest(rows, context) {
  const container = document.getElementById("researchForest");
  const description = document.getElementById("researchForestDescription");
  const selection = document.getElementById("researchForestSelection");
  container.replaceChildren();
  const plottable = [];
  let unplottable = 0;
  let deferred = 0;
  rows.forEach(function (row) {
    const metric = metricsByMethod(row).prr;
    if (!metric) { unplottable += 1; return; }
    if (metric.lower_95 === null || metric.lower_95 === undefined || metric.upper_95 === null || metric.upper_95 === undefined) {
      unplottable += 1;
      return;
    }
    const estimate = Number(metric.estimate);
    const lower = Number(metric.lower_95);
    const upper = Number(metric.upper_95);
    if (![estimate, lower, upper].every(function (value) { return Number.isFinite(value) && value > 0; })) { unplottable += 1; return; }
    if (plottable.length >= RESEARCH_FOREST_LIMIT) { deferred += 1; return; }
    plottable.push({ row: row, estimate: estimate, lower: lower, upper: upper });
  });
  description.textContent = t("researchForestDescription");
  selection.textContent = t("researchForestSelection", {
    limit: RESEARCH_FOREST_LIMIT,
    shown: plottable.length,
    unplottable: unplottable,
    deferred: deferred,
    outside: context.outsidePage
  });
  container.setAttribute("aria-label", t("forestResearchAria", {
    shown: plottable.length,
    page: context.page,
    total: context.totalRows
  }));
  if (!plottable.length) {
    container.textContent = t("forestResearchEmpty");
    return;
  }
  const longestLabel = plottable.reduce(function (longest, point) {
    return Math.max(longest, Array.from(String(point.row.event_term || "—")).length);
  }, 0);
  const left = Math.max(230, longestLabel * 7.2 + 28);
  const right = 45;
  const width = Math.max(900, left + 620 + right);
  const rowHeight = 34, top = 28, bottom = 55;
  const height = Math.max(320, top + bottom + rowHeight * plottable.length);
  const values = [1];
  plottable.forEach(function (point) { values.push(point.lower, point.estimate, point.upper); });
  let minLog = Math.log10(Math.min(...values));
  let maxLog = Math.log10(Math.max(...values));
  if (minLog === maxLog) { minLog -= 1; maxLog += 1; }
  const padding = (maxLog - minLog) * .05;
  minLog -= padding;
  maxLog += padding;
  const plotWidth = width - left - right;
  const xScale = function (value) { return left + ((Math.log10(value) - minLog) / (maxLog - minLog)) * plotWidth; };
  const svg = svgElement("svg", { viewBox: "0 0 " + width + " " + height, width: width, height: height, "aria-hidden": "true" });

  const ticks = [1];
  for (let exponent = Math.floor(minLog); exponent <= Math.ceil(maxLog); exponent += 1) {
    const tick = Math.pow(10, exponent);
    if (Math.log10(tick) >= minLog && Math.log10(tick) <= maxLog) ticks.push(tick);
  }
  Array.from(new Set(ticks)).sort(function (a, b) { return a - b; }).forEach(function (tick) {
    const x = xScale(tick);
    svg.appendChild(svgElement("line", { x1: x, x2: x, y1: top, y2: height - bottom, class: tick === 1 ? "threshold" : "grid" }));
    svg.appendChild(svgElement("text", { x: x, y: height - 24, "text-anchor": "middle" }, researchNumber(tick)));
  });
  plottable.forEach(function (point, index) {
    const y = top + index * rowHeight + rowHeight / 2;
    svg.appendChild(svgElement("text", { x: left - 10, y: y + 4, "text-anchor": "end" }, point.row.event_term || "—"));
    svg.appendChild(svgElement("line", { x1: xScale(point.lower), x2: xScale(point.upper), y1: y, y2: y, class: "ci" }));
    svg.appendChild(svgElement("line", { x1: xScale(point.lower), x2: xScale(point.lower), y1: y - 5, y2: y + 5, class: "ci" }));
    svg.appendChild(svgElement("line", { x1: xScale(point.upper), x2: xScale(point.upper), y1: y - 5, y2: y + 5, class: "ci" }));
    const marker = svgElement("circle", { cx: xScale(point.estimate), cy: y, r: 5, class: "point " + researchRowClass(point.row) });
    marker.appendChild(svgElement("title", {}, (point.row.event_term || "—") + ": PRR " + researchMetric(metricsByMethod(point.row).prr)));
    svg.appendChild(marker);
  });
  svg.appendChild(svgElement("text", { x: left + plotWidth / 2, y: height - 5, "text-anchor": "middle", class: "axis-label" }, "PRR (log scale)"));
  container.appendChild(svg);
}

function researchRowClass(row) {
  const flag = Array.isArray(row.review_flags) ? row.review_flags[0] : null;
  if (flag && flag.outcome === "meets_profile") return "review";
  if (flag && flag.outcome === "intermediate_review") return "watch";
  return "baseline";
}
