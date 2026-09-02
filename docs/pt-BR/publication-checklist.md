# Checklist de publicação, citação e DOI

## Release de software

1. Reconcilie o commit implementado com `docs/validation-and-evidence.md`; não rotule uma release como “validada” enquanto uma linha crítica não tiver sido executada.
2. Exija CI da branch protegida, revisão científica independente de métodos/alegações/riscos residuais e aprovação registrada no ambiente GitHub `software-release`.
3. Crie uma tag imutável e anotada de versão semântica após a revisão das evidências. Configure rulesets para a default branch e tags de release, checks/reviews obrigatórios, releases imutáveis e reviewers obrigatórios no ambiente `software-release` antes de habilitar a publicação.
4. Dispare manualmente `.github/workflows/release.yml` com essa tag existente. Seu job sem privilégios verifica tag/commit, testa Go e a imagem R bloqueada, examina as duas imagens de runtime, constrói binários e emite checksums, SBOMs CycloneDX do Go, o lock R e um inventário de pacotes/licenças R. Somente o job separado, protegido pelo ambiente, recebe permissões de release/OIDC, atesta os bytes verificados e cria a release no GitHub. Antes do disparo, crie o ambiente protegido `software-release`, configure required reviewers e adicione o secret de escopo exclusivo do ambiente `SOFTWARE_RELEASE_GATE` com o valor contratual exato e não secreto `enabled-after-protection-v1`. Não defina esse nome como secret de repositório ou organização: o controle é o escopo, não a confidencialidade. O primeiro step de publicação falha de forma fechada quando o valor está ausente ou diferente. Esse gate no código-fonte não comprova nem substitui required reviewers, rulesets de branch/tag ou releases imutáveis. Trate um controle externo do repositório ausente ou um gate com falha/pulado como release bloqueada.
5. Se a imagem científica R for distribuída, adicione um gate separado de release OCI: publique-a por digest imutável, gere um SBOM OCI que cubra OS e pacotes R, preserve o relatório de vulnerabilidades e assine/ateste a proveniência da imagem. O workflow atual apenas constrói/examina a imagem R e publica seu Dockerfile, lock e inventário de pacotes; ele não publica nem atesta a própria imagem.
6. Vincule os bytes restaurados das dependências, não apenas nomes e versões. Registre valores SHA-256 dos archives exatos dos pacotes ou objetos do mirror usados pelo restore; o checksum atual de `renv.lock` protege o lock como um todo, mas seus registros de pacotes não contêm hashes de conteúdo por archive.
7. Arquive o código-fonte da release, `.zenodo.json` e `CITATION.cff`. Habilite a integração GitHub–Zenodo somente pela conta do proprietário do repositório e, então, reserve/publique um DOI para a release revisada.
8. Adicione o DOI final aos metadados da release em um commit posterior. Nunca apresente em uma citação pública um DOI de rascunho reservado como se estivesse publicado.

Habilitar rulesets de branch/tag, releases imutáveis, o ambiente protegido `software-release`, Zenodo ou um DOI são ações externas de publicação e não são realizadas por esta mudança no código-fonte. A auditoria de 2026-09-02 encontrou ausência de rulesets, default branch desprotegida, ausência de ambientes GitHub e releases imutáveis desabilitadas. O código-fonte não comprova que essas configurações permaneçam ativas depois de configuradas; reverifique-as imediatamente antes de cada release.

## Release de dataset e análise

O DOI do software não identifica dados nem parâmetros de análise. Publique um registro imutável separado que contenha ou vincule:

- IDs do dataset e da análise;
- URLs oficiais das fontes, cobertura, timestamps de obtenção e valores SHA-256;
- `manifest.json`, `metadata/environment.json`, `source_manifest.csv`, `qa_summary.csv` e `checksums.sha256` de saída;
- commit exato do código-fonte e `renv.lock` revisado;
- configuração da análise e definições dos métodos em formato legível por máquina;
- digest canônico do resultado, contagem/ordem de linhas, definição da família de hipóteses e evidência de reprodução atestada de forma independente;
- resultado CSV/Parquet e relatório de métodos/limitações legível por humanos;
- evidências de benchmark numérico/reference set para perfis de threshold habilitados;
- desvios, aprovador, disposição de licença/redistribuição e rota de contato/correção.

Se os termos da fonte proibirem redistribuição, publique instruções de reconstrução, hashes da fonte, código, metadados e resultados derivados permitidos em vez dos bytes restritos.

## Mínimo para o manuscrito

Identifique versão/cobertura da base, tratamento de versão do caso, elegibilidade das notificações, mapeamento e papel do medicamento, escopo de PT/eventos, comparator, estratos, medidas, intervalos de confiança, correção de dados esparsos, procedimento de multiplicidade, threshold, versões do software/pacotes e dados ausentes. Relate estimativas de efeito e incerteza, não apenas rótulos de threshold. Cite o DOI do software e o DOI do dataset/análise separadamente e siga o [READUS-PV](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/).

## Correção ou retirada

Nunca substitua um artefato arquivado no mesmo local. Publique uma correção vinculada ou tombstone com motivo, impacto, identificador substituto, análises afetadas e data. Preserve o checksum original e o registro de evidências para que leitores consigam identificar o que mudou.
