# CI e releases no GitHub Actions

O projeto inclui dois workflows: [CI](../.github/workflows/ci.yml) e
[Release](../.github/workflows/release.yml). Eles usam Go conforme `go.mod`, Python
3.12 e runners hospedados pelo GitHub. Não precisam de contas dos agentes, API
keys, Docker ou Kubernetes: os cenários usam logs sintéticos e os binários reais.

## Fluxo de validação

A CI roda em pushes de branches, pull requests e execução manual (`workflow_dispatch`).
Também pode ser chamada pelo workflow de release (`workflow_call`).

| Job | O que verifica | Resultado |
|---|---|---|
| Test (ubuntu-latest) | Dependências, versões/manifests, go vet, gofmt, testes com race detector, cobertura e cenários dos dois agentes | `test-results-ubuntu-latest` |
| Test (macos-latest) | Mesmas verificações no macOS | `test-results-macos-latest` |
| Fuzz | Entradas aleatórias por 20 segundos em cada alvo: compressor e hook | Corpus anexado em caso de falha |
| Workflow lint | Sintaxe e expressões dos workflows com actionlint 1.7.7 | Falha se o workflow for inválido |
| Packages | Após os demais jobs: cross-compilation, conteúdo dos oito arquivos, checksums e execução dos dois binários Linux amd64 | `release-packages` |

Os testes e relatórios são retidos por 14 dias. O resumo de cada job de teste
mostra a tabela de economia de bytes; os artifacts incluem o relatório JSON,
saídas dos cenários, respostas dos hooks e `coverage.out`. A cobertura é informativa,
sem limiar arbitrário. Os benchmarks de latência são locais (`make benchmark`);
não existe um limite de tempo de performance em runners compartilhados.

Os pacotes cobrem Claude Code e Codex × Linux/macOS × amd64/arm64. A compilação
cruzada não executa os binários de todas as arquiteturas; o smoke test executa
apenas Linux amd64, enquanto a suíte Go também roda no macOS.

## Como ativar no repositório

1. Versione e envie o projeto, incluindo `.github/`, para seu repositório GitHub.
2. Em **Settings → Actions → General**, permita GitHub Actions e as actions
   oficiais `actions/*`. Se a organização restringe permissões de escrita, permita
   `contents: write` para o job de release. As permissões dos demais jobs são de leitura.
3. Em **Actions → CI → Run workflow**, execute a primeira validação. A execução
   manual fica disponível quando o workflow estiver na branch padrão.
4. Opcionalmente configure um ruleset da branch padrão exigindo os checks
   `Test (ubuntu-latest)`, `Test (macos-latest)`, `Fuzz`, `Workflow lint` e `Packages`.
   Selecione os nomes exibidos pelo GitHub após a primeira execução.

Não é necessário cadastrar PAT ou secrets: o job de release usa o `GITHUB_TOKEN`
automático. Hooks dos agentes não são instalados ou autorizados pelos workflows.
O pipeline valida o contrato local, não uma conversa autenticada com Claude/Codex.

## Criar uma release

A fonte da versão é [VERSION](../VERSION). Antes de uma release, atualize também
os campos `version` dos dois manifestos:

- `.claude-plugin/plugin.json`
- `integrations/codex/tokenslim/.codex-plugin/plugin.json`

O build injeta essa versão no binário via linker. São aceitas versões estáveis
`MAJOR.MINOR.PATCH`; prereleases ainda não fazem parte deste pipeline. A tag deve
ser exatamente `v` + o conteúdo de `VERSION`.

```sh
python3 scripts/validate-release.py
make test lint demo workflow-lint release-check
# Depois de commitar e enviar as alterações para o remoto:
version=$(cat VERSION)
git tag -a "v$version" -m "TokenSlim $version"
git push origin "v$version"
```

O push da tag inicia o workflow Release, que:

1. Confere tag, VERSION, manifestos e seleção dos adaptadores.
2. Executa a CI completa no mesmo commit da tag.
3. Baixa os pacotes produzidos por essa execução e confere SHA-256 novamente.
4. Cria uma **GitHub Release em rascunho**, com notas geradas, os oito pacotes e
   `SHA256SUMS`. Revise em **Releases** e publique quando estiver pronto.

Somente o job final tem `contents: write`. O comando `gh release create` exige a
existência da tag e não sobrescreve uma release existente. Se uma execução falhar
após criar o rascunho, confira os anexos; remova somente o rascunho incompleto antes
de repetir a execução, ou conclua o rascunho existente. Não mova tags já publicadas.

## Executar localmente

```sh
make test           # testes com race detector
make lint           # go vet e formatação
make demo           # cenários Claude e Codex
make workflow-lint  # actionlint fixado; primeiro uso baixa a ferramenta Go
make release-check # oito pacotes + validação de conteúdo e hashes
```

`make release` escreve em `dist/`; não publica nem faz upload. O script valida a
versão antes de compilar. `SHA256SUMS` inclui apenas os pacotes da versão atual;
arquivos de versões anteriores no diretório local não entram nos hashes.
Os arquivos contêm binário, manifesto, hook, skill de recuperação, licença,
versão, documentação e script de configuração do Codex.

Verificação de um download:

```sh
# No diretório com os oito arquivos e SHA256SUMS:
sha256sum --check SHA256SUMS  # Linux
shasum -a 256 -c SHA256SUMS  # macOS
```

## Manutenção e falhas comuns

- **Tag divergente:** atualize VERSION e ambos os manifestos no commit correto.
- **Formatação:** rode `gofmt -w cmd internal` e revise as alterações.
- **Erro de cenário:** consulte os artifacts `test-results-*`; saídas originais
  podem ser reproduzidas executando `make demo` localmente.
- **Erro de fuzzing:** baixe o corpus anexado, copie o caso para o diretório do
  pacote correspondente e rode `go test` para reproduzir.
- **Release sem permissão:** confira a política de Actions da organização e as
  permissões de `GITHUB_TOKEN` do job `publish`.
- **Actions ou Go indisponíveis:** confira o log de setup e a disponibilidade da
  versão declarada em `go.mod` antes de mudar o compilador.

Actions estão fixadas por SHA. O [Dependabot](../.github/dependabot.yml) propõe
atualizações semanais das actions e dependências Go. O actionlint está fixado no
Makefile e deve ser atualizado explicitamente. A análise ShellCheck do actionlint
está desabilitada para manter o mesmo comando local/CI sem dependência adicional.

Referências oficiais: [workflows reutilizáveis](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)
e [permissões de GITHUB_TOKEN](https://docs.github.com/en/actions/tutorials/authenticate-with-github_token).
