# Registro de seleção de software científico e repositórios

Última revisão: 2026-09-02. Este é um registro de decisão sobre dependências, não um endosso nem um certificado de validação. Versão, licença, manutenção, concordância numérica, modelo de dados e instalação reproduzível são gates separados; um pacote que implementa um método nomeado não é automaticamente intercambiável com outra implementação.

## Adotados no ambiente batch R isolado

| Componente | Versão congelada | Papel | Critério de seleção | Limite |
|---|---:|---|---|---|
| [`faers`](https://bioconductor.org/packages/release/bioc/html/faers.html) | 1.8.0 | ETL orientado ao FAERS e referência independente de métodos | Release no Bioconductor 3.23, repositório de código documentado, licença MIT, contrato de release R 4.6 | O projeto mantém seu próprio ETL relacional explícito; a presença do pacote não valida um trimestre nem substitui a reconciliação no nível da fonte |
| [`pvda`](https://CRAN.R-project.org/package=pvda) | 0.0.4 | Comparação independente de PRR, ROR e IC | Pacote publicado no CRAN com manual/testes de referência e licença GPL >= 3 declarada | Usado somente no processo R separado; adapters numéricos e tolerâncias declaradas são obrigatórios antes de citar concordância |
| [`openEBGM`](https://CRAN.R-project.org/package=openEBGM) | 0.9.1 | GPS/EBGM, quantis e trabalho batch estratificado | Implementação publicada no CRAN com vignettes de métodos e licenciamento GPL-2/GPL-3 | Estimação de hiperparâmetros, estratos, convergência e EB05 devem ser congelados no protocolo; nunca é aproximado em Go |

O ambiente transitivo é registrado em `research/renv.lock`, não resolvido a partir de “latest” durante a análise. Pacotes da família GPL não são vinculados ao binário Go MIT; as obrigações de distribuição da imagem batch ainda exigem revisão de licença no momento da release.

## Benchmarks exploratórios, não dependências centrais

| Candidato | Evidência considerada | Por que não é central |
|---|---|---|
| [`vigipy`](https://github.com/Shakesbeery/vigipy) | Implementações Python de BCPNN, GPS, PRR, ROR, Fisher, LASSO e análise longitudinal; GPLv3 | Útil como benchmark entre linguagens, mas o próprio repositório lista como pendentes a documentação dos métodos e um dataset de demonstração. Não é usado para definir resultados canônicos. |
| [`faers` no PyPI](https://pypi.org/project/faers/) | Versão 0.1, uma release de código-fonte de 1,4 kB enviada em 2015 | Colisão de nome com o projeto Bioconductor ativo; idade, escopo e conteúdo da release não atendem aos requisitos do pipeline. |
| [`hypokrates`](https://pypi.org/project/hypokrates/) | Pacote amplo de geração de hipóteses em múltiplas fontes, status de desenvolvimento “Alpha” no PyPI, somente AGPL-3.0 | Limite de confiança e produto diferente: geração de hipóteses entre bases/LLM, não o contrato de análise relacional FAERS congelada. Pode informar apenas pesquisa de interoperabilidade depois de revisão de linhagem de dados, numérica, de licença e de segurança. |
| [`VigiLens`](https://github.com/firassa-ai/VigiLens) | Aplicação de sinais de segurança temporais consciente de trimestres | Comparação de produto/repositório para UX temporal; não é autoridade numérica nem dependência de fonte. |
| [`PRISM-Pharmacovigilance`](https://github.com/Jehathsyed/PRISM-Pharmacovigilance) | Aplicação browser/openFDA de desproporcionalidade | Comparador útil de UI, mas um workflow openFDA ao vivo não substitui entradas trimestrais congeladas e deduplicadas por versão de caso. |

## Teste de admissão para outro pacote ou repositório

Um candidato passa de benchmark para dependência somente depois que todos os itens a seguir forem registrados:

1. versão/hash de origem imutável, runtime suportado, licença, histórico de mantenedor/releases e instalação limpa reproduzível;
2. unidade exata de entrada e semântica do comparator, tratamento de duplicatas/versão de caso, política de células esparsas, definição de intervalo, comportamento de estratos e modos de falha;
3. fixtures golden 2 x 2 e comparação numérica em escala real contra pelo menos uma implementação independente, com tolerâncias absolutas/relativas declaradas;
4. comportamento em reference sets positivos/negativos quando o método influenciar um threshold, incluindo sensibilidade, PPV, comportamento de falsos positivos e time-to-detection;
5. limites de memória/CPU, comportamento com entrada malformada, revisão de SBOM/vulnerabilidades e saída/proveniência determinísticas; e
6. motivo documentado pelo qual a capacidade não pode ser implementada com maior transparência no limite atual.

Até que esse dossiê exista, resultados exploratórios devem ser rotulados por implementação e versão e não podem substituir silenciosamente um método configurado.
