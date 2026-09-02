# Contrato de métodos e protocolo

## Unidade de análise

Para datasets de pesquisa FAERS congelados, o ETL seleciona o maior `CASEVERSION` numérico por `CASEID`; um desempate pelo maior `PRIMARYID` numérico torna duplicatas ambíguas determinísticas e visíveis no QA. Uma notificação elegível deve ter ao menos uma linha de medicamento retida e um Preferred Term de origem não vazio. Cada `PRIMARYID × drug text × drug role × PT` aparece uma vez.

`PROD_AI` é preferido como texto de origem do medicamento e `DRUGNAME` é o fallback. O texto tem espaços nas extremidades removidos, whitespace normalizado e é convertido para maiúsculas. Isso não constitui um mapeamento para RxNorm, UNII, ATC, DCB, WHODrug nem para um conceito de ingrediente. O texto do PT é preservado da fonte trimestral; o pipeline não infere uma versão do MedDRA.

## Pré-especificação obrigatória

Antes da análise, congele:

1. dataset e período de cobertura;
2. versão do conceito/mapeamento do medicamento e nomes de origem incluídos;
3. papéis do medicamento, como suspeito primário (`PS`) ou suspeito secundário (`SS`);
4. escopo de eventos e a versão de qualquer referência DME/IME ou de categoria clínica;
5. comparator e exclusões;
6. estratos demográficos, geográficos, de gravidade e temporais;
7. métodos, política de células zero, nível de confiança, procedimento de multiplicidade e perfil de threshold; e
8. controles e medidas de avaliação usados na seleção de um threshold.

Alterar qualquer um desses itens depois de examinar o resultado cria uma nova análise e deve produzir um novo identificador de análise.

## Medidas 2 × 2

Para um grupo de medicamento `D` e evento `E`, `a` é o número de notificações elegíveis que contêm ambos, `b` contém `D` sem `E`, `c` contém `E` sem `D` e `d` não contém nenhum dos dois. As contagens são de notificações, não de prescrições, pessoas expostas, casos causados nem denominadores de incidência.

- `PRR = [a/(a+b)] / [c/(c+d)]`
- `ROR = (a×d)/(b×c)`

A implementação Go e `research/R/reference_metrics.R` calculam limites de confiança assintóticos bilaterais fixos de 95% na escala log com o equivalente completo de `qnorm(0.975)`, não o atalho `1.96`. Quando uma célula é zero, a implementação de referência atual soma 0,5 a todas as células e registra em cada métrica a escolha de células de entrada/correção. O p-value bilateral do teste exato de Fisher está disponível para tabelas esparsas; a execução online falha de modo fechado em vez de enumerar mais de 100.000 termos do suporte. Se múltiplos p-values forem exibidos ou usados, o ajuste FDR de Benjamini-Hochberg é obrigatório, e tanto os valores brutos quanto os ajustados devem ser preservados.

O perfil guiado `a ≥ 3`, `PRR ≥ 2` e `χ² ≥ 4` com correção de Yates é uma heurística educacional no estilo de Evans. Não é um “critério EMA”, um threshold universal de decisão nem prova de sinal de segurança. O [adendo metodológico da EMA](https://www.ema.europa.eu/en/documents/scientific-guideline/guideline-good-pharmacovigilance-practices-gvp-module-ix-addendum-i-methodological-aspects-signal_en.pdf) exige que os thresholds sejam apropriados, documentados e avaliados para a base e a finalidade.

## Métodos adicionais

O ambiente R separado reserva versões diretas exatas de `faers 1.8.0`, `pvda 0.0.4` e `openEBGM 0.9.1` para comparação independente de ETL/métodos e trabalho com BCPNN IC/IC025 e GPS EBGM/EB05. Esses métodos não são considerados validados nem habilitados apenas porque os pacotes estão listados. Adapters, resultados golden, reference sets e revisão de licenças continuam sendo gates de release.

Nenhum threshold recomendado para pesquisa será publicado até que um reference set positivo/negativo pré-especificado relate sensibilidade, valor preditivo positivo, comportamento de falsos positivos e time-to-detection. O desempenho de um threshold em um dataset, período ou escopo de eventos não se transfere automaticamente para outro.

## Famílias de eventos e comparação entre fontes

PTs de eventos adversos clínicos devem ser analisados separadamente de circunstâncias de uso de medicamentos, problemas de qualidade do produto, erros, inefetividade, abuso, uso indevido e termos de uso off-label. Até que exista uma classificação versionada, o ETL rotula todo PT como `UNCLASSIFIED_SOURCE_PT` em vez de adivinhar.

Duas fontes de dados só podem ser comparadas depois que os mesmos conceitos versionados de medicamento e evento forem pareados. Relate lado a lado estimativas de efeito, intervalos, cobertura, dados ausentes e heterogeneidade. A interseção de conjuntos significa “observado nas duas análises configuradas”, nunca causalidade, replicação, validação ou confirmação. FAERS global e Brasil não constituem uma comparação Estados Unidos versus Brasil, a menos que o FAERS seja explicitamente filtrado para notificações dos Estados Unidos.
