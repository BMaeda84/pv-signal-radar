# Vieses, limitações e modos de falha observáveis

## Limitações interpretativas

- **Sem atribuição causal:** medicamentos e eventos que ocorrem conjuntamente em uma notificação não são vinculados individualmente. PRR/ROR elevado pode surgir sem efeito causal.
- **Sem incidência nem risco individual:** notificações espontâneas não possuem um denominador confiável de população exposta.
- **Viés de notificação:** notoriedade, notificação estimulada, cobertura da mídia, litígio, geografia, composição dos notificadores e tempo podem alterar o numerador e o comparator.
- **Comportamento de duplicatas e follow-ups:** o pipeline retém a versão mais recente do caso, mas IDs de caso distintos ainda podem descrever episódios clínicos relacionados ou duplicados.
- **Confusão e indicação:** doença, comedicação, direcionamento de tratamento e indicação podem criar ou suprimir uma associação.
- **Drift terminológico:** o texto PT trimestral não estabelece a release do MedDRA; strings de medicamento da fonte não são um vocabulário canônico de ingredientes.
- **Viés de competição:** alterar o conjunto de medicamentos ou eventos muda toda contagem do comparator.
- **Multiplicidade:** examinar muitos pares produz extremos ao acaso. FDR controla uma família declarada de testes, não causalidade.
- **Dados esparsos:** intervalos assintóticos e correções de continuidade podem dominar tabelas pequenas; sempre inspecione as células e os resultados de Fisher.
- **Instabilidade temporal:** resultados openFDA ao vivo e revisões posteriores do FAERS podem mudar. Um snapshot congelado e verificado quanto à integridade é necessário, mas não suficiente para reprodução; configuração, revisão do código, runtime e premissas de vocabulário também devem ser fixados.

A FDA descreve os dados públicos do FAERS como uma entrada de um processo pós-comercialização mais amplo, com notificações duplicadas e definições de caso variáveis; a [documentação de eventos de medicamentos da openFDA](https://open.fda.gov/apis/drug/event/) alerta explicitamente contra inferências de causalidade ou incidência.

## Condições de falha fechada

| Condição | Por que é insegura | Comportamento observável |
|---|---|---|
| SHA-256 da fonte ausente ou divergente | A identidade da entrada não está comprovada | O build termina com código diferente de zero e nomeia o arquivo |
| `CASEID`, `PRIMARYID` ou `CASEVERSION` inválido | A deduplicação se torna ambígua | O build termina com código diferente de zero e informa a contagem de linhas inválidas |
| Tabela DEMO/DRUG/REAC ausente ou duplicada | Um trimestre está incompleto ou ambíguo | O build termina com código diferente de zero e nomeia tabela/trimestre |
| Caminho de travessia em ZIP | A extração poderia sair do armazenamento temporário | O archive é rejeitado antes da extração |
| Nenhum par medicamento–evento elegível | As medidas não têm universo definido | O build termina com código diferente de zero |
| Célula 2 × 2 reconstruída negativa | As marginais são inconsistentes | A agregação termina com código diferente de zero |
| Diretório de saída não vazio | Artefatos anteriores e novos poderiam ser misturados | O build se recusa a gravar |
| Dependência R ausente ou versão de método incorreta | O runtime não é o ambiente declarado | Bootstrap/build termina com código diferente de zero |
| Revisão da aplicação suja, ausente ou conflitante | O ID da análise poderia identificar código que não foi executado | A inicialização em modo de pesquisa termina com código diferente de zero |
| Teste exato ou família de resultados excede o limite de trabalho online | Uma requisição pública poderia monopolizar CPU ou memória | A requisição retorna erro de método batch/protocolo não suportado; nenhuma linha é truncada |
| Limite de concorrência ou pacing de análise/exportação atingido | Trabalho concorrente poderia esgotar CPU ou memória | A API retorna `429` com `Retry-After` |

## Condições que não causam falha automaticamente

Dados demográficos opcionais ausentes, texto de medicamento não mapeado e PTs não classificados são preservados porque excluí-los ou adivinhá-los silenciosamente ocultaria incerteza. Suas contagens devem ser revisadas no QA e divulgadas em cada análise. Um nível cientificamente inaceitável de dados ausentes é específico do protocolo e deve ser definido antes da análise.
