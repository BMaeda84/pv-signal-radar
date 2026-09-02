# Uso pretendido e responsabilidades dos usuários

## Uso pretendido

O PV Signal Radar oferece suporte ao ensino, à exploração transparente e ao estudo acadêmico reproduzível de notificações desproporcionais em conjuntos públicos de notificações espontâneas. Seus resultados são associações estatísticas de notificação (`SDRs`, signals of disproportionate reporting) que exigem revisão qualificada e evidências externas.

Usos adequados incluem:

- ensinar tabelas 2 × 2, PRR, ROR, comportamento de dados esparsos, viés de notificação e desenho de protocolo;
- gerar hipóteses documentadas para revisão da literatura ou de séries de casos;
- reproduzir uma análise publicada a partir de um manifesto de fonte e de uma configuração congelados;
- comparar métodos, thresholds, janelas temporais ou estratos predefinidos; e
- preparar tabelas e figuras para um manuscrito quando acompanhadas do protocolo completo e das limitações.

## Usos explicitamente excluídos

A aplicação não é orientação médica, diagnóstico, estimativa de risco individual, calculadora de exposição-incidência, suporte à decisão clínica, sistema de notificação de eventos adversos, sistema GxP validado ou qualificado nem mecanismo automatizado de decisão regulatória. Ela não estabelece que um medicamento causou um evento e não deve ser usada como única evidência para alterar tratamento, rotulagem, conclusões de benefício-risco ou ação regulatória.

O endpoint openFDA ao vivo é uma conveniência exploratória. Como sua fonte muda, ele não fornece um conjunto de pesquisa congelado, e seu resultado não deve ser citado como análise reproduzível.

## Uso por público

### Estudantes

Use o modo guiado com as células 2 × 2 exibidas. Recalcule pelo menos uma linha, explique o comparator e a correção de células zero e identifique um viés capaz de alterar a medida sem alterar o risco biológico. Não chame o cruzamento de um threshold de sinal de segurança confirmado.

### Pesquisadores

Registre um protocolo antes de inspecionar os resultados. Congele as revisões do dataset e do software, declare papel do medicamento, escopo de eventos, comparator, período, estratos, estratégia de multiplicidade e perfil de threshold e, então, arquive o bundle exportado com os materiais do estudo. Siga as [recomendações READUS-PV](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/) para o relato; o checklist melhora a qualidade do relato, mas não valida um estudo nem uma alegação causal.

### Profissionais de farmacovigilância

Trate um SDR como item de triagem. Revise casos, padrões temporais, duplicatas, notificação estimulada, indicações, medicamentos concomitantes, plausibilidade biológica, literatura, contexto de exposição e outras fontes de dados. Siga o sistema de qualidade organizacional aplicável e o [EMA GVP Module IX](https://www.ema.europa.eu/en/documents/scientific-guideline/guideline-good-pharmacovigilance-practices-gvp-module-ix-signal-management-rev-1_en.pdf); este software não substitui as etapas de validação, confirmação, análise, priorização ou avaliação.

## Limite mínimo de citação

Um resultado formal deve identificar o ID do dataset, a cobertura da fonte, os checksums da fonte e da saída, o ID/configuração da análise, o commit do software, o lockfile R, as definições dos métodos, as exclusões e os desvios conhecidos. Se qualquer item estiver ausente, descreva o resultado como exploratório e não reproduzível.
