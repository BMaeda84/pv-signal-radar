# Privacidade, segurança e governança

## Dados e feedback

A aplicação pública não deve aceitar uploads de casos privados na v1. O endpoint de feedback anterior coletava e-mail, endereço IP, user agent e texto livre sem um conjunto completo de controles de retenção e privacidade; por isso, o feedback público foi substituído por GitHub Issues/Discussions até que existam controlador, finalidade, aviso, consentimento/base legal, período de retenção, processo de exclusão, controle de acesso, armazenamento durável, tratamento de abuso e processo de incidentes documentados.

Não cole em issues informações de pacientes, narrativas de notificações, credenciais, chaves de API, análises não publicadas nem dados confidenciais de instituições. O GitHub é um serviço separado, com termos e comportamento de retenção próprios.

## Governança de artefatos

- As entradas são arquivos públicos oficiais e congelados fornecidos pelo pesquisador; o serviço nunca os baixa implicitamente.
- Entradas brutas, datasets gerados e registros de fontes são ignorados pelo Git neste repositório.
- A publicação usa object storage imutável com acesso controlado e API compatível com S3, ou um repositório arquivístico. Object lock/versionamento e política de retenção devem ser configurados e testados pelo operador.
- Checksums comprovam identidade de bytes, não autenticidade, validade científica nem ausência de conteúdo malicioso.
- Um registro formal de fonte FAERS deve apontar para a URL oficial de distribuição da FDA revisada e reconciliar o trimestre declarado com nomes de arquivos e cobertura. Calcular um hash após o download congela os bytes observados, mas não é uma âncora externa de confiança; preserve a página/metadados da resposta da FDA e o registro de revisão independente.
- A retirada de um dataset nunca substitui silenciosamente um objeto. Publique um tombstone/correção vinculado ao identificador imutável.
- Segredos pertencem a secret stores de deployment e nunca devem entrar em manifestos, logs, bundles exportados nem no histórico do Git.

## Limite de disponibilidade da API pública

A aplicação limita novas análises ao vivo/de pesquisa a dois workers concorrentes, espaça o início de novas pesquisas, aplica deadline de requisição de 20 segundos, falha um resultado online acima de 50.000 linhas de eventos e limita uma exportação a 32 MiB por arquivo, 64 MiB no total e 32 arquivos. O teste exato é opt-in e rejeita um cálculo online que exija mais de 100.000 termos enumerados do suporte. Esses controles fazem a requisição inteira falhar; eles nunca truncam a família de eventos testada nem substituem silenciosamente outro método.

`RESEARCH_ALLOW_ONLINE_MATERIALIZATION` é false por padrão. Nesse modo somente de resolução, um POST determinístico pode recuperar um registro existente, mas um cache miss não pode criar estado permanente. O gate de início é local ao processo; habilitar materialização em um deployment público ou com múltiplas réplicas ainda exige identidade/quotas de taxa no gateway, alertas e quotas de capacidade do filesystem, limites de tamanho de requisição/resposta e operações de retenção/retirada para análises imutáveis. Sem esses controles externos, callers distribuídos podem contornar o pacing por processo ou esgotar o volume de resultados ao longo do tempo.

A identidade do software vem dos metadados Go VCS incorporados ao binário ou da flag de linker da release. `PV_RADAR_APPLICATION_COMMIT` só é aceito quando o binário não tem metadados de revisão, e um build sujo ou divergente é rejeitado; a variável é uma atestação do operador, não uma substituição.

O container científico é um limite batch, não um serviço de rede. Após restaurar as dependências, execute as transformações com rede desabilitada, filesystem raiz read-only, sem capabilities Linux, `no-new-privileges`, quotas explícitas de CPU/memória/tmp/saída, mounts read-only das fontes e um diretório de saída gravável novo. Limites de membros/contagem/tamanho expandido de ZIP protegem o parser, mas quotas de infraestrutura continuam necessárias para Arrow/Parquet e workloads de trimestres reais.

## Limite de licenciamento

O código-fonte da aplicação é distribuído sob MIT. Dados FDA/ANVISA, terminologia MedDRA/WHODrug, artigos científicos, containers e pacotes R têm termos independentes. Em particular, `pvda` e `openEBGM` usam licenças da família GPL. O ambiente batch R é deliberadamente separado do binário Go, e o serviço Go consome artefatos de dados gerados em vez de vincular código de pacotes R.

Essa separação reduz acoplamento, mas não é uma determinação jurídica. Antes da distribuição, registre a versão/licença de cada dependência, os termos das fontes de dados, direitos de vocabulário, se os dados serão redistribuídos ou apenas reconstruídos e o público/jurisdições pretendidos. Não empacote material proprietário MedDRA ou WHODrug sem os direitos necessários.

## Controle de mudanças

Qualquer mudança em cobertura da fonte, deduplicação, elegibilidade, mapeamento, comparator, escopo de eventos, método, correção, threshold, package lock ou schema cria uma nova versão de dataset/análise. A publicação em produção exige um gate humano após a reconciliação das evidências. Rollback significa servir o artefato imutável anterior e preservar o artefato retirado e o motivo; a recuperação permanece não comprovada até ser ensaiada no store selecionado.
