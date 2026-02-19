# zabbix-plugin-segi9

Loadable plugin para o **Zabbix Agent 2** que atua como um proxy HTTP/HTTPS.  
O agente executa a requisição **localmente** (no host onde está instalado) e devolve o corpo completo da resposta para o servidor Zabbix.

---

## Chave da métrica

```
segi9.http[<url>, <auth_type>, <user_or_token>, <password>]
```

| Parâmetro       | Obr. | Descrição                                                   |
|-----------------|------|-------------------------------------------------------------|
| `url`           | ✓    | URL alvo, ex: `https://api.exemplo.com/status`              |
| `auth_type`     |      | `none` (padrão) · `basic` · `bearer`                        |
| `user_or_token` |      | Usuário (`basic`) ou token (`bearer`)                       |
| `password`      |      | Senha (`basic` apenas; ignorado em `bearer`)                |

### Exemplos de item keys no Zabbix

```
# Sem autenticação
segi9.http[https://api.exemplo.com/status]

# Basic Auth
segi9.http[https://api.interna.com/metrics,basic,admin,s3cr3t]

# Bearer Token
segi9.http[https://api.externa.com/data,bearer,eyJhbGciOiJSUzI1NiJ9...]
```

---

## Configuração (`segi9.conf`)

```ini
# Caminho para o binário do plugin (OBRIGATÓRIO)
Plugins.Segi9.System.Path=/usr/local/lib/zabbix/plugins/zabbix-plugin-segi9

# Timeout da requisição HTTP em segundos (1–30, padrão: 10)
# Plugins.Segi9.Timeout=10

# Ignorar erros de certificado TLS/SSL (padrão: false)
# Plugins.Segi9.SkipVerify=false
```

---

## Build e instalação

### Pré-requisitos

- Go 1.21+
- Acesso a `git.zabbix.com` (para baixar o SDK do Zabbix)

### 1 – Obter o SDK

```bash
go get golang.zabbix.com/sdk@<COMMIT_HASH>
go mod tidy
```

> Encontre o hash mais recente da branch `release/7.4` em:  
> https://git.zabbix.com/projects/AP/repos/plugin-support/commits?at=refs%2Fheads%2Frelease%2F7.4

Ou use o atalho do Makefile:

```bash
make setup
```

### 2 – Compilar

```bash
make build
# ou diretamente:
go build -o zabbix-plugin-segi9 .
```

### 3 – Instalar

```bash
sudo make install
```

Isso copia o binário para `/usr/local/lib/zabbix/plugins/` e o `segi9.conf` para `/etc/zabbix/zabbix_agent2.d/`.

Edite o `segi9.conf` conforme necessário e reinicie o agente:

```bash
sudo systemctl restart zabbix-agent2
```

---

## Modo manual (teste sem o agente)

O plugin pode ser executado diretamente no terminal para depuração rápida:

```bash
# Sem autenticação
./zabbix-plugin-segi9 -manual "https://api.ipify.org"

# Basic Auth
./zabbix-plugin-segi9 -manual "http://httpbin.org/basic-auth/admin/secret" \
                      -auth basic -user admin -pass secret

# Bearer Token
./zabbix-plugin-segi9 -manual "https://httpbin.org/bearer" \
                      -auth bearer -user "meu-token"
```

Resultado impresso direto no `stdout`. Erros vão para `stderr`.

---

## Estrutura do projeto

```
.
├── main.go       ← entry point: modo plugin (Zabbix) e modo manual (-manual)
├── plugin.go     ← lógica HTTP, interfaces Exporter / Runner / Configurator
├── go.mod
├── segi9.conf    ← template de configuração para o agente
├── Makefile
└── README.md
```

---

## Pré-processamento no Zabbix (opcional)

Como o plugin retorna o **corpo bruto** da resposta, você pode usar as regras de pré-processamento do Zabbix para extrair campos específicos:

| Tipo           | Expressão de exemplo                         |
|----------------|---------------------------------------------|
| JSONPath        | `$.status`                                  |
| Regex          | `uptime: (\d+)`                             |
| JavaScript     | `return JSON.parse(value).metrics.cpu_pct;` |

---

## Segurança

- `SkipVerify=false` (padrão): certificados TLS **são** verificados.  
- Use `SkipVerify=true` apenas em ambientes controlados (ex: monitoramento interno com certificados auto-assinados).
- Credenciais passadas nos parâmetros da key ficam visíveis no banco de dados do Zabbix. Para ambientes sensíveis, considere usar macros secretas do Zabbix.
