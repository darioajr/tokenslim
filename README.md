# TokenSlim

Compressor local de saídas de ferramentas para **Claude Code e Codex**. Reduz
repetições em builds, testes e logs, preserva diagnósticos e guarda o original em
cache. Não altera código-fonte, argumentos ou execução dos comandos.

## Construir e demonstrar

Requisitos: Go 1.26.2+; Python 3 para os cenários.

```sh
make build
make test
make demo
```

Binário: `bin/tokenslim`. A demonstração gera
`scenarios/results/report.md`, com comparações safe/smart, e testa os protocolos
dos dois agentes. Veja [cenários](scenarios/README.md).

```sh
bin/tokenslim benchmark --mode smart --command 'mvn test' build.log
bin/tokenslim optimize --mode smart --command 'mvn test' build.log
bin/tokenslim stats
bin/tokenslim cache inspect ts_HASH
```

Use a referência completa emitida na saída no lugar de `ts_HASH`. O benchmark é
um dry run sem escrita no cache ou métricas. `optimize` e hooks salvam o original.
Flags precedem o nome do arquivo; `-` lê stdin. Tokens são estimativas locais.

## Claude Code

Após `make build`, na raiz deste projeto:

```sh
claude --plugin-dir .
```

O plugin registra `PostToolUse` para Bash. O adaptador devolve
`updatedToolOutput` com stdout/stderr e metadados preservados. Read/Edit/Write,
saídas de imagem e execuções interrompidas passam sem alteração. A implementação
foi validada contra a documentação atual e o manifesto foi validado pelo Claude
Code local; o teste de protocolo não substitui uma sessão autenticada.

## Codex

O pacote próprio está em `integrations/codex/tokenslim/`, incluindo manifesto,
hook, skill e binário produzido por `make build`. Para experimentar sem configurar
um marketplace, gere um arquivo de hook que pode ser adicionado à configuração do
Codex:

```sh
python3 scripts/codex-hook-config.py > /tmp/tokenslim-codex-hooks.json
```

Adicione o bloco PostToolUse gerado ao `~/.codex/hooks.json` ou
`<seu-projeto>/.codex/hooks.json`, preservando os hooks existentes. Abra `/hooks`
no Codex e revise/confie no hook. O projeto também precisa ser confiável. O script
apenas imprime a configuração; não modifica preferências globais.

Codex usa `continue: false` e `stopReason` para substituir o resultado após a
execução. Não usa o contrato de Claude nem bloqueia a execução do comando.
Strings e objetos de execuções concluídas são suportados; sessões ainda em
execução e formatos desconhecidos ficam intactos. Há limitações específicas de
code mode e limites de saída do próprio host; consulte a
[compatibilidade](docs/architecture.md). A integração requer uma versão com os
hooks descritos na [documentação do Codex](https://learn.chatgpt.com/docs/hooks).

## Configuração

Precedência: flags CLI → `.tokenslim.yaml` do diretório de trabalho →
`~/.tokenslim/config.yaml` → defaults. `TOKENSLIM_HOME` muda o diretório de estado
para testes ou isolamento. A configuração do projeto não é procurada em ancestrais.

```yaml
version: 1
mode: safe # off, safe, smart
thresholds:
  minimum_bytes: 4096
  minimum_lines: 40
  minimum_expected_reduction_percent: 10
cache:
  enabled: true
  retention: 7d
  max_size_mb: 1024
metrics:
  enabled: true
```

Os dois limiares de tamanho precisam ser atingidos. Saídas sem ganho suficiente
permanecem intactas. Modo smart acrescenta redução de transferências Maven,
suítes PASS e logs com timestamps. Terraform continua conservador. Veja
[regras](docs/compression-rules.md) e [configuração completa](docs/config.example.yaml).

```sh
bin/tokenslim version
bin/tokenslim status
bin/tokenslim config show
bin/tokenslim config path
bin/tokenslim stats --session ID
bin/tokenslim cache inspect ts_HASH --metadata
bin/tokenslim cache prune
bin/tokenslim cache clear
```

Cache: zstd + SHA-256, diretórios 0700 e arquivos 0600. Os originais podem conter
segredos que já estavam nos logs; permanecem locais. Limpar ou expirar o cache
remove a possibilidade de recuperação. Se o cache falhar ou estiver desabilitado,
a saída original é mantida. Métricas são arquivos JSON atômicos locais; não há
telemetria, serviço externo ou chamadas a modelos. `TOKENSLIM_DEBUG=1` grava os
metadados da última execução em `~/.tokenslim/logs/tokenslim.log`.

## Desenvolvimento

```sh
make lint
make benchmark
make plugin-test
make workflow-lint # valida os workflows GitHub Actions
make release-check # gera e verifica os pacotes
make install # instala apenas o binário em ~/.local/bin
```

Releases incluem pacotes distintos para Claude/Codex, macOS/Linux e arm64/amd64,
com checksums. Unitários, goldens, testes de invariantes e fuzzing cobrem os
redutores e adapters. O corpus de demonstração é sintético; economia de tokens
faturados, latência p95 em produção e aceitação em sessões reais não são afirmadas.
A especificação original está em [.spec/tokenslim-SPEC.md](.spec/tokenslim-SPEC.md).

## GitHub Actions

A [CI](.github/workflows/ci.yml) executa testes, race detector, cobertura, fuzzing,
lint e cenários de eficiência em Linux/macOS. Também verifica os oito pacotes
Claude/Codex e publica os relatórios como artifacts da execução.

Tags `vMAJOR.MINOR.PATCH` acionam a [release](.github/workflows/release.yml): após
repetir as validações no commit da tag, o pipeline cria um rascunho de GitHub
Release com os pacotes e checksums. `VERSION` e os dois manifestos devem coincidir.

Veja [ativação, etapas e publicação](docs/ci-cd.md). Os workflows passam a executar
quando o projeto for enviado a um repositório GitHub com Actions habilitado.

Licença Apache-2.0.
