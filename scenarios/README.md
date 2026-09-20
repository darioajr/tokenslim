# Demonstração de eficiência

Execute, na raiz do repositório:

```sh
make demo
```

Requisitos: Go e Python 3. Não precisa de conta, API, Kubernetes, Docker, Node ou Maven.
O script gera logs **sintéticos**, executa o binário real e testa os adaptadores
Claude Code e Codex. Não executa as ferramentas cujos logs são simulados.

- `generated/`: logs de entrada, reproduzíveis e sem dados privados.
- `results/`: saídas otimizadas, respostas dos hooks, relatório Markdown e JSON.
- `run.py`: geração, medição e verificações automáticas.

Casos: Maven com falhas, testes Node com assertions, Kubernetes com timestamps,
Docker com identidade de serviço, repetições genéricas, Unicode/ANSI, saída pequena
e saída sem repetição. Cada caso roda em `safe` e `smart`.

Verificações: economia de bytes incluindo o marcador de recuperação, estimativa
consistente de tokens, preservação dos diagnósticos selecionados, código de saída,
recuperação exata pelo cache, determinismo, Read sem alterações e fonte intacta.
O cache da demonstração usa um diretório temporário isolado, removido ao terminar.
Os relatórios permanecem disponíveis em `results/`.

Para comparar um log próprio:

```sh
bin/tokenslim benchmark --mode safe --command 'mvn test' seu-build.log
bin/tokenslim benchmark --mode smart --command 'mvn test' seu-build.log
```

A estimativa usa caracteres Unicode/4. Não é contagem de tokens do provedor nem
promessa de redução da fatura. O corpus sintético demonstra o mecanismo; valide
logs reais antes de inferir a economia do seu projeto. Estes testes exercitam o
protocolo local, não uma conversa autenticada com os agentes.
